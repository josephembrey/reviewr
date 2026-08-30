package filetree

import (
	"sort"
	"strings"
)

func build(paths []string) *directory {
	root := newDirectory()
	for _, path := range paths {
		if path == "" {
			continue
		}
		segments := strings.Split(path, "/")
		valid := true
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
		// Repeated paths overwrite the same leaf, so they need no separate
		// deduplication map.
		current.files[segments[len(segments)-1]] = path
	}
	return root
}

func (t *Tree) derive() {
	t.rows = t.rows[:0]
	t.files = t.files[:0]
	if t.allRows == nil {
		t.allRows = make(map[string]Row)
	} else {
		clear(t.allRows)
	}
	if t.directories == nil {
		t.directories = make(map[string]struct{})
	} else {
		clear(t.directories)
	}
	if t.root != nil {
		t.flatten(t.root, "", 0, true)
	}
}

func (t *Tree) flatten(node *directory, prefix string, depth int, visible bool) {
	for _, name := range sortedKeys(node.dirs) {
		display, path, child := compress(prefix, name, node.dirs[name])
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

func compress(prefix, name string, node *directory) (string, string, *directory) {
	if len(node.files) != 0 || len(node.dirs) != 1 {
		return name, join(prefix, name), node
	}

	var display strings.Builder
	display.WriteString(name)
	for len(node.files) == 0 && len(node.dirs) == 1 {
		childName := onlyKey(node.dirs)
		display.WriteByte('/')
		display.WriteString(childName)
		node = node.dirs[childName]
	}
	compacted := display.String()
	return compacted, join(prefix, compacted), node
}

func loneFile(node *directory) (string, string, bool) {
	if len(node.dirs) != 0 || len(node.files) != 1 {
		return "", "", false
	}
	name := onlyKey(node.files)
	return name, node.files[name], true
}

func onlyKey[T any](values map[string]T) string {
	for key := range values {
		return key
	}
	return ""
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(source))
	for value := range source {
		clone[value] = struct{}{}
	}
	return clone
}

func sliceSet(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path != "" {
			result[path] = struct{}{}
		}
	}
	return result
}

func sortedSet(source map[string]struct{}) []string {
	result := make([]string, 0, len(source))
	for path := range source {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func join(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "/" + child
}
