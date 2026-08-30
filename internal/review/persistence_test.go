package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestRepositoryIDCanonicalKeyAndWorktreeIsolation(t *testing.T) {
	root := t.TempDir()
	common := filepath.Join(root, "common.git")
	worktreeA := filepath.Join(root, "worktree-a")
	worktreeB := filepath.Join(root, "worktree-b")
	for _, path := range []string{common, worktreeA, worktreeB} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	commonLink := filepath.Join(root, "common-link")
	worktreeLink := filepath.Join(root, "worktree-link")
	if err := os.Symlink(common, commonLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(worktreeA, worktreeLink); err != nil {
		t.Fatal(err)
	}

	direct, err := ResolveRepositoryID(worktreeA, common)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveRepositoryID(worktreeLink, commonLink)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ResolveRepositoryID(worktreeB, common)
	if err != nil {
		t.Fatal(err)
	}
	if direct != linked || direct.FileKey() != linked.FileKey() {
		t.Fatalf("canonical ids differ: %+v %+v", direct, linked)
	}
	if direct.FileKey() == other.FileKey() {
		t.Fatal("distinct worktrees shared a state key")
	}
	if len(direct.FileKey()) != 64 {
		t.Fatalf("file key length = %d", len(direct.FileKey()))
	}
}

func TestRepositoryIDRejectsMissingIdentityPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, test := range []struct {
		worktree string
		common   string
	}{
		{worktree: "", common: root},
		{worktree: root, common: ""},
	} {
		if _, err := ResolveRepositoryID(test.worktree, test.common); err == nil {
			t.Fatalf("ResolveRepositoryID(%q, %q) accepted a missing identity", test.worktree, test.common)
		}
	}
}

func TestStoreMissingRoundTripRestartAndPrivateAtomicReplacement(t *testing.T) {
	repository := testRepositoryID(t)
	root := t.TempDir()
	ledger, store, warning := OpenStore(repository, root)
	if !strings.Contains(warning, "missing") || !store.Writable() {
		t.Fatalf("warning/writable = %q/%v", warning, store.Writable())
	}
	edge := comparison("uncommitted", endpoint("a", "old"), endpoint("a", "new"))
	delta := Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}, Retained: retained("new")}
	changed, err := store.Apply(&ledger, delta)
	if err != nil || !changed {
		t.Fatalf("apply = %v, %v", changed, err)
	}
	path := store.Path()
	if !strings.HasPrefix(path, root+string(filepath.Separator)) || strings.HasPrefix(path, repository.Worktree+string(filepath.Separator)) {
		t.Fatalf("state path %q is not application-private", path)
	}
	if runtime.GOOS != "windows" {
		if mode := fileMode(t, path); mode != 0o600 {
			t.Fatalf("state mode = %o", mode)
		}
		if mode := fileMode(t, filepath.Dir(path)); mode != 0o700 {
			t.Fatalf("state dir mode = %o", mode)
		}
		if mode := fileMode(t, strings.TrimSuffix(path, ".json")+".lock"); mode != 0o600 {
			t.Fatalf("lock mode = %o", mode)
		}
	}

	firstBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second := comparison("uncommitted", endpoint("b", "old"), endpoint("b", "new"))
	if _, err := store.Apply(&ledger, Delta{Kind: MarkDelta, Comparison: second, Bounds: Bounds{Old: second.Old, New: second.New}, Retained: retained("new")}); err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) == string(secondBytes) || strings.Contains(string(secondBytes), string(firstBytes)) {
		t.Fatal("state was not replaced atomically")
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".review-state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary state remains: %v, %v", matches, err)
	}

	restarted, _, warning := OpenStore(repository, root)
	if warning != "" || len(restarted.ReceiptData) != 2 || restarted.Assess(edge).State != Reviewed {
		t.Fatalf("restart = warning %q ledger %+v", warning, restarted)
	}
}

func TestStoreRefusesSymlinkedStateNamespace(t *testing.T) {
	t.Parallel()
	repository := testRepositoryID(t)
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "reviews")); err != nil {
		t.Fatal(err)
	}
	ledger, store, _ := OpenStore(repository, root)
	edge := comparison("branch", endpoint("a", "old"), endpoint("a", "new"))
	delta := Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}}
	changed, err := store.Apply(&ledger, delta)
	if !changed || err == nil || ledger.Assess(edge).State != Reviewed {
		t.Fatalf("symlinked apply = %v, %v, %+v", changed, err, ledger)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("review state escaped through symlink: %v, %v", entries, err)
	}
}

