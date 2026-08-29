package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type fakeSource struct {
	root     string
	files    []string
	listErr  error
	contents map[string]repository.File
}

func (s *fakeSource) Root() string { return s.root }

func (s *fakeSource) ListFiles() ([]string, error) {
	return append([]string(nil), s.files...), s.listErr
}

func (s *fakeSource) ReadFile(path string) repository.File {
	if file, ok := s.contents[path]; ok {
		return file
	}
	return repository.File{Path: path, Kind: repository.FileMissing, Err: errors.New("missing")}
}

func TestRootLoadSelectAndRefreshFlow(t *testing.T) {
	t.Parallel()
	source := &fakeSource{
		root:  "/repo",
		files: []string{"a", "b"},
		contents: map[string]repository.File{
			"a": {Path: "a", Kind: repository.FileReady, Content: "a1\na2"},
			"b": {Path: "b", Kind: repository.FileReady, Content: "b1\nb2"},
		},
	}
	model := New(source)
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})

	initial := model.Init()
	if initial == nil {
		t.Fatal("Init() returned no repository command")
	}
	loaded := initial().(filesLoadedMsg)
	next, contentCommand := model.Update(loaded)
	model = next.(Model)
	if model.readerPath != "a" || !model.readerLoading || contentCommand == nil {
		t.Fatalf("after list load: path=%q loading=%v command=%v", model.readerPath, model.readerLoading, contentCommand != nil)
	}
	next, _ = model.Update(contentCommand())
	model = next.(Model)
	if model.reader.Content != "a1\na2" || model.readerLoading {
		t.Fatalf("reader did not land: %+v", model.reader)
	}

	selectEffect := model.apply(Action{Kind: SelectNext})
	if selectEffect.kind != effectLoadContent || selectEffect.path != "b" || model.readerPath != "b" || model.reader.Kind != 0 {
		t.Fatalf("selection effect/state = %+v / path=%q reader=%+v", selectEffect, model.readerPath, model.reader)
	}
	next, _ = model.Update(model.command(selectEffect)())
	model = next.(Model)
	if model.reader.Content != "b1\nb2" {
		t.Fatalf("selected content = %q", model.reader.Content)
	}

	reloadEffect := model.apply(Action{Kind: Reload})
	if reloadEffect.kind != effectLoadFiles || !model.listLoading {
		t.Fatalf("reload effect/state = %+v / %+v", reloadEffect, model)
	}
	refreshResult := model.command(reloadEffect)().(filesLoadedMsg)
	model, selectEffect = model.landFiles(refreshResult)
	if model.reader.Content != "b1\nb2" || !model.readerLoading || selectEffect.path != "b" {
		t.Fatalf("same-identity refresh discarded stale content: reader=%+v effect=%+v", model.reader, selectEffect)
	}
}

func TestLatestGenerationWins(t *testing.T) {
	t.Parallel()
	model := New(&fakeSource{root: "/repo"})
	first := model.apply(Action{Kind: Reload})
	second := model.apply(Action{Kind: Reload})
	if first.generation >= second.generation {
		t.Fatalf("generations are not monotonic: %d, %d", first.generation, second.generation)
	}
	stale, effect := model.landFiles(filesLoadedMsg{generation: first.generation, files: []string{"stale"}})
	if len(stale.navigation.Files) != 0 || effect.kind != effectNone || !stale.listLoading {
		t.Fatalf("stale list landed: %+v, %+v", stale, effect)
	}

	model.navigation.Reconcile([]string{"a", "b"})
	loadA := model.requestSelectedContent()
	model.navigation.SelectIndex(1, 10)
	loadB := model.requestSelectedContent()
	if loadA.generation >= loadB.generation {
		t.Fatalf("content generations are not monotonic: %d, %d", loadA.generation, loadB.generation)
	}
	staleContent := contentLoadedMsg{
		generation: loadA.generation,
		path:       "a",
		file:       repository.File{Path: "a", Kind: repository.FileReady, Content: "wrong"},
	}
	model = model.landContent(staleContent)
	if model.reader.Kind != 0 || model.readerPath != "b" {
		t.Fatalf("stale content landed: path=%q reader=%+v", model.readerPath, model.reader)
	}
}

func TestWorldRemovalUsesNearestIdentityAndBlanksReader(t *testing.T) {
	t.Parallel()
	model := New(&fakeSource{root: "/repo"})
	model.geometry = ui.Calculate(80, 20)
	model.navigation = navigation.State{
		Files:        []string{"a", "b", "c"},
		Selected:     1,
		Focus:        navigation.FocusReader,
		ReaderOffset: 3,
	}
	model.readerPath = "b"
	model.reader = repository.File{Path: "b", Kind: repository.FileReady, Content: strings.Repeat("line\n", 10)}
	model.listGeneration = 4

	model, pending := model.landFiles(filesLoadedMsg{generation: 4, files: []string{"a", "c"}})
	path, _ := model.navigation.SelectedPath()
	if path != "c" || pending.path != "c" || model.readerPath != "c" || model.reader.Kind != 0 || !model.readerLoading {
		t.Fatalf("removal reconciliation = path %q, effect %+v, reader path %q, reader %+v", path, pending, model.readerPath, model.reader)
	}
	if model.navigation.ReaderOffset != 3 || model.navigation.Focus != navigation.FocusReader {
		t.Fatalf("world result reset place state: %+v", model.navigation)
	}
}

func TestResizePreservesValidSelectionFocusAndScroll(t *testing.T) {
	t.Parallel()
	model := New(&fakeSource{root: "/repo"})
	model.navigation = navigation.State{
		Files:        []string{"a", "b", "c"},
		Selected:     1,
		Top:          1,
		Focus:        navigation.FocusReader,
		ReaderOffset: 4,
	}
	model.readerPath = "b"
	model.reader = repository.File{Path: "b", Kind: repository.FileReady, Content: strings.Repeat("line\n", 20)}
	model.apply(Action{Kind: Resize, Width: 80, Height: 12})
	if model.navigation.Selected != 1 || model.navigation.Focus != navigation.FocusReader || model.navigation.ReaderOffset != 4 {
		t.Fatalf("resize reset place state: %+v", model.navigation)
	}
}

func TestListFailureKeepsLoadedWorld(t *testing.T) {
	t.Parallel()
	model := New(&fakeSource{root: "/repo"})
	model.navigation.Reconcile([]string{"kept"})
	model.readerPath = "kept"
	model.reader = repository.File{Path: "kept", Kind: repository.FileReady, Content: "content"}
	model.listGeneration = 2
	model.listLoading = true

	model, pending := model.landFiles(filesLoadedMsg{generation: 2, err: errors.New("git failed")})
	path, _ := model.navigation.SelectedPath()
	if path != "kept" || model.reader.Content != "content" || pending.kind != effectNone || model.listError == nil || model.listLoading {
		t.Fatalf("failed reload damaged world: %+v / %+v", model, pending)
	}
}

var _ tea.Model = Model{}
