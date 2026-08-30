package appstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalPathRejectsEmptyAndResolvesSymlink(t *testing.T) {
	t.Parallel()
	if _, err := CanonicalPath(""); err == nil {
		t.Fatal("empty identity path resolved to the working directory")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalPath(link)
	if err != nil || got != target {
		t.Fatalf("canonical path = %q, %v; want %q", got, err, target)
	}
}

func TestDefaultRootHonorsAbsoluteXDGStateHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	got, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "reviewr"); got != want {
		t.Fatalf("state root = %q, want %q", got, want)
	}
}

func TestEnsurePrivateSubdirectoryRejectsEscapeAndSymlink(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(base, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateSubdirectory(base, filepath.Join(base, "linked", "state")); err == nil {
		t.Fatal("private state followed a descendant symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside state created through symlink: %v", err)
	}
	if err := EnsurePrivateSubdirectory(base, filepath.Join(base, "..", "outside")); err == nil {
		t.Fatal("private state accepted a lexical escape")
	}
	directory := filepath.Join(base, "reviewr", "sessions")
	if err := EnsurePrivateSubdirectory(base, directory); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(base, "reviewr"), directory} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("private directory %q mode = %v", path, info.Mode().Perm())
		}
	}
}

func TestReplaceFileIsPrivateAtomicReplacement(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "state.json")
	if err := ReplaceFile(path, ".state-*.tmp", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(path, ".state-*.tmp", []byte("second")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "second" {
		t.Fatalf("replacement = %q, %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("replacement mode = %v", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files remain: %v, %v", matches, err)
	}
}
