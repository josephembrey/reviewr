package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestFilePaneTitlesAndStatusDescribeTypedEntries(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t,
		repository.Entry{Path: "one.go", State: repository.FileModified, Additions: 12, Deletions: 3},
		repository.Entry{Path: "path/to/file.ext", State: repository.FileIgnored},
	)
	state.readerEntry = repository.Entry{Path: "path/to/file.ext", State: repository.FileIgnored}
	view := state.viewModel(ui.Calculate(80, 20))
	if view.NavigatorTitle != "2 files" || view.ReaderTitle != "path/to/file.ext" {
		t.Fatalf("pane titles = %q / %q", view.NavigatorTitle, view.ReaderTitle)
	}
	ignored := false
	var changes ui.LineChanges
	foundChanges := false
	for _, row := range view.NavigatorRows {
		if row.Identity == filetree.FileIdentity("path/to/file.ext") {
			ignored = row.Status == ui.StatusIgnored && row.Dimmed
		}
		if row.Identity == filetree.FileIdentity("one.go") {
			changes = row.Changes
			foundChanges = true
		}
	}
	if !ignored {
		t.Fatalf("ignored row lacks explicit dimmed status: %#v", view.NavigatorRows)
	}
	if !foundChanges || changes != (ui.LineChanges{Additions: 12, Deletions: 3}) {
		t.Fatalf("changed row stats = %#v", changes)
	}
}

func TestFullyIgnoredDirectoriesAreDimmed(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t,
		repository.Entry{Path: "ignored/cache/one.bin", State: repository.FileIgnored},
		repository.Entry{Path: "ignored/cache/two.bin", State: repository.FileIgnored},
		repository.Entry{Path: "mixed/ignored.log", State: repository.FileIgnored},
		repository.Entry{Path: "mixed/tracked.go", State: repository.FileUnchanged},
	)

	rows := make(map[string]ui.NavigatorRow)
	for _, row := range state.navigatorRows() {
		rows[row.Identity] = row
	}
	ignored := rows[filetree.DirectoryIdentity("ignored/cache")]
	if !ignored.Directory || ignored.Status != ui.StatusIgnored || !ignored.Dimmed {
		t.Fatalf("fully ignored directory = %#v, want ignored presentation", ignored)
	}
	mixed := rows[filetree.DirectoryIdentity("mixed")]
	if !mixed.Directory || mixed.Status == ui.StatusIgnored || mixed.Dimmed {
		t.Fatalf("mixed directory = %#v, want ordinary presentation", mixed)
	}
}

