package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestStashesHeaderControlLoadsAndRoutesSemanticTraversal(t *testing.T) {
	t.Parallel()
	first := stashFixture("first", "stash@{0}")
	second := stashFixture("second", "stash@{1}")
	firstFiles := []repository.ChangedFile{changeFixture("a.go"), changeFixture("b.go")}
	secondFiles := []repository.ChangedFile{changeFixture("other.go")}
	source := &fakeSource{
		stashes:    []repository.Stash{first, second},
		stashFiles: map[string][]repository.ChangedFile{"first": firstFiles, "second": secondFiles},
		stashDocuments: map[string]repository.ChangeDocument{
			"first\x00" + firstFiles[0].Identity():   stashDocumentFixture(firstFiles[0], "-a\n+b"),
			"first\x00" + firstFiles[1].Identity():   stashDocumentFixture(firstFiles[1], "-b\n+c"),
			"second\x00" + secondFiles[0].Identity(): stashDocumentFixture(secondFiles[0], "-x\n+y"),
		},
	}
	model := newTestModel(source)
	model.apply(Action{Kind: Resize, Width: 100, Height: 16})
	update := func(message tea.Msg) tea.Cmd {
		next, command := model.Update(message)
		model = next.(Model)
		return command
	}
	key := func(value rune) tea.Cmd {
		return update(tea.KeyPressMsg(tea.Key{Code: value, Text: string(value)}))
	}
	if command := update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})); command != nil || model.active != workspace.Git {
		t.Fatalf("Git switch = active %v command=%v", model.active, command != nil)
	}
	if command := key('1'); command == nil || model.controls.Git != workspace.GitRefs || !model.refs.sourceLoading {
		t.Fatalf("Refs switch = controls %+v refs %+v command=%v", model.controls, model.refs, command != nil)
	}
	command := key('1')
	if command == nil || model.controls.Git != workspace.GitStashes || !model.stashes.listLoading {
		t.Fatalf("Stashes switch = controls %+v state %+v command=%v", model.controls, model.stashes, command != nil)
	}
	command = update(command())
	if command == nil || len(model.stashes.stashes) != 2 || model.stashes.filesOID != "first" {
		t.Fatalf("stash inventory landing = %+v command=%v", model.stashes, command != nil)
	}
	command = update(command())
	if command == nil || len(model.stashes.files) != 2 {
		t.Fatalf("stash files landing = %+v command=%v", model.stashes, command != nil)
	}
	update(command())
	if model.stashes.reader.Change.Path != "a.go" {
		t.Fatalf("stash reader = %+v", model.stashes.reader)
	}

	command = key('f')
	if command == nil || model.stashes.fileSelected != 1 {
		t.Fatalf("f traversal = file %d command=%v", model.stashes.fileSelected, command != nil)
	}
	update(command())
	if model.stashes.reader.Change.Path != "b.go" {
		t.Fatalf("f reader = %+v", model.stashes.reader)
	}
	command = key('F')
	if command == nil || model.stashes.fileSelected != 0 {
		t.Fatalf("F traversal = file %d command=%v", model.stashes.fileSelected, command != nil)
	}

	rowY := model.geometry.NavigatorRows.Y + 1
	command = update(tea.MouseClickMsg(tea.Mouse{X: model.geometry.NavigatorRows.X, Y: rowY, Button: tea.MouseLeft}))
	selected, _ := model.stashes.place.SelectedIdentity()
	if command == nil || selected != "second" {
		t.Fatalf("mouse stash selection = %+v command=%v", model.stashes.place, command != nil)
	}
	command = update(command())
	command = update(command())
	if command != nil || model.stashes.reader.Change.Path != "other.go" || model.stashes.place.Focus != navigation.FocusNavigator {
		t.Fatalf("mouse-selected reader/focus = stash %+v command=%v", model.stashes, command != nil)
	}
	model.files.place.ReaderOffset = 7
	model.history.place.ReaderOffset = 3
	model.stashes.place.ReaderOffset = 2
	if command := key('1'); command != nil || model.controls.Git != workspace.GitLog || model.activePlace().ReaderOffset != 3 {
		t.Fatalf("Log did not restore independent place: controls %+v place %+v", model.controls, model.activePlace())
	}
	key('1')
	if command := key('1'); command != nil || model.controls.Git != workspace.GitStashes || model.activePlace().ReaderOffset != 2 || model.stashes.reader.Change.Path != "other.go" {
		t.Fatalf("Stashes did not restore independent place: controls %+v stash %+v", model.controls, model.stashes)
	}
	model.apply(Action{Kind: ShowFiles})
	if model.files.place.ReaderOffset != 7 {
		t.Fatalf("Stashes changed Files place: %+v", model.files.place)
	}
}

