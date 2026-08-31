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
	ToneSpecial
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

// TextStyle is a foreground-only token style. Foreground is an ANSI slot;
// backgrounds remain owned by selection and diff presentation.
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
	ReaderMarkdown
	ReaderCommentHeader
	ReaderCommentBody
	ReaderCommentEnd
	ReaderCommentComposerHeader
	ReaderCommentComposerBody
	ReaderCommentComposerEnd
)

// ReaderRow is one logical rich-reader row. Text and Spans contain code or
// prose payload only: diff marker characters belong to Kind, never content.
type ReaderRow struct {
	Identity string
	Kind     ReaderRowKind
	Text     string
	Tone     Tone
	Spans    []TextSpan
	// Styled contains ANSI authored by reviewr's Markdown renderer. Source
	// content never enters this field directly.
	Styled  string
	OldLine uint64
	NewLine uint64
	// RemovedBefore/After anchor deleted source lines at a surviving full-file
	// row without inserting fake document content.
	RemovedBefore uint64
	RemovedAfter  uint64
	// ReviewFresh is derived from the exact reviewed frontier rather than stored
	// line state. ReviewRemovedBefore/After anchor frontier deletions that have
	// no surviving current line of their own.
	ReviewFresh         bool
	ReviewRemovedBefore uint64
	ReviewRemovedAfter  uint64
	// FoldExpanded keeps a fold control visible while its context rows are shown.
	FoldExpanded bool
	// FoldTarget links a secondary fold control to the stable leading fold.
	FoldTarget string
	// VisualSelected is semantic linewise selection authored by the app. It is
	// independent from the keyboard cursor and survives wrapping unchanged.
	VisualSelected bool
	// VisualCharacter selects a half-open terminal-cell range in Text. Unlike
	// VisualSelected it leaves the gutter and the rest of the source row alone.
	// ReaderLayout projects the source range onto each wrapped segment.
	VisualCharacter bool
	VisualStart     int
	VisualEnd       int
	// CommentHover replaces this source row's line number with the gutter [+]
	// affordance. Only the first wrapped segment paints it.
	CommentHover bool
	// CommentID links all rows in one inline card or composer. The header is the
	// card's only selectable row and its stable reader landmark.
	CommentID string
	// Comment anchor metadata is canonical-source information copied onto the
	// header for disposable hunk intersection/ownership derivation only.
	CommentOldSide bool
	CommentStart   uint64
	CommentEnd     uint64
	CommentStale   bool
	// ComposerCursor marks the insertion point within a pre-wrapped composer
	// body row. ComposerCursorColumn is a terminal-cell boundary.
	ComposerCursor       bool
	ComposerCursorColumn int
}

// Commentable reports whether a row names an unambiguous source side/line.
func (row ReaderRow) Commentable() bool {
	switch row.Kind {
	case ReaderFile, ReaderInsertion:
		return row.NewLine > 0
	case ReaderDeletion:
		return row.OldLine > 0
	case ReaderContext:
		return row.NewLine > 0 || row.OldLine > 0
	default:
		return false
	}
}

// Selectable reports whether ordinary reader motion may land on this row.
// Comment bodies and framing belong to their first-class header row.
func (row ReaderRow) Selectable() bool {
	switch row.Kind {
	case ReaderCommentBody, ReaderCommentEnd,
		ReaderCommentComposerHeader, ReaderCommentComposerBody, ReaderCommentComposerEnd:
		return false
	default:
		return true
	}
}

// CommentHeaderIdentity returns the saved comment controlled by this row.
func (row ReaderRow) CommentHeaderIdentity() (string, bool) {
	return row.CommentID, row.Kind == ReaderCommentHeader && row.CommentID != ""
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
	ReaderMarkdownDocument
)

// ReaderDocument is the app-to-UI seam for every rich file and diff source.
type ReaderDocument struct {
	Kind ReaderDocumentKind
	Rows []ReaderRow
}