func TestFileTreeDefaultsMatchExplorerAndSourceControlScopes(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot: snapshotOf(
			repository.Entry{Path: "src/a.go"},
			repository.Entry{Path: "src/b.go"},
			repository.Entry{Path: "src/ui/render.go"},
			repository.Entry{Path: "src/ui/theme.go"},
			repository.Entry{Path: "root.go"},
		),
	}, workspace.AllFiles, workspace.FileReader, 10)

	if got, want := state.place.Items, []string{filetree.DirectoryIdentity("src"), filetree.FileIdentity("root.go")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("initial tree identities = %#v, want %#v", got, want)
	}
	for _, path := range []string{"src", "src/ui"} {
		row, ok := state.tree.Row(filetree.DirectoryIdentity(path))
		if !ok || row.Expanded {
			t.Fatalf("initial directory %q = %#v, %v; want collapsed", path, row, ok)
		}
	}
	if pending.entry.Path != "root.go" || state.readerEntry.Path != "root.go" {
		t.Fatalf("initial visible reader = effect %+v state %+v", pending, state.readerEntry)
	}

	state.selectIdentity(filetree.DirectoryIdentity("src"))
	if !state.expandSelected(10) {
		t.Fatal("selected top-level directory did not expand")
	}
	row, _ := state.tree.Row(filetree.DirectoryIdentity("src/ui"))
	if row.Expanded {
		t.Fatalf("nested directory expanded with its parent: %+v", row)
	}

	changed := newFilesState()
	changed, _ = changed.landSnapshot(snapshotLoadedMsg{
		generation: changed.listGeneration,
		snapshot: snapshotOf(
			repository.Entry{Path: "src/a.go", State: repository.FileModified},
			repository.Entry{Path: "src/b.go", State: repository.FileModified},
			repository.Entry{Path: "src/ui/render.go", State: repository.FileModified},
			repository.Entry{Path: "src/ui/theme.go", State: repository.FileModified},
		),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	for _, path := range []string{"src", "src/ui"} {
		row, ok := changed.tree.Row(filetree.DirectoryIdentity(path))
		if !ok || !row.Expanded {
			t.Fatalf("initial Changed directory %q = %#v, %v; want expanded", path, row, ok)
		}
	}
}

func TestFileSelectionSeparatesTreeCursorFromOpenReader(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot: snapshotOf(
			repository.Entry{Path: "src/a.go"},
			repository.Entry{Path: "src/b.go"},
			repository.Entry{Path: "root.go"},
		),
	}, workspace.AllFiles, workspace.FileReader, 10)
	if pending.entry.Path != "root.go" || state.readerEntry.Path != "root.go" {
		t.Fatalf("initial file = %q effect=%+v", state.readerEntry.Path, pending)
	}
	if !state.tree.Expand(filetree.DirectoryIdentity("src")) {
		t.Fatal("src did not expand for selection test")
	}
	state.reconcileVisibleRows(10)
	state.selectIdentity(filetree.FileIdentity("src/a.go"))
	entry, _ := state.entry("src/a.go")
	state.requestReader(entry, workspace.FileReader)
	state.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "content"}
	state.readerLoading = false
	state.place.ReaderOffset = 7

	directoryIndex := identityIndex(state.place.Items, filetree.DirectoryIdentity("src"))
	if effect := state.selectIndex(directoryIndex, 10, workspace.FileReader); effect.kind != effectNone {
		t.Fatalf("directory selection effect = %+v", effect)
	}
	if state.readerEntry.Path != "src/a.go" || state.reader.Content != "content" || state.place.ReaderOffset != 7 {
		t.Fatalf("directory selection changed reader: %+v", state)
	}

	fileIndex := identityIndex(state.place.Items, filetree.FileIdentity("src/b.go"))
	if effect := state.selectIndex(fileIndex, 10, workspace.FileReader); effect.entry.Path != "src/b.go" {
		t.Fatalf("file selection effect = %+v", effect)
	}
	if state.readerEntry.Path != "src/b.go" || state.reader.Kind != 0 || state.place.ReaderOffset != 0 {
		t.Fatalf("file selection state = %+v", state)
	}
}

func TestFileRefreshPreservesHiddenOpenFileAndCollapsedDirectory(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t,
		repository.Entry{Path: "src/a.go"},
		repository.Entry{Path: "src/b.go"},
		repository.Entry{Path: "root.go"},
	)
	state.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "open"}
	state.readerEntry = repository.Entry{Path: "src/a.go"}
	state.readerLoading = false
	state.place.ReaderOffset = 4
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("src did not start collapsed: %#v, %v", row, ok)
	}
	state.place.Reconcile(state.tree.Identities())
	state.place.EnsureSelectionVisible(10)

	refresh := state.reload("uncommitted")
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: refresh.generation,
		snapshot: snapshotOf(
			repository.Entry{Path: "src/a.go"},
			repository.Entry{Path: "src/b.go"},
			repository.Entry{Path: "src/c.go"},
			repository.Entry{Path: "root.go"},
		),
	}, workspace.AllFiles, workspace.FileReader, 10)

	row, ok := state.tree.Row(filetree.DirectoryIdentity("src"))
	if !ok || row.Expanded {
		t.Fatalf("src after refresh = %#v, %v; want collapsed", row, ok)
	}
	if pending.entry.Path != "src/a.go" || state.readerEntry.Path != "src/a.go" || state.reader.Content != "open" || state.place.ReaderOffset != 4 {
		t.Fatalf("hidden open file refresh = effect %+v state %+v", pending, state)
	}
	for _, identity := range state.place.Items {
		if identity == filetree.FileIdentity("src/a.go") {
			t.Fatal("collapsed descendant became visible during refresh")
		}
	}
}

