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
		repository.Entry{Path: "one.go", State: repository.FileModified},
		repository.Entry{Path: "path/to/file.ext", State: repository.FileIgnored},
	)
	state.readerEntry = repository.Entry{Path: "path/to/file.ext", State: repository.FileIgnored}
	view := state.viewModel(ui.Calculate(80, 20))
	if view.NavigatorTitle != "2 files" || view.ReaderTitle != "path/to/file.ext" {
		t.Fatalf("pane titles = %q / %q", view.NavigatorTitle, view.ReaderTitle)
	}
	ignored := false
	for _, row := range view.NavigatorRows {
		if row.Identity == filetree.FileIdentity("path/to/file.ext") {
			ignored = row.Status == ui.StatusIgnored && row.Dimmed
		}
	}
	if !ignored {
		t.Fatalf("ignored row lacks explicit dimmed status: %#v", view.NavigatorRows)
	}
}

func TestFileTreeStartsFullyCollapsed(t *testing.T) {
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

	refresh := state.reload()
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

func TestScopeSwitchDerivesOneTreeAndPreservesRoleReaderAndFold(t *testing.T) {
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
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("surviving src fold = %#v, %v", row, ok)
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
		t.Fatalf("src fold lost returning to All: %#v, %v", row, ok)
	}
}

func TestRefreshReconcilesRenameAndDeletedReaderModes(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t, repository.Entry{Path: "old.go", State: repository.FileModified})
	state.reader = repository.File{Path: "old.go", Kind: repository.FileReady, Content: "old"}
	state.readerLoading = false
	refresh := state.reload()
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: refresh.generation,
		snapshot:   snapshotOf(repository.Entry{Path: "new.go", PreviousPath: "old.go", State: repository.FileRenamed}),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	if pending.kind != effectLoadDiff || pending.entry.Path != "new.go" || pending.entry.PreviousPath != "old.go" || state.readerEntry.Path != "new.go" {
		t.Fatalf("rename refresh = effect %+v reader %+v", pending, state.readerEntry)
	}

	deleted := repository.Entry{Path: "gone.go", State: repository.FileDeleted}
	lines := fileReaderLines(repository.File{Path: deleted.Path, Kind: repository.FileMissing}, deleted)
	if len(lines) != 1 || !strings.Contains(lines[0].Text, "deleted") {
		t.Fatalf("deleted file mode = %#v", lines)
	}
	diffLines := diffReaderLines(repository.Diff{Entry: deleted, Kind: repository.DiffReady, Content: "deleted file mode 100644\n-old"})
	if len(diffLines) < 2 || !strings.Contains(diffLines[0].Text, "deleted file mode") {
		t.Fatalf("deleted diff mode = %#v", diffLines)
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
	}, 10)
	if stale.reader.Kind != 0 || !stale.readerLoading || stale.readerMode != workspace.DiffReader {
		t.Fatalf("stale file load landed after mode change: %+v", stale)
	}
	state = state.landDiff(diffLoadedMsg{
		generation: diffEffect.generation,
		entry:      diffEffect.entry,
		diff:       repository.Diff{Entry: diffEffect.entry, Kind: repository.DiffReady, Content: "current"},
	}, 10)
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
			lines := fileReaderLines(test.file, repository.Entry{})
			texts := make([]string, len(lines))
			for index, line := range lines {
				texts[index] = line.Text
				if line.Tone != test.tone {
					t.Fatalf("tone = %v, want %v", line.Tone, test.tone)
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
