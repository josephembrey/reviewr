package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type fakeSource struct {
	files       []string
	listErr     error
	contents    map[string]repository.File
	summary     repository.ChangeSummary
	summaryErr  error
	commits     []repository.Commit
	commitErr   error
	summaries   map[string]repository.CommitSummary
	summaryErrs map[string]error
}

func (s *fakeSource) ListFiles() ([]string, error) {
	return append([]string(nil), s.files...), s.listErr
}

func (s *fakeSource) ReadFile(path string) repository.File {
	if file, ok := s.contents[path]; ok {
		return file
	}
	return repository.File{Path: path, Kind: repository.FileMissing, Err: errors.New("missing")}
}

func (s *fakeSource) WorktreeSummary() (repository.ChangeSummary, error) {
	return s.summary, s.summaryErr
}

func (s *fakeSource) ListCommits() ([]repository.Commit, error) {
	return append([]repository.Commit(nil), s.commits...), s.commitErr
}

func (s *fakeSource) ReadCommit(oid string) (repository.CommitSummary, error) {
	if err := s.summaryErrs[oid]; err != nil {
		return repository.CommitSummary{}, err
	}
	if summary, ok := s.summaries[oid]; ok {
		return summary, nil
	}
	return repository.CommitSummary{}, errors.New("missing commit")
}

func TestRootFileLoadSelectAndRefreshFlow(t *testing.T) {
	t.Parallel()
	source := &fakeSource{
		files:   []string{"a", "b"},
		summary: repository.ChangeSummary{Files: 2, Additions: 4, Deletions: 1},
		contents: map[string]repository.File{
			"a": {Path: "a", Kind: repository.FileReady, Content: "a1\na2"},
			"b": {Path: "b", Kind: repository.FileReady, Content: "b1\nb2"},
		},
	}
	model := newTestModel(source)
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})

	initial := model.Init()
	if initial == nil {
		t.Fatal("Init() returned no repository command")
	}
	batch, ok := initial().(tea.BatchMsg)
	if !ok || len(batch) != 3 {
		t.Fatalf("Init() message = %T, want three-command warmup batch", initial())
	}
	var loaded filesLoadedMsg
	var historyLoaded bool
	for _, command := range batch {
		switch message := command().(type) {
		case filesLoadedMsg:
			loaded = message
		case commitsLoadedMsg:
			historyLoaded = true
			next, _ := model.Update(message)
			model = next.(Model)
		case summaryLoadedMsg:
			next, _ := model.Update(message)
			model = next.(Model)
		}
	}
	if model.summary.value != source.summary {
		t.Fatalf("initial summary = %+v, want %+v", model.summary.value, source.summary)
	}
	if !historyLoaded || !model.history.loaded || model.history.listLoading {
		t.Fatalf("initial history was not warmed: %+v", model.history)
	}
	next, contentCommand := model.Update(loaded)
	model = next.(Model)
	if model.files.readerPath != "a" || !model.files.readerLoading || contentCommand == nil {
		t.Fatalf("after list load: %+v", model.files)
	}
	next, _ = model.Update(contentCommand())
	model = next.(Model)
	if model.files.reader.Content != "a1\na2" || model.files.readerLoading {
		t.Fatalf("reader did not land: %+v", model.files.reader)
	}

	selectEffect := model.apply(Action{Kind: SelectNext})
	if selectEffect.kind != effectLoadFile || selectEffect.identity != "b" || model.files.readerPath != "b" || model.files.reader.Kind != 0 {
		t.Fatalf("selection effect/state = %+v / %+v", selectEffect, model.files)
	}
	next, _ = model.Update(model.command(selectEffect)())
	model = next.(Model)
	if model.files.reader.Content != "b1\nb2" {
		t.Fatalf("selected content = %q", model.files.reader.Content)
	}

	reloadEffect := model.apply(Action{Kind: Reload})
	if reloadEffect.kind != effectLoadFiles || !model.files.listLoading {
		t.Fatalf("reload effect/state = %+v / %+v", reloadEffect, model.files)
	}
	refreshResult := model.command(reloadEffect)().(filesLoadedMsg)
	model.files, selectEffect = model.files.landFiles(refreshResult, model.geometry.NavigatorRows.Height)
	if model.files.reader.Content != "b1\nb2" || !model.files.readerLoading || selectEffect.identity != "b" {
		t.Fatalf("same-identity refresh discarded content: %+v / %+v", model.files, selectEffect)
	}
}

