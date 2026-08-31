package workspace

// Header action keys are local to the active workspace. Tab and Shift+Tab
// move pane focus, leaving the compact number row for view controls.
const (
	SecondaryControlKey  = "1"
	TertiaryControlKey   = "2"
	ComparisonControlKey = "3"
	DiffHighlightKey     = "4"
)

// FileSet selects the Files navigator universe.
type FileSet uint8

const (
	AllFiles FileSet = iota
	ChangedFiles
)

func (set FileSet) Label() string {
	if set == ChangedFiles {
		return "changed"
	}
	return "all"
}

func (set FileSet) Toggle() FileSet {
	if set == ChangedFiles {
		return AllFiles
	}
	return ChangedFiles
}

// ReaderMode selects complete file content or a diff document.
type ReaderMode uint8

const (
	FileReader ReaderMode = iota
	DiffReader
)

func (mode ReaderMode) Label() string {
	if mode == DiffReader {
		return "diff"
	}
	return "file"
}

func (mode ReaderMode) Toggle() ReaderMode {
	if mode == DiffReader {
		return FileReader
	}
	return DiffReader
}

// DiffHighlight selects the render-only treatment for changed reader rows.
// It is global session state; RichDiff below is derived from the document on
// screen and is never persisted as place state.
type DiffHighlight uint8

const (
	DiffHighlightSidebar DiffHighlight = iota
	DiffHighlightBackground
)

func (highlight DiffHighlight) Label() string {
	if highlight == DiffHighlightBackground {
		return "background"
	}
	return "sidebar"
}

func (highlight DiffHighlight) Toggle() DiffHighlight {
	if highlight == DiffHighlightBackground {
		return DiffHighlightSidebar
	}
	return DiffHighlightBackground
}

// Comparison selects the Files changeset basis.
type Comparison uint8

const (
	Uncommitted Comparison = iota
	Branch
	LastTurn
)

func (comparison Comparison) Label() string {
	switch comparison {
	case Branch:
		return "branch"
	case LastTurn:
		return "last-turn"
	default:
		return "uncommitted"
	}
}

func (comparison Comparison) Next() Comparison {
	switch comparison {
	case Uncommitted:
		return Branch
	case Branch:
		return LastTurn
	default:
		return Uncommitted
	}
}

// GitMode selects one of Git's two top-level read-only workspaces.
type GitMode uint8

const (
	GitHistory GitMode = iota
	GitStashes
)

func (mode GitMode) Label() string {
	if mode == GitStashes {
		return "stashes"
	}
	return "history"
}

func (mode GitMode) Toggle() GitMode {
	if mode == GitStashes {
		return GitHistory
	}
	return GitStashes
}

// GitTraversal selects the History traversal.
type GitTraversal uint8

const (
	GitGraph GitTraversal = iota
	GitFirstParent
)

func (traversal GitTraversal) Label() string {
	if traversal == GitFirstParent {
		return "first-parent"
	}
	return "graph"
}

func (traversal GitTraversal) Toggle() GitTraversal {
	if traversal == GitFirstParent {
		return GitGraph
	}
	return GitFirstParent
}

// Controls collects root-owned view controls. Files/Git axes are browser-local;
// DiffHighlight is one global session preference, and RichDiff is derived
// presentation context rather than stored place state.
type Controls struct {
	Files         FileSet
	Reader        ReaderMode
	Comparison    Comparison
	Git           GitMode
	Traversal     GitTraversal
	DiffHighlight DiffHighlight
	// MarkdownPreviewEligible and MarkdownPreview are derived from the visible
	// Files reader. They route and advertise the local m toggle without turning
	// preview into a global control axis.
	MarkdownPreviewEligible bool
	MarkdownPreview         bool
	// RichDiff is presentation context derived from the visible structured
	// reader document. It keeps input, header, mouse, and footer eligibility
	// on one predicate without becoming browser or path state.
	RichDiff bool
}
