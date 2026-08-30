package repository

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStashRepositoryReadsCombinedFilesAndPreservesAllGitState(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "modified.txt", "old\n")
	writeFile(t, root, "deleted.txt", "delete\n")
	writeFile(t, root, "rename-old.txt", "rename\n")
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "base")
	writeFile(t, root, "modified.txt", "stored\nwith tail\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "mv", "rename-old.txt", "rename-new.txt")
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte{0, 9, 8}, 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "untracked path.txt", "stored untracked\n")
	runGit(t, root, "stash", "push", "-u", "-m", "repository reader")
	writeFile(t, root, "modified.txt", "dirty after stash\n")

	type stashStateSnapshot struct {
		gitState
		reflog []byte
	}
	snapshot := func() stashStateSnapshot {
		return stashStateSnapshot{
			gitState: captureGitState(t, root),
			reflog:   runGitBytes(t, root, "reflog", "show", "-z", "--format=%H%x00%gD%x00%gs%x00%ct", "refs/stash"),
		}
	}
	before := snapshot()
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	stashes, err := repo.ListStashes()
	if err != nil || len(stashes) != 1 {
		t.Fatalf("ListStashes() = (%#v, %v)", stashes, err)
	}
	stash := stashes[0]
	if stash.Selector != "stash@{0}" || stash.Message != "repository reader" || stash.Files < 5 || stash.Additions == 0 || stash.Deletions == 0 {
		t.Fatalf("stash metadata = %+v", stash)
	}
	files, err := repo.ListStashFiles(stash.Source)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]ChangedFile, len(files))
	for _, file := range files {
		byPath[file.Path] = file
	}
	if byPath["deleted.txt"].Kind != ChangeDeleted || byPath["rename-new.txt"].Kind != ChangeRenamed ||
		byPath["rename-new.txt"].PreviousPath != "rename-old.txt" || byPath["untracked path.txt"].Kind != ChangeUntracked || !byPath["binary.dat"].Binary {
		t.Fatalf("stash files = %#v", byPath)
	}
	modified := repo.ReadStashFile(stash.Source, byPath["modified.txt"])
	if modified.Old.Content != "old\n" || modified.New.Content != "stored\nwith tail\n" ||
		modified.Patch.Kind != FileReady || !strings.Contains(modified.Patch.Content, "+with tail") {
		t.Fatalf("modified document = %+v", modified)
	}
	deleted := repo.ReadStashFile(stash.Source, byPath["deleted.txt"])
	if deleted.Old.Content != "delete\n" || deleted.New.Kind != FileMissing {
		t.Fatalf("deleted document = %+v", deleted)
	}
	renamed := repo.ReadStashFile(stash.Source, byPath["rename-new.txt"])
	if renamed.Old.Content != "rename\n" || renamed.New.Content != "rename\n" {
		t.Fatalf("renamed document = %+v", renamed)
	}
	untracked := repo.ReadStashFile(stash.Source, byPath["untracked path.txt"])
	if untracked.Old.Kind != FileMissing || untracked.New.Content != "stored untracked\n" {
		t.Fatalf("untracked document = %+v", untracked)
	}
	binary := repo.ReadStashFile(stash.Source, byPath["binary.dat"])
	if binary.Old.Kind != FileBinary || binary.New.Kind != FileBinary {
		t.Fatalf("binary document = %+v", binary)
	}

	repo.maxBytes = 4
	tooLarge := repo.ReadStashFile(stash.Source, byPath["modified.txt"])
	if tooLarge.New.Kind != FileTooLarge || tooLarge.Patch.Kind != FileTooLarge {
		t.Fatalf("bounded stash document = %+v", tooLarge)
	}
	if after := snapshot(); !reflect.DeepEqual(after, before) {
		t.Fatalf("stash reads changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
}
