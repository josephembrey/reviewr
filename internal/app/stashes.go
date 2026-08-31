package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// stashState owns only the stash collection around the shared immutable
// files/diff inspection state machine.
type stashState struct {
	place      navigation.State
	focus      workspace.GitFocus
	stashes    []repository.Stash
	inspection changeInspectionState

	listGeneration uint64
	loaded         bool
	listLoading    bool
	listError      error
}

func newStashState() stashState {
	return stashState{
		place:      navigation.State{Focus: navigation.FocusNavigator},
		focus:      workspace.GitStash,
		inspection: newChangeInspectionState(),
	}
}

func (state *stashState) reload() effect {
	state.listGeneration++
	state.listLoading = true
	state.listError = nil
	return effect{kind: effectLoadStashes, generation: state.listGeneration}
}

func (state *stashState) poll() effect {
	state.listGeneration++
	return effect{kind: effectLoadStashes, generation: state.listGeneration, background: true}
}

func (state stashState) landStashes(msg stashesLoadedMsg, visibleRows int) (stashState, effect) {
	if msg.generation != state.listGeneration {
		return state, effect{}
	}
	state.loaded = true
	state.listLoading = false
	if msg.err != nil {
		if !msg.background {
			state.listError = msg.err
		}
		return state, effect{}
	}
	state.listError = nil
	oldOID, hadSelection := state.place.SelectedIdentity()
	state.inspection.saveReaderPlace()
	state.stashes = append([]repository.Stash(nil), msg.stashes...)
	identities := make([]string, len(state.stashes))
	for index, stash := range state.stashes {
		identities[index] = stash.OID
	}
	state.place.Reconcile(identities)
	state.place.EnsureSelectionVisible(visibleRows)
	if len(state.stashes) == 0 {
		state.inspection = newChangeInspectionState()
		return state, effect{}
	}
	selectedOID, selected := state.place.SelectedIdentity()
	quiet := msg.background && selected && hadSelection && selectedOID == oldOID
	return state, state.requestSelectedFiles(quiet)
}

func (state *stashState) landFiles(msg stashFilesLoadedMsg) effect {
	accepted, loadReader := state.inspection.landFiles(msg.generation, msg.oid, msg.files, msg.err, msg.background)
	if !accepted || !loadReader {
		return effect{}
	}
	return state.requestSelectedFile(msg.background)
}

func (state *stashState) landReader(msg stashFileLoadedMsg) {
	state.inspection.landReader(msg.generation, msg.oid, msg.fileIdentity, msg.document, msg.presentation)
}

func (state *stashState) selectStashDelta(delta, visibleRows int) effect {
	return state.selectStashIndex(state.place.Selected+delta, visibleRows)
}

func (state *stashState) selectStashIndex(index, visibleRows int) effect {
	state.inspection.saveReaderPlace()
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	return state.requestSelectedFiles(false)
}

func (state *stashState) selectFileDelta(delta, visibleRows int) effect {
	return state.selectFileIndex(state.inspection.place.Selected+delta, visibleRows)
}

func (state *stashState) selectFileIndex(index, visibleRows int) effect {
	if !state.inspection.selectIndex(index, visibleRows) {
		return effect{}
	}
	return state.requestSelectedFile(false)
}

func (state *stashState) requestSelectedFiles(quiet bool) effect {
	stash, ok := state.selectedStash()
	if !ok {
		state.inspection = newChangeInspectionState()
		return effect{}
	}
	generation := state.inspection.beginFiles(stash.OID, quiet)
	return effect{
		kind: effectLoadStashFiles, generation: generation, identity: stash.OID,
		stashSource: stash.Source, background: quiet,
	}
}

func (state *stashState) requestSelectedFile(quiet bool) effect {
	stash, ok := state.selectedStash()
	if !ok {
		return effect{}
	}
	generation, file, ok := state.inspection.beginReader(quiet)
	if !ok {
		return effect{}
	}
	return effect{
		kind: effectLoadStashFile, generation: generation, identity: stash.OID,
		stashSource: stash.Source, changedFile: file, background: quiet,
	}
}

func (state stashState) selectedStash() (repository.Stash, bool) {
	oid, ok := state.place.SelectedIdentity()
	if !ok {
		return repository.Stash{}, false
	}
	for _, stash := range state.stashes {
		if stash.OID == oid {
			return stash, true
		}
	}
	return repository.Stash{}, false
}

func (state *stashState) ensureFileVisible(visibleRows int) {
	state.inspection.place.EnsureSelectionVisible(visibleRows)
}

func (state stashState) readerDocument() ui.ReaderDocument {
	return state.inspection.readerDocument()
}

func (state stashState) readerRows() []ui.ReaderRow {
	return state.inspection.readerRows()
}

func (state *stashState) toggleReaderContextFold(identity string) (bool, bool) {
	return state.inspection.changeReaderContextFold(identity, nil)
}

func (state *stashState) setReaderContextFold(identity string, expanded bool) (bool, bool) {
	return state.inspection.changeReaderContextFold(identity, &expanded)
}

func (state *stashState) advanceReaderContext(generation uint64) (bool, bool) {
	return state.inspection.advanceReaderContext(generation)
}
