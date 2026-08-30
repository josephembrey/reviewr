package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/josephembrey/reviewr/internal/scratch"
	"github.com/josephembrey/reviewr/internal/ui"
)

const scratchSaveDebounce = 350 * time.Millisecond

type scratchExit uint8

const (
	scratchExitNone scratchExit = iota
	scratchExitClose
	scratchExitFiles
	scratchExitGit
	scratchExitQuit
	scratchExitScope
)

// scopedScratchState owns the two possible Scratch places while keeping the
// root model unaware of their persistence and transition details.
type scopedScratchState struct {
	scratchState
	worktree        scratchState
	worktreeEnabled bool
	scope           scratch.Scope
	pendingScope    scratch.Scope
	switchPending   bool
}

func newScopedScratchState(stores scratch.Stores) scopedScratchState {
	state := scopedScratchState{scratchState: newScratchState(stores.Project), scope: scratch.Project}
	if stores.Worktree != nil {
		state.worktree = newScratchState(stores.Worktree)
		state.worktreeEnabled = true
	}
	return state
}

func (state *scopedScratchState) hasWorktree() bool { return state.worktreeEnabled }

func (state *scopedScratchState) supports(scope scratch.Scope) bool {
	return scope == scratch.Project || (scope == scratch.Worktree && state.worktreeEnabled)
}

func (state *scopedScratchState) normalize(scope scratch.Scope) scratch.Scope {
	if scope == scratch.Worktree && state.worktreeEnabled {
		return scope
	}
	return scratch.Project
}

func (state *scopedScratchState) forScope(scope scratch.Scope) *scratchState {
	if state.normalize(scope) == scratch.Worktree {
		return &state.worktree
	}
	return &state.scratchState
}

func (state *scopedScratchState) current() *scratchState {
	return state.forScope(state.scope)
}

func (state *scopedScratchState) tag(pending effect, scope scratch.Scope) effect {
	if pending.kind == effectLoadScratch || pending.kind == effectDebounceScratch || pending.kind == effectSaveScratch {
		pending.scratchScope = state.normalize(scope)
	}
	return pending
}

// open always enters at project scope; the worktree scope remains available
// in every checkout, including the primary one.
func (state *scopedScratchState) open() effect {
	state.current().finishPointer()
	state.scope = scratch.Project
	state.switchPending = false
	return state.tag(state.scratchState.open(), scratch.Project)
}

func (state *scopedScratchState) selectScope(scope scratch.Scope) effect {
	scope = state.normalize(scope)
	if !state.hasWorktree() || scope == state.scope || state.switchPending {
		return effect{}
	}
	state.pendingScope = scope
	state.switchPending = true
	currentScope := state.scope
	pending := state.current().requestExit(scratchExitScope)
	if pending.kind != effectNone || state.current().saving {
		return state.tag(pending, currentScope)
	}
	return state.finishScopeSwitch()
}

func (state *scopedScratchState) toggleScope() effect {
	if state.scope == scratch.Project {
		return state.selectScope(scratch.Worktree)
	}
	return state.selectScope(scratch.Project)
}

func (state *scopedScratchState) finishScopeSwitch() effect {
	state.current().pendingExit = scratchExitNone
	state.scope = state.normalize(state.pendingScope)
	state.switchPending = false
	note := state.current()
	if note.loaded && !note.readOnly {
		note.pendingExit = scratchExitNone
		note.finishPointer()
		return effect{}
	}
	return state.tag(note.open(), state.scope)
}

func (state *scopedScratchState) landLoad(scope scratch.Scope, msg scratchLoadedMsg, geometry ui.Geometry) {
	if !state.supports(scope) {
		return
	}
	note := state.forScope(scope)
	note.landLoad(msg)
	note.resize(geometry)
}

func (state *scopedScratchState) due(scope scratch.Scope, msg scratchSaveDueMsg) effect {
	if !state.supports(scope) {
		return effect{}
	}
	return state.tag(state.forScope(scope).due(msg), scope)
}

