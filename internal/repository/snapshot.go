package repository

import "sort"

// FileState is explicit Git metadata for one repository-relative path.
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

// Entry is the typed repository/app file identity. Path is the current stable
// repository-relative identity; PreviousPath records an explicit Git rename.
type Entry struct {
	Path         string
	PreviousPath string
	State        FileState
}

// Changed reports whether the entry belongs to the Changed projection.
func (entry Entry) Changed() bool {
	return entry.State != FileUnchanged && entry.State != FileIgnored
}

// Snapshot is the single immutable source from which Files scopes are derived.
type Snapshot struct {
	entries []Entry
}

// NewSnapshot copies, deduplicates, and sorts typed entries by current path.
func NewSnapshot(entries []Entry) Snapshot {
	byPath := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		if entry.Path != "" {
			byPath[entry.Path] = entry
		}
	}
	result := make([]Entry, 0, len(byPath))
	for _, entry := range byPath {
		result = append(result, entry)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Path < result[right].Path })
	return Snapshot{entries: result}
}

// All returns tracked, untracked, and ignored entries.
func (snapshot Snapshot) All() []Entry {
	return append([]Entry(nil), snapshot.entries...)
}

// Changed returns actual changes and untracked paths, excluding unchanged and
// ignored-only entries.
func (snapshot Snapshot) Changed() []Entry {
	entries := make([]Entry, 0, len(snapshot.entries))
	for _, entry := range snapshot.entries {
		if entry.Changed() {
			entries = append(entries, entry)
		}
	}
	return entries
}
