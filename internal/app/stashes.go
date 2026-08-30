package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type stashReaderPlace struct {
	fileIdentity string
	readerOffset int
	readerColumn int
}

// stashState owns Stashes independently from Log and Files. Selectors are
// presentation; all reconciliation and reader place use immutable OIDs.
type stashState struct {
	place navigation.State

	stashes                 []repository.Stash
	files                   []repository.ChangedFile
	fileSelected            int
	filesOID                string
	reader                  repository.ChangeDocument
	readerPresentation      *ui.ReaderDocument
	readerContextExpanded   bool
	readerContextProgress   int
	readerContextGeneration uint64
	restoredReaderRows      []string
	readerOID               string
	readerFileID            string
	readerPlaces            map[string]stashReaderPlace

	listGeneration   uint64
	filesGeneration  uint64
	readerGeneration uint64
	loaded           bool
	listLoading      bool
	filesLoading     bool
	readerLoading    bool
	listError        error
	filesError       error
}

func newStashState() stashState {
	return stashState{
		place:        navigation.State{Focus: navigation.FocusNavigator},
		readerPlaces: make(map[string]stashReaderPlace),
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
		if msg.background {
			return state, effect{}
		}
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	oldOID, hadSelection := state.place.SelectedIdentity()
	state.saveReaderPlace()
	state.stashes = append([]repository.Stash(nil), msg.stashes...)
	identities := make([]string, len(state.stashes))
	for index, stash := range state.stashes {
		identities[index] = stash.OID
	}
	state.place.Reconcile(identities)
	state.place.EnsureSelectionVisible(visibleRows)
	if len(state.stashes) == 0 {
		state.clearFiles()
		return state, effect{}
	}
	if selectedOID, ok := state.place.SelectedIdentity(); msg.background && ok && hadSelection && selectedOID == oldOID {
		return state, state.requestSelectedFilesQuiet()
	}
	return state, state.requestSelectedFiles()
}

func (state stashState) landFiles(msg stashFilesLoadedMsg, _ int) (stashState, effect) {
	selectedOID, selected := state.place.SelectedIdentity()
	if msg.generation != state.filesGeneration || !selected || msg.oid != selectedOID || msg.oid != state.filesOID {
		return state, effect{}
	}
	state.filesLoading = false
	if msg.err != nil {
		if msg.background {
			return state, effect{}
		}
		state.filesError = msg.err
		state.clearReader()
		return state, effect{}
	}
	state.filesError = nil
	oldFiles := state.files
	oldIdentity := state.selectedFileIdentity()
	state.files = append([]repository.ChangedFile(nil), msg.files...)
	if len(state.files) == 0 {
		state.fileSelected = 0
		state.clearReader()
		state.place.ReaderOffset = 0
		state.place.ReaderColumn = 0
		return state, effect{}
	}
	identity := state.reconcileFilePlace(selectedOID, oldFiles, oldIdentity)
	if msg.background && identity == oldIdentity {
		return state, state.requestSelectedFileQuiet()
	}
	return state, state.requestSelectedFile()
}

func (state *stashState) reconcileFilePlace(selectedOID string, oldFiles []repository.ChangedFile, oldIdentity string) string {
	wanted := oldIdentity
	saved := state.readerPlaces[selectedOID]
	if saved.fileIdentity != "" {
		wanted = saved.fileIdentity
	}
	oldIdentities := changedFileIdentities(oldFiles)
	newIdentities := changedFileIdentities(state.files)
	identity, ok := navigation.ReconcileIdentity(oldIdentities, wanted, newIdentities)
	if !ok {
		identity = newIdentities[0]
	}
	state.fileSelected = indexIdentity(newIdentities, identity)
	if saved.fileIdentity == identity {
		state.place.ReaderOffset = saved.readerOffset
		state.place.ReaderColumn = saved.readerColumn
	} else if oldIdentity != identity {
		state.place.ReaderOffset = 0
		state.place.ReaderColumn = 0
	}
	return identity
}

func (state *stashState) selectStashDelta(delta, visibleRows int) effect {
	return state.selectStashIndex(state.place.Selected+delta, visibleRows)
}

func (state *stashState) selectStashIndex(index, visibleRows int) effect {
	state.saveReaderPlace()
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	return state.requestSelectedFiles()
}

func (state *stashState) selectFileDelta(delta, visibleRows int) effect {
	if len(state.files) == 0 {
		return effect{}
	}
	state.saveReaderPlace()
	next := max(0, min(len(state.files)-1, state.fileSelected+delta))
	if next == state.fileSelected {
		return effect{}
	}
	state.fileSelected = next
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
	if oid, ok := state.place.SelectedIdentity(); ok {
		if saved := state.readerPlaces[oid]; saved.fileIdentity == state.selectedFileIdentity() {
			state.place.ReaderOffset = saved.readerOffset
			state.place.ReaderColumn = saved.readerColumn
		}
	}
	return state.requestSelectedFile()
}

func (state *stashState) requestSelectedFiles() effect {
	stash, ok := state.selectedStash()
	if !ok {
		state.clearFiles()
		return effect{}
	}
	state.filesGeneration++
	state.filesLoading = true
	state.filesError = nil
	if state.filesOID != stash.OID {
		state.files = nil
		state.fileSelected = 0
		state.clearReader()
		state.place.ReaderOffset = 0
		state.place.ReaderColumn = 0
	}
	state.filesOID = stash.OID
	return effect{kind: effectLoadStashFiles, generation: state.filesGeneration, identity: stash.OID, stashSource: stash.Source}
}

func (state *stashState) requestSelectedFilesQuiet() effect {
	stash, ok := state.selectedStash()
	if !ok {
		return effect{}
	}
	state.filesGeneration++
	state.filesOID = stash.OID
	return effect{
		kind: effectLoadStashFiles, generation: state.filesGeneration,
		identity: stash.OID, stashSource: stash.Source, background: true,
	}
}

func (state *stashState) clearFiles() {
	state.filesGeneration++
	state.files = nil
	state.fileSelected = 0
	state.filesOID = ""
	state.filesLoading = false
	state.filesError = nil
	state.clearReader()
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
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

func (state stashState) selectedFileIdentity() string {
	if state.fileSelected < 0 || state.fileSelected >= len(state.files) {
		return ""
	}
	return state.files[state.fileSelected].Identity()
}

func changedFileIdentities(files []repository.ChangedFile) []string {
	identities := make([]string, len(files))
	for index, file := range files {
		identities[index] = file.Identity()
	}
	return identities
}

func indexIdentity(identities []string, identity string) int {
	for index, candidate := range identities {
		if candidate == identity {
			return index
		}
	}
	return 0
}
