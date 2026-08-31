package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// changeReaderPlace is the authored file/diff place for one immutable owner
// (a commit OID or stash OID).
type changeReaderPlace struct {
	fileIdentity string
	fileTop      int
	readerOffset int
	readerColumn int
	readerCursor int
}

// changeInspectionState is the shared state machine behind immutable commit
// and stash file/diff inspection. Both consumers therefore use the production
// reader document, folding, wrapping, syntax, cursor, and place reconciliation.
type changeInspectionState struct {
	place navigation.State

	ownerID            string
	files              []repository.ChangedFile
	reader             repository.ChangeDocument
	readerPresentation *ui.ReaderDocument
	readerContext      readerContextState
	restoredReaderRows []string
	readerOwnerID      string
	readerFileID       string
	readerPlaces       map[string]changeReaderPlace

	filesGeneration  uint64
	readerGeneration uint64
	filesLoading     bool
	readerLoading    bool
	filesError       error
}

func newChangeInspectionState() changeInspectionState {
	return changeInspectionState{
		place:        navigation.State{Focus: navigation.FocusNavigator},
		readerPlaces: make(map[string]changeReaderPlace),
	}
}

func (state *changeInspectionState) beginFiles(ownerID string, quiet bool) uint64 {
	state.saveReaderPlace()
	state.filesGeneration++
	state.filesLoading = !quiet
	if !quiet {
		state.filesError = nil
	}
	if state.ownerID != ownerID {
		state.ownerID = ownerID
		state.files = nil
		state.place.Items = nil
		state.place.Selected = 0
		state.place.Top = 0
		state.clearReader()
	}
	return state.filesGeneration
}

func (state *changeInspectionState) landFiles(generation uint64, ownerID string, files []repository.ChangedFile, err error, background bool) (bool, bool) {
	if generation != state.filesGeneration || ownerID != state.ownerID {
		return false, false
	}
	state.filesLoading = false
	if err != nil {
		if !background {
			state.filesError = err
			state.clearReader()
		}
		return true, false
	}
	state.filesError = nil
	oldIdentity, hadSelection := state.place.SelectedIdentity()
	oldItems := append([]string(nil), state.place.Items...)
	state.files = append([]repository.ChangedFile(nil), files...)
	identities := changedFileIdentities(state.files)
	state.place.Reconcile(identities)
	if saved := state.readerPlaces[ownerID]; saved.fileIdentity != "" {
		if index := indexIdentity(identities, saved.fileIdentity); index >= 0 && index < len(identities) && identities[index] == saved.fileIdentity {
			state.place.Selected = index
			state.place.Top = max(0, saved.fileTop)
			state.place.ReaderOffset = max(0, saved.readerOffset)
			state.place.ReaderColumn = max(0, saved.readerColumn)
			state.place.ReaderCursor = max(0, saved.readerCursor)
		}
	}
	if len(state.files) == 0 {
		state.clearReader()
		return true, false
	}
	selected, _ := state.place.SelectedIdentity()
	preserve := background && hadSelection && selected == oldIdentity && sameIdentities(oldItems, identities)
	return true, !preserve || state.readerFileID == selected
}

func sameIdentities(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
	return -1
}

func (state *changeInspectionState) selectDelta(delta, visibleRows int) bool {
	state.saveReaderPlace()
	if !state.place.SelectDelta(delta, visibleRows) {
		return false
	}
	state.restoreSelectedReaderPlace()
	return true
}

func (state *changeInspectionState) selectIndex(index, visibleRows int) bool {
	state.saveReaderPlace()
	if !state.place.SelectIndex(index, visibleRows) {
		return false
	}
	state.restoreSelectedReaderPlace()
	return true
}

func (state *changeInspectionState) restoreSelectedReaderPlace() {
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
	state.place.ReaderCursor = 0
	if saved := state.readerPlaces[state.ownerID]; saved.fileIdentity == state.selectedFileIdentity() {
		state.place.ReaderOffset = saved.readerOffset
		state.place.ReaderColumn = saved.readerColumn
		state.place.ReaderCursor = saved.readerCursor
	}
}

func (state *changeInspectionState) beginReader(quiet bool) (uint64, repository.ChangedFile, bool) {
	file, ok := state.selectedFile()
	if !ok || state.ownerID == "" {
		state.clearReader()
		return 0, repository.ChangedFile{}, false
	}
	state.readerGeneration++
	state.readerLoading = !quiet
	identity := file.Identity()
	if state.readerOwnerID != state.ownerID || state.readerFileID != identity {
		state.reader = repository.ChangeDocument{}
		state.readerPresentation = nil
		// Saved rows denote session restoration. They and the restored fold
		// policy bridge the first fresh document; a genuinely new reader starts
		// from the configured default instead.
		if state.readerOwnerID != "" || state.readerFileID != "" || len(state.restoredReaderRows) == 0 {
			state.readerContext.reset()
			state.restoredReaderRows = nil
		}
	}
	state.readerOwnerID = state.ownerID
	state.readerFileID = identity
	if len(state.restoredReaderRows) == 0 {
		state.place.ClampReaderSource(len(state.readerRows()))
	}
	return state.readerGeneration, file, true
}

