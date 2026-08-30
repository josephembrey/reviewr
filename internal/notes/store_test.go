package notes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if one != again || one == two || !strings.HasPrefix(one.Directory, filepath.Join(base, "reviewr", "v1", "notes")) {
		t.Fatalf("paths one=%+v again=%+v two=%+v", one, again, two)
	}
	if strings.Contains(one.Directory, "/clone/one") {
		t.Fatalf("common directory leaked into path: %q", one.Directory)
	}
}

func TestWorktreeStatePathsPreserveProjectCompatibilityAndStayOpaque(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	commonDir := "/projects/example/.git"
	root := "/worktrees/example feature"

	project, err := StatePaths(commonDir, lookup)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := WorktreeStatePaths(commonDir, root, lookup)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := StatePaths(commonDir, lookup)
	otherRoot, _ := WorktreeStatePaths(commonDir, "/worktrees/other", lookup)
	otherProject, _ := WorktreeStatePaths("/projects/other/.git", root, lookup)

	if project != again || project.Note != filepath.Join(project.Directory, "note.txt") || project.Lock != filepath.Join(project.Directory, "edit.lock") {
		t.Fatalf("project compatibility changed: project=%+v again=%+v", project, again)
	}
	if filepath.Dir(filepath.Dir(worktree.Directory)) != project.Directory {
		t.Fatalf("worktree path %q is not below project path %q", worktree.Directory, project.Directory)
	}
	if worktree == otherRoot || worktree == otherProject {
		t.Fatalf("worktree identity was not keyed by both inputs: worktree=%+v otherRoot=%+v otherProject=%+v", worktree, otherRoot, otherProject)
	}
	if strings.Contains(worktree.Directory, commonDir) || strings.Contains(worktree.Directory, root) {
		t.Fatalf("private identities leaked into worktree path %q", worktree.Directory)
	}
	stores := NewStores(commonDir, root, lookup)
	t.Cleanup(func() { _ = stores.Close() })
	if !stores.HasWorktree() {
		t.Fatal("checkout did not construct its worktree store")
	}
}

