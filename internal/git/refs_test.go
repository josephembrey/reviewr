package git

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseRefsDataKeepsTypedIdentitySeparateFromLabelsAndTips(t *testing.T) {
	t.Parallel()
	worktrees, err := parseWorktreeList([]byte(
		"worktree /repo/current\x00HEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x00branch refs/heads/main\x00\x00" +
			"worktree /repo/linked path\x00HEAD bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\x00detached\x00\x00",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 2 || worktrees[0].branch != "main" || worktrees[1].path != "/repo/linked path" || worktrees[1].branch != "" {
		t.Fatalf("parseWorktreeList() = %#v", worktrees)
	}

	refs, err := parseRefList([]byte(
		"refs/heads/topic,unicode-λ\x00aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x00\x00origin/topic\x00>\x00123\x00\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].name != "refs/heads/topic,unicode-λ" || refs[0].upstream != "origin/topic" || refs[0].tracking != ">" {
		t.Fatalf("parseRefList() = %#v", refs)
	}

	branch := RefSourceID{Kind: RefSourceLocalBranch, Name: "refs/heads/same"}
	tag := RefSourceID{Kind: RefSourceTag, Name: "refs/tags/same"}
	if branch == tag || branch.Key() == tag.Key() {
		t.Fatalf("typed same-tip identities alias: branch=%+v tag=%+v", branch, tag)
	}
}

func TestRefSourcesCoverWorktreesBranchesRemotesTagsPackedRefsAndSameTips(t *testing.T) {
	root := initGitTestRepository(t)
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "root")
	rootOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	currentBranch := strings.TrimSpace(runGitTest(t, root, "branch", "--show-current"))
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "tip")
	tipOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "branch", "linked", tipOID)
	runGitTest(t, root, "branch", "topic,unicode-λ", rootOID)
	runGitTest(t, root, "remote", "add", "origin", "https://example.invalid/reviewr.git")
	runGitTest(t, root, "update-ref", "refs/remotes/origin/main", tipOID)
	runGitTest(t, root, "branch", "--set-upstream-to=origin/main", currentBranch)
	runGitTest(t, root, "tag", "-a", "v1", rootOID, "-m", "v1")
	linkedPath := filepath.Join(t.TempDir(), "linked worktree")
	runGitTest(t, root, "worktree", "add", "-q", linkedPath, "linked")

	client := New()
	assertSources := func(t *testing.T) []RefSource {
		t.Helper()
		sources, err := client.ListRefSources(root)
		if err != nil {
			t.Fatal(err)
		}
		kinds := make([]RefSourceKind, len(sources))
		for index, source := range sources {
			kinds[index] = source.Kind()
		}
		wantKinds := []RefSourceKind{
			RefSourceAll,
			RefSourceCurrentWorktree,
			RefSourceLinkedWorktree,
			RefSourceLocalBranch,
			RefSourceRemoteBranch,
			RefSourceTag,
		}
		if !reflect.DeepEqual(kinds, wantKinds) {
			t.Fatalf("source kinds = %#v, want %#v\nsources=%#v", kinds, wantKinds, sources)
		}
		current := sources[1]
		if current.Branch != currentBranch || current.Upstream != "origin/main" || current.OID != tipOID {
			t.Fatalf("current worktree = %+v", current)
		}
		if sources[2].Path != filepath.Clean(linkedPath) || sources[2].Branch != "linked" || sources[2].OID != tipOID {
			t.Fatalf("linked worktree = %+v", sources[2])
		}
		if sources[3].Label != "topic,unicode-λ" || sources[3].OID != rootOID {
			t.Fatalf("local branch = %+v", sources[3])
		}
		if sources[4].Label != "origin/main" || sources[4].Remote != "origin" || sources[4].OID != tipOID {
			t.Fatalf("remote = %+v", sources[4])
		}
		if sources[5].Label != "v1" || sources[5].OID != rootOID {
			t.Fatalf("annotated tag did not peel to commit: %+v", sources[5])
		}
		for left := range sources {
			for right := left + 1; right < len(sources); right++ {
				if sources[left].ID == sources[right].ID {
					t.Fatalf("duplicate typed identity at %d and %d: %+v", left, right, sources[left].ID)
				}
			}
		}
		return sources
	}

	sources := assertSources(t)
	runGitTest(t, root, "pack-refs", "--all", "--prune")
	packed := assertSources(t)
	if !reflect.DeepEqual(sources, packed) {
		t.Fatalf("packed refs changed discovery\nbefore=%#v\nafter=%#v", sources, packed)
	}

	branch := sources[3]
	commits, err := client.ListRefCommits(root, branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].OID != rootOID || commits[0].Subject != "root" {
		t.Fatalf("branch preview = %#v", commits)
	}
	if !slices.ContainsFunc(commits[0].Decorations, func(decoration RefDecoration) bool {
		return decoration.Kind == RefDecorationTag && decoration.Label == "tag: v1"
	}) {
		t.Fatalf("branch preview decorations = %#v", commits[0].Decorations)
	}

	all, err := client.ListRefCommits(root, sources[0])
	if err != nil || len(all) != 2 || all[0].OID != tipOID {
		t.Fatalf("All refs preview = (%#v, %v)", all, err)
	}
}

func TestRefSourcesHandleDetachedUnbornAndRemovedWorktreesCalmly(t *testing.T) {
	t.Run("unborn", func(t *testing.T) {
		root := initGitTestRepository(t)
		client := New()
		sources, err := client.ListRefSources(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 2 || sources[0].Kind() != RefSourceAll || sources[1].Kind() != RefSourceCurrentWorktree || sources[1].OID != "" {
			t.Fatalf("unborn sources = %#v", sources)
		}
		for _, source := range sources {
			commits, previewErr := client.ListRefCommits(root, source)
			if previewErr != nil || len(commits) != 0 {
				t.Fatalf("unborn preview for %+v = (%#v, %v)", source, commits, previewErr)
			}
		}
	})

	t.Run("detached and removed linked worktree", func(t *testing.T) {
		root := initGitTestRepository(t)
		runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "root")
		oid := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
		linkedPath := filepath.Join(t.TempDir(), "detached")
		runGitTest(t, root, "worktree", "add", "-q", "--detach", linkedPath, oid)
		client := New()
		sources, err := client.ListRefSources(root)
		if err != nil {
			t.Fatal(err)
		}
		linked := slices.IndexFunc(sources, func(source RefSource) bool {
			return source.Kind() == RefSourceLinkedWorktree && source.Path == filepath.Clean(linkedPath)
		})
		if linked < 0 || !strings.HasPrefix(sources[linked].Label, "detached ") {
			t.Fatalf("detached source = %#v", sources)
		}
		removedID := sources[linked].ID
		runGitTest(t, root, "worktree", "remove", "--force", linkedPath)
		sources, err = client.ListRefSources(root)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(sources, func(source RefSource) bool { return source.ID == removedID }) {
			t.Fatalf("removed worktree survived discovery: %#v", sources)
		}
	})
}