func TestComparisonSwitchBlanksOnlyTheMismatchedDerivedView(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{{Path: "changed.go", State: repository.FileModified}},
			repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: "head"},
		),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	state.readerEntry = repository.Entry{Path: "changed.go", State: repository.FileModified}
	state.diff = repository.Diff{Kind: repository.DiffReady, Content: "old comparison"}
	presentation := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{{Identity: "old", Text: "old comparison"}}}
	state.readerPresentation = &presentation
	state.readerLoading = false
	selected, _ := state.place.SelectedIdentity()

	pending := state.reload(repository.ComparisonBranch)
	view := state.viewModel(ui.Calculate(80, 20))
	if !state.comparisonPending() || len(view.NavigatorRows) != 0 || len(view.ReaderDocument.Rows) != 0 ||
		view.ReaderEmpty.Text != "Loading comparison…" || state.tree.FileCount() != 1 {
		t.Fatalf("pending comparison view leaked stale content: state=%+v view=%+v", state, view)
	}
	if current, _ := state.place.SelectedIdentity(); current != selected {
		t.Fatalf("pending comparison moved selection from %q to %q", selected, current)
	}

	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation: pending.generation,
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{{Path: "changed.go", State: repository.FileModified}},
			repository.Comparison{Scope: repository.ComparisonBranch, Basis: "base"},
		),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	if state.comparisonPending() {
		t.Fatal("landed comparison remained pending")
	}
	if current, _ := state.place.SelectedIdentity(); current != selected {
		t.Fatalf("landed comparison moved selection from %q to %q", selected, current)
	}
}

func TestScopeSwitchUsesIndependentFoldsAndPreservesRoleReader(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t,
		repository.Entry{Path: "src/a.go", State: repository.FileUnchanged},
		repository.Entry{Path: "src/b.go", State: repository.FileModified},
		repository.Entry{Path: "src/c.go", State: repository.FileUntracked},
		repository.Entry{Path: "changed.go", State: repository.FileModified},
		repository.Entry{Path: "ignored.log", State: repository.FileIgnored},
		repository.Entry{Path: "z.go", State: repository.FileUnchanged},
	)
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("src did not start collapsed: %#v, %v", row, ok)
	}
	state.place.Reconcile(state.tree.Identities())
	state.selectIdentity(filetree.FileIdentity("z.go"))
	state.readerEntry = repository.Entry{Path: "z.go", State: repository.FileUnchanged}
	state.reader = repository.File{Path: "z.go", Kind: repository.FileReady, Content: "z"}
	state.readerLoading = false
	state.place.ReaderOffset = 3

	pending := state.switchScope(workspace.ChangedFiles, workspace.FileReader, 10)
	selected, _ := state.place.SelectedIdentity()
	selectedRow, _ := state.tree.Row(selected)
	if selectedRow.Kind != filetree.File {
		t.Fatalf("file cursor fell back across role to %#v", selectedRow)
	}
	if pending.entry.Path == "" || pending.entry.Path == "z.go" || state.readerEntry.Path != pending.entry.Path {
		t.Fatalf("reader fallback = %+v / %+v", pending, state.readerEntry)
	}
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || !row.Expanded {
		t.Fatalf("first Changed projection did not default src open: %#v, %v", row, ok)
	}
	state.selectIdentity(filetree.DirectoryIdentity("src"))
	if !state.collapseSelected(10) {
		t.Fatal("authored Changed fold did not collapse src")
	}
	for _, entry := range state.entries {
		if entry.State == repository.FileIgnored || entry.State == repository.FileUnchanged {
			t.Fatalf("Changed contains excluded entry: %+v", entry)
		}
	}

	readerPath := state.readerEntry.Path
	state.reader = repository.File{Path: readerPath, Kind: repository.FileReady, Content: "b"}
	state.readerLoading = false
	state.place.ReaderOffset = 2
	if effect := state.switchScope(workspace.AllFiles, workspace.FileReader, 10); effect.kind != effectNone {
		t.Fatalf("surviving reader reloaded on scope switch: %+v", effect)
	}
	if state.readerEntry.Path != readerPath || state.reader.Content != "b" || state.place.ReaderOffset != 2 {
		t.Fatalf("surviving reader place changed: %+v", state)
	}
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("All fold state lost after visiting Changed: %#v, %v", row, ok)
	}
	_ = state.switchScope(workspace.ChangedFiles, workspace.FileReader, 10)
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("authored Changed fold state was not restored: %#v, %v", row, ok)
	}
}

