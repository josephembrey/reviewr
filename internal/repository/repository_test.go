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
	"github.com/josephembrey/reviewr/internal/review"
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
			got := repo.ReadFile(Entry{Path: test.path})
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
	got := repo.ReadFile(Entry{Path: "private.txt"})
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
	snapshot, err := repo.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	files := snapshot.All()
	if !slices.ContainsFunc(files, func(entry Entry) bool { return entry.Path == "tracked.txt" && entry.State == FileUnchanged }) ||
		!slices.ContainsFunc(files, func(entry Entry) bool { return entry.Path == "untracked space.txt" && entry.State == FileUntracked }) ||
		!slices.ContainsFunc(files, func(entry Entry) bool { return entry.Path == "ignored.txt" && entry.State == FileIgnored }) {
		t.Fatalf("Snapshot().All() = %#v", files)
	}
	for _, entry := range files {
		result := repo.ReadFile(entry)
		if result.Kind != FileReady {
			t.Fatalf("ReadFile(%q) = %+v", entry.Path, result)
		}
	}
	diff := repo.ReadDiff(Entry{Path: "untracked space.txt", State: FileUntracked})
	if diff.Kind != DiffReady || !strings.Contains(diff.Content, "+untracked") {
		t.Fatalf("ReadDiff(untracked) = %+v", diff)
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

func TestDeletedAndRenamedEntriesReadCoherently(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "old.go", "package old\n")
	writeFile(t, root, "gone.go", "package gone\n")
	runGit(t, root, "add", "old.go", "gone.go")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	runGit(t, root, "mv", "old.go", "new.go")
	if err := os.Remove(filepath.Join(root, "gone.go")); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]Entry)
	for _, entry := range snapshot.Changed() {
		byPath[entry.Path] = entry
	}
	renamed := byPath["new.go"]
	deleted := byPath["gone.go"]
	if renamed.State != FileRenamed || renamed.PreviousPath != "old.go" || deleted.State != FileDeleted {
		t.Fatalf("changed entries = %#v", byPath)
	}
	if file := repo.ReadFile(renamed); file.Kind != FileReady || !strings.Contains(file.Content, "package old") {
		t.Fatalf("renamed file read = %+v", file)
	}
	if file := repo.ReadFile(deleted); file.Kind != FileMissing {
		t.Fatalf("deleted file read = %+v", file)
	}
	if diff := repo.ReadDiff(renamed); diff.Kind != DiffReady || !strings.Contains(diff.Content, "old.go") || !strings.Contains(diff.Content, "new.go") {
		t.Fatalf("renamed diff = %+v", diff)
	}
	if diff := repo.ReadDiff(deleted); diff.Kind != DiffReady || !strings.Contains(diff.Content, "-package gone") {
		t.Fatalf("deleted diff = %+v", diff)
	}
}

func TestReviewComparisonsEnrichTypedSnapshotWithExactEndpoints(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "modified.txt", "old\n")
	writeFile(t, root, "deleted.txt", "gone\n")
	writeFile(t, root, "rename-old.txt", "rename\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "fixture")

	writeFile(t, root, "modified.txt", "new\n")
	if err := os.Chmod(filepath.Join(root, "modified.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "mv", "rename-old.txt", "rename-new.txt")
	writeFile(t, root, "added.bin", "a\x00b")
	if err := os.Symlink("modified.txt", filepath.Join(root, "added-link")); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]review.Candidate, 0)
	for _, entry := range snapshot.Changed() {
		action := review.Modified
		switch entry.State {
		case FileUntracked, FileAdded:
			action = review.Added
		case FileDeleted:
			action = review.Deleted
		case FileRenamed:
			action = review.Renamed
		}
		candidates = append(candidates, review.Candidate{Path: entry.Path, PreviousPath: entry.PreviousPath, Action: action})
	}
	reviews, err := repo.ReviewComparisons("uncommitted", candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews.Comparisons) != len(candidates) {
		t.Fatalf("review comparisons = %#v, want %d", reviews.Comparisons, len(candidates))
	}

	modified := reviews.Comparisons["modified.txt"]
	if modified.Action != review.Modified || modified.Old.Mode != 0o100644 || modified.New.Mode != 0o100755 ||
		!strings.HasPrefix(modified.Old.ContentID, "git:") || !strings.HasPrefix(modified.New.ContentID, "git:") || modified.Old == modified.New {
		t.Fatalf("modified comparison = %+v", modified)
	}
	if content := repo.ReadReviewContent(modified.OldSource, modified.Old); content.Endpoint != modified.Old || content.State != review.ContentText || content.Text != "old\n" {
		t.Fatalf("old content = %+v", content)
	}
	if content := repo.ReadReviewContent(modified.NewSource, modified.New); content.Endpoint != modified.New || content.State != review.ContentText || content.Text != "new\n" {
		t.Fatalf("new content = %+v", content)
	}

	deleted := reviews.Comparisons["deleted.txt"]
	if deleted.Action != review.Deleted || deleted.Old.Kind != review.Regular || deleted.New.Kind != review.Absent {
		t.Fatalf("deleted comparison = %+v", deleted)
	}
	writeFile(t, root, "deleted.txt", "resurrected\n")
	if content := repo.ReadReviewContent(deleted.NewSource, deleted.New); content.Endpoint == deleted.New || content.Endpoint.Kind != review.Regular {
		t.Fatalf("resurrected deletion still verified absent: %+v", content)
	}
	renamed := reviews.Comparisons["rename-new.txt"]
	if renamed.Action != review.Renamed || renamed.Old.Path != "rename-old.txt" || renamed.New.Path != "rename-new.txt" || renamed.BasisReason == "" {
		t.Fatalf("renamed comparison = %+v", renamed)
	}
	added := reviews.Comparisons["added.bin"]
	if added.Action != review.Added || added.Old.Kind != review.Absent {
		t.Fatalf("added comparison = %+v", added)
	}
	if content := repo.ReadReviewContent(added.NewSource, added.New); content.Endpoint != added.New || content.State != review.ContentBinary || content.Text != "" {
		t.Fatalf("binary content = %+v", content)
	}
	link := reviews.Comparisons["added-link"]
	if link.Old.Kind != review.Absent || link.New.Kind != review.Symlink || link.New.Mode != 0o120000 {
		t.Fatalf("symlink comparison = %+v", link)
	}
	if content := repo.ReadReviewContent(link.NewSource, link.New); content.Endpoint != link.New || content.State != review.ContentText || content.Text != "modified.txt" {
		t.Fatalf("symlink content = %+v", content)
	}
	identity, err := repo.ReviewRepositoryID()
	if err != nil || identity.Worktree != root || identity.CommonGitDir == "" {
		t.Fatalf("ReviewRepositoryID() = (%+v, %v)", identity, err)
	}
}

