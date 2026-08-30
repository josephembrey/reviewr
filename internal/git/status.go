package git

import (
	"bytes"
	"fmt"
	"sort"
)

// FileState is the one status assigned to a repository-relative file identity.
type FileState uint8

const (
	FileUnchanged FileState = iota
	FileModified
	FileAdded
	FileDeleted
	FileRenamed
	FileUntracked
	FileIgnored
)

// FileEntry carries a current path identity and its machine-readable Git state.
// PreviousPath is set only when porcelain reports a rename relation.
type FileEntry struct {
	Path         string
	PreviousPath string
	State        FileState
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// Snapshot reads all tracked, untracked, and ignored file identities, then
// overlays porcelain-v2 state and per-file line statistics into one result.
func (client Client) Snapshot(root string) ([]FileEntry, error) {
	entries, err := client.Inventory(root)
	if err != nil {
		return nil, err
	}
	untracked := make([]string, 0)
	for _, entry := range entries {
		if entry.State == FileUntracked {
			untracked = append(untracked, entry.Path)
		}
	}
	stats, err := client.worktreeStats(root, untracked)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		stat, changed := stats[entries[index].Path]
		if !changed {
			continue
		}
		entries[index].Additions = stat.additions
		entries[index].Deletions = stat.deletions
		entries[index].Binary = stat.binary
	}
	return entries, nil
}

// Inventory reads all tracked, untracked, and ignored identities without
// calculating comparison-specific line statistics.
func (client Client) Inventory(root string) ([]FileEntry, error) {
	trackedOutput, err := run(root, "ls-files", "-z", "--cached")
	if err != nil {
		return nil, err
	}
	statusOutput, err := run(
		root,
		"status",
		"--porcelain=v2",
		"-z",
		"--untracked-files=all",
		"--ignored=traditional",
		"--renames",
	)
	if err != nil {
		return nil, err
	}
	status, err := ParsePorcelainV2(statusOutput)
	if err != nil {
		return nil, err
	}
	return MergeFileEntries(ParseNUL(trackedOutput), status), nil
}

// ParsePorcelainV2 parses only NUL-delimited porcelain-v2 records. Paths remain
// opaque byte strings; in particular, whitespace and newlines are not syntax.
func ParsePorcelainV2(data []byte) ([]FileEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	entries := make([]FileEntry, 0, bytes.Count(data, []byte{0}))
	reader := nulReader{data: data}
	for record, ok := reader.next(); ok; record, ok = reader.next() {
		entry, err := parsePorcelainRecord(record, &reader)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parsePorcelainRecord(record []byte, reader *nulReader) (FileEntry, error) {
	if len(record) < 2 || record[1] != ' ' {
		return FileEntry{}, fmt.Errorf("parse git status record %q", record)
	}
	switch record[0] {
	case '?':
		return simpleStatusEntry(record[2:], FileUntracked)
	case '!':
		return simpleStatusEntry(record[2:], FileIgnored)
	case '1':
		xy, _, path, err := statusFields(record, 9)
		return FileEntry{Path: string(path), State: stateFromXY(xy)}, err
	case '2':
		return parseRenameStatus(record, reader)
	case 'u':
		_, _, path, err := statusFields(record, 11)
		return FileEntry{Path: string(path), State: FileModified}, err
	default:
		return FileEntry{}, fmt.Errorf("parse unsupported git status record %q", record)
	}
}

func simpleStatusEntry(path []byte, state FileState) (FileEntry, error) {
	if len(path) == 0 {
		return FileEntry{}, fmt.Errorf("parse empty git status path")
	}
	return FileEntry{Path: string(path), State: state}, nil
}

func parseRenameStatus(record []byte, reader *nulReader) (FileEntry, error) {
	_, score, path, err := statusFields(record, 10)
	if err != nil {
		return FileEntry{}, err
	}
	previousPath, ok := reader.next()
	if !ok || len(previousPath) == 0 {
		return FileEntry{}, fmt.Errorf("parse truncated git status rename %q", record)
	}
	entry := FileEntry{Path: string(path), PreviousPath: string(previousPath), State: FileRenamed}
	if len(score) > 0 && score[0] == 'C' {
		entry.State = FileAdded
		entry.PreviousPath = ""
	}
	return entry, nil
}

// statusFields returns the XY field, the field immediately before the path,
// and the opaque path without allocating a slice for every metadata field.
func statusFields(record []byte, count int) ([]byte, []byte, []byte, error) {
	fieldIndex := 0
	fieldStart := 0
	var xy, beforePath []byte
	for index, value := range record {
		if value != ' ' || fieldIndex >= count-1 {
			continue
		}
		field := record[fieldStart:index]
		if fieldIndex == 1 {
			xy = field
		}
		if fieldIndex == count-2 {
			beforePath = field
		}
		fieldIndex++
		fieldStart = index + 1
	}
	if fieldIndex != count-1 || fieldStart >= len(record) {
		return nil, nil, nil, fmt.Errorf("parse git status record %q", record)
	}
	return xy, beforePath, record[fieldStart:], nil
}

func stateFromXY(xy []byte) FileState {
	if bytes.IndexByte(xy, 'R') >= 0 {
		return FileRenamed
	}
	if bytes.IndexByte(xy, 'D') >= 0 {
		return FileDeleted
	}
	if bytes.IndexByte(xy, 'A') >= 0 {
		return FileAdded
	}
	if len(xy) == 2 && xy[0] == '.' && xy[1] == '.' {
		return FileUnchanged
	}
	return FileModified
}

// MergeFileEntries overlays status on the complete tracked identity set.
func MergeFileEntries(tracked []string, status []FileEntry) []FileEntry {
	byPath := make(map[string]FileEntry, len(tracked)+len(status))
	for _, path := range tracked {
		if path != "" {
			byPath[path] = FileEntry{Path: path, State: FileUnchanged}
		}
	}
	for _, entry := range status {
		if entry.Path == "" {
			continue
		}
		if entry.State == FileRenamed && entry.PreviousPath != "" && entry.PreviousPath != entry.Path {
			delete(byPath, entry.PreviousPath)
		}
		byPath[entry.Path] = entry
	}
	entries := make([]FileEntry, 0, len(byPath))
	for _, entry := range byPath {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	return entries
}