func TestPrimaryWorkspaceLifecycleAndActiveRefresh(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	if model.active != workspace.Files || !model.files.listLoading || !model.history.listLoading {
		t.Fatalf("initial workspace state = %+v", model)
	}
	filesGeneration := model.files.listGeneration
	gitLoad := model.apply(Action{Kind: ShowGit})
	if model.active != workspace.Git || gitLoad.kind != effectNone || !model.history.listLoading {
		t.Fatalf("Git activation started a duplicate warmup: active %v effect %+v history %+v", model.active, gitLoad, model.history)
	}
	gitGeneration := model.history.listGeneration
	model.history, _ = model.history.landCommits(commitsLoadedMsg{generation: gitGeneration}, 10)
	if !model.history.loaded || model.history.listLoading {
		t.Fatalf("empty Git history did not become loaded: %+v", model.history)
	}
	if effect := model.apply(Action{Kind: ShowFiles}); effect.kind != effectNone || model.active != workspace.Files {
		t.Fatalf("switch to loading Files = active %v effect %+v", model.active, effect)
	}
	if effect := model.apply(Action{Kind: ShowGit}); effect.kind != effectNone {
		t.Fatalf("return to loaded Git reloaded: %+v", effect)
	}
	gitRefresh := model.apply(Action{Kind: Reload})
	if gitRefresh.kind != effectLoadCommits || model.history.listGeneration <= gitGeneration || model.files.listGeneration != filesGeneration {
		t.Fatalf("Git refresh crossed workspace generations: %+v", model)
	}
	model.apply(Action{Kind: ShowFiles})
	filesRefresh := model.apply(Action{Kind: Reload})
	if filesRefresh.kind != effectLoadFiles || model.files.listGeneration <= filesGeneration || model.history.listGeneration != gitRefresh.generation {
		t.Fatalf("Files refresh crossed workspace generations: %+v", model)
	}
}

func TestWorkspaceToggleChangesHeaderAndBodyInSameFrame(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.geometry = ui.Calculate(80, 24)
	model.files.tree.Rebuild([]string{"file.go"})
	model.files.place.Reconcile(model.files.tree.Identities())
	filesFrame := ansi.Strip(model.View().Content)
	if !strings.HasPrefix(filesFrame, "1 [files] git  | esc  scratch") || !strings.Contains(filesFrame, "\n1 files") {
		t.Fatalf("Files frame = %q", filesFrame)
	}

	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model = next.(Model)
	if model.active != workspace.Git || command != nil {
		t.Fatalf("workspace toggle = active %v command=%v", model.active, command != nil)
	}
	gitFrame := ansi.Strip(model.View().Content)
	if !strings.HasPrefix(gitFrame, "1  files [git] | esc  scratch") || !strings.Contains(gitFrame, "\n0 commits") || strings.Contains(gitFrame, "Navigator") {
		t.Fatalf("Git frame = %q", gitFrame)
	}
}

