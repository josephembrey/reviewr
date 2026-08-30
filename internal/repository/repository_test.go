package repository

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
	"github.com/josephembrey/reviewr/internal/scratch"
)

func TestOpenResolvesWorktreeRoot(t *testing.T) {
	root := initRepository(t)
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("explicit nested path", func(t *testing.T) {
		repo, err := Open(nested)
		if err != nil {
			t.Fatal(err)
		}
		if repo.Root() != root {
			t.Fatalf("Root() = %q, want %q", repo.Root(), root)
		}
	})

	t.Run("current directory", func(t *testing.T) {
		t.Chdir(nested)
		repo, err := Open("")
		if err != nil {
			t.Fatal(err)
		}
		if repo.Root() != root {
			t.Fatalf("Root() = %q, want %q", repo.Root(), root)
		}
	})
}

func TestOpenRejectsNonRepository(t *testing.T) {
	t.Parallel()
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("Open() succeeded outside a Git repository")
	}
}

func TestCommonDirSharesLinkedWorktreesAndIsolatesClones(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "tracked\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")

	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, root, "worktree", "add", "-q", "-b", "linked-test", linked)
	t.Cleanup(func() { runGit(t, root, "worktree", "remove", "--force", linked) })
	clone := filepath.Join(t.TempDir(), "clone")
	command := exec.Command("git", "clone", "-q", root, clone)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v: %s", err, bytes.TrimSpace(out))
	}

	openCommon := func(path string) string {
		t.Helper()
		repo, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		common, err := repo.CommonDir()
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(common) {
			t.Fatalf("common directory is not absolute: %q", common)
		}
		return common
	}
	mainCommon := openCommon(root)
	if linkedCommon := openCommon(linked); linkedCommon != mainCommon {
		t.Fatalf("linked common directory = %q, want %q", linkedCommon, mainCommon)
	}
	if cloneCommon := openCommon(clone); cloneCommon == mainCommon {
		t.Fatalf("separate clone reused common directory %q", cloneCommon)
	}
}

func TestReadFileClassifications(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(path string, data []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	write("ready.txt", []byte("hello"), 0o644)
	write("empty.txt", nil, 0o644)
	write("binary.dat", []byte{'a', 0, 'b'}, 0o644)
	write("large.txt", []byte("123456789"), 0o644)
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ready.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeFile(t, outside, "secret.txt", "secret")
	if err := os.Symlink(outside, filepath.Join(root, "escape-dir")); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{root: root, git: gitadapter.New(), maxBytes: 8}
	tests := []struct {
		name    string
		path    string
		kind    FileKind
		content string
		size    int64
		symlink bool
		wantErr bool
	}{
		{name: "ready", path: "ready.txt", kind: FileReady, content: "hello", size: 5},
		{name: "empty", path: "empty.txt", kind: FileReady, content: "", size: 0},
		{name: "binary", path: "binary.dat", kind: FileBinary, size: 3},
		{name: "too large", path: "large.txt", kind: FileTooLarge, size: 9},
		{name: "missing", path: "missing.txt", kind: FileMissing, wantErr: true},
		{name: "directory", path: "directory", kind: FileUnreadable, wantErr: true},
		{name: "traversal", path: "../outside", kind: FileUnreadable, wantErr: true},
		{name: "absolute", path: filepath.Join(root, "ready.txt"), kind: FileUnreadable, wantErr: true},
		{name: "parent symlink escape", path: "escape-dir/secret.txt", kind: FileUnreadable, wantErr: true},
		{name: "symlink", path: "link", kind: FileReady, content: "ready.txt", size: 9, symlink: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := repo.ReadFile(test.path)
			sizeMismatch := (test.kind == FileReady || test.kind == FileBinary || test.kind == FileTooLarge) && got.Size != test.size
			if got.Kind != test.kind || got.Content != test.content || sizeMismatch || got.Symlink != test.symlink || (got.Err != nil) != test.wantErr {
				t.Fatalf("ReadFile(%q) = %+v", test.path, got)
			}
		})
	}
}

func TestReadFileUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not reliably deny reads")
	}
	root := t.TempDir()
	path := filepath.Join(root, "private.txt")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	repo := &Repository{root: root, git: gitadapter.New(), maxBytes: DefaultMaxFileBytes}
	got := repo.ReadFile("private.txt")
	if got.Kind != FileUnreadable || got.Err == nil {
		t.Fatalf("ReadFile() = %+v, want unreadable error", got)
	}
}

