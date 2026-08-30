package scratch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatePathsUseXDGAndCloneIdentity(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) {
		if key == "XDG_STATE_HOME" {
			return base, true
		}
		return "", false
	}
	one, err := StatePaths("/clone/one/.git", lookup)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := StatePaths("/clone/one/.git", lookup)
	two, _ := StatePaths("/clone/two/.git", lookup)
	if one != again || one == two || !strings.HasPrefix(one.Directory, filepath.Join(base, "reviewr", "v1", "scratch")) {
		t.Fatalf("paths one=%+v again=%+v two=%+v", one, again, two)
	}
	if strings.Contains(one.Directory, "/clone/one") {
		t.Fatalf("common directory leaked into path: %q", one.Directory)
	}
}

func TestStatePathsUseLinuxFallbackAndIgnoreRelativeXDG(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	paths, err := StatePaths("/clone/.git", func(key string) (string, bool) {
		switch key {
		case "XDG_STATE_HOME":
			return "relative", true
		case "HOME":
			return home, true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(paths.Directory, filepath.Join(home, ".local", "state")) {
		t.Fatalf("fallback path = %q", paths.Directory)
	}
}

func TestPrivateStorePermissionsAtomicReplacementAndContention(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) {
		if key == "XDG_STATE_HOME" {
			return base, true
		}
		return "", false
	}
	first := NewPrivateStore("/clone/.git", lookup)
	t.Cleanup(func() { _ = first.Close() })
	text, readOnly, err := first.Load()
	if err != nil || readOnly || text != "" {
		t.Fatalf("initial load = %q, readOnly %v, err %v", text, readOnly, err)
	}
	if err := first.Save("old"); err != nil {
		t.Fatal(err)
	}
	paths, _ := StatePaths("/clone/.git", lookup)
	assertMode(t, paths.Directory, 0o700)
	assertMode(t, paths.Note, 0o600)
	assertMode(t, paths.Lock, 0o600)
	if err := first.Save("new"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths.Note)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement = %q, %v", data, err)
	}
	matches, err := filepath.Glob(filepath.Join(paths.Directory, ".note-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %#v, %v", matches, err)
	}

	second := NewPrivateStore("/clone/.git", lookup)
	defer second.Close()
	text, readOnly, err = second.Load()
	if err != nil || !readOnly || text != "new" {
		t.Fatalf("contended load = %q, readOnly %v, err %v", text, readOnly, err)
	}
	if err := second.Save("bad"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("contended save error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	text, readOnly, err = second.Load()
	if err != nil || readOnly || text != "new" {
		t.Fatalf("lock retry = %q, readOnly %v, err %v", text, readOnly, err)
	}
}

func TestPrivateStoreReadAndWriteFailuresAreRecoverable(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	store := NewPrivateStore("/clone/.git", lookup)
	defer store.Close()
	if _, _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	paths, _ := StatePaths("/clone/.git", lookup)
	if err := os.Mkdir(paths.Note, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, readOnly, err := store.Load(); err == nil || readOnly {
		t.Fatalf("read failure = readOnly %v, err %v", readOnly, err)
	}
	if err := os.Remove(paths.Note); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("valid"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths.Note); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.Note, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("replacement"); err == nil {
		t.Fatal("rename over directory unexpectedly succeeded")
	}
	if info, err := os.Stat(paths.Note); err != nil || !info.IsDir() {
		t.Fatalf("failed replacement changed prior target: %+v, %v", info, err)
	}
}

func TestPrivateStoreFailedStageKeepsPriorValidNote(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny root")
	}
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	store := NewPrivateStore("/clone/.git", lookup)
	defer store.Close()
	if _, _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("prior valid"); err != nil {
		t.Fatal(err)
	}
	paths, _ := StatePaths("/clone/.git", lookup)
	if err := os.Chmod(paths.Directory, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(paths.Directory, 0o700) })
	if err := store.Save("must not replace"); err == nil {
		t.Fatal("save unexpectedly succeeded without directory write permission")
	}
	data, err := os.ReadFile(paths.Note)
	if err != nil || string(data) != "prior valid" {
		t.Fatalf("failed save left %q, %v", data, err)
	}
}

func TestPrivateStoreReportsInvalidUTF8WithoutBlockingText(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	store := NewPrivateStore("/clone/.git", lookup)
	defer store.Close()
	if _, _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	paths, _ := StatePaths("/clone/.git", lookup)
	if err := os.WriteFile(paths.Note, []byte{'a', 0xff, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}
	text, readOnly, err := store.Load()
	if !errors.Is(err, ErrInvalidUTF8) || readOnly || len(text) != 3 {
		t.Fatalf("invalid UTF-8 = %q, readOnly %v, err %v", text, readOnly, err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o, want %#o", path, got, want)
	}
}