func TestStashStatePreservesOIDFileAndReaderPlaceAcrossRenumbering(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.listGeneration = 1
	state.listLoading = true
	stashes := []repository.Stash{
		stashFixture("a", "stash@{0}"),
		stashFixture("b", "stash@{1}"),
		stashFixture("c", "stash@{2}"),
	}
	var pending effect
	state, pending = state.landStashes(stashesLoadedMsg{generation: 1, stashes: stashes}, 4)
	state, pending = landStashFilesForTest(state, pending, []repository.ChangedFile{changeFixture("a.go"), changeFixture("b.go")})
	state = landStashReaderForTest(state, pending, strings.Repeat("line\n", 30))

	pending = state.selectStashIndex(1, 4)
	state, pending = landStashFilesForTest(state, pending, []repository.ChangedFile{changeFixture("a.go"), changeFixture("b.go")})
	state = landStashReaderForTest(state, pending, strings.Repeat("line\n", 30))
	pending = state.selectFileDelta(1, 8)
	state = landStashReaderForTest(state, pending, strings.Repeat("line\n", 30))
	state.place.ReaderOffset = 5
	state.saveReaderPlace()

	reload := state.reload()
	renumbered := []repository.Stash{
		stashFixture("inserted", "stash@{0}"),
		stashFixture("a", "stash@{1}"),
		stashFixture("b", "stash@{2}"),
		stashFixture("c", "stash@{3}"),
	}
	state, pending = state.landStashes(stashesLoadedMsg{generation: reload.generation, stashes: renumbered}, 4)
	selected, _ := state.place.SelectedIdentity()
	if selected != "b" || state.place.Selected != 2 || state.place.ReaderOffset != 5 {
		t.Fatalf("renumbered selection/place = %q %+v", selected, state.place)
	}
	state, pending = landStashFilesForTest(state, pending, []repository.ChangedFile{changeFixture("a.go"), changeFixture("b.go")})
	if state.fileSelected != 1 || state.place.ReaderOffset != 5 {
		t.Fatalf("renumbered file place = file %d offset %d", state.fileSelected, state.place.ReaderOffset)
	}
	state = landStashReaderForTest(state, pending, strings.Repeat("line\n", 30))
	if state.place.ReaderOffset != 5 {
		t.Fatalf("reader reload reset surviving OID offset to %d", state.place.ReaderOffset)
	}

	reload = state.reload()
	dropped := []repository.Stash{
		stashFixture("inserted", "stash@{0}"),
		stashFixture("a", "stash@{1}"),
		stashFixture("c", "stash@{2}"),
	}
	state, pending = state.landStashes(stashesLoadedMsg{generation: reload.generation, stashes: dropped}, 4)
	selected, _ = state.place.SelectedIdentity()
	if selected != "c" || state.place.Selected != 2 || pending.kind != effectLoadStashFiles {
		t.Fatalf("dropped fallback = selected %q row %d effect %+v", selected, state.place.Selected, pending)
	}
}

func TestStashStateFileTraversalAndCalmEmptyErrorStates(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.listGeneration = 1
	state.listLoading = true
	state, pending := state.landStashes(stashesLoadedMsg{
		generation: 1,
		stashes:    []repository.Stash{stashFixture("stash", "stash@{0}")},
	}, 5)
	files := []repository.ChangedFile{changeFixture("one.go"), changeFixture("two.go")}
	state, pending = landStashFilesForTest(state, pending, files)
	state = landStashReaderForTest(state, pending, "-old\n+new\n")
	if state.fileSelected != 0 || state.reader.Change.Path != "one.go" {
		t.Fatalf("initial reader = file %d %+v", state.fileSelected, state.reader)
	}
	pending = state.selectFileDelta(1, 6)
	if pending.kind != effectLoadStashFile || pending.changedFile.Path != "two.go" || state.place.ReaderOffset != 0 {
		t.Fatalf("next file = effect %+v state %+v", pending, state)
	}
	state = landStashReaderForTest(state, pending, "-before\n+after\n")
	if effect := state.selectFileDelta(1, 6); effect.kind != effectNone || state.fileSelected != 1 {
		t.Fatalf("file traversal did not clamp: effect %+v selected %d", effect, state.fileSelected)
	}
	if effect := state.selectFileDelta(-1, 6); effect.kind != effectLoadStashFile || state.fileSelected != 0 {
		t.Fatalf("previous file = effect %+v selected %d", effect, state.fileSelected)
	}

	empty := newStashState()
	empty.listGeneration = 1
	empty, _ = empty.landStashes(stashesLoadedMsg{generation: 1}, 4)
	model := empty.viewModel(ui.Calculate(60, 12), time.Unix(2_000_000_000, 0))
	if model.NavigatorEmpty.Text != "No stashes yet." || model.ReaderEmpty.Text != "No stashes yet." {
		t.Fatalf("empty stash presentation = %+v", model)
	}
	emptySelected := newStashState()
	emptySelected.stashes = []repository.Stash{stashFixture("empty", "stash@{0}")}
	emptySelected.place.Items = []string{"empty"}
	emptySelected.filesOID = "empty"
	model = emptySelected.viewModel(ui.Calculate(60, 12), time.Unix(2_000_000_000, 0))
	if model.ReaderEmpty.Text != "No files stored in this stash." {
		t.Fatalf("empty selected stash presentation = %+v", model.ReaderEmpty)
	}

	state.filesGeneration++
	state.filesLoading = true
	state, _ = state.landFiles(stashFilesLoadedMsg{
		generation: state.filesGeneration, oid: "stash", err: errors.New("object disappeared"),
	})
	model = state.viewModel(ui.Calculate(60, 12), time.Unix(2_000_000_000, 0))
	if !strings.Contains(model.ReaderEmpty.Text, "no longer available") || model.ReaderEmpty.Tone != ui.ToneError {
		t.Fatalf("stale stash presentation = %+v", model.ReaderEmpty)
	}
}

