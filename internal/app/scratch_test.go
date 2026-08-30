package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/scratch"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type fakeScratchStore struct {
	text     string
	readOnly bool
	loadErr  error
	saveErr  error
	loads    int
	saves    []string
	closed   bool
	closeErr error
}

func (store *fakeScratchStore) Load() (string, bool, error) {
	store.loads++
	return store.text, store.readOnly, store.loadErr
}

func (store *fakeScratchStore) Save(text string) error {
	store.saves = append(store.saves, text)
	if store.saveErr == nil {
		store.text = text
	}
	return store.saveErr
}

func (store *fakeScratchStore) Close() error {
	store.closed = true
	return store.closeErr
}

func newScratchTestModel(store *fakeScratchStore) Model {
	model := NewWithScratch(&fakeSource{}, herdr.Context{}, store)
	model.apply(Action{Kind: Resize, Width: 80, Height: 16})
	return model
}

func newScopedScratchTestModel(project, worktree *fakeScratchStore) Model {
	model := NewWithScratchScopes(&fakeSource{}, herdr.Context{}, scratch.Stores{Project: project, Worktree: worktree})
	model.apply(Action{Kind: Resize, Width: 80, Height: 16})
	return model
}

func openScratch(t *testing.T, model Model) Model {
	t.Helper()
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if !model.scratch || command == nil {
		t.Fatalf("open Scratch = active %v command=%v", model.scratch, command != nil)
	}
	next, command = model.Update(command())
	if command != nil {
		t.Fatal("Scratch load produced an unexpected follow-up command")
	}
	return next.(Model)
}

func typeScratch(t *testing.T, model Model, text string) Model {
	t.Helper()
	for _, value := range text {
		next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: value, Text: string(value)}))
		model = next.(Model)
		if command == nil {
			t.Fatalf("typing %q scheduled no debounce", value)
		}
	}
	return model
}