func TestProjectAndWorktreeStoresHaveIndependentFilesAndLocks(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	commonDir := "/projects/example/.git"
	root := "/worktrees/example"

	owner := NewPrivateStore(commonDir, lookup)
	t.Cleanup(func() { _ = owner.Close() })
	if _, readOnly, err := owner.Load(); err != nil || readOnly {
		t.Fatalf("project owner Load() = readOnly %v, %v", readOnly, err)
	}
	if err := owner.Save("project note"); err != nil {
		t.Fatal(err)
	}

	stores := NewStores(commonDir, root, lookup)
	t.Cleanup(func() { _ = stores.Close() })
	projectText, projectReadOnly, projectErr := stores.Project.Load()
	worktreeText, worktreeReadOnly, worktreeErr := stores.Worktree.Load()
	if projectErr != nil || !projectReadOnly || projectText != "project note" {
		t.Fatalf("contended project = %q, readOnly %v, %v", projectText, projectReadOnly, projectErr)
	}
	if worktreeErr != nil || worktreeReadOnly || worktreeText != "" {
		t.Fatalf("independent worktree = %q, readOnly %v, %v", worktreeText, worktreeReadOnly, worktreeErr)
	}
	if err := stores.Worktree.Save("local note"); err != nil {
		t.Fatal(err)
	}
	if err := stores.Project.Save("must fail"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("contended project Save() = %v", err)
	}
	projectPaths, _ := StatePaths(commonDir, lookup)
	worktreePaths, _ := WorktreeStatePaths(commonDir, root, lookup)
	projectData, projectErr := os.ReadFile(projectPaths.Note)
	worktreeData, worktreeErr := os.ReadFile(worktreePaths.Note)
	if projectErr != nil || worktreeErr != nil || string(projectData) != "project note" || string(worktreeData) != "local note" {
		t.Fatalf("independent files = project %q (%v), worktree %q (%v)", projectData, projectErr, worktreeData, worktreeErr)
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

func TestPrivateStoreRejectsInvalidUTF8SaveAndSymlinkedNamespace(t *testing.T) {
	t.Parallel()

	t.Run("invalid UTF-8 save", func(t *testing.T) {
		base := t.TempDir()
		lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
		store := NewPrivateStore("/clone/.git", lookup)
		defer store.Close()
		if _, _, err := store.Load(); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(string([]byte{0xff})); !errors.Is(err, ErrInvalidUTF8) {
			t.Fatalf("invalid save error = %v", err)
		}
		paths, _ := StatePaths("/clone/.git", lookup)
		if _, err := os.Stat(paths.Note); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid save created a note: %v", err)
		}
	})

	t.Run("symlinked namespace", func(t *testing.T) {
		base := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(base, "reviewr")); err != nil {
			t.Fatal(err)
		}
		lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
		store := NewPrivateStore("/clone/.git", lookup)
		defer store.Close()
		if _, readOnly, err := store.Load(); err == nil || !readOnly {
			t.Fatalf("symlinked load = readOnly %v, %v", readOnly, err)
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("notes state escaped through symlink: %v, %v", entries, err)
		}
	})
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

func TestLegacyImportPreservesProjectAndWorktreeSources(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	commonDir := "/projects/import/.git"
	worktreeRoot := "/worktrees/import"
	projectPaths, err := StatePaths(commonDir, lookup)
	if err != nil {
		t.Fatal(err)
	}
	worktreePaths, err := WorktreeStatePaths(commonDir, worktreeRoot, lookup)
	if err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{
		projectPaths.legacyNote:  "project legacy",
		worktreePaths.legacyNote: "worktree legacy",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	stores := NewStores(commonDir, worktreeRoot, lookup)
	t.Cleanup(func() { _ = stores.Close() })
	project, projectReadOnly, projectErr := stores.Project.Load()
	worktree, worktreeReadOnly, worktreeErr := stores.Worktree.Load()
	if projectErr != nil || worktreeErr != nil || projectReadOnly || worktreeReadOnly || project != "project legacy" || worktree != "worktree legacy" {
		t.Fatalf("import = project %q/%v/%v worktree %q/%v/%v", project, projectReadOnly, projectErr, worktree, worktreeReadOnly, worktreeErr)
	}
	for source, want := range map[string]string{projectPaths.legacyNote: "project legacy", worktreePaths.legacyNote: "worktree legacy"} {
		data, err := os.ReadFile(source)
		if err != nil || string(data) != want {
			t.Fatalf("legacy source %q = %q, %v", source, data, err)
		}
	}
	for _, target := range []Paths{projectPaths, worktreePaths} {
		assertMode(t, target.Directory, 0o700)
		assertMode(t, target.Note, 0o600)
		assertMode(t, target.Lock, 0o600)
	}
}

func TestLegacyImportNeverCrossesProjectAndWorktreeScopes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		legacyScope  Scope
		wantProject  string
		wantWorktree string
	}{
		{name: "project only", legacyScope: Project, wantProject: "project legacy"},
		{name: "worktree only", legacyScope: Worktree, wantWorktree: "worktree legacy"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
			commonDir := "/projects/scope-import/.git"
			worktreeRoot := "/worktrees/scope-import"
			projectPaths, err := StatePaths(commonDir, lookup)
			if err != nil {
				t.Fatal(err)
			}
			worktreePaths, err := WorktreeStatePaths(commonDir, worktreeRoot, lookup)
			if err != nil {
				t.Fatal(err)
			}
			legacyPath := projectPaths.legacyNote
			legacyText := "project legacy"
			if test.legacyScope == Worktree {
				legacyPath = worktreePaths.legacyNote
				legacyText = "worktree legacy"
			}
			if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(legacyPath, []byte(legacyText), 0o600); err != nil {
				t.Fatal(err)
			}

			stores := NewStores(commonDir, worktreeRoot, lookup)
			defer stores.Close()
			project, _, projectErr := stores.Project.Load()
			worktree, _, worktreeErr := stores.Worktree.Load()
			if projectErr != nil || worktreeErr != nil || project != test.wantProject || worktree != test.wantWorktree {
				t.Fatalf("scope import = project %q/%v worktree %q/%v", project, projectErr, worktree, worktreeErr)
			}
		})
	}
}