func TestStoreRejectsCorruptNewerInvalidAndMismatchedStateWithoutLosingLocalAction(t *testing.T) {
	repository := testRepositoryID(t)
	edge := comparison("branch", endpoint("a", "old"), endpoint("a", "new"))
	cases := []struct {
		name    string
		content func(RepositoryID) string
		warning string
	}{
		{"corrupt", func(RepositoryID) string { return "not json" }, "corrupt"},
		{"newer", func(id RepositoryID) string {
			return fmt.Sprintf(`{"version":%d,"repository":%s,"ledger":{}}`, StateVersion+1, mustJSON(t, id))
		}, "newer"},
		{"invalid older", func(id RepositoryID) string {
			return fmt.Sprintf(`{"version":0,"repository":%s,"ledger":{}}`, mustJSON(t, id))
		}, "corrupt"},
		{"identity", func(RepositoryID) string {
			return `{"version":1,"repository":{"common_git_dir":"other","worktree":"other"},"ledger":{"receipts":[],"next_sequence":0}}`
		}, "identity"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			_, initial, _ := OpenStore(repository, root)
			path := initial.Path()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			content := test.content(repository)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			ledger, store, warning := OpenStore(repository, root)
			if !strings.Contains(warning, test.warning) || store.Writable() || len(ledger.ReceiptData) != 0 {
				t.Fatalf("load = warning %q writable %v ledger %+v", warning, store.Writable(), ledger)
			}
			changed, err := store.Apply(&ledger, Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}, Retained: retained("new")})
			if !changed || err == nil || ledger.Assess(edge).State != Reviewed {
				t.Fatalf("local recovery = changed %v err %v ledger %+v", changed, err, ledger)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != content {
				t.Fatalf("suspect state overwritten: %q, %v", got, err)
			}
		})
	}
}

func TestLockedDeltaReplayMergesConcurrentMarksAndStaleClears(t *testing.T) {
	repository := testRepositoryID(t)
	root := t.TempDir()
	edges := make([]FileComparison, 4)
	for index := range edges {
		edges[index] = comparison("branch", endpoint(fmt.Sprintf("%c", 'a'+index), "old"), endpoint(fmt.Sprintf("%c", 'a'+index), "new"))
	}
	var wait sync.WaitGroup
	start := make(chan struct{})
	errors := make(chan error, len(edges))
	for _, edge := range edges {
		edge := edge
		wait.Add(1)
		go func() {
			defer wait.Done()
			ledger, store, _ := OpenStore(repository, root)
			<-start
			_, err := store.Apply(&ledger, Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}, Retained: retained("new")})
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	current, _, warning := OpenStore(repository, root)
	if warning != "" || len(current.ReceiptData) != len(edges) {
		t.Fatalf("merged marks = warning %q receipts %d", warning, len(current.ReceiptData))
	}

	staleA, storeA, _ := OpenStore(repository, root)
	staleB, storeB, _ := OpenStore(repository, root)
	if _, err := storeA.Apply(&staleA, Delta{Kind: ClearDelta, Comparison: edges[0]}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeB.Apply(&staleB, Delta{Kind: ClearDelta, Comparison: edges[1]}); err != nil {
		t.Fatal(err)
	}
	final, _, _ := OpenStore(repository, root)
	if final.Assess(edges[0]).State == Reviewed || final.Assess(edges[1]).State == Reviewed || len(final.ReceiptData) != 2 {
		t.Fatalf("stale clears resurrected state: %+v", final.ReceiptData)
	}
}

func TestPersistenceFailurePreservesValidInMemoryReceipt(t *testing.T) {
	repository := testRepositoryID(t)
	edge := comparison("branch", endpoint("a", "old"), endpoint("a", "new"))
	delta := Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}, Retained: retained("new")}
	for _, stage := range []string{"encode", "write", "sync", "rename"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			ledger, store, _ := OpenStore(repository, root)
			store.replace = func(string, RepositoryID, Ledger) error { return fmt.Errorf("injected %s failure", stage) }
			changed, err := store.Apply(&ledger, delta)
			if !changed || err == nil || ledger.Assess(edge).State != Reviewed {
				t.Fatalf("failure = changed %v err %v ledger %+v", changed, err, ledger)
			}
			if _, statErr := os.Stat(store.Path()); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("failed replacement left state: %v", statErr)
			}
		})
	}

	t.Run("directory", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "state-root")
		ledger, store, _ := OpenStore(repository, root)
		if err := os.WriteFile(root, []byte("block"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Apply(&ledger, delta); err == nil || ledger.Assess(edge).State != Reviewed {
			t.Fatalf("directory apply = err %v ledger %+v", err, ledger)
		}
	})

	t.Run("lock", func(t *testing.T) {
		root := t.TempDir()
		ledger, store, _ := OpenStore(repository, root)
		if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
			t.Fatal(err)
		}
		lockPath := strings.TrimSuffix(store.Path(), ".json") + ".lock"
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Apply(&ledger, delta); err == nil || ledger.Assess(edge).State != Reviewed {
			t.Fatalf("lock apply = err %v ledger %+v", err, ledger)
		}
	})

	t.Run("locked reload", func(t *testing.T) {
		root := t.TempDir()
		ledger, store, _ := OpenStore(repository, root)
		if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path(), []byte("foreign corruption"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Apply(&ledger, delta); err == nil || ledger.Assess(edge).State != Reviewed {
			t.Fatalf("reload apply = err %v ledger %+v", err, ledger)
		}
		content, err := os.ReadFile(store.Path())
		if err != nil || string(content) != "foreign corruption" {
			t.Fatalf("foreign state changed: %q, %v", content, err)
		}
	})

	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	blockedLedger, blocked, warning := OpenStore(repository, blocker)
	if !strings.Contains(warning, "unreadable") {
		t.Fatalf("blocked warning = %q", warning)
	}
	if _, err := blocked.Apply(&blockedLedger, delta); err == nil || blockedLedger.Assess(edge).State != Reviewed {
		t.Fatalf("blocked apply = err %v ledger %+v", err, blockedLedger)
	}
}