func TestScratchEscapeRestoresExactUnderlyingPlace(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{text: "note"}
	model := newScratchTestModel(store)
	model.files.place = navigation.State{Items: []string{"a", "b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 7}
	model.controls = workspace.Controls{Files: workspace.ChangedFiles, Reader: workspace.DiffReader, Comparison: workspace.Branch}
	model.layout.customized = true
	model.layout.navigatorWidth = 33
	beforePlace := model.files.place
	beforeControls := model.controls
	beforeLayout := model.layout

	model = openScratch(t, model)
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command != nil || model.scratch {
		t.Fatalf("clean close = scratch %v command=%v", model.scratch, command != nil)
	}
	if !reflect.DeepEqual(model.files.place, beforePlace) || model.controls != beforeControls || model.layout != beforeLayout {
		t.Fatalf("underlying state changed: place=%+v controls=%+v layout=%+v", model.files.place, model.controls, model.layout)
	}
}

func TestScratchRestoresIndependentGitSubstates(t *testing.T) {
	t.Parallel()
	model := newScratchTestModel(&fakeScratchStore{text: "note"})
	model.active = workspace.Git
	model.controls.Git = workspace.GitStashes
	model.controls.Traversal = workspace.GitFirstParent
	model.history.place = navigation.State{Items: []string{"log-a", "log-b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 4}
	model.refs.place = navigation.State{Items: []string{"ref-a", "ref-b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 5}
	model.stashes.place = navigation.State{Items: []string{"stash-a", "stash-b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 6}
	model.stashes.readerPlaces["stash-b"] = stashReaderPlace{fileIdentity: "file-b", readerOffset: 7}
	model.layout.customized = true
	model.layout.navigatorWidth = 31

	beforeHistory := model.history
	beforeRefs := model.refs
	beforeStashes := model.stashes
	beforeControls := model.controls
	beforeLayout := model.layout

	model = openScratch(t, model)
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command != nil || model.scratch || model.active != workspace.Git {
		t.Fatalf("clean close = scratch %v active %v command=%v", model.scratch, model.active, command != nil)
	}
	if !reflect.DeepEqual(model.history, beforeHistory) || !reflect.DeepEqual(model.refs, beforeRefs) ||
		!reflect.DeepEqual(model.stashes, beforeStashes) || model.controls != beforeControls || model.layout != beforeLayout {
		t.Fatalf("Git substates changed behind Scratch: history=%+v refs=%+v stashes=%+v controls=%+v layout=%+v",
			model.history, model.refs, model.stashes, model.controls, model.layout)
	}
}

func TestScratchPrintablePasteAndBackgroundRefreshIsolation(t *testing.T) {
	t.Parallel()
	model := openScratch(t, newScratchTestModel(&fakeScratchStore{}))
	model = typeScratch(t, model, "hjklq234")
	next, _ := model.Update(tea.PasteMsg{Content: "\nwide 界\x1b"})
	model = next.(Model)
	want := "hjklq234\nwide 界␛"
	if got := model.note.editor.Text(); got != want {
		t.Fatalf("Scratch text = %q, want %q", got, want)
	}
	cursor := model.note.editor.Cursor()
	generation := model.note.generation
	next, _ = model.Update(snapshotLoadedMsg{
		generation: model.files.listGeneration,
		snapshot:   repository.NewSnapshot([]repository.Entry{{Path: "world.go"}}),
	})
	model = next.(Model)
	next, _ = model.Update(refSourcesLoadedMsg{
		generation: model.refs.sourceGeneration,
		sources:    []repository.RefSource{repository.AllRefsSource()},
	})
	model = next.(Model)
	next, _ = model.Update(stashesLoadedMsg{
		generation: model.stashes.listGeneration,
		stashes:    []repository.Stash{{OID: "stash-world"}},
	})
	model = next.(Model)
	if model.note.editor.Text() != want || model.note.editor.Cursor() != cursor || model.note.generation != generation {
		t.Fatalf("background refresh disturbed Scratch: %+v", model.note)
	}
}

func TestScratchAutosaveGenerationsAndStatusTransitions(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{}
	model := openScratch(t, newScratchTestModel(store))
	model = typeScratch(t, model, "ab")
	current := model.note.generation
	if status, _ := model.note.status(); !strings.Contains(status, "modified") {
		t.Fatalf("modified status = %q", status)
	}

	next, command := model.Update(scratchSaveDueMsg{generation: current - 1})
	model = next.(Model)
	if command != nil || len(store.saves) != 0 {
		t.Fatal("stale debounce started a save")
	}
	next, command = model.Update(scratchSaveDueMsg{generation: current})
	model = next.(Model)
	if command == nil || !model.note.saving {
		t.Fatal("current debounce did not start saving")
	}
	if status, _ := model.note.status(); !strings.Contains(status, "saving") {
		t.Fatalf("saving status = %q", status)
	}
	saveCommand := command
	model = typeScratch(t, model, "c")
	newGeneration := model.note.generation
	next, command = model.Update(saveCommand())
	model = next.(Model)
	if model.note.savedGeneration == newGeneration || !model.note.modified() || command == nil {
		t.Fatalf("stale save labeled newer text saved: %+v", model.note)
	}
	next, command = model.Update(scratchSaveDueMsg{generation: newGeneration})
	model = next.(Model)
	if command == nil {
		t.Fatal("new generation did not start save")
	}
	next, _ = model.Update(command())
	model = next.(Model)
	if model.note.modified() || store.text != "abc" {
		t.Fatalf("autosave result = note %+v store %q", model.note, store.text)
	}
	if status, _ := model.note.status(); !strings.Contains(status, "saved") {
		t.Fatalf("saved status = %q", status)
	}
}

func TestScratchCloseQuitAndFailedSaveKeepMemory(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{}
	model := typeScratch(t, openScratch(t, newScratchTestModel(store)), "close")
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command == nil || !model.scratch {
		t.Fatal("dirty Escape did not wait for save")
	}
	next, _ = model.Update(command())
	model = next.(Model)
	if model.scratch || store.text != "close" {
		t.Fatalf("close save = scratch %v text %q", model.scratch, store.text)
	}

	model = openScratch(t, model)
	model = typeScratch(t, model, " quit")
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil {
		t.Fatal("normal quit did not save")
	}
	next, quitCommand := model.Update(command())
	model = next.(Model)
	if quitCommand == nil || store.text != "close quit" {
		t.Fatalf("quit save = command %v text %q", quitCommand != nil, store.text)
	}
	if _, ok := quitCommand().(tea.QuitMsg); !ok {
		t.Fatalf("quit follow-up = %T", quitCommand())
	}

	failing := &fakeScratchStore{saveErr: errors.New("disk full")}
	model = typeScratch(t, openScratch(t, newScratchTestModel(failing)), "memory")
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	next, _ = model.Update(command())
	model = next.(Model)
	if model.scratch || model.note.editor.Text() != "memory" || !model.note.modified() {
		t.Fatalf("failed close lost memory: %+v", model.note)
	}
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command != nil || !model.scratch || failing.loads != 1 || model.note.editor.Text() != "memory" {
		t.Fatalf("reopen after failure = command %v loads %d note %+v", command != nil, failing.loads, model.note)
	}
	if status, isError := model.note.status(); !isError || !strings.Contains(status, "disk full") {
		t.Fatalf("failed status = %q, error %v", status, isError)
	}
}

func TestScratchReadOnlyLoadAndShutdown(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{text: "shared", readOnly: true}
	model := openScratch(t, newScratchTestModel(store))
	before := model.note.editor.Text()
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = next.(Model)
	if command != nil || model.note.editor.Text() != before {
		t.Fatalf("read-only edit = command %v text %q", command != nil, model.note.editor.Text())
	}
	if status, _ := model.note.status(); !strings.Contains(status, "read-only") || !strings.Contains(status, "another reviewr") {
		t.Fatalf("read-only status = %q", status)
	}
	if err := model.Shutdown(); err != nil || !store.closed || len(store.saves) != 0 {
		t.Fatalf("read-only shutdown = err %v closed %v saves %#v", err, store.closed, store.saves)
	}
}

func TestScratchMouseSelectionWheelAndScrollbar(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{text: strings.Repeat("abcdefghij\n", 40)}
	model := openScratch(t, newScratchTestModel(store))
	g := model.geometry
	update := func(msg tea.Msg) {
		next, command := model.Update(msg)
		model = next.(Model)
		if command != nil {
			t.Fatalf("pointer input %T produced command", msg)
		}
	}
	update(tea.MouseClickMsg(tea.Mouse{X: g.ScratchText.X + 1, Y: g.ScratchText.Y, Button: tea.MouseLeft}))
	update(tea.MouseMotionMsg(tea.Mouse{X: g.ScratchText.X + 5, Y: g.ScratchText.Y + 1, Button: tea.MouseLeft}))
	update(tea.MouseReleaseMsg(tea.Mouse{X: g.ScratchText.X + 5, Y: g.ScratchText.Y + 1, Button: tea.MouseLeft}))
	if _, _, selected := model.note.editor.Selection(); !selected || model.note.editor.Dragging() {
		t.Fatalf("pointer selection = note %+v", model.note)
	}
	update(tea.MouseWheelMsg(tea.Mouse{X: g.ScratchText.X, Y: g.ScratchText.Y, Button: tea.MouseWheelDown}))
	if model.note.editor.ScrollOffset() == 0 {
		t.Fatal("wheel did not scroll wrapped note")
	}
	presentation := model.note.editor.Presentation()
	bar, ok := ui.CalculateScrollbar(g.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("long note has no scrollbar")
	}
	if presentation.Document.Width != bar.Content.Width {
		t.Fatalf("overflowing Scratch document width = %d, want reserved content width %d", presentation.Document.Width, bar.Content.Width)
	}
	bottom := bar.Track.Y + bar.Track.Height - 1
	update(tea.MouseClickMsg(tea.Mouse{X: bar.Track.X, Y: bottom, Button: tea.MouseLeft}))
	if !model.note.scrollbarDragging || model.note.editor.ScrollOffset() <= 3 {
		t.Fatalf("track click = note %+v", model.note)
	}
	update(tea.MouseMotionMsg(tea.Mouse{X: bar.Track.X, Y: bar.Track.Y, Button: tea.MouseLeft}))
	if model.note.editor.ScrollOffset() != 0 {
		t.Fatalf("thumb drag to top = %d", model.note.editor.ScrollOffset())
	}
	update(tea.MouseReleaseMsg(tea.Mouse{X: bar.Track.X, Y: bar.Track.Y, Button: tea.MouseLeft}))
	if model.note.scrollbarDragging {
		t.Fatal("release left Scratch scrollbar dragging")
	}
}

func TestScratchReservesLaneOnlyAcrossOverflowBoundary(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{text: "short"}
	model := openScratch(t, newScratchTestModel(store))
	if got := model.note.editor.Presentation().Document.Width; got != model.geometry.ScratchRows.Width {
		t.Fatalf("fitting Scratch width = %d, want full width %d", got, model.geometry.ScratchRows.Width)
	}

	model.note.editor.Load(strings.Repeat("long line\n", model.geometry.ScratchRows.Height+5))
	model.note.resize(model.geometry)
	presentation := model.note.editor.Presentation()
	bar, ok := ui.CalculateScrollbar(model.geometry.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok || presentation.Document.Width != bar.Content.Width {
		t.Fatalf("overflowing Scratch layout = presentation %+v bar %+v visible %v", presentation, bar, ok)
	}

	model.note.editor.Load("short again")
	model.note.resize(model.geometry)
	presentation = model.note.editor.Presentation()
	if _, ok := ui.CalculateScrollbar(model.geometry.ScratchRows, len(presentation.Document.Rows), presentation.Top); ok {
		t.Fatal("fitting Scratch content retained a scrollbar")
	}
	if presentation.Document.Width != model.geometry.ScratchRows.Width {
		t.Fatalf("Scratch width after fitting again = %d, want %d", presentation.Document.Width, model.geometry.ScratchRows.Width)
	}
}

func TestScratchAutosaveFailureDoesNotPoll(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{saveErr: errors.New("offline")}
	model := typeScratch(t, openScratch(t, newScratchTestModel(store)), "x")
	next, command := model.Update(scratchSaveDueMsg{generation: model.note.generation})
	model = next.(Model)
	if command == nil {
		t.Fatal("autosave did not start")
	}
	next, retry := model.Update(command())
	model = next.(Model)
	if retry != nil || len(store.saves) != 1 || !model.note.modified() {
		t.Fatalf("failed autosave polled: retry %v saves %d note %+v", retry != nil, len(store.saves), model.note)
	}
}

func TestScratchLoadFailureIsRecoverable(t *testing.T) {
	t.Parallel()
	store := &fakeScratchStore{loadErr: errors.New("unreadable state")}
	model := openScratch(t, newScratchTestModel(store))
	if !model.scratch || model.note.editor.Text() != "" {
		t.Fatalf("load failure blocked Scratch: %+v", model.note)
	}
	if status, isError := model.note.status(); !isError || !strings.Contains(status, "unreadable state") {
		t.Fatalf("load error status = %q, %v", status, isError)
	}
}

func TestScratchScopesSaveBeforeSwitchAndRestoreIndependentEditorPlace(t *testing.T) {
	t.Parallel()
	project := &fakeScratchStore{text: "project"}
	worktree := &fakeScratchStore{text: "worktree"}
	model := openScratch(t, newScopedScratchTestModel(project, worktree))
	if model.note.scope != scratch.Project {
		t.Fatalf("initial scope = %v, want project", model.note.scope)
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	model = next.(Model)
	model = typeScratch(t, model, "!")
	projectPlace := model.note.current().editor.Presentation()
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil || model.note.scope != scratch.Project || !model.note.switchPending || worktree.loads != 0 {
		t.Fatalf("dirty switch started incorrectly: scope=%v pending=%v command=%v worktree loads=%d", model.note.scope, model.note.switchPending, command != nil, worktree.loads)
	}
	next, loadCommand := model.Update(command())
	model = next.(Model)
	if loadCommand == nil || model.note.scope != scratch.Worktree || project.text != "project!" {
		t.Fatalf("project save did not precede switch: scope=%v project=%q load=%v", model.note.scope, project.text, loadCommand != nil)
	}
	next, command = model.Update(loadCommand())
	model = next.(Model)
	if command != nil || model.note.current().editor.Text() != "worktree" {
		t.Fatalf("worktree load = command %v note %q", command != nil, model.note.current().editor.Text())
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	model = next.(Model)
	model = typeScratch(t, model, "!")
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft, Mod: tea.ModShift}))
	model = next.(Model)
	worktreePlace := model.note.current().editor.Presentation()
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil {
		t.Fatal("dirty worktree switch did not save")
	}
	next, command = model.Update(command())
	model = next.(Model)
	if command != nil || model.note.scope != scratch.Project || !reflect.DeepEqual(model.note.current().editor.Presentation(), projectPlace) {
		t.Fatalf("project place was not restored: scope=%v command=%v place=%+v", model.note.scope, command != nil, model.note.current().editor.Presentation())
	}

	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil || model.note.current().editor.Text() != "project" {
		t.Fatalf("project undo history did not survive switch: command=%v text=%q", command != nil, model.note.current().editor.Text())
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Mod: tea.ModCtrl}))
	model = next.(Model)
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil {
		t.Fatal("redone project text did not flush before returning to worktree")
	}
	next, command = model.Update(command())
	model = next.(Model)
	if command != nil || model.note.scope != scratch.Worktree || !reflect.DeepEqual(model.note.current().editor.Presentation(), worktreePlace) {
		t.Fatalf("worktree place was not restored: scope=%v command=%v place=%+v", model.note.scope, command != nil, model.note.current().editor.Presentation())
	}
}

func TestScratchScopeTagsContainDirtySaveAndLoadRaces(t *testing.T) {
	t.Parallel()
	project := &fakeScratchStore{}
	worktree := &fakeScratchStore{text: "local"}
	model := typeScratch(t, openScratch(t, newScopedScratchTestModel(project, worktree)), "a")
	firstGeneration := model.note.generation
	next, firstSave := model.Update(scratchSaveDueMsg{scope: scratch.Project, generation: firstGeneration})
	model = next.(Model)
	if firstSave == nil || !model.note.current().saving {
		t.Fatal("first project autosave did not start")
	}
	model = typeScratch(t, model, "b")
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command != nil || !model.note.switchPending || model.note.scope != scratch.Project {
		t.Fatalf("switch did not wait for in-flight save: command=%v scope=%v pending=%v", command != nil, model.note.scope, model.note.switchPending)
	}

	next, secondSave := model.Update(firstSave())
	model = next.(Model)
	if secondSave == nil || model.note.scope != scratch.Project || project.text != "a" {
		t.Fatalf("stale save completion switched early: scope=%v project=%q command=%v", model.note.scope, project.text, secondSave != nil)
	}
	next, _ = model.Update(scratchLoadedMsg{scope: scratch.Worktree, generation: 0, text: "wrong"})
	model = next.(Model)
	if model.note.forScope(scratch.Worktree).loaded || model.note.forScope(scratch.Worktree).editor.Text() != "" {
		t.Fatal("unrequested worktree load landed")
	}
	next, loadWorktree := model.Update(secondSave())
	model = next.(Model)
	if loadWorktree == nil || model.note.scope != scratch.Worktree || project.text != "ab" {
		t.Fatalf("latest project save did not complete switch: scope=%v project=%q load=%v", model.note.scope, project.text, loadWorktree != nil)
	}
	projectBefore := model.note.forScope(scratch.Project).editor.Presentation()
	next, _ = model.Update(scratchLoadedMsg{scope: scratch.Project, generation: model.note.forScope(scratch.Project).loadGeneration, text: "stale"})
	model = next.(Model)
	if !reflect.DeepEqual(model.note.forScope(scratch.Project).editor.Presentation(), projectBefore) || model.note.scope != scratch.Worktree {
		t.Fatal("stale project load disturbed scoped state")
	}
	next, command = model.Update(loadWorktree())
	model = next.(Model)
	if command != nil || model.note.current().editor.Text() != "local" || model.note.forScope(scratch.Project).editor.Text() != "ab" {
		t.Fatalf("scoped race result: command=%v project=%q worktree=%q", command != nil, model.note.forScope(scratch.Project).editor.Text(), model.note.current().editor.Text())
	}
}

func TestScratchScopeDefaultsToProjectAndReadOnlyProjectDoesNotBlockWorktree(t *testing.T) {
	t.Parallel()
	project := &fakeScratchStore{text: "shared", readOnly: true}
	worktree := &fakeScratchStore{text: "local"}
	model := openScratch(t, newScopedScratchTestModel(project, worktree))
	next, loadWorktree := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = next.(Model)
	if loadWorktree == nil || model.note.scope != scratch.Worktree {
		t.Fatalf("read-only project blocked local scope: scope=%v command=%v", model.note.scope, loadWorktree != nil)
	}
	next, _ = model.Update(loadWorktree())
	model = next.(Model)
	model = typeScratch(t, model, "x")
	if got := model.note.current().editor.Text(); got != "xlocal" {
		t.Fatalf("local edit = %q", got)
	}
	next, saveWorktree := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if saveWorktree == nil {
		t.Fatal("dirty local note did not save on close")
	}
	next, _ = model.Update(saveWorktree())
	model = next.(Model)
	if model.scratch {
		t.Fatal("Scratch did not close after local save")
	}
	next, reloadProject := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if reloadProject == nil || model.note.scope != scratch.Project || worktree.loads != 1 {
		t.Fatalf("re-entry did not default to project: scope=%v command=%v worktree loads=%d", model.note.scope, reloadProject != nil, worktree.loads)
	}
}

func TestScratchShutdownSavesAndClosesEveryScopeWithJoinedErrors(t *testing.T) {
	t.Parallel()
	projectSaveErr := errors.New("project save")
	projectCloseErr := errors.New("project close")
	worktreeSaveErr := errors.New("worktree save")
	worktreeCloseErr := errors.New("worktree close")
	project := &fakeScratchStore{saveErr: projectSaveErr, closeErr: projectCloseErr}
	worktree := &fakeScratchStore{saveErr: worktreeSaveErr, closeErr: worktreeCloseErr}
	model := newScopedScratchTestModel(project, worktree)
	for scope, text := range map[scratch.Scope]string{scratch.Project: "shared", scratch.Worktree: "local"} {
		note := model.note.forScope(scope)
		note.editor.Load(text)
		note.loaded = true
		note.generation = 2
		note.savedGeneration = 1
	}

	err := model.Shutdown()
	for _, want := range []error{projectSaveErr, projectCloseErr, worktreeSaveErr, worktreeCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("Shutdown() error %v does not contain %v", err, want)
		}
	}
	if !project.closed || !worktree.closed || !reflect.DeepEqual(project.saves, []string{"shared"}) || !reflect.DeepEqual(worktree.saves, []string{"local"}) {
		t.Fatalf("Shutdown() project=%+v worktree=%+v", project, worktree)
	}
}
