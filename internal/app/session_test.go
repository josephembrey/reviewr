package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/session"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestWorktreeSessionRestoresFilesByIdentityBeforeFreshLoads(t *testing.T) {
	t.Parallel()
	content := strings.Repeat("line\n", 40)
	document := fileReaderDocument(repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: content}, repository.Entry{Path: "src/a.go"})
	oldRows := readerRowIdentities(document.Rows)
	source := &fakeSource{
		snapshot: repository.NewSnapshot([]repository.Entry{
			{Path: "root.go"}, {Path: "src/a.go"}, {Path: "src/b.go"},
		}),
		contents: map[string]repository.File{
			"src/a.go": {Path: "src/a.go", Kind: repository.FileReady, Content: content},
		},
	}
	restored := session.State{
		Active:   "files",
		Controls: session.Controls{Files: "all", Reader: "file", DiffHighlight: "background"},
		Layout:   session.Layout{NavigatorWidth: 29, Customized: true, Swapped: true},
		Files: session.Files{
			Place: session.Place{
				Items:    []string{filetree.DirectoryIdentity("src"), filetree.FileIdentity("root.go")},
				Selected: 1, Top: 0, Focus: "reader", ReaderOffset: 3,
			},
			ReaderPath: "src/a.go", ReaderRows: oldRows,
			Folds: map[string]session.Folds{
				"all": {Known: []string{"src"}, Collapsed: []string{"src"}},
			},
		},
	}
	model := NewWithSession(source, herdr.Context{}, notes.NewMemoryStore(), nil, restored)
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	var pending effect
	model.files, pending = model.files.landSnapshot(snapshotLoadedMsg{
		generation: model.files.listGeneration, snapshot: source.snapshot,
	}, model.controls.Files, model.controls.Reader, model.geometry.NavigatorRows.Height)

	selected, _ := model.files.place.SelectedIdentity()
	row, _ := model.files.tree.Row(filetree.DirectoryIdentity("src"))
	if selected != filetree.FileIdentity("root.go") || row.Expanded || model.files.readerEntry.Path != "src/a.go" ||
		model.files.place.Focus != navigation.FocusReader || pending.entry.Path != "src/a.go" {
		t.Fatalf("restored files before content = selected %q row %+v reader %q focus %v effect %+v",
			selected, row, model.files.readerEntry.Path, model.files.place.Focus, pending)
	}
	next, _ := model.Update(model.command(pending)())
	model = next.(Model)
	if model.files.place.ReaderOffset != 3 || model.files.place.ReaderColumn != 0 {
		t.Fatalf("restored reader place = %d:%d, want 3:0", model.files.place.ReaderOffset, model.files.place.ReaderColumn)
	}
	if !model.layout.swapped || !model.layout.customized || model.layout.navigatorWidth != 29 ||
		model.controls.DiffHighlight != workspace.DiffHighlightBackground {
		t.Fatalf("restored presentation state = layout %+v controls %+v", model.layout, model.controls)
	}
}

func TestWorktreeSessionRoundTripsEveryBrowserPlace(t *testing.T) {
	t.Parallel()
	original := NewWithSession(&fakeSource{}, herdr.Context{}, notes.NewMemoryStore(), nil, session.State{
		Active: "git",
		Controls: session.Controls{
			Files: "changed", Reader: "diff", Comparison: "branch",
			Git: "stashes", Traversal: "first-parent", DiffHighlight: "background",
		},
		Files: session.Files{
			Place:      session.Place{Items: []string{"file:a.go"}, Focus: "reader", ReaderOffset: 4, ReaderColumn: 8},
			ReaderPath: "a.go", ReaderRows: []string{"old-a", "old-b"}, ContextExpanded: true,
			ReviewFull: map[string]bool{"a.go": true}, ReviewCursor: 2, ReviewAnchor: 1,
		},
		History: session.Place{Items: []string{"commit-1"}, Focus: "reader", ReaderOffset: 5},
		Refs: session.Refs{
			Place:       session.Place{Items: []string{"ref-1"}, Focus: "reader", ReaderOffset: 3},
			PreviewRows: []string{"commit-1", "commit-0"},
		},
		Stashes: session.Stashes{
			Place:      session.Place{Items: []string{"stash-1"}, Focus: "reader", ReaderOffset: 7},
			ReaderRows: []string{"stash-row"}, ContextExpanded: true,
			ReaderPlaces: map[string]session.StashReaderPlace{
				"stash-1": {FileIdentity: "\x00a.go", ReaderOffset: 7, ReaderColumn: 2},
			},
		},
	})

	saved := original.sessionState()
	restarted := NewWithSession(&fakeSource{}, herdr.Context{}, notes.NewMemoryStore(), nil, saved)
	again := restarted.sessionState()
	if !reflect.DeepEqual(again, saved) {
		t.Fatalf("session round trip changed state:\nfirst  %#v\nsecond %#v", saved, again)
	}
}

