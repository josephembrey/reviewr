// Package review owns exact, private file-review coverage.
//
// A receipt records one explicit comparison edge. Coverage composes only when
// adjacent endpoint identities are exactly equal; filenames and display rows
// are never proof.
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

const (
	// MaxRetainedBytes is the largest text snapshot retained for an incremental view.
	MaxRetainedBytes = 2_000_000
	// MaxReceipts bounds a long-running private ledger.
	MaxReceipts = 4_096
	// MaxRetainedTotal bounds all retained text while preserving cheap exact identities.
	MaxRetainedTotal = 16_000_000
)

// FileKind identifies the object at an endpoint. Mode remains separate so an
// executable-bit change remains reviewable.
type FileKind string

const (
	Absent    FileKind = "absent"
	Regular   FileKind = "regular"
	Symlink   FileKind = "symlink"
	Submodule FileKind = "submodule"
)

// SourceKind identifies where endpoint content is read from.
type SourceKind string

const (
	GitTreeSource  SourceKind = "git-tree"
	WorktreeSource SourceKind = "worktree"
)

// EndpointSource is immutable provenance, not a substitute for endpoint equality.
type EndpointSource struct {
	Kind  SourceKind `json:"kind"`
	Value string     `json:"value,omitempty"`
}

// ComparisonIdentity distinguishes active comparison bases and scopes.
// Scope is stable across basis movement; Basis pins one concrete comparison.
type ComparisonIdentity struct {
	Scope string `json:"scope"`
	Basis string `json:"basis"`
}

// Endpoint is one exact state of one repository-relative path. Empty ContentID
// means exact identity was unavailable and can never prove coverage.
type Endpoint struct {
	Path      string   `json:"path"`
	Kind      FileKind `json:"kind"`
	Mode      uint32   `json:"mode"`
	ContentID string   `json:"content_id"`
}

// AbsentEndpoint returns the exact missing state of path.
func AbsentEndpoint(path string) Endpoint {
	return Endpoint{Path: path, Kind: Absent, ContentID: "absent"}
}

// Exact reports whether the endpoint can participate in coverage proof.
func (e Endpoint) Exact() bool { return e.ContentID != "" }

