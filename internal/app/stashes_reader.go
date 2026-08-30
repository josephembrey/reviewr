package app

import (
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (state stashState) landReader(msg stashFileLoadedMsg, _ int) stashState {
	selectedOID, selected := state.place.SelectedIdentity()
	if msg.generation != state.readerGeneration || !selected || msg.oid != selectedOID ||
		msg.oid != state.readerOID || msg.fileIdentity != state.readerFileID {
		return state
	}
	oldLines := readerRowIdentities(state.readerRows())
	if len(oldLines) == 0 && len(state.restoredReaderRows) != 0 {
		oldLines = append([]string(nil), state.restoredReaderRows...)
	}
	oldOffset := state.place.ReaderOffset
	state.reader = msg.document
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	state.saveReaderPlace()
	return state
}

func (state *stashState) requestSelectedFile() effect {
	stash, stashOK := state.selectedStash()
	if !stashOK || state.fileSelected < 0 || state.fileSelected >= len(state.files) {
		state.clearReader()
		return effect{}
	}
	change := state.files[state.fileSelected]
	fileIdentity := change.Identity()
	state.readerGeneration++
	state.readerLoading = true
	if state.readerOID != stash.OID || state.readerFileID != fileIdentity {
		state.reader = repository.ChangeDocument{}
		state.readerPresentation = nil
		state.resetReaderContext()
		state.restoredReaderRows = nil
	}
	state.readerOID = stash.OID
	state.readerFileID = fileIdentity
	if len(state.restoredReaderRows) == 0 {
		state.place.ClampReaderSource(len(state.readerRows()))
	}
	return effect{
		kind: effectLoadStashFile, generation: state.readerGeneration, identity: stash.OID,
		stashSource: stash.Source, changedFile: change,
	}
}

func (state *stashState) requestSelectedFileQuiet() effect {
	stash, stashOK := state.selectedStash()
	if !stashOK || state.fileSelected < 0 || state.fileSelected >= len(state.files) {
		return effect{}
	}
	change := state.files[state.fileSelected]
	state.readerGeneration++
	state.readerOID = stash.OID
	state.readerFileID = change.Identity()
	return effect{
		kind: effectLoadStashFile, generation: state.readerGeneration,
		identity: stash.OID, stashSource: stash.Source, changedFile: change, background: true,
	}
}

func (state *stashState) saveReaderPlace() {
	oid, ok := state.place.SelectedIdentity()
	if !ok {
		return
	}
	identity := state.selectedFileIdentity()
	if identity == "" {
		return
	}
	state.readerPlaces[oid] = stashReaderPlace{
		fileIdentity: identity, readerOffset: state.place.ReaderOffset,
		readerColumn: state.place.ReaderColumn,
	}
}

func (state *stashState) clearReader() {
	state.readerGeneration++
	state.reader = repository.ChangeDocument{}
	state.readerPresentation = nil
	state.resetReaderContext()
	state.restoredReaderRows = nil
	state.readerOID = ""
	state.readerFileID = ""
	state.readerLoading = false
}

func (state stashState) readerDocument() ui.ReaderDocument {
	return state.rawReaderDocument().WithContextFoldProgress(state.readerContextProgress, readerContextAnimationSteps)
}

func (state stashState) rawReaderDocument() ui.ReaderDocument {
	if state.readerPresentation != nil {
		return *state.readerPresentation
	}
	return state.deriveReaderDocument()
}

func (state *stashState) setReaderContextExpanded(expanded bool) (bool, bool) {
	if state.readerContextExpanded == expanded || !state.rawReaderDocument().ContextFoldable() {
		return false, false
	}
	state.readerContextExpanded = expanded
	state.readerContextGeneration++
	state.advanceReaderContextPresentation()
	return true, readerContextAnimating(state.readerContextProgress, expanded)
}

func (state *stashState) advanceReaderContext(generation uint64) (bool, bool) {
	if generation != state.readerContextGeneration || !readerContextAnimating(state.readerContextProgress, state.readerContextExpanded) {
		return false, false
	}
	state.advanceReaderContextPresentation()
	return true, readerContextAnimating(state.readerContextProgress, state.readerContextExpanded)
}

func (state *stashState) advanceReaderContextPresentation() {
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	state.readerContextProgress = stepReaderContext(state.readerContextProgress, state.readerContextExpanded)
	state.place.ReaderOffset = reconcileLogicalLine(oldRows, oldOffset, readerRowIdentities(state.readerRows()))
	if state.place.ReaderOffset != oldOffset {
		state.place.ReaderColumn = 0
	}
	state.place.ClampReaderSource(len(state.readerRows()))
	state.saveReaderPlace()
}

func (state *stashState) resetReaderContext() {
	state.readerContextExpanded = false
	state.readerContextProgress = 0
	state.readerContextGeneration++
}

func (state stashState) readerRows() []ui.ReaderRow {
	return state.readerDocument().Rows
}

func (state stashState) deriveReaderDocument() ui.ReaderDocument {
	if state.readerFileID == "" || state.reader.Change.Path == "" {
		return ui.ReaderDocument{}
	}
	return (readerDocument{Change: &state.reader, Mode: workspace.DiffReader}).build()
}