func (state *changeInspectionState) landReader(generation uint64, ownerID, fileIdentity string, document repository.ChangeDocument, presentation ui.ReaderDocument) bool {
	if generation != state.readerGeneration || ownerID != state.ownerID || ownerID != state.readerOwnerID || fileIdentity != state.readerFileID || fileIdentity != state.selectedFileIdentity() {
		return false
	}
	oldRows := readerRowIdentities(state.readerRows())
	if len(oldRows) == 0 && len(state.restoredReaderRows) != 0 {
		oldRows = append([]string(nil), state.restoredReaderRows...)
	}
	oldOffset := state.place.ReaderOffset
	oldCursor := state.place.ReaderCursor
	state.reader = document
	state.readerLoading = false
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.readerContext.reconcile(presentation)
	state.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
	state.restoredReaderRows = nil
	state.saveReaderPlace()
	return true
}

func (state *changeInspectionState) selectedFile() (repository.ChangedFile, bool) {
	if state.place.Selected < 0 || state.place.Selected >= len(state.files) {
		return repository.ChangedFile{}, false
	}
	return state.files[state.place.Selected], true
}

func (state changeInspectionState) selectedFileIdentity() string {
	file, ok := state.selectedFile()
	if !ok {
		return ""
	}
	return file.Identity()
}

func (state *changeInspectionState) saveReaderPlace() {
	if state.ownerID == "" || state.selectedFileIdentity() == "" {
		return
	}
	state.readerPlaces[state.ownerID] = changeReaderPlace{
		fileIdentity: state.selectedFileIdentity(), fileTop: state.place.Top,
		readerOffset: state.place.ReaderOffset, readerColumn: state.place.ReaderColumn,
		readerCursor: state.place.ReaderCursor,
	}
}

func (state *changeInspectionState) clearReader() {
	state.readerGeneration++
	state.reader = repository.ChangeDocument{}
	state.readerPresentation = nil
	state.readerContext.reset()
	state.restoredReaderRows = nil
	state.readerOwnerID = ""
	state.readerFileID = ""
	state.readerLoading = false
}

func (state changeInspectionState) rawReaderDocument() ui.ReaderDocument {
	if state.readerPresentation != nil {
		return *state.readerPresentation
	}
	return state.deriveReaderDocument()
}

func (state changeInspectionState) readerDocument() ui.ReaderDocument {
	return state.readerContext.document(state.rawReaderDocument())
}

func (state changeInspectionState) deriveReaderDocument() ui.ReaderDocument {
	if state.readerFileID == "" || state.reader.Change.Path == "" {
		return ui.ReaderDocument{}
	}
	return (readerDocument{Change: &state.reader, Mode: workspace.DiffReader}).build()
}

func (state changeInspectionState) readerRows() []ui.ReaderRow {
	return state.readerDocument().Rows
}

func (state *changeInspectionState) changeReaderContextFold(identity string, expanded *bool) (bool, bool) {
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	oldCursor := state.place.ReaderCursor
	var changed, animating bool
	if expanded == nil {
		changed, animating = state.readerContext.toggleFold(state.rawReaderDocument(), identity)
	} else {
		changed, animating = state.readerContext.setFold(state.rawReaderDocument(), identity, *expanded)
	}
	if changed {
		state.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
	}
	return changed, animating
}

func (state *changeInspectionState) advanceReaderContext(generation uint64) (bool, bool) {
	if generation != state.readerContext.generation || !state.readerContext.animating(state.rawReaderDocument()) {
		return false, false
	}
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	oldCursor := state.place.ReaderCursor
	if !state.readerContext.advance(state.rawReaderDocument()) {
		return false, false
	}
	state.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
	return true, state.readerContext.animating(state.rawReaderDocument())
}

func (state *changeInspectionState) reconcileReaderPlace(oldRows []string, oldOffset, oldCursor int) {
	current := readerRowIdentities(state.readerRows())
	state.place.ReaderOffset = reconcileLogicalLine(oldRows, oldOffset, current)
	if state.place.ReaderOffset != oldOffset {
		state.place.ReaderColumn = 0
	}
	state.place.ReaderCursor = reconcileLogicalLine(oldRows, oldCursor, current)
	state.place.ClampReaderSource(len(current))
	state.saveReaderPlace()
}