func TestRepositoryOperationsDoNotWriteGitState(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, ".gitignore", "ignored.txt\n")
	writeFile(t, root, "tracked.txt", "tracked\n")
	runGit(t, root, "add", ".gitignore", "tracked.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	writeFile(t, root, "untracked space.txt", "untracked\n")
	writeFile(t, root, "ignored.txt", "ignored\n")

	before := captureGitState(t, root)
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if commonDir, err := repo.CommonDir(); err != nil || commonDir == "" {
		t.Fatalf("CommonDir() = %q, %v", commonDir, err)
	}
	files, err := repo.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, "tracked.txt") || !slices.Contains(files, "untracked space.txt") || slices.Contains(files, "ignored.txt") {
		t.Fatalf("ListFiles() = %#v", files)
	}
	for _, path := range files {
		result := repo.ReadFile(path)
		if result.Kind != FileReady {
			t.Fatalf("ReadFile(%q) = %+v", path, result)
		}
	}
	commits, err := repo.ListCommits()
	if err != nil || len(commits) != 1 {
		t.Fatalf("ListCommits() = (%#v, %v)", commits, err)
	}
	if _, err := repo.ReadCommit(commits[0].OID); err != nil {
		t.Fatal(err)
	}
	summary, err := repo.WorktreeSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary != (ChangeSummary{Files: 1, Additions: 1}) {
		t.Fatalf("WorktreeSummary() = %+v", summary)
	}
	after := captureGitState(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repository operations changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestScratchPrivateStateDoesNotWriteRepository(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "tracked\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	commonDir, err := repo.CommonDir()
	if err != nil {
		t.Fatal(err)
	}
	stateHome := t.TempDir()
	store := scratch.NewPrivateStore(commonDir, func(key string) (string, bool) {
		return stateHome, key == "XDG_STATE_HOME"
	})
	defer store.Close()
	before := captureGitState(t, root)
	if _, readOnly, err := store.Load(); err != nil || readOnly {
		t.Fatalf("Load() = readOnly %v, %v", readOnly, err)
	}
	if err := store.Save("private note"); err != nil {
		t.Fatal(err)
	}
	after := captureGitState(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("Scratch private state changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".reviewr")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Scratch created repository state: %v", err)
	}
}

func TestCommitHistoryIncludesRootAndMergeSummaries(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "root.txt", "root\n")
	runGit(t, root, "add", "root.txt")
	runGit(t, root, "commit", "-q", "-m", "root subject", "-m", "root body")
	rootOID := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))
	mainBranch := strings.TrimSpace(string(runGitBytes(t, root, "branch", "--show-current")))

	runGit(t, root, "checkout", "-q", "-b", "feature")
	writeFile(t, root, "feature.txt", "feature\n")
	runGit(t, root, "add", "feature.txt")
	runGit(t, root, "commit", "-q", "-m", "feature subject")
	runGit(t, root, "checkout", "-q", mainBranch)
	writeFile(t, root, "main.txt", "main\n")
	runGit(t, root, "add", "main.txt")
	runGit(t, root, "commit", "-q", "-m", "main subject")
	runGit(t, root, "merge", "-q", "--no-ff", "feature", "-m", "merge subject")
	mergeOID := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := repo.ListCommits()
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 4 || commits[0].OID != mergeOID {
		t.Fatalf("ListCommits() = %#v", commits)
	}
	if !slices.ContainsFunc(commits, func(commit Commit) bool { return commit.OID == rootOID }) {
		t.Fatalf("history omitted root commit %s: %#v", rootOID, commits)
	}

	rootSummary, err := repo.ReadCommit(rootOID)
	if err != nil {
		t.Fatal(err)
	}
	if rootSummary.OID != rootOID || rootSummary.AuthorName != "Reviewr Tests" ||
		rootSummary.AuthorEmail != "reviewr@example.invalid" || !strings.Contains(rootSummary.Message, "root body") ||
		!strings.Contains(rootSummary.Stat, "root.txt") {
		t.Fatalf("root summary = %+v", rootSummary)
	}
	mergeSummary, err := repo.ReadCommit(mergeOID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mergeSummary.Message, "merge subject") || !strings.Contains(mergeSummary.Stat, "feature.txt") {
		t.Fatalf("merge summary = %+v", mergeSummary)
	}
}

func TestCommitHistoryHandlesUnbornAndMissingObjects(t *testing.T) {
	root := initRepository(t)
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := repo.ListCommits()
	if err != nil || len(commits) != 0 {
		t.Fatalf("unborn ListCommits() = (%#v, %v)", commits, err)
	}
	if _, err := repo.ReadCommit(strings.Repeat("f", 40)); err == nil {
		t.Fatal("ReadCommit() accepted a missing object")
	}
}

type gitState struct {
	status []byte
	index  []byte
	head   string
	refs   string
}

func captureGitState(t *testing.T, root string) gitState {
	t.Helper()
	status := runGitBytes(t, root, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	indexPath := strings.TrimSuffix(string(runGitBytes(t, root, "rev-parse", "--git-path", "index")), "\n")
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(root, indexPath)
	}
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	return gitState{
		status: status,
		index:  index,
		head:   string(runGitBytes(t, root, "rev-parse", "HEAD")),
		refs:   string(runGitBytes(t, root, "show-ref", "--head", "--dereference")),
	}
}

func initRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "Reviewr Tests")
	runGit(t, root, "config", "user.email", "reviewr@example.invalid")
	return root
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_ = runGitBytes(t, root, args...)
}

func runGitBytes(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return out
}

func (s gitState) String() string {
	return fmt.Sprintf("status=%q index=%x head=%q refs=%q", s.status, s.index, s.head, s.refs)
}
