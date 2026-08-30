// Package review owns exact, private file-review coverage.
//
// A receipt records one explicit comparison edge. Coverage composes only when
// adjacent endpoint identities are exactly equal; filenames and display rows
// are never proof.
package review

import (
	"crypto/sha256"
	"encoding/hex"
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
