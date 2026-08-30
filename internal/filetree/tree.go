// Package filetree turns repository-relative paths into a foldable hierarchy.
package filetree

import (
	"strings"
)

// Kind identifies the semantic target represented by a tree row.
type Kind uint8

const (
	Directory Kind = iota
	File
)

const (
	directoryPrefix = "directory:"
	filePrefix      = "file:"
)

// Row is one logical tree row. Identity remains stable across row movement.
type Row struct {
	Identity string
	Path     string
	Name     string
	Depth    int
	Kind     Kind
	Expanded bool
}

// Tree owns hierarchy and fold state while exposing a flat visible projection.
type Tree struct {
	root        *directory
	collapsed   map[string]struct{}
	rows        []Row
	allRows     map[string]Row
	directories map[string]struct{}
	files       []string
}

// FoldState is a scope-local snapshot of known and collapsed directories.
// Its internals stay private so only Tree can apply it to a new hierarchy.
type FoldState struct {
	known     map[string]struct{}
	collapsed map[string]struct{}
}

type directory struct {
	dirs  map[string]*directory
	files map[string]string
}

func newDirectory() *directory {
	return &directory{dirs: make(map[string]*directory), files: make(map[string]string)}
}

// New builds an initially expanded tree from repository-relative file paths.
func New(paths []string) Tree {
	tree := Tree{collapsed: make(map[string]struct{})}
	tree.Rebuild(paths)
	return tree
}

// Rebuild replaces the hierarchy while retaining folds for surviving directory rows.
func (t *Tree) Rebuild(paths []string) {
	t.root = build(paths)
	if t.collapsed == nil {
		t.collapsed = make(map[string]struct{})
	}
	t.derive()
	for path := range t.collapsed {
		if _, ok := t.directories[path]; !ok {
			delete(t.collapsed, path)
		}
	}
}

// Rows returns the current visible depth-first projection.
func (t Tree) Rows() []Row { return append([]Row(nil), t.rows...) }

// Identities returns visible row identities in navigator order.
func (t Tree) Identities() []string {
	identities := make([]string, len(t.rows))
	for index, row := range t.rows {
		identities[index] = row.Identity
	}
	return identities
}

// Files returns every file path in tree order, including hidden descendants.
func (t Tree) Files() []string { return append([]string(nil), t.files...) }

// FileCount reports complete repository file count without allocating a copy.
func (t Tree) FileCount() int { return len(t.files) }

// Folds captures directory state independently of the current visible rows.
func (t Tree) Folds() FoldState {
	return FoldState{
		known:     cloneSet(t.directories),
		collapsed: cloneSet(t.collapsed),
	}
}

// NewFoldState restores a persisted directory snapshot. Unknown and duplicate
// paths are harmless because Tree validates them against its current hierarchy.
func NewFoldState(known, collapsed []string) FoldState {
	return FoldState{known: sliceSet(known), collapsed: sliceSet(collapsed)}
}

// Paths exposes a deterministic persistence representation without making the
// FoldState maps mutable outside this package.
func (state FoldState) Paths() (known, collapsed []string) {
	known = sortedSet(state.known)
	collapsed = sortedSet(state.collapsed)
	return known, collapsed
}

// RestoreFolds applies authored state to surviving directories. Directories
// absent from the saved hierarchy follow collapseNew.
func (t *Tree) RestoreFolds(state FoldState, collapseNew bool) {
	if t.collapsed == nil {
		t.collapsed = make(map[string]struct{})
	} else {
		clear(t.collapsed)
	}
	for path := range t.directories {
		_, known := state.known[path]
		_, collapsed := state.collapsed[path]
		if collapsed || (collapseNew && !known) {
			t.collapsed[path] = struct{}{}
		}
	}
	t.derive()
}

// Row resolves either a visible or hidden row identity.
func (t Tree) Row(identity string) (Row, bool) {
	row, ok := t.allRows[identity]
	return row, ok
}

// FirstVisibleFile returns the first file row without stopping on a directory.
func (t Tree) FirstVisibleFile() (Row, bool) {
	for _, row := range t.rows {
		if row.Kind == File {
			return row, true
		}
	}
	return Row{}, false
}

// CollapseAll hides every directory subtree, including directories that are
// already hidden below another collapsed directory.
func (t *Tree) CollapseAll() {
	for path := range t.directories {
		t.collapsed[path] = struct{}{}
	}
	t.derive()
}

// Collapse hides the selected directory's descendants.
func (t *Tree) Collapse(identity string) bool {
	row, ok := t.allRows[identity]
	if !ok || row.Kind != Directory || !row.Expanded {
		return false
	}
	t.collapsed[row.Path] = struct{}{}
	t.derive()
	return true
}

// Expand reveals the selected directory's descendants.
func (t *Tree) Expand(identity string) bool {
	row, ok := t.allRows[identity]
	if !ok || row.Kind != Directory || row.Expanded {
		return false
	}
	delete(t.collapsed, row.Path)
	t.derive()
	return true
}

// Toggle reverses the selected directory's expansion state.
func (t *Tree) Toggle(identity string) bool {
	row, ok := t.allRows[identity]
	if !ok || row.Kind != Directory {
		return false
	}
	if row.Expanded {
		return t.Collapse(identity)
	}
	return t.Expand(identity)
}

// ExpandParents reveals path without creating a second tree or changing any
// unrelated fold. It is used by review-gap navigation for hidden descendants.
func (t *Tree) ExpandParents(path string) bool {
	changed := false
	for directory := range t.collapsed {
		if strings.HasPrefix(path, directory+"/") {
			delete(t.collapsed, directory)
			changed = true
		}
	}
	if changed {
		t.derive()
	}
	return changed
}

// DirectoryIdentity keeps directory and file identities disjoint.
func DirectoryIdentity(path string) string { return directoryPrefix + path }

// FileIdentity keeps directory and file identities disjoint.
func FileIdentity(path string) string { return filePrefix + path }
