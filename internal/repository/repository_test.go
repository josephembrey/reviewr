package repository

import (
	"bytes"
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
	after := captureGitState(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repository operations changed Git state\nbefore: %+v\nafter:  %+v", before, after)
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
