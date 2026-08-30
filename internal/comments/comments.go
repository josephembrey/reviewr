// Package comments owns review comments that deliberately live only for the
// current process. A Store consumes comments only after an explicit export
// succeeds; repository refreshes and reader presentation never mutate it.
package comments

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Side identifies the coherent source of a line range in a diff.
type Side uint8

const (
	CurrentSide Side = iota
	NewSide
	OldSide
)

const ContextRadius = 2

// SourceLine is a stable reader identity plus its displayed source number.
// Identity drives Continuity while Number is immutable authored export data.
type SourceLine struct {
	Identity string
	Number   uint64
}

// Range is one inclusive, coherent-side source range.
type Range struct {
	Side  Side
	Start SourceLine
	End   SourceLine
}

// ContextFingerprint is the bounded semantic source immediately around the
// selected lines. Values use the same verbatim marker-preserving encoding as
// Snippet and are sufficient to disambiguate repeated selections safely.
type ContextFingerprint struct {
	Before []string
	After  []string
}

// SnapshotLine is one side-specific source line available for relocation.
type SnapshotLine struct {
	Identity string
	Number   uint64
	Text     string
}

// SourceSnapshot identifies one immutable observation of a semantic source.
type SourceSnapshot struct {
	Identity string
	Lines    []SnapshotLine
}

// Normalize orders a range by source number without discarding endpoint
// identities. Diff-side validation belongs to the reader that creates it.
func (r Range) Normalize() Range {
	if r.End.Number < r.Start.Number {
		r.Start, r.End = r.End, r.Start
	}
	return r
}

// Draft is the complete authored input needed to create a saved comment.
type Draft struct {
	FileIdentity   string
	File           string
	Context        string
	SourceIdentity string
	Range          Range
	PreferredLine  uint64
	Snippet        string
	Fingerprint    ContextFingerprint
	Text           string
}

// Comment is one immutable saved inline comment.
type Comment struct {
	ID             string
	FileIdentity   string
	File           string
	Context        string
	SourceIdentity string
	Range          Range
	PreferredLine  uint64
	Snippet        string
	Fingerprint    ContextFingerprint
	Text           string
	// Stale means the authored source changed and no unique confident
	// relocation exists. The prior range and exact snippet remain untouched.
	Stale bool
}

// Location returns the concise path and inclusive source range used by cards
// and export. Old-side ranges are explicitly marked as removed.
func (c Comment) Location() string {
	r := c.Range.Normalize()
	location := fmt.Sprintf("%s:%d", c.File, r.Start.Number)
	if r.Start.Number != r.End.Number {
		location += fmt.Sprintf("-%d", r.End.Number)
	}
	if r.Side == OldSide {
		location += " (removed)"
	}
	return location
}

// Store is an in-memory, insertion-ordered comment collection.
type Store struct {
	next  uint64
	items []Comment
}

// Add saves a comment and returns its stable session identity.
func (s *Store) Add(draft Draft) Comment {
	s.next++
	comment := Comment{
		ID:             fmt.Sprintf("comment:%d", s.next),
		FileIdentity:   draft.FileIdentity,
		File:           draft.File,
		Context:        draft.Context,
		SourceIdentity: draft.SourceIdentity,
		Range:          draft.Range.Normalize(),
		PreferredLine:  draft.PreferredLine,
		Snippet:        draft.Snippet,
		Fingerprint: ContextFingerprint{
			Before: append([]string(nil), draft.Fingerprint.Before...),
			After:  append([]string(nil), draft.Fingerprint.After...),
		},
		Text: strings.TrimSpace(draft.Text),
	}
	if comment.FileIdentity == "" {
		comment.FileIdentity = comment.File
	}
	if comment.PreferredLine == 0 {
		comment.PreferredLine = comment.Range.Start.Number
	}
	s.items = append(s.items, comment)
	return comment
}

// Len returns the number of comments still awaiting explicit export.
func (s Store) Len() int { return len(s.items) }

// Items returns a copy so presentation cannot mutate the store.
func (s Store) Items() []Comment { return append([]Comment(nil), s.items...) }

// In returns comments belonging to one reader file/context in insertion order.
func (s Store) In(file, context string) []Comment {
	result := make([]Comment, 0)
	for _, comment := range s.items {
		if comment.File == file && comment.Context == context {
			result = append(result, comment)
		}
	}
	return result
}

// Reconcile updates current/new-side comments for one semantic file and
// reader context. Old-side comments remain tied to their immutable revision.
// It reports whether any canonical resolution state changed.
func (s *Store) Reconcile(fileIdentity, context string, side Side, snapshot SourceSnapshot) bool {
	changed := false
	for index := range s.items {
		comment := s.items[index]
		if comment.FileIdentity != fileIdentity || comment.Context != context || comment.Range.Side != side || side == OldSide {
			continue
		}
		next := Reconcile(comment, snapshot)
		if !sameResolution(comment, next) {
			s.items[index] = next
			changed = true
		}
	}
	return changed
}