func TestReviewPersistenceMakesNoRepositoryOrGitWrites(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "a.go")
	runGit(t, repo, "-c", "user.name=Reviewr", "-c", "user.email=reviewr@example.invalid", "commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	common := strings.TrimSpace(runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	repository, err := ResolveRepositoryID(repo, common)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotRepository(t, repo)

	ledger, store, _ := OpenStore(repository, t.TempDir())
	edge := comparison("uncommitted", endpoint("a.go", "base\n"), endpoint("a.go", "changed\n"))
	if _, err := store.Apply(&ledger, Delta{Kind: MarkDelta, Comparison: edge, Bounds: Bounds{Old: edge.Old, New: edge.New}, Retained: retained("changed\n")}); err != nil {
		t.Fatal(err)
	}
	restarted, _, _ := OpenStore(repository, filepath.Dir(filepath.Dir(store.Path())))
	if restarted.Assess(edge).State != Reviewed {
		t.Fatal("restart did not recover receipt")
	}
	after := snapshotRepository(t, repo)
	if fmt.Sprintf("%#v", before) != fmt.Sprintf("%#v", after) {
		t.Fatalf("repository changed:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func testRepositoryID(t *testing.T) RepositoryID {
	t.Helper()
	root := t.TempDir()
	common := filepath.Join(root, "common.git")
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(common, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	id, err := ResolveRepositoryID(worktree, common)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

type repositorySnapshot struct {
	Status  string
	Head    string
	Refs    string
	Index   string
	Objects []string
}

func snapshotRepository(t *testing.T, root string) repositorySnapshot {
	t.Helper()
	index, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	objectsRoot := filepath.Join(root, ".git", "objects")
	var objects []string
	err = filepath.WalkDir(objectsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, _ := filepath.Rel(objectsRoot, path)
		objects = append(objects, relative+":"+ContentIdentity(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(objects)
	return repositorySnapshot{
		Status:  runGit(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all"),
		Head:    runGit(t, root, "rev-parse", "HEAD"),
		Refs:    runGit(t, root, "show-ref"),
		Index:   ContentIdentity(index),
		Objects: objects,
	}
}