func TestScratchIsAStubThatPreservesPrimaryWorkspace(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 24})
	model.files.place = navigation.State{Items: []string{"a", "b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 3}

	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if !model.scratch || model.active != workspace.Files || command != nil {
		t.Fatalf("Scratch activation = scratch %v primary %v command=%v", model.scratch, model.active, command != nil)
	}
	frame := ansi.Strip(model.View().Content)
	if !strings.Contains(frame, "Scratch editor coming next.") || strings.Contains(frame, "│") || strings.Contains(frame, "Navigator") {
		t.Fatalf("Scratch stub frame = %q", frame)
	}
	if model.files.place.Selected != 1 || model.files.place.Top != 1 || model.files.place.ReaderOffset != 3 {
		t.Fatalf("Scratch activation changed Files place: %+v", model.files.place)
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model = next.(Model)
	if model.scratch || model.active != workspace.Files {
		t.Fatalf("1 from Scratch = scratch %v primary %v", model.scratch, model.active)
	}

	model.apply(Action{Kind: ShowGit})
	model.apply(Action{Kind: ShowScratch})
	model.apply(Action{Kind: ToggleWorkspace})
	if model.scratch || model.active != workspace.Git {
		t.Fatalf("1 from Scratch after Git = scratch %v primary %v", model.scratch, model.active)
	}
}

func TestFilesDirectoryFoldingKeysAndMousePreserveReader(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	model.files, _ = model.files.landFiles(filesLoadedMsg{
		generation: model.files.listGeneration,
		files:      []string{"src/a.go", "src/b.go", "root.go"},
	}, model.geometry.NavigatorRows.Height)
	model.files.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: strings.Repeat("line\n", 20)}
	model.files.readerLoading = false
	model.files.place.ReaderOffset = 3

	update := func(msg tea.Msg) tea.Cmd {
		next, command := model.Update(msg)
		model = next.(Model)
		return command
	}
	if command := update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"})); command != nil {
		t.Fatal("directory selection produced a reader command")
	}
	selected, _ := model.files.place.SelectedIdentity()
	if selected != filetree.DirectoryIdentity("src") || model.files.readerPath != "src/a.go" || model.files.place.ReaderOffset != 3 {
		t.Fatalf("directory selection = %q files %+v", selected, model.files)
	}

	update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	row, _ := model.files.tree.Row(filetree.DirectoryIdentity("src"))
	if row.Expanded || len(model.files.place.Items) != 2 || model.files.readerPath != "src/a.go" || model.files.place.ReaderOffset != 3 {
		t.Fatalf("collapsed tree = row %+v files %+v", row, model.files)
	}
	update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if len(model.files.place.Items) != 2 {
		t.Fatal("repeated collapse changed visible rows")
	}
	update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	row, _ = model.files.tree.Row(filetree.DirectoryIdentity("src"))
	if !row.Expanded || len(model.files.place.Items) != 4 {
		t.Fatalf("expanded tree = row %+v items %#v", row, model.files.place.Items)
	}

	directoryY := model.geometry.NavigatorRows.Y + model.files.place.Selected - model.files.place.Top
	update(tea.MouseClickMsg(tea.Mouse{X: model.geometry.NavigatorRows.X, Y: directoryY, Button: tea.MouseLeft}))
	row, _ = model.files.tree.Row(filetree.DirectoryIdentity("src"))
	if row.Expanded || model.files.place.Focus != navigation.FocusNavigator || model.files.readerPath != "src/a.go" || model.files.place.ReaderOffset != 3 {
		t.Fatalf("mouse fold = row %+v files %+v", row, model.files)
	}
}