func TestRestoredDestinationWarmsItsOwnDataAndNotesPlace(t *testing.T) {
	t.Parallel()
	source := &fakeSource{}
	store := &fakeNotesStore{text: "alpha beta gamma"}
	model := NewWithSession(source, herdr.Context{}, store, nil, session.State{
		Active: "notes",
		Notes: session.Notes{
			Scope:   "project",
			Project: session.NotePlace{Valid: true, Cursor: 8, Anchor: 2, PreferredCol: 4, Scroll: 0},
		},
	})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	batch, ok := model.Init()().(tea.BatchMsg)
	if !ok || len(batch) != 3 {
		t.Fatalf("restored Notes init = %T with %d commands, want three warm loads", model.Init()(), len(batch))
	}
	var noteMessage notesLoadedMsg
	for _, command := range batch {
		if message, ok := command().(notesLoadedMsg); ok {
			noteMessage = message
		}
	}
	if noteMessage.generation == 0 {
		t.Fatal("restored Notes destination did not load")
	}
	next, _ := model.Update(noteMessage)
	model = next.(Model)
	if got := model.note.current().editor.Place(); got != (notes.Place{Cursor: 8, Anchor: 2, PreferredCol: 4, Scroll: 0}) {
		t.Fatalf("restored Notes place = %+v", got)
	}
}

func TestRestoredRefsReconcileSourceAndPreviewByOID(t *testing.T) {
	t.Parallel()
	sourceID := repository.RefSourceID{Kind: repository.RefSourceLocalBranch, Name: "refs/heads/topic"}
	model := NewWithSession(&fakeSource{}, herdr.Context{}, notes.NewMemoryStore(), nil, session.State{
		Active:   "git",
		Controls: session.Controls{Git: "refs"},
		Refs: session.Refs{
			Place: session.Place{
				Items:    []string{repository.AllRefsSource().ID.Key(), sourceID.Key()},
				Selected: 1, Focus: "reader", ReaderOffset: 1,
			},
			PreviewRows: []string{"old-tip", "keep"},
		},
	})
	batch := model.Init()().(tea.BatchMsg)
	foundInitialLoad := false
	for _, command := range batch {
		if message, ok := command().(refSourcesLoadedMsg); ok {
			foundInitialLoad = message.generation == model.refs.sourceGeneration
		}
	}
	if !foundInitialLoad {
		t.Fatal("restored Refs destination did not start its tagged load")
	}
	var pending effect
	model.refs, pending = model.refs.landSources(refSourcesLoadedMsg{
		generation: model.refs.sourceGeneration,
		sources: []repository.RefSource{
			repository.AllRefsSource(),
			{ID: sourceID, Label: "topic", OID: "new-tip"},
		},
	}, 10)
	if model.refs.selected != sourceID || pending.kind != effectLoadRefCommits {
		t.Fatalf("restored ref source = %+v effect %+v", model.refs.selected, pending)
	}
	model.refs = model.refs.landPreview(refCommitsLoadedMsg{
		generation: model.refs.previewGeneration, sourceID: sourceID,
		commits: []repository.RefCommit{{OID: "new-tip"}, {OID: "old-tip"}, {OID: "keep"}},
	}, 1)
	if model.refs.place.ReaderOffset != 2 || model.refs.place.Focus != navigation.FocusReader {
		t.Fatalf("restored ref preview place = %+v", model.refs.place)
	}
}

func TestRestoredStashReconcilesNestedFilePlace(t *testing.T) {
	t.Parallel()
	const oid = "stash-oid"
	file := repository.ChangedFile{Path: "src/a.go", Kind: repository.ChangeModified}
	model := NewWithSession(&fakeSource{}, herdr.Context{}, notes.NewMemoryStore(), nil, session.State{
		Active:   "git",
		Controls: session.Controls{Git: "stashes"},
		Stashes: session.Stashes{
			Place:      session.Place{Items: []string{oid}, Focus: "reader", ReaderOffset: 4, ReaderColumn: 2},
			ReaderRows: []string{"row-0", "keep-row"}, ContextExpanded: true,
			ReaderPlaces: map[string]session.StashReaderPlace{
				oid: {FileIdentity: file.Identity(), ReaderOffset: 4, ReaderColumn: 2},
			},
		},
	})
	batch := model.Init()().(tea.BatchMsg)
	foundInitialLoad := false
	for _, command := range batch {
		if message, ok := command().(stashesLoadedMsg); ok {
			foundInitialLoad = message.generation == model.stashes.listGeneration
		}
	}
	if !foundInitialLoad {
		t.Fatal("restored Stashes destination did not start its tagged load")
	}
	var filesEffect effect
	model.stashes, filesEffect = model.stashes.landStashes(stashesLoadedMsg{
		generation: model.stashes.listGeneration,
		stashes:    []repository.Stash{{OID: oid, Source: repository.ChangeSource{OID: oid}}},
	}, 10)
	var readerEffect effect
	model.stashes, readerEffect = model.stashes.landFiles(stashFilesLoadedMsg{
		generation: filesEffect.generation, oid: oid, files: []repository.ChangedFile{file},
	}, 10)
	if readerEffect.kind != effectLoadStashFile || model.stashes.selectedFileIdentity() != file.Identity() ||
		model.stashes.place.ReaderOffset != 4 || !model.stashes.readerContextExpanded {
		t.Fatalf("restored stash state = effect %+v state %+v", readerEffect, model.stashes)
	}
}
