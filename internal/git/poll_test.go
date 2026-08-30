package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPollStateSeparatesWorktreePublicRefsAndPrivateRefs(t *testing.T) {
	root := initGitTestRepository(t)
	writePollFixture(t, root, "main.go", "package main\n")
	runGitTest(t, root, "add", "main.go")
	runGitTest(t, root, "commit", "-qm", "initial")
	client := New()
	initial, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}

	writePollFixture(t, root, "main.go", "package mains\n")
	changed, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Worktree == initial.Worktree {
		t.Fatal("worktree edit did not change its fingerprint")
	}
	if changed.Refs != initial.Refs {
		t.Fatal("worktree edit changed the public-ref fingerprint")
	}

	head := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "update-ref", "refs/heads/side", head)
	publicRef, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}
	if publicRef.Refs == changed.Refs {
		t.Fatal("public branch creation did not change the ref fingerprint")
	}

	runGitTest(t, root, "update-ref", "refs/worktree/reviewr/test", head)
	privateRef, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}
	if privateRef != publicRef {
		t.Fatalf("private review ref changed poll state: before=%+v after=%+v", publicRef, privateRef)
	}
}

func TestPollStateSeesSameStatusContentEdits(t *testing.T) {
	root := initGitTestRepository(t)
	writePollFixture(t, root, "main.go", "one\n")
	runGitTest(t, root, "add", "main.go")
	runGitTest(t, root, "commit", "-qm", "initial")
	writePollFixture(t, root, "main.go", "two\n")
	client := New()
	first, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.go")
	writePollFixture(t, root, "main.go", "six\n")
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	second, err := client.PollState(root)
	if err != nil {
		t.Fatal(err)
	}
	if second.Worktree == first.Worktree {
		t.Fatal("second modified content with the same Git status was not detected")
	}
}

func writePollFixture(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