func TestMinimumSizeScreenGatesPlaceInputAndRecoversOnResize(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: ui.MinimumWidth - 1, Height: ui.MinimumHeight - 1})

	frame := ansi.Strip(model.View().Content)
	if !strings.Contains(frame, "terminal too small") || !strings.Contains(frame, "minimum  60 × 12") {
		t.Fatalf("undersized frame = %q", frame)
	}
	for _, key := range []tea.Key{
		{Code: '1', Text: "1"},
		{Code: tea.KeyEscape},
		{Code: '2', Text: "2"},
		{Code: 'j', Text: "j"},
	} {
		next, command := model.Update(tea.KeyPressMsg(key))
		model = next.(Model)
		if command != nil || model.active != workspace.Files || model.scratch || model.controls != (workspace.Controls{}) {
			t.Fatalf("undersized input %q changed place: %+v command=%v", key.String(), model, command != nil)
		}
	}

	// Repository completions remain world-state updates and continue warming
	// while place input is gated.
	next, _ := model.Update(filesLoadedMsg{generation: model.files.listGeneration, files: []string{"ready.go"}})
	model = next.(Model)
	if !model.files.loaded {
		t.Fatal("minimum-size screen blocked repository warmup")
	}
	if _, quit := model.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"})); quit == nil {
		t.Fatal("minimum-size screen blocked quit")
	}

	next, command := model.Update(tea.WindowSizeMsg{Width: ui.MinimumWidth, Height: ui.MinimumHeight})
	model = next.(Model)
	if command != nil || !ui.MeetsMinimumSize(model.geometry.Screen.Width, model.geometry.Screen.Height) {
		t.Fatalf("minimum-size recovery = geometry %+v command=%v", model.geometry.Screen, command != nil)
	}
	frame = ansi.Strip(model.View().Content)
	if strings.Contains(frame, "terminal too small") || !strings.HasPrefix(frame, "1 [files]") {
		t.Fatalf("recovered frame = %q", frame)
	}
}

func TestDividerDragClampsAndPersistsTheUserSplit(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 24})
	original := model.geometry.Divider.X

	update := func(msg tea.Msg) tea.Cmd {
		next, command := model.Update(msg)
		model = next.(Model)
		return command
	}
	if command := update(tea.MouseClickMsg(tea.Mouse{X: original, Y: model.geometry.Divider.Y, Button: tea.MouseLeft})); command != nil || !model.layout.dragging {
		t.Fatalf("divider press = dragging %v command=%v", model.layout.dragging, command != nil)
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: 1, Y: model.geometry.Divider.Y, Button: tea.MouseLeft}))
	if model.geometry.Navigator.Width != ui.MinimumPaneWidth {
		t.Fatalf("left-clamped navigator width = %d, want %d", model.geometry.Navigator.Width, ui.MinimumPaneWidth)
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: 79, Y: model.geometry.Divider.Y, Button: tea.MouseLeft}))
	if model.geometry.Reader.Width != ui.MinimumPaneWidth {
		t.Fatalf("right-clamped reader width = %d, want %d", model.geometry.Reader.Width, ui.MinimumPaneWidth)
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: 34, Y: model.geometry.Divider.Y, Button: tea.MouseLeft}))
	if model.geometry.Divider.X != 34 || model.geometry.Navigator.Width != 34 {
		t.Fatalf("dragged geometry = %+v", model.geometry)
	}
	update(tea.MouseReleaseMsg(tea.Mouse{X: 34, Y: model.geometry.Divider.Y, Button: tea.MouseLeft}))
	if model.layout.dragging {
		t.Fatal("mouse release left divider drag active")
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: 50, Y: model.geometry.Divider.Y, Button: tea.MouseLeft}))
	if model.geometry.Divider.X != 34 {
		t.Fatalf("motion after release moved divider to %d", model.geometry.Divider.X)
	}
	update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if model.geometry.Divider.X != 34 {
		t.Fatalf("terminal resize discarded custom divider: %+v", model.geometry)
	}
}

