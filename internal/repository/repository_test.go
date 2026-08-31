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
	"github.com/josephembrey/reviewr/internal/notes"
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

	t.Run("symlinked checkout path", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), "checkout-link")
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		repo, err := Open(filepath.Join(alias, "one"))
		if err != nil {
			t.Fatal(err)
		}
		if repo.Root() != root {
			t.Fatalf("Root() = %q, want canonical %q", repo.Root(), root)
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
		common := repo.CommonDir()
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

func TestNotesStoresExposeProjectAndLocalNotesInEveryCheckout(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "tracked\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	linked := filepath.Join(t.TempDir(), "linked checkout")
	runGit(t, root, "worktree", "add", "-q", "-b", "notes-linked", linked)
	t.Cleanup(func() { runGit(t, root, "worktree", "remove", "--force", linked) })

	stateHome := t.TempDir()
	lookup := func(key string) (string, bool) { return stateHome, key == "XDG_STATE_HOME" }
	primaryRepo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	linkedRepo, err := Open(linked)
	if err != nil {
		t.Fatal(err)
	}
	primaryStores := primaryRepo.NotesStores(lookup)
	t.Cleanup(func() { _ = primaryStores.Close() })
	linkedStores := linkedRepo.NotesStores(lookup)
	t.Cleanup(func() { _ = linkedStores.Close() })

	if !primaryStores.HasWorktree() || !linkedStores.HasWorktree() {
		t.Fatalf("worktree scopes = primary %v linked %v; want both", primaryStores.HasWorktree(), linkedStores.HasWorktree())
	}
	if _, readOnly, err := primaryStores.Project.Load(); err != nil || readOnly {
		t.Fatalf("primary project Load() = readOnly %v, %v", readOnly, err)
	}
	if err := primaryStores.Project.Save("shared"); err != nil {
		t.Fatal(err)
	}
	if text, readOnly, err := linkedStores.Project.Load(); err != nil || !readOnly || text != "shared" {
		t.Fatalf("linked project Load() = %q, readOnly %v, %v", text, readOnly, err)
	}
	if text, readOnly, err := primaryStores.Worktree.Load(); err != nil || readOnly || text != "" {
		t.Fatalf("primary worktree Load() = %q, readOnly %v, %v", text, readOnly, err)
	}
	if err := primaryStores.Worktree.Save("primary local"); err != nil {
		t.Fatal(err)
	}
	if text, readOnly, err := linkedStores.Worktree.Load(); err != nil || readOnly || text != "" {
		t.Fatalf("linked worktree Load() = %q, readOnly %v, %v", text, readOnly, err)
	}
	if err := linkedStores.Worktree.Save("local"); err != nil {
		t.Fatal(err)
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
	if commonDir := repo.CommonDir(); commonDir == "" {
		t.Fatal("CommonDir() is empty")
	}
	snapshot, err := repo.Snapshot(ComparisonUncommitted)
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
	diff := repo.ReadDiff(snapshot.Comparison(), Entry{Path: "untracked space.txt", State: FileUntracked})
	if diff.Kind != DiffReady || !strings.Contains(diff.Content, "+untracked") {
		t.Fatalf("ReadDiff(untracked) = %+v", diff)
	}
	commits, err := repo.ListCommits(CommitQuery{})
	if err != nil || len(commits) != 1 {
		t.Fatalf("ListCommits() = (%#v, %v)", commits, err)
	}
	refSources, err := repo.ListRefSources()
	if err != nil || len(refSources) < 2 {
		t.Fatalf("ListRefSources() = (%#v, %v)", refSources, err)
	}
	if lineage, err := repo.ListCommits(CommitQuery{Traversal: CommitFirstParent, StartOID: commits[0].OID}); err != nil || len(lineage) != 1 {
		t.Fatalf("first-parent ListCommits() = (%#v, %v)", lineage, err)
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

func TestNotesPrivateStateDoesNotWriteRepository(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "tracked\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	commonDir := repo.CommonDir()
	stateHome := t.TempDir()
	store := notes.NewPrivateStore(commonDir, func(key string) (string, bool) {
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
		t.Fatalf("Notes private state changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".reviewr")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Notes created repository state: %v", err)
	}
}

func TestRefRepositoryBoundaryPreservesTypedSameTipSources(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "root.txt", "root\n")
	runGit(t, root, "add", "root.txt")
	runGit(t, root, "commit", "-q", "-m", "root")
	oid := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))
	runGit(t, root, "branch", "same-tip")
	runGit(t, root, "tag", "same-tip")

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := repo.ListRefSources()
	if err != nil {
		t.Fatal(err)
	}
	branch := slices.IndexFunc(sources, func(source RefSource) bool {
		return source.ID == (RefSourceID{Kind: RefSourceLocalBranch, Name: "refs/heads/same-tip"})
	})
	tag := slices.IndexFunc(sources, func(source RefSource) bool {
		return source.ID == (RefSourceID{Kind: RefSourceTag, Name: "refs/tags/same-tip"})
	})
	if branch < 0 || tag < 0 || sources[branch].OID != oid || sources[tag].OID != oid || sources[branch].ID == sources[tag].ID {
		t.Fatalf("same-tip sources lost type identity: %#v", sources)
	}
	for _, index := range []int{branch, tag} {
		history, historyErr := repo.ListCommits(CommitQuery{SourceOID: sources[index].OID})
		if historyErr != nil || len(history) != 1 || history[0].OID != oid {
			t.Fatalf("history for %+v = (%#v, %v)", sources[index].ID, history, historyErr)
		}
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
	snapshot, err := repo.Snapshot(ComparisonUncommitted)
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
	if diff := repo.ReadDiff(snapshot.Comparison(), renamed); diff.Kind != DiffReady || !strings.Contains(diff.Content, "old.go") || !strings.Contains(diff.Content, "new.go") {
		t.Fatalf("renamed diff = %+v", diff)
	}
	if diff := repo.ReadDiff(snapshot.Comparison(), deleted); diff.Kind != DiffReady || !strings.Contains(diff.Content, "-package gone") {
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
	snapshot, err := repo.Snapshot(ComparisonUncommitted)
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
	reviews, err := repo.ReviewComparisons(ComparisonUncommitted, snapshot.Comparison().Basis, candidates)
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

func TestBranchComparisonUsesDefaultBranchMergeBaseThroughTheWorktree(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "base\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))
	runGit(t, root, "update-ref", "refs/remotes/origin/main", base)
	runGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	runGit(t, root, "checkout", "-qb", "feature")

	writeFile(t, root, "tracked.txt", "committed\n")
	writeFile(t, root, "branch.txt", "branch\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "feature")
	writeFile(t, root, "tracked.txt", "committed\nworking\n")
	writeFile(t, root, "loose.txt", "loose\n")

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.Snapshot(ComparisonBranch)
	if err != nil {
		t.Fatal(err)
	}
	if comparison := snapshot.Comparison(); comparison.Scope != ComparisonBranch || comparison.Basis != base || !comparison.Available() {
		t.Fatalf("branch comparison = %+v, want merge base %s", comparison, base)
	}
	changed := entriesByPath(snapshot.Changed())
	for path, state := range map[string]FileState{
		"branch.txt":  FileAdded,
		"tracked.txt": FileModified,
		"loose.txt":   FileUntracked,
	} {
		if entry, ok := changed[path]; !ok || entry.State != state {
			t.Fatalf("branch entry %q = %+v, %v; want state %v", path, entry, ok, state)
		}
	}
	if diff := repo.ReadDiff(snapshot.Comparison(), changed["tracked.txt"]); diff.Kind != DiffReady || !strings.Contains(diff.Content, "+working") || !strings.Contains(diff.Content, "-base") {
		t.Fatalf("branch worktree diff = %+v", diff)
	}
	reviews, err := repo.ReviewComparisons(ComparisonBranch, snapshot.Comparison().Basis, []review.Candidate{{Path: "tracked.txt", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := reviews.Comparisons["tracked.txt"]
	if comparison.Identity.Basis != base || comparison.OldSource.Value != base {
		t.Fatalf("branch review comparison = %+v", comparison)
	}
	if content := repo.ReadReviewContent(comparison.OldSource, comparison.Old); content.State != review.ContentText || content.Text != "base\n" {
		t.Fatalf("branch review old content = %+v", content)
	}
}

func TestLastTurnComparisonUsesPersistedWorktreeSnapshot(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, ".gitignore", "ignored/\n")
	writeFile(t, root, "tracked.txt", "before\n")
	runGit(t, root, "add", ".gitignore", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	writeFile(t, root, "existing-untracked.txt", "before loose\n")
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "ignored/cache.txt", "before ignored\n")

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := repo.SnapshotTurnWorktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.WriteTurnBaseline(baseline); err != nil {
		t.Fatal(err)
	}

	writeFile(t, root, "tracked.txt", "committed during turn\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "agent commit")
	writeFile(t, root, "tracked.txt", "committed during turn\nworking\n")
	writeFile(t, root, "existing-untracked.txt", "after loose\n")
	writeFile(t, root, "new-untracked.txt", "new loose\n")
	writeFile(t, root, "ignored/cache.txt", "after ignored\n")

	snapshot, err := repo.Snapshot(ComparisonLastTurn)
	if err != nil {
		t.Fatal(err)
	}
	if comparison := snapshot.Comparison(); comparison.Scope != ComparisonLastTurn || comparison.Basis != baseline || !comparison.Available() {
		t.Fatalf("last-turn comparison = %+v, want baseline %s", comparison, baseline)
	}
	changed := entriesByPath(snapshot.Changed())
	for _, path := range []string{"tracked.txt", "existing-untracked.txt", "new-untracked.txt"} {
		if _, ok := changed[path]; !ok {
			t.Fatalf("last-turn changes omit %q: %#v", path, changed)
		}
	}
	if _, ok := changed["ignored/cache.txt"]; ok {
		t.Fatalf("last-turn included ignored file: %#v", changed)
	}
	if entry := entriesByPath(snapshot.All())["ignored/cache.txt"]; entry.State != FileIgnored {
		t.Fatalf("ignored all-files entry = %+v", entry)
	}
	if diff := repo.ReadDiff(snapshot.Comparison(), changed["existing-untracked.txt"]); diff.Kind != DiffReady || !strings.Contains(diff.Content, "-before loose") || !strings.Contains(diff.Content, "+after loose") {
		t.Fatalf("last-turn diff for baseline-untracked file = %+v", diff)
	}
	reviews, err := repo.ReviewComparisons(ComparisonLastTurn, snapshot.Comparison().Basis, []review.Candidate{{Path: "existing-untracked.txt", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := reviews.Comparisons["existing-untracked.txt"]
	if comparison.Identity.Basis != baseline || comparison.OldSource.Value != baseline {
		t.Fatalf("last-turn review comparison = %+v", comparison)
	}
	if content := repo.ReadReviewContent(comparison.OldSource, comparison.Old); content.State != review.ContentText || content.Text != "before loose\n" {
		t.Fatalf("last-turn review old content = %+v", content)
	}
}

func TestUnavailableComparisonsStayEmptyAndExplainWhy(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "base\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	writeFile(t, root, "tracked.txt", "changed\n")
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{ComparisonBranch, ComparisonLastTurn} {
		snapshot, snapshotErr := repo.Snapshot(scope)
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		if comparison := snapshot.Comparison(); comparison.Available() || comparison.Reason == "" || len(snapshot.Changed()) != 0 {
			t.Fatalf("unavailable %s snapshot = comparison %+v changed %#v", scope, comparison, snapshot.Changed())
		}
	}
}

func TestTurnSnapshotAndBaselinePreserveWorktreeIndexAndPublicRefs(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "base\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	writeFile(t, root, "tracked.txt", "worktree\n")
	writeFile(t, root, "untracked.txt", "loose\n")

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before := captureGitState(t, root)
	publicBefore := publicRefs(t, root)
	tree, err := repo.SnapshotTurnWorktree()
	if err != nil {
		t.Fatal(err)
	}
	afterSnapshot := captureGitState(t, root)
	if !bytes.Equal(afterSnapshot.status, before.status) || !bytes.Equal(afterSnapshot.index, before.index) ||
		afterSnapshot.head != before.head || afterSnapshot.refs != before.refs || publicRefs(t, root) != publicBefore {
		t.Fatalf("turn snapshot changed repository state\nbefore: %+v\nafter:  %+v", before, afterSnapshot)
	}
	if err := repo.WriteTurnBaseline(tree); err != nil {
		t.Fatal(err)
	}
	afterBaseline := captureGitState(t, root)
	if !bytes.Equal(afterBaseline.status, before.status) || !bytes.Equal(afterBaseline.index, before.index) ||
		afterBaseline.head != before.head || publicRefs(t, root) != publicBefore {
		t.Fatalf("turn baseline changed worktree, index, HEAD, or public refs\nbefore: %+v\nafter:  %+v", before, afterBaseline)
	}
	if got := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "refs/worktree/reviewr/turn-base^{tree}"))); got != tree {
		t.Fatalf("turn baseline = %q, want %q", got, tree)
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
	reviews, err := repo.ReviewComparisons(ComparisonUncommitted, comparisonBasis(t, repo, ComparisonUncommitted), []review.Candidate{{Path: "large.txt", Action: review.Modified}})
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
	reviews, err := repo.ReviewComparisons(ComparisonUncommitted, comparisonBasis(t, repo, ComparisonUncommitted), []review.Candidate{{Path: "first.go", Action: review.Added}})
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
	basis := comparisonBasis(t, repo, ComparisonUncommitted)
	resurrected, err := repo.ReviewComparisons(ComparisonUncommitted, basis, []review.Candidate{{Path: "tracked.go", Action: review.Deleted}})
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
	disappeared, err := repo.ReviewComparisons(ComparisonUncommitted, basis, []review.Candidate{{Path: "tracked.go", Action: review.Modified}})
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
	featureOID := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))
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
	commits, err := repo.ListCommits(CommitQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 4 || commits[0].OID != mergeOID {
		t.Fatalf("ListCommits() = %#v", commits)
	}
	if !commits[0].Head || !commits[0].Merge || len(commits[0].Parents) != 2 || commits[0].Author != "Reviewr Tests" || commits[0].AuthoredUnix <= 0 {
		t.Fatalf("merge row metadata = %+v", commits[0])
	}
	if !slices.ContainsFunc(commits, func(commit Commit) bool {
		return commit.OID == featureOID && slices.ContainsFunc(commit.Refs, func(reference CommitRef) bool {
			return reference.Kind == CommitBranchRef && reference.Name == "feature"
		})
	}) {
		t.Fatalf("history omitted semantic feature ref: %#v", commits)
	}
	if !slices.ContainsFunc(commits, func(commit Commit) bool { return commit.OID == rootOID }) {
		t.Fatalf("history omitted root commit %s: %#v", rootOID, commits)
	}
	lineage, err := repo.ListCommits(CommitQuery{Traversal: CommitFirstParent, StartOID: featureOID})
	if err != nil || len(lineage) != 2 || lineage[0].OID != featureOID || lineage[1].OID != rootOID {
		t.Fatalf("selected first-parent lineage = (%#v, %v)", lineage, err)
	}

}

func TestCommitHistoryHandlesUnbornAndMissingObjects(t *testing.T) {
	root := initRepository(t)
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := repo.ListCommits(CommitQuery{})
	if err != nil || len(commits) != 0 {
		t.Fatalf("unborn ListCommits() = (%#v, %v)", commits, err)
	}
	if _, err := repo.ListCommitFiles(strings.Repeat("f", 40)); err == nil {
		t.Fatal("ListCommitFiles() accepted a missing object")
	}
}

type gitState struct {
	status  []byte
	index   []byte
	head    string
	refs    string
	objects string
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
		status:  status,
		index:   index,
		head:    string(runGitBytes(t, root, "rev-parse", "HEAD")),
		refs:    string(runGitBytes(t, root, "show-ref", "--head", "--dereference")),
		objects: string(runGitBytes(t, root, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)")),
	}
}

func comparisonBasis(t *testing.T, repo *Repository, scope string) string {
	t.Helper()
	snapshot, err := repo.Snapshot(scope)
	if err != nil {
		t.Fatal(err)
	}
	comparison := snapshot.Comparison()
	if !comparison.Available() {
		t.Fatalf("comparison %q unavailable: %s", scope, comparison.Reason)
	}
	return comparison.Basis
}

func entriesByPath(entries []Entry) map[string]Entry {
	byPath := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	return byPath
}

func publicRefs(t *testing.T, root string) string {
	t.Helper()
	return string(runGitBytes(t, root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/remotes", "refs/tags"))
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
	return fmt.Sprintf("status=%q index=%x head=%q refs=%q objects=%q", s.status, s.index, s.head, s.refs, s.objects)
}
