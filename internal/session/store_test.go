package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorktreeSessionRoundTripsNewestGenerationPrivately(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	common := filepath.Join(root, "repo", ".git")
	worktree := filepath.Join(root, "repo")
	if err := os.MkdirAll(common, 0o700); err != nil {
		t.Fatal(err)
	}
	store, initial, err := Open(filepath.Join(root, "state"), common, worktree)
	if err != nil || !reflect.DeepEqual(initial, State{}) {
		t.Fatalf("initial session = %+v, %v", initial, err)
	}
	want := State{
		Active:   "git",
		Controls: Controls{Git: "stashes", DiffHighlight: "background"},
		Layout:   Layout{NavigatorWidth: 37, Customized: true, Swapped: true},
		Files: Files{
			Place:      Place{Items: []string{"file:a.go", "file:b.go"}, Selected: 1, Focus: "reader", ReaderOffset: 12},
			ReaderPath: "b.go", Folds: map[string]Folds{"all": {Known: []string{"src"}, Collapsed: []string{"src"}}},
		},
	}
	if err := store.Save(2, want); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(1, State{Active: "files"}); err != nil {
		t.Fatal(err)
	}

	_, got, err := Open(filepath.Join(root, "state"), common, worktree)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened session = %#v, %v; want %#v", got, err, want)
	}
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %o, want 600", info.Mode().Perm())
	}
}

func TestSessionIdentityCanonicalizesSymlinksAndSeparatesWorktrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	common := filepath.Join(root, "common")
	worktree := filepath.Join(root, "worktree")
	other := filepath.Join(root, "other")
	for _, path := range []string{common, worktree, other} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "linked-worktree")
	if err := os.Symlink(worktree, link); err != nil {
		t.Fatal(err)
	}
	direct, _, err := Open(filepath.Join(root, "state"), common, worktree)
	if err != nil {
		t.Fatal(err)
	}
	linked, _, err := Open(filepath.Join(root, "state"), common, link)
	if err != nil {
		t.Fatal(err)
	}
	distinct, _, err := Open(filepath.Join(root, "state"), common, other)
	if err != nil {
		t.Fatal(err)
	}
	if direct.path != linked.path {
		t.Fatalf("symlink session paths differ: %q != %q", direct.path, linked.path)
	}
	if direct.path == distinct.path {
		t.Fatal("distinct worktrees shared a session path")
	}
}

func TestCorruptSessionCanBeRepairedByNextAuthoredSave(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	common := filepath.Join(root, "repo", ".git")
	worktree := filepath.Join(root, "repo")
	if err := os.MkdirAll(common, 0o700); err != nil {
		t.Fatal(err)
	}
	store, _, err := Open(filepath.Join(root, "state"), common, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _, err = Open(filepath.Join(root, "state"), common, worktree)
	if err == nil {
		t.Fatal("corrupt session loaded without an error")
	}
	want := State{Active: "notes"}
	if err := store.Save(1, want); err != nil {
		t.Fatal(err)
	}
	_, got, err := Open(filepath.Join(root, "state"), common, worktree)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("repaired session = %+v, %v", got, err)
	}
}