func TestReviewContentProvesNestedAbsenceWithoutFollowingParentSymlinks(t *testing.T) {
	root := initRepository(t)
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	absent := review.AbsentEndpoint("missing/child.txt")
	if content := repo.ReadReviewContent(review.EndpointSource{Kind: review.WorktreeSource}, absent); content.Endpoint != absent || content.State != review.ContentAbsent {
		t.Fatalf("nested absence = %+v", content)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	escape := review.AbsentEndpoint("escape/child.txt")
	if content := repo.ReadReviewContent(review.EndpointSource{Kind: review.WorktreeSource}, escape); content.State != review.ContentUnavailable || content.Endpoint.Exact() {
		t.Fatalf("symlink-parent absence = %+v", content)
	}
}

func TestReviewContentKeepsExactIdentityWhenSnapshotIsOversized(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "large.txt", "old")
	runGit(t, root, "add", "large.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	writeFile(t, root, "large.txt", strings.Repeat("x", 32))
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	repo.maxBytes = 8
	reviews, err := repo.ReviewComparisons("uncommitted", []review.Candidate{{Path: "large.txt", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := reviews.Comparisons["large.txt"]
	content := repo.ReadReviewContent(comparison.NewSource, comparison.New)
	if content.State != review.ContentTooLarge || !content.Endpoint.Exact() || content.Endpoint != comparison.New || content.Text != "" {
		t.Fatalf("oversized content = %+v comparison=%+v", content, comparison)
	}
}

func TestReviewComparisonsSupportAddedFilesOnUnbornHead(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "first.go", "package first\n")
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	reviews, err := repo.ReviewComparisons("uncommitted", []review.Candidate{{Path: "first.go", Action: review.Added}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := reviews.Comparisons["first.go"]
	if comparison.Action != review.Added || comparison.Old != review.AbsentEndpoint("first.go") || !comparison.New.Exact() || comparison.Identity.Basis == "" {
		t.Fatalf("unborn comparison = %+v", comparison)
	}
}

func TestReviewComparisonsRejectStatusContentRace(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.go", "base\n")
	runGit(t, root, "add", "tracked.go")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	resurrected, err := repo.ReviewComparisons("uncommitted", []review.Candidate{{Path: "tracked.go", Action: review.Deleted}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := resurrected.Comparisons["tracked.go"]
	if comparison.Exact() || !strings.Contains(comparison.BasisReason, "changed") {
		t.Fatalf("stale deleted candidate = %+v", comparison)
	}
	if err := os.Remove(filepath.Join(root, "tracked.go")); err != nil {
		t.Fatal(err)
	}
	disappeared, err := repo.ReviewComparisons("uncommitted", []review.Candidate{{Path: "tracked.go", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison = disappeared.Comparisons["tracked.go"]
	if comparison.Exact() || !strings.Contains(comparison.BasisReason, "changed") {
		t.Fatalf("stale modified candidate = %+v", comparison)
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