func TestStashViewRendersCompactRowsTitleDiffAndSharedGeometry(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.stashes = []repository.Stash{{
		OID: "oid", Selector: "stash@{0}", Branch: "feature/reader",
		Message: "a deliberately long hostile\nmessage", Timestamp: 1_999_996_400,
		Files: 3, Additions: 12, Deletions: 4,
	}}
	state.place = navigation.State{Items: []string{"oid"}, Focus: navigation.FocusNavigator}
	state.filesOID = "oid"
	state.files = []repository.ChangedFile{
		{Path: "old.go", Kind: repository.ChangeDeleted},
		{Path: "renamed.go", PreviousPath: "before.go", Kind: repository.ChangeRenamed},
		{Path: "new.go", Kind: repository.ChangeUntracked},
	}
	state.fileSelected = 1
	state.readerOID = "oid"
	state.readerFileID = state.files[1].Identity()
	state.reader = repository.ChangeDocument{
		Change: state.files[1],
		Patch:  repository.File{Path: "renamed.go", Kind: repository.FileReady, Content: "diff --git a/before.go b/renamed.go\n@@ -1 +1 @@\n-old\n+new"},
	}

	geometry := ui.Calculate(120, 14)
	model := state.viewModel(geometry, time.Unix(2_000_000_000, 0))
	model.Workspace = workspace.Git
	model.Controls.Git = workspace.GitStashes
	frame := ui.Render(model)
	plain := ansi.Strip(frame)
	for _, want := range []string{
		"1 [stashes]", "stashes · 1", "stash@{0}", "feature/reader", "3f", "+12", "-4", "1h", "stash@{0} · 2/3 · before.go → renamed.go", "Renamed:", "old", "new", "f/F move files",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("stash frame misses %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(frame, "\x1b[35m") ||
		!strings.Contains(frame, "\x1b[32m") ||
		!strings.Contains(frame, "\x1b[31m") {
		t.Fatalf("stash selector/stats lack semantic colors: %q", frame)
	}
	rowY := geometry.NavigatorRows.Y
	hit := geometry.HitTest(geometry.NavigatorRows.X, rowY, workspace.Git, model.Controls, state.place.Top, len(state.place.Items), state.place.ReaderOffset, len(state.readerRows()))
	if hit.Kind != ui.HitNavigatorRow || hit.Index != 0 {
		t.Fatalf("stash mouse row hit = %+v", hit)
	}
	blank := geometry.HitTest(geometry.NavigatorRows.X, rowY+1, workspace.Git, model.Controls, state.place.Top, len(state.place.Items), state.place.ReaderOffset, len(state.readerRows()))
	if blank.Kind != ui.HitNavigator {
		t.Fatalf("blank stash row hit = %+v", blank)
	}

	narrow := state.viewModel(ui.Calculate(60, 12), time.Unix(2_000_000_000, 0))
	narrow.Workspace = workspace.Git
	narrow.Controls.Git = workspace.GitStashes
	narrowFrame := ansi.Strip(ui.Render(narrow))
	if !strings.Contains(narrowFrame, "stash@{0}") || !strings.Contains(narrowFrame, "feature/") || strings.Contains(narrowFrame, "deliberately long hostile↵message") {
		t.Fatalf("narrow stash row priority/clipping is incoherent:\n%s", narrowFrame)
	}
}

func TestStashRowsOmitZeroChangeTotals(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.stashes = []repository.Stash{
		{OID: "added", Selector: "stash@{0}", Files: 1, Additions: 4},
		{OID: "removed", Selector: "stash@{1}", Files: 1, Deletions: 6},
		{OID: "empty", Selector: "stash@{2}", Files: 1},
	}
	state.place = navigation.State{Items: []string{"added", "removed", "empty"}, Focus: navigation.FocusNavigator}
	model := state.viewModel(ui.Calculate(100, 12), time.Unix(2_000_000_000, 0))
	model.Workspace = workspace.Git
	model.Controls.Git = workspace.GitStashes
	plain := ansi.Strip(ui.Render(model))
	if strings.Contains(plain, "+0") || strings.Contains(plain, "-0") {
		t.Fatalf("stash rows include zero totals:\n%s", plain)
	}
	for _, want := range []string{"stash@{0}", "+4", "stash@{1}", "-6", "stash@{2}"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("stash rows miss %q:\n%s", want, plain)
		}
	}
}

func TestStashViewUsesSharedNavigatorAndReaderScrollbars(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.stashes = make([]repository.Stash, 30)
	state.place.Items = make([]string, 30)
	for index := range state.stashes {
		oid := string(rune('a' + index))
		state.stashes[index] = stashFixture(oid, "stash@{"+fmt.Sprint(index)+"}")
		state.place.Items[index] = oid
	}
	state.place.Selected = 12
	state.place.Top = 8
	state.place.Focus = navigation.FocusReader
	state.place.ReaderOffset = 10
	state.filesOID = state.stashes[12].OID
	state.files = []repository.ChangedFile{changeFixture("large.go")}
	state.readerOID = state.filesOID
	state.readerFileID = state.files[0].Identity()
	state.reader = stashDocumentFixture(state.files[0], strings.Repeat("line\n", 40))

	geometry := ui.Calculate(80, 14)
	presentation := state.viewModel(geometry, time.Unix(2_000_000_000, 0))
	presentation.Workspace = workspace.Git
	presentation.Controls.Git = workspace.GitStashes
	plain := ansi.Strip(ui.Render(presentation))
	if strings.Count(plain, "▐") < 2 {
		t.Fatalf("stash frame lacks independent shared scrollbar thumbs:\n%s", plain)
	}
	navigator, ok := ui.CalculateScrollbar(geometry.NavigatorRows, len(state.place.Items), state.place.Top)
	if !ok {
		t.Fatal("scrollable stash rows produced no navigator scrollbar")
	}
	reader, ok := ui.CalculateScrollbar(geometry.ReaderRows, len(state.readerRows()), state.place.ReaderOffset)
	if !ok {
		t.Fatal("scrollable stash diff produced no reader scrollbar")
	}
	if hit := geometry.HitTest(navigator.Thumb.X, navigator.Thumb.Y, workspace.Git, presentation.Controls, state.place.Top, len(state.place.Items), state.place.ReaderOffset, len(state.readerRows())); hit.Kind != ui.HitNavigatorScrollbar {
		t.Fatalf("stash navigator scrollbar hit = %+v", hit)
	}
	if hit := geometry.HitTest(reader.Thumb.X, reader.Thumb.Y, workspace.Git, presentation.Controls, state.place.Top, len(state.place.Items), state.place.ReaderOffset, len(state.readerRows())); hit.Kind != ui.HitReaderScrollbar {
		t.Fatalf("stash reader scrollbar hit = %+v", hit)
	}
}

func landStashFilesForTest(state stashState, pending effect, files []repository.ChangedFile) (stashState, effect) {
	return state.landFiles(stashFilesLoadedMsg{
		generation: pending.generation, oid: pending.identity, files: files,
	})
}

func landStashReaderForTest(state stashState, pending effect, patch string) stashState {
	return state.landReader(stashFileLoadedMsg{
		generation: pending.generation, oid: pending.identity,
		fileIdentity: pending.changedFile.Identity(),
		document: repository.ChangeDocument{
			Change: pending.changedFile,
			Patch:  repository.File{Path: pending.changedFile.Path, Kind: repository.FileReady, Content: patch},
		},
	})
}

func stashFixture(oid, selector string) repository.Stash {
	return repository.Stash{
		OID: oid, Selector: selector, Branch: "main", Message: oid,
		Files: 2, Additions: 2, Deletions: 1, Source: repository.ChangeSource{OID: oid, BaseOID: "base-" + oid},
	}
}

func changeFixture(path string) repository.ChangedFile {
	return repository.ChangedFile{Path: path, Kind: repository.ChangeModified, Additions: 1, Deletions: 1}
}

func stashDocumentFixture(change repository.ChangedFile, patch string) repository.ChangeDocument {
	return repository.ChangeDocument{
		Change: change,
		Patch:  repository.File{Path: change.Path, Kind: repository.FileReady, Content: patch},
	}
}