func TestPaneScrollbarsSupportTrackClicksAndDragging(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 24})
	model.files.place.Items = make([]string, 100)
	model.files.reader = repository.File{
		Kind:    repository.FileReady,
		Content: strings.Repeat("line\n", 100),
	}

	update := func(msg tea.Msg) {
		next, command := model.Update(msg)
		model = next.(Model)
		if command != nil {
			t.Fatalf("scrollbar input produced command for %T", msg)
		}
	}
	navigatorBar, _ := ui.CalculateScrollbar(model.geometry.NavigatorRows, len(model.files.place.Items), model.files.place.Top)
	navigatorBottom := navigatorBar.Track.Y + navigatorBar.Track.Height - 1
	update(tea.MouseClickMsg(tea.Mouse{X: navigatorBar.Track.X, Y: navigatorBottom, Button: tea.MouseLeft}))
	if !model.scrollbar.active || model.scrollbar.pane != navigation.FocusNavigator || model.files.place.Top == 0 {
		t.Fatalf("navigator track click = drag %+v place %+v", model.scrollbar, model.files.place)
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: navigatorBar.Track.X, Y: navigatorBar.Track.Y, Button: tea.MouseLeft}))
	if model.files.place.Top != 0 {
		t.Fatalf("navigator thumb drag to top = %d", model.files.place.Top)
	}
	update(tea.MouseReleaseMsg(tea.Mouse{X: navigatorBar.Track.X, Y: navigatorBar.Track.Y, Button: tea.MouseLeft}))
	if model.scrollbar.active {
		t.Fatal("navigator scrollbar release left drag active")
	}

	readerBar, _ := ui.CalculateScrollbar(model.geometry.ReaderRows, model.activeReaderLineCount(), model.files.place.ReaderOffset)
	readerBottom := readerBar.Track.Y + readerBar.Track.Height - 1
	update(tea.MouseClickMsg(tea.Mouse{X: readerBar.Track.X, Y: readerBottom, Button: tea.MouseLeft}))
	if !model.scrollbar.active || model.scrollbar.pane != navigation.FocusReader || model.files.place.ReaderOffset == 0 || model.files.place.Focus != navigation.FocusReader {
		t.Fatalf("reader track click = drag %+v place %+v", model.scrollbar, model.files.place)
	}
	update(tea.MouseReleaseMsg(tea.Mouse{X: readerBar.Track.X, Y: readerBottom, Button: tea.MouseLeft}))
}

func TestBrowserLocalHeaderControlsCycleWithoutCrossingWorkspaces(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: ui.MinimumWidth, Height: ui.MinimumHeight})
	press := func(key rune) tea.Cmd {
		next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: key, Text: string(key)}))
		model = next.(Model)
		return command
	}
	pressEscape := func() tea.Cmd {
		next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		model = next.(Model)
		return command
	}

	if command := press('2'); command != nil || model.controls.Files != workspace.ChangedFiles {
		t.Fatalf("Files 2 = controls %+v command=%v", model.controls, command != nil)
	}
	press('3')
	press('4')
	if model.controls.Reader != workspace.DiffReader || model.controls.Comparison != workspace.Branch {
		t.Fatalf("Files controls = %+v", model.controls)
	}
	press('2')
	if model.controls.Files != workspace.AllFiles || model.controls.Reader != workspace.FileReader {
		t.Fatalf("Changed -> All did not reset reader to File: %+v", model.controls)
	}

	pressEscape()
	beforeScratch := model.controls
	press('2')
	press('3')
	press('4')
	if model.controls != beforeScratch {
		t.Fatalf("Scratch keys changed hidden controls: before %+v after %+v", beforeScratch, model.controls)
	}
	pressEscape()

	if command := press('1'); command != nil || model.active != workspace.Git {
		t.Fatalf("Git activation = active %v command=%v", model.active, command != nil)
	}
	press('2')
	if model.controls.Git != workspace.GitRefs {
		t.Fatalf("Git 2 = %+v", model.controls)
	}
	press('3')
	if model.controls.Traversal != workspace.GitGraph {
		t.Fatalf("Git Refs exposed a tertiary toggle: %+v", model.controls)
	}
	press('2')
	press('2')
	press('3')
	if model.controls.Git != workspace.GitLog || model.controls.Traversal != workspace.GitFirstParent {
		t.Fatalf("Git Log controls = %+v", model.controls)
	}
	comparison := model.controls.Comparison
	press('4')
	if model.controls.Comparison != comparison {
		t.Fatalf("Git 4 changed hidden Files comparison: %+v", model.controls)
	}
}