func (state *scopedScratchState) landSave(scope scratch.Scope, msg scratchSavedMsg) (scratchExit, effect) {
	if !state.supports(scope) {
		return scratchExitNone, effect{}
	}
	exit, pending := state.forScope(scope).landSave(msg)
	if exit == scratchExitScope {
		if state.switchPending && state.scope == state.normalize(scope) {
			return scratchExitNone, state.finishScopeSwitch()
		}
		return scratchExitNone, effect{}
	}
	return exit, state.tag(pending, scope)
}

func (state *scopedScratchState) apply(action Action, geometry ui.Geometry) effect {
	return state.tag(state.current().apply(action, geometry), state.scope)
}

func (state *scopedScratchState) requestExit(exit scratchExit) effect {
	return state.tag(state.current().requestExit(exit), state.scope)
}

func (state *scopedScratchState) finishExit() {
	state.current().pendingExit = scratchExitNone
	state.switchPending = false
}

func (state *scopedScratchState) resize(geometry ui.Geometry) {
	state.scratchState.resize(geometry)
	if state.worktreeEnabled {
		state.worktree.resize(geometry)
	}
}

func (state *scopedScratchState) shutdown() error {
	projectErr := state.scratchState.shutdown()
	if !state.worktreeEnabled {
		return projectErr
	}
	return errors.Join(projectErr, state.worktree.shutdown())
}

type scratchState struct {
	editor              scratch.Editor
	store               scratch.Store
	loaded              bool
	loading             bool
	readOnly            bool
	generation          uint64
	savedGeneration     uint64
	loadGeneration      uint64
	savingGeneration    uint64
	saving              bool
	statusErr           error
	pendingExit         scratchExit
	scrollbarDragging   bool
	scrollbarGrabOffset int
}

func newScratchState(store scratch.Store) scratchState {
	if store == nil {
		store = scratch.NewMemoryStore()
	}
	return scratchState{editor: scratch.NewEditor(), store: store}
}

func (state *scratchState) open() effect {
	state.pendingExit = scratchExitNone
	state.finishPointer()
	// Never replace an edited buffer after a failed save. A clean or read-only
	// buffer reloads on every open so it sees the latest disk note.
	if state.loaded && state.modified() {
		return effect{}
	}
	state.loadGeneration++
	state.loading = true
	return effect{kind: effectLoadScratch, generation: state.loadGeneration}
}

func (state *scratchState) landLoad(msg scratchLoadedMsg) {
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
	state.pendingExit = scratchExitNone
	if wasLoaded {
		state.editor.Reconcile(msg.text)
	} else {
		state.editor.Load(msg.text)
	}
}

func (state *scratchState) apply(action Action, geometry ui.Geometry) effect {
	if state.loading || state.pendingExit != scratchExitNone {
		return effect{}
	}
	changed := false
	switch action.Kind {
	case ScratchInsert:
		if !state.readOnly {
			changed = state.editor.Insert(action.Text)
		}
	case ScratchBackspace:
		if !state.readOnly {
			changed = state.editor.Backspace()
		}
	case ScratchDelete:
		if !state.readOnly {
			changed = state.editor.Delete()
		}
	case ScratchUndo:
		if !state.readOnly {
			changed = state.editor.Undo()
		}
	case ScratchRedo:
		if !state.readOnly {
			changed = state.editor.Redo()
		}
	case ScratchSelectAll:
		state.editor.SelectAll()
	case ScratchMoveLeft:
		state.editor.MoveHorizontal(-1, action.Selecting)
	case ScratchMoveRight:
		state.editor.MoveHorizontal(1, action.Selecting)
	case ScratchMoveUp:
		state.editor.MoveVertical(-1, action.Selecting)
	case ScratchMoveDown:
		state.editor.MoveVertical(1, action.Selecting)
	case ScratchMoveWordLeft:
		state.editor.MoveWord(-1, action.Selecting)
	case ScratchMoveWordRight:
		state.editor.MoveWord(1, action.Selecting)
	case ScratchMoveHome:
		state.editor.MoveHome(action.Selecting)
	case ScratchMoveEnd:
		state.editor.MoveEnd(action.Selecting)
	case ScratchPageUp:
		state.editor.MovePage(-1, action.Selecting)
	case ScratchPageDown:
		state.editor.MovePage(1, action.Selecting)
	case ScratchBeginSelection:
		state.scrollbarDragging = false
		state.editor.BeginDrag(action.X, action.Y)
	case ScratchDragSelection:
		state.editor.DragTo(action.X, action.Y)
	case ScratchEndSelection:
		state.editor.EndDrag()
	case ScratchScroll:
		state.editor.Scroll(action.Amount)
	case StartScratchScrollbarDrag:
		state.editor.EndDrag()
		state.scrollbarDragging = true
		state.scrollbarGrabOffset = action.Grab
		state.dragScrollbarTo(action.Position, geometry)
	case DragScratchScrollbar:
		state.dragScrollbarTo(action.Position, geometry)
	case FinishScratchScrollbarDrag:
		state.scrollbarDragging = false
	}
	if !changed {
		return effect{}
	}
	state.resize(geometry)
	state.generation++
	return effect{kind: effectDebounceScratch, generation: state.generation}
}

