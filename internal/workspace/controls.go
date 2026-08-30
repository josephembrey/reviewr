package workspace

// Header action keys are local to the active tab. Tab and Shift+Tab own
// top-level navigation, leaving the compact number row for view controls.
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

// GitView selects the Git navigator surface.
type GitView uint8

const (
	GitLog GitView = iota
	GitRefs
	GitStashes
)

func (view GitView) Label() string {
	switch view {
	case GitRefs:
		return "refs"
	case GitStashes:
		return "stashes"
	default:
		return "log"
	}
}

func (view GitView) Next() GitView {
	switch view {
	case GitLog:
		return GitRefs
	case GitRefs:
		return GitStashes
	default:
		return GitLog
	}
}

// GitTraversal selects the Log history traversal.
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
	Git           GitView
	Traversal     GitTraversal
	DiffHighlight DiffHighlight
	// RichDiff is presentation context derived from the visible structured
	// reader document. It keeps input, header, mouse, and footer eligibility
	// on one predicate without becoming browser or path state.
	RichDiff bool
}
