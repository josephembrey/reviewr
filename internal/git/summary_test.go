package git

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseNumstatHandlesFilesRenamesAndBinary(t *testing.T) {
	t.Parallel()
	data := []byte("3\t1\tplain.go\x00-\t-\timage.png\x004\t2\t\x00old name.go\x00new name.go\x00")
	want := map[string]changeStat{
		"plain.go":    {additions: 3, deletions: 1},
		"image.png":   {binary: true},
		"new name.go": {additions: 4, deletions: 2},
	}
	got, err := parseNumstatDetails(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNumstat() = %#v, want %#v", got, want)
	}
	if _, err := parseNumstatDetails([]byte("broken\x00")); err == nil {
		t.Fatal("parseNumstatDetails() accepted malformed output")
	}
}

func TestWorktreeSummaryCountsTrackedAndUntrackedChanges(t *testing.T) {
	root := initGitTestRepository(t)
	writeGitTestFile(t, root, ".gitignore", "ignored.txt\n")
	writeGitTestFile(t, root, "tracked.txt", "one\ntwo\n")
	runGitTest(t, root, "add", ".gitignore", "tracked.txt")
	runGitTest(t, root, "commit", "-q", "-m", "fixture")

	writeGitTestFile(t, root, "tracked.txt", "one\nchanged\nthree\n")
	writeGitTestFile(t, root, "untracked.txt", "new\nfile")
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	writeGitTestFile(t, root, "ignored.txt", "ignored\n")

	got, err := New().WorktreeSummary(root)
	if err != nil {
		t.Fatal(err)
	}
	want := ChangeSummary{Files: 3, Additions: 4, Deletions: 1}
	if got != want {
		t.Fatalf("WorktreeSummary() = %+v, want %+v", got, want)
	}
}

func TestWorktreeSummaryHandlesUnbornHead(t *testing.T) {
	root := initGitTestRepository(t)
	writeGitTestFile(t, root, "new.txt", "one\ntwo\n")
	got, err := New().WorktreeSummary(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := (ChangeSummary{Files: 1, Additions: 2}); got != want {
		t.Fatalf("unborn WorktreeSummary() = %+v, want %+v", got, want)
	}
}

func writeGitTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
