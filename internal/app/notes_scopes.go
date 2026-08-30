package app

import (
	"errors"

	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/ui"
)

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
