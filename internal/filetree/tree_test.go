package filetree

import (
	"reflect"
	"testing"
)

func TestTreeBuildsHierarchyAndStableOrder(t *testing.T) {
	t.Parallel()
	tree := New([]string{
		"z.go",
		"src/ui/render.go",
		"src/app.go",
		"README.md",
		"src/app.go",
	})

	want := []Row{
		{Identity: DirectoryIdentity("src"), Path: "src", Name: "src", Kind: Directory, Expanded: true},
		{Identity: FileIdentity("src/ui/render.go"), Path: "src/ui/render.go", Name: "ui/render.go", Depth: 1, Kind: File},
		{Identity: FileIdentity("src/app.go"), Path: "src/app.go", Name: "app.go", Depth: 1, Kind: File},
		{Identity: FileIdentity("README.md"), Path: "README.md", Name: "README.md", Kind: File},
		{Identity: FileIdentity("z.go"), Path: "z.go", Name: "z.go", Kind: File},
	}
	if got := tree.Rows(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Rows() = %#v, want %#v", got, want)
	}
	if got, want := tree.Files(), []string{"src/ui/render.go", "src/app.go", "README.md", "z.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Files() = %#v, want %#v", got, want)
	}
}

func TestTreeCompactsUnaryChainsAndHandlesEdgeInputs(t *testing.T) {
	t.Parallel()
	tree := New([]string{
		"",
		"bad//path.go",
		"docs/plans/2026/plan.md",
		"line\nname.go",
		"日本語/設定.nix",
	})

	want := []Row{
		{Identity: FileIdentity("docs/plans/2026/plan.md"), Path: "docs/plans/2026/plan.md", Name: "docs/plans/2026/plan.md", Kind: File},
		{Identity: FileIdentity("日本語/設定.nix"), Path: "日本語/設定.nix", Name: "日本語/設定.nix", Kind: File},
		{Identity: FileIdentity("line\nname.go"), Path: "line\nname.go", Name: "line\nname.go", Kind: File},
	}
	if got := tree.Rows(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Rows() = %#v, want %#v", got, want)
	}
}

func TestTreeCollapseExpandToggleAndRebuild(t *testing.T) {
	t.Parallel()
	tree := New([]string{"src/a.go", "src/b.go", "test/x.go", "test/y.go"})
	src := DirectoryIdentity("src")

	if !tree.Collapse(src) {
		t.Fatal("Collapse(src) = false, want state change")
	}
	if got, want := tree.Identities(), []string{src, DirectoryIdentity("test"), FileIdentity("test/x.go"), FileIdentity("test/y.go")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collapsed identities = %#v, want %#v", got, want)
	}
	if tree.Collapse(src) || tree.Collapse(FileIdentity("test/x.go")) {
		t.Fatal("collapse changed an already-collapsed directory or file")
	}
	if !tree.Expand(src) || tree.Expand(src) {
		t.Fatal("expand transition did not change exactly once")
	}
	if !tree.Toggle(src) || !tree.Toggle(src) {
		t.Fatal("toggle failed to reverse directory state")
	}

	tree.Collapse(src)
	tree.Rebuild([]string{"src/c.go", "src/d.go", "z.go"})
	row, ok := tree.Row(src)
	if !ok || row.Expanded {
		t.Fatalf("surviving src row = %#v, %v; want collapsed", row, ok)
	}
	if got, want := tree.Files(), []string{"src/c.go", "src/d.go", "z.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("complete files = %#v, want %#v", got, want)
	}

	tree.Rebuild([]string{"other/a.go", "other/b.go"})
	if tree.Expand(src) {
		t.Fatal("removed directory retained actionable collapse state")
	}
}

func TestCollapseAllKeepsNestedDirectoriesCollapsed(t *testing.T) {
	t.Parallel()
	tree := New([]string{
		"src/a.go",
		"src/b.go",
		"src/ui/render.go",
		"src/ui/theme.go",
		"root.go",
	})

	tree.CollapseAll()
	if got, want := tree.Identities(), []string{DirectoryIdentity("src"), FileIdentity("root.go")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collapsed identities = %#v, want %#v", got, want)
	}
	for _, path := range []string{"src", "src/ui"} {
		row, ok := tree.Row(DirectoryIdentity(path))
		if !ok || row.Expanded {
			t.Fatalf("collapsed row %q = %#v, %v", path, row, ok)
		}
	}

	if !tree.Expand(DirectoryIdentity("src")) {
		t.Fatal("top-level directory did not expand")
	}
	row, ok := tree.Row(DirectoryIdentity("src/ui"))
	if !ok || row.Expanded {
		t.Fatalf("nested directory after parent expansion = %#v, %v; want collapsed", row, ok)
	}
}

func TestFoldStateRestoresAuthoredFoldersAndDefaultsNewFoldersByScope(t *testing.T) {
	t.Parallel()
	tree := New([]string{"src/a.go", "src/b.go", "test/a.go", "test/b.go"})
	if !tree.Collapse(DirectoryIdentity("src")) {
		t.Fatal("fixture src did not collapse")
	}
	state := tree.Folds()

	tree.Rebuild([]string{"src/c.go", "src/d.go", "new/a.go", "new/b.go"})
	tree.RestoreFolds(state, true)
	src, _ := tree.Row(DirectoryIdentity("src"))
	newDirectory, _ := tree.Row(DirectoryIdentity("new"))
	if src.Expanded || newDirectory.Expanded {
		t.Fatalf("collapsed-default restore = src %+v new %+v", src, newDirectory)
	}

	tree.RestoreFolds(state, false)
	src, _ = tree.Row(DirectoryIdentity("src"))
	newDirectory, _ = tree.Row(DirectoryIdentity("new"))
	if src.Expanded || !newDirectory.Expanded {
		t.Fatalf("expanded-default restore = src %+v new %+v", src, newDirectory)
	}
}

func TestFirstVisibleFileSkipsDirectories(t *testing.T) {
	t.Parallel()
	tree := New([]string{"src/a.go", "src/b.go", "root.go"})
	row, ok := tree.FirstVisibleFile()
	if !ok || row.Path != "src/a.go" || row.Kind != File {
		t.Fatalf("FirstVisibleFile() = %#v, %v", row, ok)
	}
	tree.Collapse(DirectoryIdentity("src"))
	row, ok = tree.FirstVisibleFile()
	if !ok || row.Path != "root.go" {
		t.Fatalf("collapsed FirstVisibleFile() = %#v, %v", row, ok)
	}
}

func TestExpandParentsRevealsOnlyTheRequestedHiddenFile(t *testing.T) {
	tree := New([]string{"a/deep/one.go", "a/deep/two.go", "b/deep/three.go", "b/deep/four.go", "root.go"})
	if !tree.Collapse(DirectoryIdentity("a/deep")) || !tree.Collapse(DirectoryIdentity("b/deep")) {
		t.Fatal("fixture directories did not collapse")
	}
	if !tree.ExpandParents("a/deep/two.go") {
		t.Fatal("ExpandParents reported no change")
	}
	a, _ := tree.Row(DirectoryIdentity("a/deep"))
	b, _ := tree.Row(DirectoryIdentity("b/deep"))
	if !a.Expanded || b.Expanded {
		t.Fatalf("folds after expansion: a=%+v b=%+v", a, b)
	}
	if tree.ExpandParents("a/deep/two.go") {
		t.Fatal("already visible path reported a change")
	}
}