// HasReviewFreshness reports whether the document needs the optional reviewed-
// frontier rail. It remains absent for unreviewed and proof-broken comparisons.
func (document ReaderDocument) HasReviewFreshness() bool {
	for _, row := range document.Rows {
		if row.ReviewFresh || row.ReviewRemovedBefore > 0 || row.ReviewRemovedAfter > 0 {
			return true
		}
	}
	return false
}

// WithoutReviewFreshness removes derived paint without changing source
// identities. Coverage changes can therefore update the rail without moving
// the reader's authored place state.
func (document ReaderDocument) WithoutReviewFreshness() ReaderDocument {
	if !document.HasReviewFreshness() {
		return document
	}
	result := document
	result.Rows = append([]ReaderRow(nil), document.Rows...)
	for index := range result.Rows {
		result.Rows[index].ReviewFresh = false
		result.Rows[index].ReviewRemovedBefore = 0
		result.Rows[index].ReviewRemovedAfter = 0
	}
	return result
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

// SelectionTarget maps a painted auxiliary comment row back to its selectable
// header. Other rows remain their own target.
func (document ReaderDocument) SelectionTarget(index int) int {
	if len(document.Rows) == 0 {
		return 0
	}
	index = max(0, min(index, len(document.Rows)-1))
	row := document.Rows[index]
	if row.Selectable() || row.CommentID == "" {
		return index
	}
	for candidate := index; candidate >= 0; candidate-- {
		if document.Rows[candidate].Kind == ReaderCommentHeader && document.Rows[candidate].CommentID == row.CommentID {
			return candidate
		}
	}
	return index
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

// SettingEntry is the narrow render seam for the session-scoped Settings
// list. Storage and behavior remain owned by the application model.
type SettingEntry struct {
	Label    string
	Enabled  bool
	Selected bool
}

type Settings struct {
	Open    bool
	Entries []SettingEntry
}

// GitModel is the read-only presentation seam for Git's mode-specific nested
// regions. It deliberately reuses NavigatorRow, ReaderDocument, ReaderLayout,
// and commitrow.Row so Git never grows parallel list or diff renderers.
type GitModel struct {
	Geometry        GitGeometry
	Focus           workspace.GitFocus
	DividerDragging GitDividerKind

	RailTitle    string
	RailRows     []NavigatorRow
	RailEmpty    Line
	RailSelected int
	RailTop      int

	FilesTitle    string
	FilesRows     []NavigatorRow
	FilesEmpty    Line
	FilesSelected int
	FilesTop      int

	TimelineTitle    string
	TimelineRows     []commitrow.Row
	TimelineEmpty    Line
	TimelineSelected int
	TimelineTop      int
	Status           Line

	ReaderTitle           string
	ReaderDocument        ReaderDocument
	ReaderLayout          *ReaderLayout
	ReaderContextFoldable bool
	ReaderEmpty           Line
	ReaderOffset          int
	ReaderColumn          int
	ReaderCursor          int
}

// FileFooterActions describes only workflow actions that are meaningful for
// the current Files selection. Routine navigation belongs in the help popup.
type FileFooterActions struct {
	Review       bool
	ReviewBounds bool
	NextGap      bool
}

// Model contains only the workspace-neutral derived state needed to paint a frame.
type Model struct {
	Geometry        Geometry
	Workspace       workspace.Kind
	DividerDragging bool
	HelpOpen        bool
	Settings        Settings
	Controls        workspace.Controls
	FileActions     FileFooterActions
	Changes         ChangeSummary
	Git             *GitModel

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
	ReaderCursor             int
	FooterWarning            string
	ReaderVisualSelection    bool
	ReaderCharacterSelection bool
	ReaderComposingComment   bool
	ReaderCommentHeader      bool
	ReaderCommentExpanded    bool
	ReaderCommentable        bool

	Notes       notes.Presentation
	NotesStatus string
	NotesError  bool
	// NotesStatusPriority keeps read-only and error state ahead of optional help
	// when the footer cannot fit every entry.
	NotesStatusPriority bool
	NotesScope          notes.Scope
	NotesHasWorktree    bool
}
