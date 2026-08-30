package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/josephembrey/reviewr/internal/highlight"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/ui"
)

const notesSaveDebounce = 350 * time.Millisecond

type notesExit uint8

const (
	notesExitNone notesExit = iota
	notesExitFiles
	notesExitGit
	notesExitQuit
	notesExitScope
)

// scopedNotesState owns the two possible Notes places while keeping the
// root model unaware of their persistence and transition details.
type scopedNotesState struct {
	notesState
	worktree        notesState
	worktreeEnabled bool
	scope           notes.Scope
	pendingScope    notes.Scope
	switchPending   bool
}

func newScopedNotesState(stores notes.Stores) scopedNotesState {
	state := scopedNotesState{notesState: newNotesState(stores.Project), scope: notes.Project}
	if stores.Worktree != nil {
		state.worktree = newNotesState(stores.Worktree)
		state.worktreeEnabled = true
	}
	return state
}

func (state *scopedNotesState) hasWorktree() bool { return state.worktreeEnabled }

func (state *scopedNotesState) supports(scope notes.Scope) bool {
	return scope == notes.Project || (scope == notes.Worktree && state.worktreeEnabled)
}

func (state *scopedNotesState) normalize(scope notes.Scope) notes.Scope {
	if scope == notes.Worktree && state.worktreeEnabled {
		return scope
	}
	return notes.Project
}

func (state *scopedNotesState) forScope(scope notes.Scope) *notesState {
	if state.normalize(scope) == notes.Worktree {
		return &state.worktree
	}
	return &state.notesState
}

func (state *scopedNotesState) current() *notesState {
	return state.forScope(state.scope)
}

func (state *scopedNotesState) tag(pending effect, scope notes.Scope) effect {
	if pending.kind == effectLoadNotes || pending.kind == effectDebounceNotes || pending.kind == effectSaveNotes {
		pending.notesScope = state.normalize(scope)
	}
	return pending
}

// open preserves the current scope and editor place across destination visits.
func (state *scopedNotesState) open() effect {
	state.current().finishPointer()
	state.switchPending = false
	return state.tag(state.current().open(), state.scope)
}

func (state scopedNotesState) initialLoad() effect {
	note := state.current()
	if !note.loading {
		return effect{}
	}
	return state.tag(effect{kind: effectLoadNotes, generation: note.loadGeneration}, state.scope)
}

func (state *scopedNotesState) selectScope(scope notes.Scope) effect {
	scope = state.normalize(scope)
	if !state.hasWorktree() || scope == state.scope || state.switchPending {
		return effect{}
	}
	state.pendingScope = scope
	state.switchPending = true
	currentScope := state.scope
	pending := state.current().requestExit(notesExitScope)
	if pending.kind != effectNone || state.current().saving {
		return state.tag(pending, currentScope)
	}
	return state.finishScopeSwitch()
}

func (state *scopedNotesState) toggleScope() effect {
	if state.scope == notes.Project {
		return state.selectScope(notes.Worktree)
	}
	return state.selectScope(notes.Project)
}

func (state *scopedNotesState) finishScopeSwitch() effect {
	state.current().pendingExit = notesExitNone
	state.scope = state.normalize(state.pendingScope)
	state.switchPending = false
	note := state.current()
	if note.loaded && !note.readOnly {
		note.pendingExit = notesExitNone
		note.finishPointer()
		return effect{}
	}
	return state.tag(note.open(), state.scope)
}

func (state *scopedNotesState) landLoad(scope notes.Scope, msg notesLoadedMsg, geometry ui.Geometry) {
	if !state.supports(scope) {
		return
	}
	note := state.forScope(scope)
	note.landLoad(msg)
	note.resize(geometry)
}

func (state *scopedNotesState) due(scope notes.Scope, msg notesSaveDueMsg) effect {
	if !state.supports(scope) {
		return effect{}
	}
	return state.tag(state.forScope(scope).due(msg), scope)
}

func (state *scopedNotesState) landSave(scope notes.Scope, msg notesSavedMsg) (notesExit, effect) {
	if !state.supports(scope) {
		return notesExitNone, effect{}
	}
	exit, pending := state.forScope(scope).landSave(msg)
	if msg.err != nil && state.switchPending && state.scope == state.normalize(scope) {
		state.switchPending = false
	}
	if exit == notesExitScope {
		if state.switchPending && state.scope == state.normalize(scope) {
			return notesExitNone, state.finishScopeSwitch()
		}
		return notesExitNone, effect{}
	}
	return exit, state.tag(pending, scope)
}

func (state *scopedNotesState) apply(action Action, geometry ui.Geometry) effect {
	return state.tag(state.current().apply(action, geometry), state.scope)
}

