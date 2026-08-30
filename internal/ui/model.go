package ui

import (
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Tone is semantic text emphasis chosen before terminal styling.
type Tone uint8

const (
	ToneDefault Tone = iota
	ToneQuiet
	ToneError
	ToneAccent
	ToneAdded
	ToneRemoved
	ToneInfo
	ToneWarning
)

// Line is one logical reader or empty-state line.
type Line struct {
	Text  string
	Tone  Tone
	Spans []TextSpan
}

// TextStyle is a foreground-only token style. Foreground accepts an ANSI slot
// or truecolor value; backgrounds remain owned by selection and diff presentation.
type TextStyle struct {
	Foreground string
	Bold       bool
	Italic     bool
	Underline  bool
}

// TextSpan is one styled fragment within a reader line.
type TextSpan struct {
	Text  string
	Tone  Tone
	Style TextStyle
}

// ReaderRowKind owns reader presentation independently from source text.
type ReaderRowKind uint8

const (
	ReaderFile ReaderRowKind = iota
	ReaderContext
	ReaderInsertion
	ReaderDeletion
	ReaderMetadata
	ReaderNotice
	ReaderFold
	ReaderFoldEnd
)

// ReaderRow is one logical rich-reader row. Text and Spans contain code or
// prose payload only: diff marker characters belong to Kind, never content.
type ReaderRow struct {
	Identity string
	Kind     ReaderRowKind
	Text     string
	Tone     Tone
	Spans    []TextSpan
	OldLine  uint64
	NewLine  uint64
	// RemovedBefore/After anchor deleted source lines at a surviving full-file
	// row without inserting fake document content.
	RemovedBefore uint64
	RemovedAfter  uint64
	// FoldExpanded keeps a fold control visible while its context rows are shown.
	FoldExpanded bool
	// FoldTarget links a secondary fold control to the stable leading fold.
	FoldTarget string
}

// ContextFoldIdentity returns the independently authored fold controlled by
// this row. The leading control owns its identity; an expanded end marker
// points back to that same identity while retaining its own place identity.
func (row ReaderRow) ContextFoldIdentity() (string, bool) {
	switch row.Kind {
	case ReaderFold:
		return row.Identity, row.Identity != ""
	case ReaderFoldEnd:
		return row.FoldTarget, row.FoldTarget != ""
	default:
		return "", false
	}
}

// DisplayLine is the semantic identity shown in the one-sided gutter.
func (row ReaderRow) DisplayLine() uint64 {
	switch row.Kind {
	case ReaderDeletion:
		return row.OldLine
	case ReaderFile, ReaderContext, ReaderInsertion:
		return row.NewLine
	default:
		if row.NewLine > 0 {
			return row.NewLine
		}
		return row.OldLine
	}
}

// ReaderDocumentKind distinguishes complete files from actual diff readers.
type ReaderDocumentKind uint8

const (
	ReaderDocumentNone ReaderDocumentKind = iota
	ReaderFileDocument
	ReaderDiffDocument
)

// ReaderDocument is the app-to-UI seam for every rich file and diff source.
type ReaderDocument struct {
	Kind ReaderDocumentKind
	Rows []ReaderRow
}

// DiffEligible reports whether a non-empty semantic diff is actually visible.
func (document ReaderDocument) DiffEligible() bool {
	if document.Kind != ReaderDiffDocument {
		return false
	}
	for _, row := range document.Rows {
		switch row.Kind {
		case ReaderContext, ReaderInsertion, ReaderDeletion:
			return true
		}
	}
	return false
}

// GutterDigits is stable for the whole document and includes hidden/future
// rows because it measures semantic identities rather than the viewport.
func (document ReaderDocument) GutterDigits() int {
	maximum := uint64(0)
	for _, row := range document.Rows {
		switch row.Kind {
		case ReaderFile, ReaderInsertion:
			maximum = max(maximum, row.NewLine)
		case ReaderDeletion:
			maximum = max(maximum, row.OldLine)
		case ReaderContext:
			maximum = max(maximum, row.OldLine, row.NewLine)
		case ReaderMetadata, ReaderNotice, ReaderFold, ReaderFoldEnd:
			maximum = max(maximum, row.DisplayLine())
		}
	}
	digits := 1
	for maximum >= 10 {
		digits++
		maximum /= 10
	}
	return max(3, digits)
}

// Segment is one styled, single-line fragment in a compact row.
type Segment struct {
	Text string
	Tone Tone
}

// NavigatorStatus is semantic repository state. Rendering owns its restrained
// marker while later icon work can style the same metadata independently.
type NavigatorStatus uint8

const (
	StatusNone NavigatorStatus = iota
	StatusModified
	StatusAdded
	StatusDeleted
	StatusRenamed
	StatusUntracked
	StatusIgnored
)

// LineChanges is one changed file's diff statistic.
type LineChanges struct {
	Additions uint64
	Deletions uint64
}

// NavigatorRow separates stable identity from its display label.
type NavigatorRow struct {
	Identity  string
	Label     string
	Prefix    []Segment
	Suffix    []Segment
	Commit    *commitrow.Row
	Tree      bool
	Depth     int
	Directory bool
	Expanded  bool
	Status    NavigatorStatus
	Dimmed    bool
	// Changes is a right-aligned per-file statistic. Zero keeps unchanged and
	// binary rows visually quiet.
	Changes LineChanges
	// Review is an independent right-side file badge. Reviewable distinguishes
	// Unreviewed, whose enum value is zero, from observation-only rows.
	Reviewable bool
	Review     review.State
	// Progress is derived directory reviewed/changed coverage.
	Progress string
}

// ChangeSummary is the aggregate worktree status shown in the header.
type ChangeSummary struct {
	Files     uint64
	Additions uint64
	Deletions uint64
	Ready     bool
}

// Model contains only the workspace-neutral derived state needed to paint a frame.
type Model struct {
	Geometry        Geometry
	Workspace       workspace.Kind
	DividerDragging bool
	HelpOpen        bool
	Controls        workspace.Controls
	Changes         ChangeSummary

	NavigatorTitle string
	NavigatorRows  []NavigatorRow
	NavigatorEmpty Line
	Selected       int
	Top            int
	Focus          navigation.Focus

	ReaderTitle    string
	ReaderDocument ReaderDocument
	// ReaderLayout reuses the app's input geometry when the document and pane
	// width have not changed. Scrolling should never rewrap the whole file.
	ReaderLayout          *ReaderLayout
	ReaderContextFoldable bool
	ReaderLines           []Line
	ReaderCommitRows      []commitrow.Row
	ReaderEmpty           Line
	ReaderOffset          int
	// ReaderColumn is the wrapped segment's source-cell offset within
	// ReaderOffset's stable logical row.
	ReaderColumn int
	// ReaderCursor is the selected logical row in a structured file reader.
	ReaderCursor  int
	FooterWarning string

	Notes       notes.Presentation
	NotesStatus string
	NotesError  bool
	// NotesStatusPriority keeps read-only and error state ahead of optional help
	// when the footer cannot fit every entry.
	NotesStatusPriority bool
	NotesScope          notes.Scope
	NotesHasWorktree    bool
}