func TestRefreshReconcilesRenameAndDeletedReaderModes(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t, repository.Entry{Path: "old.go", State: repository.FileModified})
	state.reader = repository.File{Path: "old.go", Kind: repository.FileReady, Content: "old"}
	state.readerLoading = false
	refresh := state.reload("uncommitted")
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: refresh.generation,
		snapshot:   snapshotOf(repository.Entry{Path: "new.go", PreviousPath: "old.go", State: repository.FileRenamed}),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	if pending.kind != effectLoadDiff || pending.entry.Path != "new.go" || pending.entry.PreviousPath != "old.go" || state.readerEntry.Path != "new.go" {
		t.Fatalf("rename refresh = effect %+v reader %+v", pending, state.readerEntry)
	}

	deleted := repository.Entry{Path: "gone.go", State: repository.FileDeleted}
	rows := fileReaderDocument(repository.File{Path: deleted.Path, Kind: repository.FileMissing}, deleted).Rows
	if len(rows) != 1 || !strings.Contains(rows[0].Text, "deleted") {
		t.Fatalf("deleted file mode = %#v", rows)
	}
	diffRows := diffReaderDocument(repository.Diff{Entry: deleted, Kind: repository.DiffReady, Content: "deleted file mode 100644\n@@ -1 +0,0 @@\n-old"}).Rows
	if len(diffRows) < 2 || !strings.Contains(diffRows[0].Text, "deleted file mode") {
		t.Fatalf("deleted diff mode = %#v", diffRows)
	}
}

func TestFileAndDiffLoadsAreLatestWinsAcrossModeChanges(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, fileEffect := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot:   snapshotOf(repository.Entry{Path: "changed.go", State: repository.FileModified}),
	}, workspace.ChangedFiles, workspace.FileReader, 10)
	diffEffect := state.requestMode(workspace.DiffReader)
	if fileEffect.generation >= diffEffect.generation || diffEffect.kind != effectLoadDiff {
		t.Fatalf("reader generations = file %+v diff %+v", fileEffect, diffEffect)
	}

	stale := state.landFile(fileLoadedMsg{
		generation: fileEffect.generation,
		entry:      fileEffect.entry,
		file:       repository.File{Path: "changed.go", Kind: repository.FileReady, Content: "stale"},
	})
	if stale.reader.Kind != 0 || !stale.readerLoading || stale.readerMode != workspace.DiffReader {
		t.Fatalf("stale file load landed after mode change: %+v", stale)
	}
	state = state.landDiff(diffLoadedMsg{
		generation: diffEffect.generation,
		entry:      diffEffect.entry,
		diff:       repository.Diff{Entry: diffEffect.entry, Kind: repository.DiffReady, Content: "current"},
	})
	if state.diff.Content != "current" || state.readerLoading {
		t.Fatalf("current diff did not land: %+v", state)
	}
}

func TestFileReaderLinesPreserveExplicitSafetyStates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file repository.File
		want string
		tone ui.Tone
	}{
		{name: "ready", file: repository.File{Kind: repository.FileReady, Content: "one\ntwo"}, want: "one\ntwo"},
		{name: "symlink", file: repository.File{Kind: repository.FileReady, Symlink: true, Content: "target"}, want: "symlink → target"},
		{name: "missing", file: repository.File{Kind: repository.FileMissing}, want: "File is missing from the worktree.", tone: ui.ToneError},
		{name: "unreadable", file: repository.File{Kind: repository.FileUnreadable, Err: errors.New("denied")}, want: "File is unreadable: denied", tone: ui.ToneError},
		{name: "binary", file: repository.File{Kind: repository.FileBinary, Size: 42}, want: "Binary file (42 bytes); plain reader disabled.", tone: ui.ToneError},
		{name: "too large", file: repository.File{Kind: repository.FileTooLarge, Size: repository.DefaultMaxFileBytes + 1}, want: "File is too large", tone: ui.ToneError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rows := fileReaderDocument(test.file, repository.Entry{}).Rows
			texts := make([]string, len(rows))
			for index, row := range rows {
				texts[index] = row.Text
				if row.Tone != test.tone {
					t.Fatalf("tone = %v, want %v", row.Tone, test.tone)
				}
			}
			if got := strings.Join(texts, "\n"); !strings.Contains(got, test.want) {
				t.Fatalf("reader lines = %q, want substring %q", got, test.want)
			}
		})
	}
}

func loadedFilesState(t *testing.T, entries ...repository.Entry) filesState {
	t.Helper()
	state := newFilesState()
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot:   snapshotOf(entries...),
	}, workspace.AllFiles, workspace.FileReader, 10)
	return state
}

func snapshotOf(entries ...repository.Entry) repository.Snapshot {
	return repository.NewSnapshot(entries)
}

func identityIndex(identities []string, want string) int {
	for index, identity := range identities {
		if identity == want {
			return index
		}
	}
	return -1
}