func (state *scopedNotesState) requestExit(exit notesExit) effect {
	return state.tag(state.current().requestExit(exit), state.scope)
}

func (state *scopedNotesState) finishExit() {
	state.current().pendingExit = notesExitNone
	state.switchPending = false
}

func (state *scopedNotesState) finishPointers() {
	state.notesState.finishPointer()
	if state.worktreeEnabled {
		state.worktree.finishPointer()
	}
}

func (state *scopedNotesState) resize(geometry ui.Geometry) {
	state.notesState.resize(geometry)
	if state.worktreeEnabled {
		state.worktree.resize(geometry)
	}
}

func (state *scopedNotesState) shutdown() error {
	projectErr := state.notesState.shutdown()
	if !state.worktreeEnabled {
		return projectErr
	}
	return errors.Join(projectErr, state.worktree.shutdown())
}

type notesState struct {
	editor              notes.Editor
	store               notes.Store
	restoredPlace       *notes.Place
	loaded              bool
	loading             bool
	readOnly            bool
	generation          uint64
	savedGeneration     uint64
	loadGeneration      uint64
	savingGeneration    uint64
	saving              bool
	statusErr           error
	pendingExit         notesExit
	scrollbarDragging   bool
	scrollbarGrabOffset int
	markdownGeneration  uint64
	markdownStyles      []highlight.Style
}

func newNotesState(store notes.Store) notesState {
	if store == nil {
		store = notes.NewMemoryStore()
	}
	return notesState{editor: notes.NewEditor(), store: store}
}

func (state *notesState) open() effect {
	state.pendingExit = notesExitNone
	state.finishPointer()
	// Never replace an edited buffer after a failed save. A clean or read-only
	// buffer reloads on every open so it sees the latest disk note.
	if state.loaded && state.modified() {
		return effect{}
	}
	state.loadGeneration++
	state.loading = true
	return effect{kind: effectLoadNotes, generation: state.loadGeneration}
}

func (state *notesState) landLoad(msg notesLoadedMsg) {
	if !state.loading || msg.generation != state.loadGeneration {
		return
	}
	wasLoaded := state.loaded
	state.loading = false
	state.loaded = true
	state.readOnly = msg.readOnly
	state.statusErr = msg.err
	state.generation++
	state.savedGeneration = state.generation
	state.saving = false
	state.pendingExit = notesExitNone
	if wasLoaded {
		state.editor.Reconcile(msg.text)
	} else {
		state.editor.Load(msg.text)
	}
	if state.restoredPlace != nil {
		state.editor.RestorePlace(*state.restoredPlace)
		state.restoredPlace = nil
	}
	state.refreshMarkdown()
}

func (state *notesState) apply(action Action, geometry ui.Geometry) effect {
	if state.loading || state.pendingExit != notesExitNone {
		return effect{}
	}
	changed := false
	switch action.Kind {
	case NotesInsert:
		if !state.readOnly {
			changed = state.editor.Insert(action.Text)
		}
	case NotesBackspace:
		if !state.readOnly {
			changed = state.editor.Backspace()
		}
	case NotesDelete:
		if !state.readOnly {
			changed = state.editor.Delete()
		}
	case NotesUndo:
		if !state.readOnly {
			changed = state.editor.Undo()
		}
	case NotesRedo:
		if !state.readOnly {
			changed = state.editor.Redo()
		}
	case NotesSelectAll:
		state.editor.SelectAll()
	case NotesMoveLeft:
		state.editor.MoveHorizontal(-1, action.Selecting)
	case NotesMoveRight:
		state.editor.MoveHorizontal(1, action.Selecting)
	case NotesMoveUp:
		state.editor.MoveVertical(-1, action.Selecting)
	case NotesMoveDown:
		state.editor.MoveVertical(1, action.Selecting)
	case NotesMoveWordLeft:
		state.editor.MoveWord(-1, action.Selecting)
	case NotesMoveWordRight:
		state.editor.MoveWord(1, action.Selecting)
	case NotesMoveHome:
		state.editor.MoveHome(action.Selecting)
	case NotesMoveEnd:
		state.editor.MoveEnd(action.Selecting)
	case NotesPageUp:
		state.editor.MovePage(-1, action.Selecting)
	case NotesPageDown:
		state.editor.MovePage(1, action.Selecting)
	case NotesBeginSelection:
		state.scrollbarDragging = false
		state.editor.BeginDrag(action.X, action.Y)
	case NotesDragSelection:
		state.editor.DragTo(action.X, action.Y)
	case NotesEndSelection:
		state.editor.EndDrag()
	case NotesScroll:
		state.editor.Scroll(action.Amount)
	case StartNotesScrollbarDrag:
		state.editor.EndDrag()
		state.scrollbarDragging = true
		state.scrollbarGrabOffset = action.Grab
		state.dragScrollbarTo(action.Position, geometry)
	case DragNotesScrollbar:
		state.dragScrollbarTo(action.Position, geometry)
	case FinishNotesScrollbarDrag:
		state.scrollbarDragging = false
	}
	if !changed {
		return effect{}
	}
	state.resize(geometry)
	state.generation++
	state.refreshMarkdown()
	return effect{kind: effectDebounceNotes, generation: state.generation}
}