// ContentIdentity returns a stable SHA-256 identity for exact raw content.
func ContentIdentity(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FileAction is part of exact review identity.
type FileAction string

const (
	Added    FileAction = "added"
	Modified FileAction = "modified"
	Deleted  FileAction = "deleted"
	Renamed  FileAction = "renamed"
	Copied   FileAction = "copied"
)

// FileComparison is the exact changed-file edge supplied by a comparison provider.
type FileComparison struct {
	Identity    ComparisonIdentity `json:"identity"`
	OldSource   EndpointSource     `json:"old_source"`
	NewSource   EndpointSource     `json:"new_source"`
	Action      FileAction         `json:"action"`
	Old         Endpoint           `json:"old"`
	New         Endpoint           `json:"new"`
	BasisReason string             `json:"basis_reason,omitempty"`
}

// Exact reports whether both endpoints can be reviewed.
func (c FileComparison) Exact() bool { return c.Old.Exact() && c.New.Exact() }

// Bounds are the exact endpoints represented by a reader.
type Bounds struct {
	Old Endpoint `json:"old"`
	New Endpoint `json:"new"`
}

// Receipt is one explicit review action. Retained text belongs to New. A nil
// pointer means binary, oversized, or unavailable content was not retained.
type Receipt struct {
	Comparison ComparisonIdentity `json:"comparison"`
	OldSource  EndpointSource     `json:"old_source"`
	NewSource  EndpointSource     `json:"new_source"`
	Action     FileAction         `json:"action"`
	Old        Endpoint           `json:"old"`
	New        Endpoint           `json:"new"`
	Retained   *string            `json:"retained,omitempty"`
	Sequence   uint64             `json:"sequence"`
}

// State is one of the five honest derived review states.
type State uint8

const (
	Unreviewed State = iota
	Reviewed
	Updated
	Partial
	BasisChanged
)

// Badge returns the fixed-width ASCII presentation.
func (s State) Badge() string {
	switch s {
	case Reviewed:
		return "[x]"
	case Updated:
		return "[+]"
	case Partial:
		return "[~]"
	case BasisChanged:
		return "[!]"
	default:
		return "[ ]"
	}
}

// Label returns a concise user-facing description.
func (s State) Label() string {
	switch s {
	case Reviewed:
		return "reviewed"
	case Updated:
		return "updated"
	case Partial:
		return "partial review"
	case BasisChanged:
		return "review basis changed"
	default:
		return "unreviewed"
	}
}

// GapPriority orders X navigation. Reviewed has no review gap.
func (s State) GapPriority() (int, bool) {
	switch s {
	case BasisChanged:
		return 0, true
	case Updated:
		return 1, true
	case Partial:
		return 2, true
	case Unreviewed:
		return 3, true
	default:
		return 0, false
	}
}

// Assessment is derived state plus the reviewed frontier used by an incremental reader.
type Assessment struct {
	State    State
	Frontier *Endpoint
	Retained *string
	Reason   string
}

// Ledger is a private per-worktree set of exact explicit receipts. Comments do
// not belong to this type or its serialized form.
type Ledger struct {
	ReceiptData  []Receipt `json:"receipts"`
	NextSequence uint64    `json:"next_sequence"`
}

// Receipts returns a defensive copy of the ledger receipts.
func (l Ledger) Receipts() []Receipt {
	return cloneReceipts(l.ReceiptData)
}

// Clone returns an independent ledger copy.
func (l Ledger) Clone() Ledger {
	l.ReceiptData = cloneReceipts(l.ReceiptData)
	return l
}

// Assess derives coverage without mutating the ledger.
func (l Ledger) Assess(comparison FileComparison) Assessment {
	related := make([]Receipt, 0)
	for _, receipt := range l.ReceiptData {
		if receiptRelated(receipt, comparison) {
			related = append(related, receipt)
		}
	}
	if !comparison.Exact() {
		if len(related) == 0 {
			return Assessment{State: Unreviewed}
		}
		return Assessment{State: BasisChanged, Reason: "exact file identity unavailable"}
	}
	for _, receipt := range related {
		if receipt.Action == comparison.Action && receipt.Old == comparison.Old && receipt.New == comparison.New {
			return Assessment{State: Reviewed}
		}
	}
	if comparison.BasisReason != "" && len(related) > 0 {
		return Assessment{State: BasisChanged, Reason: comparison.BasisReason}
	}

	reachable := l.reachableFrom(comparison.Old, comparison)
	if _, ok := reachable[comparison.New]; ok {
		return Assessment{State: Reviewed}
	}

	var frontier *Receipt
	for index := range l.ReceiptData {
		receipt := &l.ReceiptData[index]
		if _, ok := reachable[receipt.New]; !ok || !canContinue(receipt.New, comparison) {
			continue
		}
		if frontier == nil || receipt.Sequence > frontier.Sequence {
			frontier = receipt
		}
	}
	if frontier != nil {
		endpoint := frontier.New
		if frontier.Retained != nil {
			return Assessment{
				State: Updated, Frontier: &endpoint, Retained: cloneString(frontier.Retained),
			}
		}
		if frontier.New.Kind == Absent {
			empty := ""
			return Assessment{State: Updated, Frontier: &endpoint, Retained: &empty}
		}
		return Assessment{State: BasisChanged, Reason: "reviewed snapshot unavailable for an incremental diff"}
	}

	if len(related) == 0 {
		return Assessment{State: Unreviewed}
	}
	for _, receipt := range related {
		if receipt.Comparison.Scope == comparison.Identity.Scope {
			return Assessment{State: BasisChanged, Reason: "comparison basis changed"}
		}
	}
	return Assessment{State: Partial, Reason: "an older comparison gap remains"}
}

// AssessAll derives a comparison snapshot with one path index. Exact endpoint
// composition is unchanged; the index only excludes receipt components that
// cannot connect to a comparison, keeping refresh cost bounded for many files.
func (l Ledger) AssessAll(comparisons map[string]FileComparison) map[string]Assessment {
	byPath := make(map[string][]int)
	for index, receipt := range l.ReceiptData {
		byPath[receipt.Old.Path] = append(byPath[receipt.Old.Path], index)
		if receipt.New.Path != receipt.Old.Path {
			byPath[receipt.New.Path] = append(byPath[receipt.New.Path], index)
		}
	}
	result := make(map[string]Assessment, len(comparisons))
	for path, comparison := range comparisons {
		paths := []string{comparison.Old.Path}
		if comparison.New.Path != comparison.Old.Path {
			paths = append(paths, comparison.New.Path)
		}
		seenPaths := make(map[string]struct{}, len(paths))
		seenReceipts := make(map[int]struct{})
		receipts := make([]Receipt, 0)
		for len(paths) > 0 {
			current := paths[0]
			paths = paths[1:]
			if _, seen := seenPaths[current]; seen {
				continue
			}
			seenPaths[current] = struct{}{}
			for _, index := range byPath[current] {
				if _, seen := seenReceipts[index]; seen {
					continue
				}
				seenReceipts[index] = struct{}{}
				receipt := l.ReceiptData[index]
				receipts = append(receipts, receipt)
				paths = append(paths, receipt.Old.Path, receipt.New.Path)
			}
		}
		subset := Ledger{ReceiptData: receipts, NextSequence: l.NextSequence}
		result[path] = subset.Assess(comparison)
	}
	return result
}

// Mark adds exactly the represented bounds. This is the only coverage-creating operation.
func (l *Ledger) Mark(comparison FileComparison, bounds Bounds, retained *string) bool {
	if !bounds.Old.Exact() || !bounds.New.Exact() {
		return false
	}
	action := actionForBounds(comparison, bounds)
	oldSource := comparison.OldSource
	if bounds.Old != comparison.Old {
		var sourceSequence uint64
		for _, receipt := range l.ReceiptData {
			if receipt.New == bounds.Old && receipt.Sequence >= sourceSequence {
				oldSource = receipt.NewSource
				sourceSequence = receipt.Sequence
			}
		}
	}
	retained = boundedRetained(retained)
	l.NextSequence++
	if l.NextSequence == 0 {
		l.NextSequence = 1
	}
	for index := range l.ReceiptData {
		receipt := &l.ReceiptData[index]
		if receipt.Action == action && receipt.Old == bounds.Old && receipt.New == bounds.New {
			receipt.Comparison = comparison.Identity
			receipt.OldSource = oldSource
			receipt.NewSource = comparison.NewSource
			receipt.Retained = cloneString(retained)
			receipt.Sequence = l.NextSequence
			l.Compact()
			return true
		}
	}
	l.ReceiptData = append(l.ReceiptData, Receipt{
		Comparison: comparison.Identity,
		OldSource:  oldSource,
		NewSource:  comparison.NewSource,
		Action:     action,
		Old:        bounds.Old,
		New:        bounds.New,
		Retained:   cloneString(retained),
		Sequence:   l.NextSequence,
	})
	l.Compact()
	return true
}

// Clear removes the newest checkpoint proving the exact active current result.
func (l *Ledger) Clear(comparison FileComparison) bool {
	remove := -1
	var sequence uint64
	for index, receipt := range l.ReceiptData {
		if receipt.Action == comparison.Action && receipt.Old == comparison.Old && receipt.New == comparison.New && receipt.Sequence >= sequence {
			remove, sequence = index, receipt.Sequence
		}
	}
	if remove < 0 {
		reachable := l.reachableFrom(comparison.Old, comparison)
		for index, receipt := range l.ReceiptData {
			_, oldReached := reachable[receipt.Old]
			if receipt.New == comparison.New && oldReached && actionComposes(receipt, comparison) && receipt.Sequence >= sequence {
				remove, sequence = index, receipt.Sequence
			}
		}
	}
	if remove < 0 {
		return false
	}
	l.ReceiptData = append(l.ReceiptData[:remove], l.ReceiptData[remove+1:]...)
	return true
}

// Compact deduplicates exact edges and bounds history and retained payloads.
func (l *Ledger) Compact() {
	sort.SliceStable(l.ReceiptData, func(i, j int) bool {
		return l.ReceiptData[i].Sequence > l.ReceiptData[j].Sequence
	})
	seen := make(map[edgeKey]struct{}, len(l.ReceiptData))
	unique := l.ReceiptData[:0]
	for _, receipt := range l.ReceiptData {
		key := edgeKey{Action: receipt.Action, Old: receipt.Old, New: receipt.New}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, receipt)
	}
	l.ReceiptData = unique
	sort.SliceStable(l.ReceiptData, func(i, j int) bool {
		return l.ReceiptData[i].Sequence < l.ReceiptData[j].Sequence
	})
	if len(l.ReceiptData) > MaxReceipts {
		drop := len(l.ReceiptData) - MaxReceipts
		l.ReceiptData = append([]Receipt(nil), l.ReceiptData[drop:]...)
	}
	total := 0
	for _, receipt := range l.ReceiptData {
		if receipt.Retained != nil {
			total += len(*receipt.Retained)
		}
	}
	for index := range l.ReceiptData {
		if total <= MaxRetainedTotal {
			break
		}
		if l.ReceiptData[index].Retained != nil {
			total -= len(*l.ReceiptData[index].Retained)
			l.ReceiptData[index].Retained = nil
		}
	}
	for _, receipt := range l.ReceiptData {
		if receipt.Sequence > l.NextSequence {
			l.NextSequence = receipt.Sequence
		}
	}
}

