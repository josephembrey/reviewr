package repository

import (
	"math"
	"sort"
)

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
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// Changed reports whether the entry belongs to the Changed projection.
func (entry Entry) Changed() bool {
	return entry.State != FileUnchanged && entry.State != FileIgnored
}

// Snapshot is the single immutable source from which Files scopes are derived.
type Snapshot struct {
	entries []Entry
	summary ChangeSummary
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
	summary := ChangeSummary{}
	for _, entry := range byPath {
		result = append(result, entry)
		if entry.Changed() {
			summary.Files++
			summary.Additions = addCount(summary.Additions, entry.Additions)
			summary.Deletions = addCount(summary.Deletions, entry.Deletions)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Path < result[right].Path })
	return Snapshot{entries: result, summary: summary}
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

// Summary returns aggregate line statistics for changed entries.
func (snapshot Snapshot) Summary() ChangeSummary { return snapshot.summary }

func addCount(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}
