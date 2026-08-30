package git

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePorcelainV2KeepsHostilePathsAndRenamePairs(t *testing.T) {
	t.Parallel()
	data := []byte(
		"1 .M N... 100644 100644 100644 aaaaaaa bbbbbbb modified name.go\x00" +
			"1 D. N... 100644 000000 000000 aaaaaaa 0000000 deleted\nname.go\x00" +
			"2 R. N... 100644 100644 100644 aaaaaaa bbbbbbb R100 new\tname.go\x00old\nname.go\x00" +
			"u UU N... 100644 100644 100644 100644 aaaaaaa bbbbbbb ccccccc conflict.go\x00" +
			"? :(literal)*?.go\x00" +
			"! ignored dir/日本語.txt\x00",
	)
	want := []FileEntry{
		{Path: "modified name.go", State: FileModified},
		{Path: "deleted\nname.go", State: FileDeleted},
		{Path: "new\tname.go", PreviousPath: "old\nname.go", State: FileRenamed},
		{Path: "conflict.go", State: FileModified},
		{Path: ":(literal)*?.go", State: FileUntracked},
		{Path: "ignored dir/日本語.txt", State: FileIgnored},
	}
	got, err := ParsePorcelainV2(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePorcelainV2() = %#v, want %#v", got, want)
	}
}

func TestParsePorcelainV2RejectsMalformedRecords(t *testing.T) {
	t.Parallel()
	for _, data := range [][]byte{
		[]byte("human output\x00"),
		[]byte("? \x00"),
		[]byte("1 .M too-short\x00"),
		[]byte("2 R. N... 100644 100644 100644 a b R100 new.go\x00"),
	} {
		if entries, err := ParsePorcelainV2(data); err == nil {
			t.Fatalf("ParsePorcelainV2(%q) = %#v, want error", data, entries)
		}
	}
}

func TestMergeFileEntriesReplacesRenameSourceAndSorts(t *testing.T) {
	t.Parallel()
	tracked := []string{"z.go", "old.go", "same.go"}
	status := []FileEntry{
		{Path: "new.go", PreviousPath: "old.go", State: FileRenamed},
		{Path: "same.go", State: FileModified},
		{Path: "ignored.go", State: FileIgnored},
	}
	want := []FileEntry{
		{Path: "ignored.go", State: FileIgnored},
		{Path: "new.go", PreviousPath: "old.go", State: FileRenamed},
		{Path: "same.go", State: FileModified},
		{Path: "z.go", State: FileUnchanged},
	}
	if got := MergeFileEntries(tracked, status); !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeFileEntries() = %#v, want %#v", got, want)
	}
}

func TestSnapshotIncludesAllStatesAndIgnoredFiles(t *testing.T) {
	root := initGitTestRepository(t)
	write := func(path, content string) {
		t.Helper()
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "ignored/\n")
	write("unchanged.go", "same\n")
	write("modified.go", "before\n")
	write("deleted.go", "deleted\n")
	write("old.go", "renamed\n")
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "commit", "-q", "-m", "fixture")

	write("modified.go", "after\n")
	if err := os.Remove(filepath.Join(root, "deleted.go")); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, root, "mv", "old.go", "renamed.go")
	write("added.go", "added\n")
	runGitTest(t, root, "add", "added.go")
	write("untracked.go", "untracked\n")
	write("ignored/nested.txt", "ignored\n")

	entries, err := New().Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	states := make(map[string]FileEntry, len(entries))
	for _, entry := range entries {
		states[entry.Path] = entry
	}
	want := map[string]FileState{
		".gitignore":         FileUnchanged,
		"added.go":           FileAdded,
		"deleted.go":         FileDeleted,
		"ignored/nested.txt": FileIgnored,
		"modified.go":        FileModified,
		"renamed.go":         FileRenamed,
		"unchanged.go":       FileUnchanged,
		"untracked.go":       FileUntracked,
	}
	for path, state := range want {
		entry, ok := states[path]
		if !ok || entry.State != state {
			t.Fatalf("Snapshot()[%q] = %+v, %v; want state %v; all=%#v", path, entry, ok, state, entries)
		}
	}
	if states["renamed.go"].PreviousPath != "old.go" {
		t.Fatalf("rename = %+v, want old.go relation", states["renamed.go"])
	}
	if _, exists := states["old.go"]; exists {
		t.Fatalf("snapshot retained rename source: %#v", entries)
	}
	stats := map[string][2]uint64{
		"added.go":     {1, 0},
		"deleted.go":   {0, 1},
		"modified.go":  {1, 1},
		"renamed.go":   {0, 0},
		"untracked.go": {1, 0},
	}
	for path, want := range stats {
		entry := states[path]
		if got := [2]uint64{entry.Additions, entry.Deletions}; got != want || entry.Binary {
			t.Fatalf("Snapshot()[%q] stats = %v binary=%v, want %v", path, got, entry.Binary, want)
		}
	}
}
