package git

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadTreeEntriesKeepsLiteralHostilePathsAndMissingEntries(t *testing.T) {
	root := initGitTestRepository(t)
	paths := []string{"plain.txt", "space name.txt", "line\nbreak.txt", ":(glob)*.txt"}
	for _, path := range paths {
		writeGitFixture(t, root, path, path+"\n")
	}
	runGitTest(t, root, "add", "--", ".")
	runGitTest(t, root, "commit", "-qm", "tree fixture")
	oid := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))

	entries, err := New().ReadTreeEntries(root, oid, append(paths, "missing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(paths) {
		t.Fatalf("ReadTreeEntries() = %#v", entries)
	}
	for _, path := range paths {
		entry, exists := entries[path]
		if !exists || entry.Path != path || entry.Mode != 0o100644 || entry.Type != "blob" || !validObjectID(entry.OID) {
			t.Fatalf("ReadTreeEntries()[%q] = %+v, %v", path, entry, exists)
		}
		single, singleExists, singleErr := New().ReadTreeEntry(root, oid, path)
		if singleErr != nil || !singleExists || !reflect.DeepEqual(single, entry) {
			t.Fatalf("ReadTreeEntry(%q) = (%+v, %v, %v), want %+v", path, single, singleExists, singleErr, entry)
		}
	}
	if _, exists := entries["missing.txt"]; exists {
		t.Fatalf("missing path was returned: %#v", entries)
	}
}

func TestTreePathspecChunkBoundsArguments(t *testing.T) {
	paths := []string{strings.Repeat("a", maxTreePathspecBytes), "b", "c"}
	if end := treePathspecChunk(paths, 0); end != 1 {
		t.Fatalf("first treePathspecChunk() = %d, want 1", end)
	}
	if end := treePathspecChunk(paths, 1); end != len(paths) {
		t.Fatalf("second treePathspecChunk() = %d, want %d", end, len(paths))
	}
}

func TestParseTreeEntriesRejectsMalformedRecords(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("100644 blob oid missing-tab\x00"),
		[]byte("100644 blob\tpath\x00"),
		[]byte("not-octal blob aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\tpath\x00"),
		[]byte("\x00"),
	} {
		if err := parseTreeEntries(data, make(map[string]TreeEntry)); err == nil {
			t.Fatalf("parseTreeEntries(%q) accepted malformed data", data)
		}
	}
}
