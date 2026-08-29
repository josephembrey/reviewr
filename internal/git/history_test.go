package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCommitLogUsesMachineDelimitedFields(t *testing.T) {
	t.Parallel()
	data := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x00aaaaaaa\x00line\nbreak \x1b[31m\x00\x00" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\x00bbbbbbb\x00日本語\x00\x00")
	want := []Commit{
		{OID: strings.Repeat("a", 40), ShortOID: "aaaaaaa", Subject: "line\nbreak \x1b[31m"},
		{OID: strings.Repeat("b", 40), ShortOID: "bbbbbbb", Subject: "日本語"},
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
	commits, err := client.ListCommits(root)
	if err != nil || len(commits) != 0 {
		t.Fatalf("unborn ListCommits() = (%#v, %v), want empty", commits, err)
	}

	for index := 0; index < CommitLimit+5; index++ {
		runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", fmt.Sprintf("commit %03d", index))
	}
	commits, err = client.ListCommits(root)
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

func TestReadCommitParsesSummaryAndEnforcesOutputLimit(t *testing.T) {
	root := initGitTestRepository(t)
	message := "subject\n\nbody line"
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", message)
	client := New()
	commits, err := client.ListCommits(root)
	if err != nil || len(commits) != 1 {
		t.Fatalf("ListCommits() = (%#v, %v)", commits, err)
	}
	summary, err := client.ReadCommit(root, commits[0].OID, DefaultMaxHistoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if summary.OID != commits[0].OID || summary.AuthorName != "Reviewr Tests" || summary.AuthorEmail != "reviewr@example.invalid" ||
		!strings.Contains(summary.Message, "subject") || !strings.Contains(summary.Message, "body line") || summary.AuthoredAt == "" {
		t.Fatalf("ReadCommit() = %+v", summary)
	}
	if _, err := client.ReadCommit(root, commits[0].OID, 32); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("small ReadCommit() error = %v, want ErrOutputTooLarge", err)
	}
	if _, err := client.ReadCommit(root, "--help", DefaultMaxHistoryBytes); err == nil {
		t.Fatal("ReadCommit() accepted a non-object identity")
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

func initGitTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "config", "user.name", "Reviewr Tests")
	runGitTest(t, root, "config", "user.email", "reviewr@example.invalid")
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
