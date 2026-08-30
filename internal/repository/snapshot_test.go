package repository

import (
	"reflect"
	"testing"
)

func TestSnapshotDerivesAllAndChangedFromOneEntrySet(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Path: "added.go", State: FileAdded},
		{Path: "deleted.go", State: FileDeleted},
		{Path: "ignored.go", State: FileIgnored},
		{Path: "modified.go", State: FileModified},
		{Path: "renamed.go", PreviousPath: "old.go", State: FileRenamed},
		{Path: "unchanged.go", State: FileUnchanged},
		{Path: "untracked.go", State: FileUntracked},
	}
	snapshot := NewSnapshot(entries)
	if got := snapshot.All(); !reflect.DeepEqual(got, entries) {
		t.Fatalf("All() = %#v, want %#v", got, entries)
	}
	wantChanged := []Entry{entries[0], entries[1], entries[3], entries[4], entries[6]}
	if got := snapshot.Changed(); !reflect.DeepEqual(got, wantChanged) {
		t.Fatalf("Changed() = %#v, want %#v", got, wantChanged)
	}

	all := snapshot.All()
	all[0].Path = "mutated"
	if snapshot.All()[0].Path != entries[0].Path {
		t.Fatal("All() exposed mutable snapshot storage")
	}
}