func TestLegacyImportNeverReplacesAnyNotesTarget(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		target func(string) error
		want   string
	}{
		{name: "empty", target: func(path string) error { return os.WriteFile(path, nil, 0o600) }},
		{name: "newer", target: func(path string) error { return os.WriteFile(path, []byte("newer Notes"), 0o600) }, want: "newer Notes"},
		{name: "invalid UTF-8", target: func(path string) error { return os.WriteFile(path, []byte{0xff}, 0o600) }, want: string([]byte{0xff})},
		{name: "read error", target: func(path string) error { return os.Mkdir(path, 0o700) }},
		{name: "dangling symlink", target: func(path string) error { return os.Symlink("missing-target", path) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
			paths, err := StatePaths("/projects/target-wins/.git", lookup)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(paths.legacyNote), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.legacyNote, []byte("legacy must lose"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(paths.Directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := test.target(paths.Note); err != nil {
				t.Fatal(err)
			}
			store := NewPrivateStore("/projects/target-wins/.git", lookup)
			defer store.Close()
			text, _, loadErr := store.Load()
			if text != test.want {
				t.Fatalf("target text = %q, want %q (err %v)", text, test.want, loadErr)
			}
			if test.name == "invalid UTF-8" && !errors.Is(loadErr, ErrInvalidUTF8) {
				t.Fatalf("invalid target error = %v", loadErr)
			}
			if (test.name == "read error" || test.name == "dangling symlink") && loadErr == nil {
				t.Fatal("target read error was replaced by legacy data")
			}
		})
	}
}

func TestLegacyImportRejectsInvalidUTF8AndIsSafeUnderContention(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	lookup := func(key string) (string, bool) { return base, key == "XDG_STATE_HOME" }
	commonDir := "/projects/concurrent-import/.git"
	paths, err := StatePaths(commonDir, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.legacyNote), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.legacyNote, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := NewPrivateStore(commonDir, lookup)
	if _, _, err := invalid.Load(); !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("invalid legacy load = %v", err)
	}
	_ = invalid.Close()
	if _, err := os.Lstat(paths.Note); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid legacy source created target: %v", err)
	}
	if err := os.WriteFile(paths.legacyNote, []byte("concurrent legacy"), 0o600); err != nil {
		t.Fatal(err)
	}

	stores := []*PrivateStore{NewPrivateStore(commonDir, lookup), NewPrivateStore(commonDir, lookup)}
	defer stores[0].Close()
	defer stores[1].Close()
	type result struct {
		text     string
		readOnly bool
		err      error
	}
	results := make([]result, len(stores))
	var wait sync.WaitGroup
	for index, store := range stores {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index].text, results[index].readOnly, results[index].err = store.Load()
		}()
	}
	wait.Wait()
	data, err := os.ReadFile(paths.Note)
	if err != nil || string(data) != "concurrent legacy" {
		t.Fatalf("concurrent target = %q, %v; results %+v", data, err, results)
	}
	owners := 0
	for _, result := range results {
		if result.err != nil {
			t.Fatalf("concurrent load error: %+v", results)
		}
		if result.text != "concurrent legacy" {
			t.Fatalf("concurrent session missed legacy text: %+v", results)
		}
		if !result.readOnly {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("concurrent lock owners = %d, want 1: %+v", owners, results)
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