func (state *notesState) refreshMarkdown() {
	if state.markdownGeneration == state.generation {
		return
	}
	state.markdownStyles = notes.MarkdownStyles(state.editor.Text())
	state.markdownGeneration = state.generation
}

func (state notesState) presentation() notes.Presentation {
	presentation := state.editor.Presentation()
	presentation.Styles = state.markdownStyles
	return presentation
}

func (state *notesState) dragScrollbarTo(y int, geometry ui.Geometry) {
	if !state.scrollbarDragging {
		return
	}
	presentation := state.editor.Presentation()
	bar, ok := ui.CalculateScrollbar(geometry.NotesRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		state.scrollbarDragging = false
		return
	}
	state.editor.SetScroll(bar.OffsetAt(y, state.scrollbarGrabOffset))
}

func (state *notesState) due(msg notesSaveDueMsg) effect {
	if msg.generation != state.generation || !state.modified() || state.readOnly || state.loading || state.pendingExit != notesExitNone {
		return effect{}
	}
	if state.saving {
		return effect{}
	}
	return state.beginSave()
}

func (state *notesState) beginSave() effect {
	state.saving = true
	state.savingGeneration = state.generation
	return effect{kind: effectSaveNotes, generation: state.generation, text: state.editor.Text()}
}

func (state *notesState) requestExit(exit notesExit) effect {
	state.finishPointer()
	if state.modified() && !state.readOnly {
		state.pendingExit = exit
		if state.saving {
			return effect{}
		}
		return state.beginSave()
	}
	state.pendingExit = exit
	return effect{}
}

func (state *notesState) landSave(msg notesSavedMsg) (notesExit, effect) {
	if !state.saving || msg.generation != state.savingGeneration {
		return notesExitNone, effect{}
	}
	state.saving = false
	if msg.err != nil {
		state.statusErr = msg.err
		if state.pendingExit != notesExitNone {
			// A save-gated transition cannot complete while its authored text is
			// still only in memory. Keep the editor visible and allow an explicit
			// retry after the user resolves the error.
			state.pendingExit = notesExitNone
			return notesExitNone, effect{}
		}
	} else if msg.generation == state.generation {
		state.savedGeneration = msg.generation
		state.statusErr = nil
	}
	if state.pendingExit != notesExitNone {
		if msg.generation != state.generation && !state.readOnly {
			return notesExitNone, state.beginSave()
		}
		exit := state.pendingExit
		state.pendingExit = notesExitNone
		return exit, effect{}
	}
	if state.modified() && !state.readOnly && msg.generation != state.generation {
		return notesExitNone, effect{kind: effectDebounceNotes, generation: state.generation}
	}
	return notesExitNone, effect{}
}

func (state *notesState) modified() bool {
	return state.loaded && state.generation != state.savedGeneration
}

func (state *notesState) finishPointer() {
	state.editor.EndDrag()
	state.scrollbarDragging = false
}

func (state notesState) status() (string, bool) {
	line, column := state.editor.CursorLineColumn()
	label := "saved"
	errorTone := false
	switch {
	case state.loading:
		label = "loading…"
	case state.readOnly && state.statusErr != nil:
		label = "read-only • " + state.statusErr.Error()
		errorTone = true
	case state.readOnly:
		label = "read-only • another reviewr is editing"
	case state.saving:
		label = "saving…"
	case state.statusErr != nil:
		label = "modified • " + state.statusErr.Error()
		errorTone = true
	case state.modified():
		label = "modified"
	}
	return fmt.Sprintf("Ln %d, Col %d  •  %s", line, column, label), errorTone
}

func (state *notesState) resize(geometry ui.Geometry) {
	rows := geometry.NotesRows
	state.editor.Resize(rows.Width, rows.Height)
	presentation := state.editor.Presentation()
	if bar, ok := ui.CalculateScrollbar(rows, len(presentation.Document.Rows), presentation.Top); ok {
		state.editor.Resize(bar.Content.Width, bar.Content.Height)
	}
}

func (state *notesState) shutdown() error {
	var saveErr error
	if state.modified() && !state.readOnly {
		saveErr = state.store.Save(state.editor.Text())
		if saveErr == nil {
			state.savedGeneration = state.generation
		}
	}
	return errors.Join(saveErr, state.store.Close())
}
