package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCommitLogUsesMachineDelimitedFields(t *testing.T) {
	t.Parallel()
	data := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x00aaaaaaa\x00Author A\x001700000000\x00line\nbreak \x1b[31m\x00\x00" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\x00bbbbbbb\x00Author B\x001800000000\x00日本語\x00\x00")
	want := []Commit{
		{OID: strings.Repeat("a", 40), ShortOID: "aaaaaaa", Author: "Author A", AuthoredUnix: 1_700_000_000, Subject: "line\nbreak \x1b[31m"},
		{OID: strings.Repeat("b", 40), ShortOID: "bbbbbbb", Author: "Author B", AuthoredUnix: 1_800_000_000, Subject: "日本語"},
	}
	got, err := parseCommitLog(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCommitLog() = %#v, want %#v", got, want)
	}
	if _, err := parseCommitLog([]byte("oid\x00short\x00")); err == nil {
		t.Fatal("parseCommitLog() accepted malformed record")
	}
}

func TestListCommitsIsBoundedAndHandlesUnbornHead(t *testing.T) {
	root := initGitTestRepository(t)
	client := New()
	commits, err := client.ListCommits(root, HistoryQuery{})
	if err != nil || len(commits) != 0 {
		t.Fatalf("unborn ListCommits() = (%#v, %v), want empty", commits, err)
	}

	for index := 0; index < CommitLimit+5; index++ {
		runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", fmt.Sprintf("commit %03d", index))
	}
	commits, err = client.ListCommits(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != CommitLimit {
		t.Fatalf("ListCommits() returned %d commits, want %d", len(commits), CommitLimit)
	}
	if commits[0].Subject != "commit 204" || commits[len(commits)-1].Subject != "commit 005" {
		t.Fatalf("bounded history endpoints = %q .. %q", commits[0].Subject, commits[len(commits)-1].Subject)
	}
}

func TestGraphAndFirstParentTraverseDifferentUniverses(t *testing.T) {
	root := initGitTestRepository(t)
	client := New()
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "root")
	rootOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	mainBranch := strings.TrimSpace(runGitTest(t, root, "branch", "--show-current"))

	runGitTest(t, root, "checkout", "-q", "-b", "side")
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "side")
	sideOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "checkout", "-q", mainBranch)
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "main")
	mainOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "merge", "-q", "--no-ff", "-m", "merge", "side")
	mergeOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	treeOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD^{tree}"))
	privateOID := strings.TrimSpace(runGitTest(t, root, "commit-tree", treeOID, "-m", "private"))
	runGitTest(t, root, "update-ref", "refs/stash", privateOID)
	runGitTest(t, root, "update-ref", "refs/worktree/reviewr/test", privateOID)

	graphRows, err := client.ListCommits(root, HistoryQuery{Traversal: GraphTraversal})
	if err != nil {
		t.Fatal(err)
	}
	if len(graphRows) != 4 || graphRows[0].OID != mergeOID {
		t.Fatalf("graph rows = %#v", graphRows)
	}
	for _, row := range graphRows {
		if row.OID == privateOID {
			t.Fatalf("private/stash-only commit entered public graph: %+v", row)
		}
	}
	merge := findCommit(t, graphRows, mergeOID)
	if !merge.Merge || !reflect.DeepEqual(merge.Parents, []string{mainOID, sideOID}) {
		t.Fatalf("merge metadata = %+v", merge)
	}
	if findCommit(t, graphRows, rootOID).Merge {
		t.Fatal("root commit marked as merge")
	}

	mainline, err := client.ListCommits(root, HistoryQuery{Traversal: FirstParentTraversal})
	if err != nil {
		t.Fatal(err)
	}
	if got := subjects(mainline); !reflect.DeepEqual(got, []string{"merge", "main", "root"}) {
		t.Fatalf("HEAD first-parent subjects = %q", got)
	}
	if !mainline[0].Merge || !reflect.DeepEqual(mainline[0].Parents, []string{mainOID, sideOID}) {
		t.Fatalf("first-parent lost merge metadata: %+v", mainline[0])
	}

	selectedLineage, err := client.ListCommits(root, HistoryQuery{Traversal: FirstParentTraversal, StartOID: sideOID})
	if err != nil {
		t.Fatal(err)
	}
	if got := subjects(selectedLineage); !reflect.DeepEqual(got, []string{"side", "root"}) {
		t.Fatalf("selected first-parent subjects = %q", got)
	}
}