// DeltaKind identifies one user-authored persistent ledger mutation.
type DeltaKind string

const (
	MarkDelta  DeltaKind = "mark"
	ClearDelta DeltaKind = "clear"
)

// Delta is replayed against locked current state instead of replacing a stale ledger.
type Delta struct {
	Kind       DeltaKind
	Comparison FileComparison
	Bounds     Bounds
	Retained   *string
}

// Apply mutates a ledger exactly as the authored action requested.
func (d Delta) Apply(ledger *Ledger) bool {
	if d.Kind == ClearDelta {
		return ledger.Clear(d.Comparison)
	}
	return ledger.Mark(d.Comparison, d.Bounds, d.Retained)
}

func (l Ledger) reachableFrom(start Endpoint, comparison FileComparison) map[Endpoint]struct{} {
	reached := map[Endpoint]struct{}{start: {}}
	adjacent := make(map[Endpoint][]Endpoint)
	for _, receipt := range l.ReceiptData {
		if receipt.Old.Exact() && receipt.New.Exact() && actionComposes(receipt, comparison) {
			adjacent[receipt.Old] = append(adjacent[receipt.Old], receipt.New)
		}
	}
	queue := []Endpoint{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if _, ok := reached[next]; ok {
				continue
			}
			reached[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return reached
}

type edgeKey struct {
	Action FileAction
	Old    Endpoint
	New    Endpoint
}

func actionComposes(receipt Receipt, comparison FileComparison) bool {
	if receipt.Old.Path == receipt.New.Path {
		return true
	}
	switch comparison.Action {
	case Copied:
		return receipt.Action == Copied
	case Renamed:
		return receipt.Action == Renamed
	default:
		return false
	}
}

func actionForBounds(comparison FileComparison, bounds Bounds) FileAction {
	if bounds.Old == comparison.Old && bounds.New == comparison.New {
		return comparison.Action
	}
	switch {
	case bounds.Old.Kind == Absent:
		return Added
	case bounds.New.Kind == Absent:
		return Deleted
	case bounds.Old.Path != bounds.New.Path && comparison.Action == Copied:
		return Copied
	case bounds.Old.Path != bounds.New.Path:
		return Renamed
	default:
		return Modified
	}
}

func receiptRelated(receipt Receipt, comparison FileComparison) bool {
	return receipt.Old.Path == comparison.Old.Path || receipt.Old.Path == comparison.New.Path ||
		receipt.New.Path == comparison.Old.Path || receipt.New.Path == comparison.New.Path
}

func canContinue(frontier Endpoint, comparison FileComparison) bool {
	return frontier != comparison.New && (frontier.Path == comparison.New.Path ||
		frontier.Path == comparison.Old.Path || comparison.Action == Renamed || comparison.Action == Copied)
}

func boundedRetained(retained *string) *string {
	if retained == nil || len(*retained) > MaxRetainedBytes {
		return nil
	}
	return cloneString(retained)
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneReceipts(receipts []Receipt) []Receipt {
	result := append([]Receipt(nil), receipts...)
	for index := range result {
		result[index].Retained = cloneString(result[index].Retained)
	}
	return result
}
