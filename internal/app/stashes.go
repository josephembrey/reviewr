package app

import (
	"fmt"
	"time"

	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type stashReaderPlace struct {
	fileIdentity string
	readerOffset int
}

// stashState owns Stashes independently from Log and Files. Selectors are
// presentation; all reconciliation and reader place use immutable OIDs.
type stashState struct {
	place navigation.State

	stashes            []repository.Stash
	files              []repository.ChangedFile
	fileSelected       int
	filesOID           string
	reader             repository.ChangeDocument
	readerPresentation *ui.ReaderDocument
	readerOID          string
	readerFileID       string
	readerPlaces       map[string]stashReaderPlace

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

func (state stashState) landFiles(msg stashFilesLoadedMsg, visibleRows int) (stashState, effect) {
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
		return state, effect{}
	}
	wanted := oldIdentity
	if saved := state.readerPlaces[selectedOID]; saved.fileIdentity != "" {
		wanted = saved.fileIdentity
	}
	oldIdentities := changedFileIdentities(oldFiles)
	newIdentities := changedFileIdentities(state.files)
	identity, ok := navigation.ReconcileIdentity(oldIdentities, wanted, newIdentities)
	if !ok {
		identity = newIdentities[0]
	}
	state.fileSelected = indexIdentity(newIdentities, identity)
	if saved := state.readerPlaces[selectedOID]; saved.fileIdentity == identity {
		state.place.ReaderOffset = saved.readerOffset
	} else if oldIdentity != identity {
		state.place.ReaderOffset = 0
	}
	if msg.background && identity == oldIdentity {
		return state, state.requestSelectedFileQuiet()
	}
	return state, state.requestSelectedFile(visibleRows)
}

func (state stashState) landReader(msg stashFileLoadedMsg, visibleRows int) stashState {
	selectedOID, selected := state.place.SelectedIdentity()
	if msg.generation != state.readerGeneration || !selected || msg.oid != selectedOID ||
		msg.oid != state.readerOID || msg.fileIdentity != state.readerFileID {
		return state
	}
	oldLines := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	state.reader = msg.document
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.place.ClampReader(len(state.readerRows()), visibleRows)
	state.saveReaderPlace()
	return state
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
	if oid, ok := state.place.SelectedIdentity(); ok {
		if saved := state.readerPlaces[oid]; saved.fileIdentity == state.selectedFileIdentity() {
			state.place.ReaderOffset = saved.readerOffset
		}
	}
	return state.requestSelectedFile(visibleRows)
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

func (state *stashState) requestSelectedFile(visibleRows int) effect {
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
	}
	state.readerOID = stash.OID
	state.readerFileID = fileIdentity
	state.place.ClampReader(len(state.readerRows()), visibleRows)
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
	state.readerPlaces[oid] = stashReaderPlace{fileIdentity: identity, readerOffset: state.place.ReaderOffset}
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
}

func (state *stashState) clearReader() {
	state.readerGeneration++
	state.reader = repository.ChangeDocument{}
	state.readerPresentation = nil
	state.readerOID = ""
	state.readerFileID = ""
	state.readerLoading = false
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

func (state stashState) readerDocument() ui.ReaderDocument {
	if state.readerPresentation != nil {
		return *state.readerPresentation
	}
	return state.deriveReaderDocument()
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

func (state stashState) viewModel(geometry ui.Geometry, now time.Time) ui.Model {
	rows := make([]ui.NavigatorRow, len(state.stashes))
	for index, stash := range state.stashes {
		prose := stash.Message
		if stash.Branch != "" {
			prose = stash.Branch + " · " + prose
		}
		if prose == "" {
			prose = "(no message)"
		}
		rows[index] = ui.NavigatorRow{
			Identity: stash.OID,
			Prefix:   []ui.Segment{{Text: stash.Selector + " ", Tone: ui.ToneAccent}},
			Label:    prose,
			Suffix: []ui.Segment{
				{Text: fmt.Sprintf("  %df ", stash.Files), Tone: ui.ToneQuiet},
				{Text: fmt.Sprintf("+%d ", stash.Additions), Tone: ui.ToneAdded},
				{Text: fmt.Sprintf("-%d ", stash.Deletions), Tone: ui.ToneRemoved},
				{Text: ageLabel(now, stash.Timestamp), Tone: ui.ToneQuiet},
			},
		}
	}

	emptyNavigator := ui.Line{Text: "No stashes yet.", Tone: ui.ToneQuiet}
	if state.listLoading && len(rows) == 0 {
		emptyNavigator.Text = "Loading stashes…"
	} else if state.listError != nil && len(rows) == 0 {
		emptyNavigator = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}
	title := fmt.Sprintf("stashes · %d", len(rows))
	if state.listError != nil && len(rows) > 0 {
		title += " · refresh failed"
	}

	readerTitle := "No stash selected"
	if stash, ok := state.selectedStash(); ok {
		readerTitle = stash.Selector
		if len(state.files) > 0 && state.fileSelected >= 0 && state.fileSelected < len(state.files) {
			change := state.files[state.fileSelected]
			path := change.Path
			if change.PreviousPath != "" {
				path = change.PreviousPath + " → " + change.Path
			}
			readerTitle = fmt.Sprintf("%s · %d/%d · %s", stash.Selector, state.fileSelected+1, len(state.files), path)
		}
	}
	if state.readerLoading {
		readerTitle += " · loading…"
	}

	readerEmpty := ui.Line{Text: "Select a stash to inspect its files.", Tone: ui.ToneQuiet}
	switch {
	case len(state.stashes) == 0 && state.listLoading:
		readerEmpty.Text = "Loading stashes…"
	case len(state.stashes) == 0 && state.listError != nil:
		readerEmpty = ui.Line{Text: "Stashes are unavailable: " + state.listError.Error(), Tone: ui.ToneError}
	case len(state.stashes) == 0:
		readerEmpty.Text = "No stashes yet."
	case state.filesLoading:
		readerEmpty.Text = "Loading files stored in this stash…"
	case state.filesError != nil:
		readerEmpty = ui.Line{Text: "Stash is no longer available: " + state.filesError.Error(), Tone: ui.ToneError}
	case len(state.files) == 0:
		readerEmpty.Text = "No files stored in this stash."
	case state.readerLoading:
		readerEmpty.Text = "Loading stash diff…"
	}

	return ui.Model{
		Geometry: geometry, NavigatorTitle: title, NavigatorRows: rows,
		NavigatorEmpty: emptyNavigator, Selected: state.place.Selected, Top: state.place.Top,
		Focus: state.place.Focus, ReaderTitle: readerTitle, ReaderDocument: state.readerDocument(),
		ReaderEmpty: readerEmpty, ReaderOffset: state.place.ReaderOffset,
	}
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

func ageLabel(now time.Time, timestamp int64) string {
	seconds := max(int64(0), now.Unix()-timestamp)
	switch {
	case seconds < 60:
		return "now"
	case seconds < 60*60:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 24*60*60:
		return fmt.Sprintf("%dh", seconds/(60*60))
	case seconds < 7*24*60*60:
		return fmt.Sprintf("%dd", seconds/(24*60*60))
	case seconds < 30*24*60*60:
		return fmt.Sprintf("%dw", seconds/(7*24*60*60))
	case seconds < 365*24*60*60:
		return fmt.Sprintf("%dmo", seconds/(30*24*60*60))
	default:
		return fmt.Sprintf("%dy", seconds/(365*24*60*60))
	}
}
