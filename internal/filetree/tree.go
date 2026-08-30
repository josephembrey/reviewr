// Package filetree turns repository-relative paths into a foldable hierarchy.
package filetree

import (
	"sort"
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

func build(paths []string) *directory {
	root := newDirectory()
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		segments := strings.Split(path, "/")
		valid := len(segments) > 0
		for _, segment := range segments {
			if segment == "" {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		current := root
		for _, segment := range segments[:len(segments)-1] {
			next := current.dirs[segment]
			if next == nil {
				next = newDirectory()
				current.dirs[segment] = next
			}
			current = next
		}
		current.files[segments[len(segments)-1]] = path
	}
	return root
}

func (t *Tree) derive() {
	t.rows = nil
	t.files = nil
	t.allRows = make(map[string]Row)
	t.directories = make(map[string]struct{})
	if t.root != nil {
		t.flatten(t.root, "", 0, true)
	}
}

func (t *Tree) flatten(node *directory, prefix string, depth int, visible bool) {
	for _, name := range sortedKeys(node.dirs) {
		display, path, child := compress(name, join(prefix, name), node.dirs[name])
		if fileName, filePath, ok := loneFile(child); ok {
			t.addFile(filePath, display+"/"+fileName, depth, visible)
			continue
		}

		_, collapsed := t.collapsed[path]
		row := Row{
			Identity: DirectoryIdentity(path),
			Path:     path,
			Name:     display,
			Depth:    depth,
			Kind:     Directory,
			Expanded: !collapsed,
		}
		t.directories[path] = struct{}{}
		t.allRows[row.Identity] = row
		if visible {
			t.rows = append(t.rows, row)
		}
		t.flatten(child, path, depth+1, visible && row.Expanded)
	}

	for _, name := range sortedKeys(node.files) {
		t.addFile(node.files[name], name, depth, visible)
	}
}

func (t *Tree) addFile(path, name string, depth int, visible bool) {
	row := Row{Identity: FileIdentity(path), Path: path, Name: name, Depth: depth, Kind: File}
	t.files = append(t.files, path)
	t.allRows[row.Identity] = row
	if visible {
		t.rows = append(t.rows, row)
	}
}

func compress(name, path string, node *directory) (string, string, *directory) {
	display := name
	for len(node.files) == 0 && len(node.dirs) == 1 {
		childName := sortedKeys(node.dirs)[0]
		display += "/" + childName
		path = join(path, childName)
		node = node.dirs[childName]
	}
	return display, path, node
}

func loneFile(node *directory) (string, string, bool) {
	if len(node.dirs) != 0 || len(node.files) != 1 {
		return "", "", false
	}
	name := sortedKeys(node.files)[0]
	return name, node.files[name], true
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func join(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "/" + child
}

// DirectoryIdentity keeps directory and file identities disjoint.
func DirectoryIdentity(path string) string { return directoryPrefix + path }

// FileIdentity keeps directory and file identities disjoint.
func FileIdentity(path string) string { return filePrefix + path }
