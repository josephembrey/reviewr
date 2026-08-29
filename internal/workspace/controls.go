package workspace

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

// Controls is the browser-local header state. The Go foundation exposes these
// switches before their full legacy data sources are ported.
type Controls struct {
	Files      FileSet
	Reader     ReaderMode
	Comparison Comparison
	Git        GitView
	Traversal  GitTraversal
}