func TestListCommitsKeepsHostileMetadataStructuredAndInert(t *testing.T) {
	root := initGitTestRepository(t)
	subject := "hostile \x1b[31m $(touch reviewr-pwned)"
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", subject)
	oid := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	branch := "topic,$(touch-reviewr-pwned)"
	remote := "origin/topic,$(touch-reviewr-pwned)"
	tag := "release,tag"
	runGitTest(t, root, "branch", branch, oid)
	runGitTest(t, root, "update-ref", "refs/remotes/"+remote, oid)
	runGitTest(t, root, "tag", "-a", "-m", "annotated", tag, oid)

	rows, err := New().ListCommits(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	row := findCommit(t, rows, oid)
	if row.Subject != subject || row.Author != "Reviewr Tests" || row.AuthoredUnix <= 0 || !row.Head {
		t.Fatalf("hostile row = %+v", row)
	}
	wantRefs := []CommitRef{
		{Kind: BranchRef, Name: branch},
		{Kind: RemoteRef, Name: remote},
		{Kind: TagRef, Name: tag},
	}
	if !containsRefs(row.Refs, wantRefs) {
		t.Fatalf("semantic refs = %#v, want at least %#v", row.Refs, wantRefs)
	}
	if _, statErr := os.Stat(filepath.Join(root, "reviewr-pwned")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("hostile metadata was evaluated: %v", statErr)
	}
}

func TestGraphIncludesDetachedHeadOutsidePublicRefs(t *testing.T) {
	root := initGitTestRepository(t)
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "root")
	rootOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "detached")
	tipOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	branch := strings.TrimSpace(runGitTest(t, root, "symbolic-ref", "--short", "HEAD"))
	runGitTest(t, root, "checkout", "-q", "--detach", tipOID)
	runGitTest(t, root, "update-ref", "refs/heads/"+branch, rootOID)

	rows, err := New().ListCommits(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if row := findCommit(t, rows, tipOID); !row.Head {
		t.Fatalf("detached row did not carry HEAD identity: %+v", row)
	}
}

func TestShallowBoundaryRetainsRawParent(t *testing.T) {
	origin := initGitTestRepository(t)
	runGitTest(t, origin, "commit", "--allow-empty", "-q", "-m", "root")
	rootOID := strings.TrimSpace(runGitTest(t, origin, "rev-parse", "HEAD"))
	runGitTest(t, origin, "commit", "--allow-empty", "-q", "-m", "tip")
	tipOID := strings.TrimSpace(runGitTest(t, origin, "rev-parse", "HEAD"))

	clone := filepath.Join(t.TempDir(), "shallow")
	output, err := exec.Command("git", "clone", "-q", "--depth=1", "file://"+origin, clone).CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %v: %s", err, output)
	}
	rows, err := New().ListCommits(clone, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OID != tipOID || !reflect.DeepEqual(rows[0].Parents, []string{rootOID}) {
		t.Fatalf("shallow rows = %#v", rows)
	}
}

func TestBoundedBufferCapsMemoryButConsumesWrites(t *testing.T) {
	t.Parallel()
	buffer := boundedBuffer{limit: 4}
	if count, err := buffer.Write([]byte("abcdef")); err != nil || count != 6 {
		t.Fatalf("Write() = (%d, %v)", count, err)
	}
	if got := string(buffer.Bytes()); got != "abcd" || !buffer.truncated {
		t.Fatalf("bounded buffer = %q truncated=%v", got, buffer.truncated)
	}
}

func findCommit(t *testing.T, rows []Commit, oid string) Commit {
	t.Helper()
	for _, row := range rows {
		if row.OID == oid {
			return row
		}
	}
	t.Fatalf("commit %s not found in %#v", oid, rows)
	return Commit{}
}

func subjects(rows []Commit) []string {
	result := make([]string, len(rows))
	for index, row := range rows {
		result[index] = row.Subject
	}
	return result
}

func containsRefs(got, want []CommitRef) bool {
	for _, expected := range want {
		found := false
		for _, actual := range got {
			found = found || actual == expected
		}
		if !found {
			return false
		}
	}
	return true
}

func initGitTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "config", "user.name", "Reviewr Tests")
	runGitTest(t, root, "config", "user.email", "reviewr@example.invalid")
	runGitTest(t, root, "config", "gc.auto", "0")
	runGitTest(t, root, "config", "maintenance.auto", "false")
	return root
}

func runGitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", filepath.Clean(root)}, args...)
	out, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