func TestHistoryLatestWinsAndReconcilesByFullOID(t *testing.T) {
	t.Parallel()
	const oidA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const oidB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	model := newTestModel(&fakeSource{})
	model.geometry = ui.Calculate(80, 20)
	first := model.apply(Action{Kind: ShowGit})
	second := model.apply(Action{Kind: Reload})
	stale, pending := model.history.landCommits(commitsLoadedMsg{
		generation: first.generation,
		commits:    []repository.Commit{{OID: oidA, ShortOID: "aaaaaaa", Subject: "stale"}},
	}, model.geometry.NavigatorRows.Height)
	if len(stale.commits) != 0 || pending.kind != effectNone || !stale.listLoading {
		t.Fatalf("stale history landed: %+v / %+v", stale, pending)
	}

	model.history, pending = model.history.landCommits(commitsLoadedMsg{
		generation: second.generation,
		commits: []repository.Commit{
			{OID: oidA, ShortOID: "aaaaaaa", Subject: "first"},
			{OID: oidB, ShortOID: "bbbbbbb", Subject: "second"},
		},
	}, model.geometry.NavigatorRows.Height)
	if selected, _ := model.history.place.SelectedIdentity(); selected != oidA || pending.identity != oidA {
		t.Fatalf("history selection/effect = %q / %+v", selected, pending)
	}
	loadA := pending
	model.history.place.SelectIndex(1, model.geometry.NavigatorRows.Height)
	loadB := model.history.requestSelectedSummary()
	model.history = model.history.landSummary(commitLoadedMsg{
		generation: loadA.generation,
		oid:        oidA,
		summary:    repository.CommitSummary{OID: oidA, Message: "wrong"},
	}, model.geometry.ReaderRows.Height)
	if model.history.summary.OID != "" || model.history.summaryOID != oidB {
		t.Fatalf("stale summary landed: %+v", model.history)
	}
	model.history = model.history.landSummary(commitLoadedMsg{
		generation: loadB.generation,
		oid:        oidB,
		summary:    repository.CommitSummary{OID: oidB, Message: "right"},
	}, model.geometry.ReaderRows.Height)
	if model.history.summary.Message != "right" || model.history.summaryLoading {
		t.Fatalf("current summary did not land: %+v", model.history)
	}

	model.history.place.Top = 1
	model.history.place.Focus = navigation.FocusReader
	model.history.place.ReaderOffset = 3
	refresh := model.history.reload()
	model.history, _ = model.history.landCommits(commitsLoadedMsg{
		generation: refresh.generation,
		commits: []repository.Commit{
			{OID: oidB, ShortOID: "bbbbbbb", Subject: "second updated"},
			{OID: oidA, ShortOID: "aaaaaaa", Subject: "first"},
		},
	}, model.geometry.NavigatorRows.Height)
	if selected, _ := model.history.place.SelectedIdentity(); selected != oidB || model.history.place.Focus != navigation.FocusReader || model.history.place.ReaderOffset != 3 {
		t.Fatalf("history refresh reset place: selected=%q state=%+v", selected, model.history.place)
	}
}