// Reconcile retains exact ranges on an unchanged source and otherwise moves
// only to a unique exact selection match that is unique after bounded context
// is considered. Missing or ambiguous matches become stale without retargeting.
func Reconcile(comment Comment, snapshot SourceSnapshot) Comment {
	if comment.Range.Side == OldSide {
		return comment
	}
	if snapshot.Identity == "" {
		comment.Stale = true
		return comment
	}
	if comment.SourceIdentity == snapshot.Identity {
		comment.Stale = false
		return comment
	}
	selected := splitSnippet(comment.Snippet)
	if len(selected) == 0 || len(selected) > len(snapshot.Lines) {
		comment.Stale = true
		return comment
	}
	candidates := make([]int, 0)
	for start := 0; start+len(selected) <= len(snapshot.Lines); start++ {
		if snapshotMatches(snapshot.Lines, start, selected) && contextMatches(snapshot.Lines, start, len(selected), comment.Fingerprint) {
			candidates = append(candidates, start)
		}
	}
	if len(candidates) != 1 {
		comment.Stale = true
		return comment
	}
	start := candidates[0]
	end := start + len(selected) - 1
	offset := uint64(0)
	normalized := comment.Range.Normalize()
	if comment.PreferredLine > normalized.Start.Number {
		offset = comment.PreferredLine - normalized.Start.Number
	}
	comment.Range.Start = SourceLine{Identity: snapshot.Lines[start].Identity, Number: snapshot.Lines[start].Number}
	comment.Range.End = SourceLine{Identity: snapshot.Lines[end].Identity, Number: snapshot.Lines[end].Number}
	comment.PreferredLine = min(comment.Range.End.Number, comment.Range.Start.Number+offset)
	comment.SourceIdentity = snapshot.Identity
	comment.Stale = false
	return comment
}

func sameResolution(left, right Comment) bool {
	return left.SourceIdentity == right.SourceIdentity && left.Range == right.Range &&
		left.PreferredLine == right.PreferredLine && left.Stale == right.Stale
}

func splitSnippet(snippet string) []string {
	snippet = strings.ReplaceAll(snippet, "\r", "")
	if snippet == "" {
		return nil
	}
	return strings.Split(snippet, "\n")
}

func snapshotMatches(lines []SnapshotLine, start int, selected []string) bool {
	for offset, text := range selected {
		if lines[start+offset].Text != text {
			return false
		}
	}
	return true
}

func contextMatches(lines []SnapshotLine, start, selected int, fingerprint ContextFingerprint) bool {
	before := fingerprint.Before
	if len(before) > ContextRadius {
		before = before[len(before)-ContextRadius:]
	}
	for offset := 1; offset <= len(before); offset++ {
		index := start - offset
		if index < 0 || lines[index].Text != before[len(before)-offset] {
			return false
		}
	}
	after := fingerprint.After
	if len(after) > ContextRadius {
		after = after[:ContextRadius]
	}
	for offset, text := range after {
		index := start + selected + offset
		if index >= len(lines) || lines[index].Text != text {
			return false
		}
	}
	return true
}

// Exporter is one explicit, all-or-nothing comment destination.
type Exporter interface {
	Export(string) error
}

var ErrNoComments = errors.New("no comments to export")

// Export sends every comment and consumes the store only after success.
func (s *Store) Export(target Exporter) error {
	if len(s.items) == 0 {
		return ErrNoComments
	}
	if err := target.Export(FormatAll(s.items)); err != nil {
		return err
	}
	s.items = nil
	return nil
}

// Format returns one location/snippet/body export block.
func Format(comment Comment) string {
	parts := []string{comment.Location()}
	if comment.Snippet != "" {
		// Snippets are authored evidence, not prose: keep the first context
		// marker and every other byte exactly as captured from the reader.
		parts = append(parts, comment.Snippet)
	}
	if body := normalizeBody(comment.Text); body != "" {
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n")
}

// FormatAll sorts by file and source line while retaining insertion order for
// equal anchors, then separates comments with one blank line.
func FormatAll(comments []Comment) string {
	ordered := append([]Comment(nil), comments...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].File != ordered[right].File {
			return ordered[left].File < ordered[right].File
		}
		return ordered[left].Range.Normalize().Start.Number < ordered[right].Range.Normalize().Start.Number
	})
	blocks := make([]string, len(ordered))
	for index, comment := range ordered {
		blocks[index] = Format(comment)
	}
	return strings.Join(blocks, "\n\n")
}

func normalizeBody(text string) string {
	text = strings.ReplaceAll(text, "\r", "")
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