func (state *scratchState) dragScrollbarTo(y int, geometry ui.Geometry) {
	if !state.scrollbarDragging {
		return
	}
	presentation := state.editor.Presentation()
	bar, ok := ui.CalculateScrollbar(geometry.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		state.scrollbarDragging = false
		return
	}
	state.editor.SetScroll(bar.OffsetAt(y, state.scrollbarGrabOffset))
}

func (state *scratchState) due(msg scratchSaveDueMsg) effect {
	if msg.generation != state.generation || !state.modified() || state.readOnly || state.loading || state.pendingExit != scratchExitNone {
		return effect{}
	}
	if state.saving {
		return effect{}
	}
	return state.beginSave()
}

func (state *scratchState) beginSave() effect {
	state.saving = true
	state.savingGeneration = state.generation
	return effect{kind: effectSaveScratch, generation: state.generation, text: state.editor.Text()}
}

func (state *scratchState) requestExit(exit scratchExit) effect {
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

func (state *scratchState) landSave(msg scratchSavedMsg) (scratchExit, effect) {
	if !state.saving || msg.generation != state.savingGeneration {
		return scratchExitNone, effect{}
	}
	state.saving = false
	if msg.err != nil {
		state.statusErr = msg.err
	} else if msg.generation == state.generation {
		state.savedGeneration = msg.generation
		state.statusErr = nil
	}
	if state.pendingExit != scratchExitNone {
		if msg.generation != state.generation && !state.readOnly {
			return scratchExitNone, state.beginSave()
		}
		exit := state.pendingExit
		state.pendingExit = scratchExitNone
		return exit, effect{}
	}
	if state.modified() && !state.readOnly && msg.generation != state.generation {
		return scratchExitNone, effect{kind: effectDebounceScratch, generation: state.generation}
	}
	return scratchExitNone, effect{}
}

func (state *scratchState) modified() bool {
	return state.loaded && state.generation != state.savedGeneration
}

func (state *scratchState) finishPointer() {
	state.editor.EndDrag()
	state.scrollbarDragging = false
}

func (state scratchState) status() (string, bool) {
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

func (state *scratchState) resize(geometry ui.Geometry) {
	rows := geometry.ScratchRows
	state.editor.Resize(rows.Width, rows.Height)
	presentation := state.editor.Presentation()
	if bar, ok := ui.CalculateScrollbar(rows, len(presentation.Document.Rows), presentation.Top); ok {
		state.editor.Resize(bar.Content.Width, bar.Content.Height)
	}
}

func (state *scratchState) shutdown() error {
	var saveErr error
	if state.modified() && !state.readOnly {
		saveErr = state.store.Save(state.editor.Text())
		if saveErr == nil {
			state.savedGeneration = state.generation
		}
	}
	return errors.Join(saveErr, state.store.Close())
}