func TestWorkspacesKeepIndependentPlace(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.geometry = ui.Calculate(80, 12)
	model.files.place = navigation.State{Items: []string{"a", "b", "c"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 4}
	model.history.place = navigation.State{Items: []string{"oid-1", "oid-2"}, Selected: 0, Focus: navigation.FocusNavigator, ReaderOffset: 2}
	model.history.loaded = true

	model.apply(Action{Kind: ShowGit})
	model.apply(Action{Kind: SelectNext})
	model.apply(Action{Kind: ToggleFocus})
	model.apply(Action{Kind: ShowFiles})
	if model.files.place.Selected != 1 || model.files.place.Top != 1 || model.files.place.Focus != navigation.FocusReader || model.files.place.ReaderOffset != 4 {
		t.Fatalf("Git input changed Files place: %+v", model.files.place)
	}
	if model.history.place.Selected != 1 || model.history.place.Focus != navigation.FocusReader {
		t.Fatalf("Git place did not retain user input: %+v", model.history.place)
	}
}

func TestHistoryPresentationCoversLoadingEmptyAndErrors(t *testing.T) {
	t.Parallel()
	geometry := ui.Calculate(80, 20)
	state := newHistoryState()
	state.listLoading = true
	view := state.viewModel(geometry)
	if view.NavigatorEmpty.Text != "Loading commits…" {
		t.Fatalf("loading presentation = %+v", view.NavigatorEmpty)
	}
	state.listLoading = false
	view = state.viewModel(geometry)
	if view.NavigatorEmpty.Text != "No commits" || view.ReaderEmpty.Text != "Select a commit to inspect its summary." {
		t.Fatalf("empty presentation = %+v / %+v", view.NavigatorEmpty, view.ReaderEmpty)
	}
	state.listError = errors.New("broken\x1b[31m")
	view = state.viewModel(geometry)
	if view.NavigatorEmpty.Tone != ui.ToneError || !strings.Contains(view.NavigatorEmpty.Text, "broken") {
		t.Fatalf("error presentation = %+v", view.NavigatorEmpty)
	}
	state.listError = nil
	state.summary = repository.CommitSummary{
		OID:         strings.Repeat("a", 40),
		AuthorName:  "Author",
		AuthorEmail: "author@example.invalid",
		AuthoredAt:  "2026-08-29T12:00:00Z",
		Message:     "subject\n\nbody",
		Stat:        " file.go | 2 ++",
	}
	lines := commitSummaryLines(state.summary)
	joined := make([]string, len(lines))
	for index, line := range lines {
		joined[index] = line.Text
	}
	text := strings.Join(joined, "\n")
	for _, want := range []string{"commit " + state.summary.OID, "Author: Author <author@example.invalid>", "Date:", "subject", "body", "file.go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("commit summary %q lacks %q", text, want)
		}
	}
}

func TestFileLatestGenerationAndRemovalContinuity(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	first := model.apply(Action{Kind: Reload})
	second := model.apply(Action{Kind: Reload})
	if first.generation >= second.generation {
		t.Fatalf("generations are not monotonic: %d, %d", first.generation, second.generation)
	}
	stale, pending := model.files.landFiles(filesLoadedMsg{generation: first.generation, files: []string{"stale"}}, 10)
	if len(stale.place.Items) != 0 || pending.kind != effectNone || !stale.listLoading {
		t.Fatalf("stale list landed: %+v, %+v", stale, pending)
	}

	model.geometry = ui.Calculate(80, 20)
	model.files.tree.Rebuild([]string{"a", "b", "c"})
	model.files.place = navigation.State{Items: model.files.tree.Identities(), Selected: 1, Focus: navigation.FocusReader, ReaderOffset: 3}
	model.files.loaded = true
	model.files.readerPath = "b"
	model.files.reader = repository.File{Path: "b", Kind: repository.FileReady, Content: strings.Repeat("line\n", 10)}
	model.files.listGeneration = second.generation
	model.files, pending = model.files.landFiles(filesLoadedMsg{generation: second.generation, files: []string{"a", "c"}}, model.geometry.NavigatorRows.Height)
	path, _ := model.files.place.SelectedIdentity()
	if path != filetree.FileIdentity("c") || pending.identity != "c" || model.files.readerPath != "c" || model.files.reader.Kind != 0 || !model.files.readerLoading {
		t.Fatalf("removal reconciliation = path %q, effect %+v, files %+v", path, pending, model.files)
	}
	if model.files.place.ReaderOffset != 3 || model.files.place.Focus != navigation.FocusReader {
		t.Fatalf("world result reset place state: %+v", model.files.place)
	}
}

var _ tea.Model = Model{}
