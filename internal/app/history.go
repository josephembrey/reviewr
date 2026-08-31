package app

import (
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// historyState owns grouped sources, the filtered timeline, and an optional
// immutable commit inspection. Overview place remains untouched while the
// inspection is active.
type historyState struct {
	sourcePlace    navigation.State
	timelinePlace  navigation.State
	focus          workspace.GitFocus
	sources        []repository.RefSource
	sourceRows     []historySourceRow
	selectedSource string
	sourceFolds    map[historySourceGroup]bool
	preferredOID   string
	commits        []repository.Commit
	rows           []commitrow.Row
	traversal      workspace.GitTraversal

	inspecting    bool
	inspectionOID string
	inspection    changeInspectionState

	sourceGeneration uint64
	listGeneration   uint64
	sourcesLoaded    bool
	loaded           bool
	sourceLoading    bool
	listLoading      bool
	sourceError      error
	listError        error
}

func newHistoryState() historyState {
	return historyState{
		sourcePlace:      navigation.State{Focus: navigation.FocusNavigator},
		timelinePlace:    navigation.State{Focus: navigation.FocusNavigator},
		focus:            workspace.GitSource,
		sourceFolds:      make(map[historySourceGroup]bool),
		inspection:       newChangeInspectionState(),
		sourceGeneration: 1,
		sourceLoading:    true,
	}
}

func (state *historyState) initialSourcesEffect() effect {
	return effect{kind: effectLoadHistorySources, generation: state.sourceGeneration}
}

func (state *historyState) reload() effect {
	state.sourceGeneration++
	state.sourceLoading = true
	state.sourceError = nil
	return effect{kind: effectLoadHistorySources, generation: state.sourceGeneration}
}

func (state *historyState) poll() effect {
	state.sourceGeneration++
	return effect{kind: effectLoadHistorySources, generation: state.sourceGeneration, background: true}
}

func (state historyState) landSources(msg historySourcesLoadedMsg, visibleRows int) (historyState, effect) {
	if msg.generation != state.sourceGeneration {
		return state, effect{}
	}
	firstLoad := !state.sourcesLoaded
	_, hadRestoredCursor := state.sourcePlace.SelectedIdentity()
	state.sourcesLoaded = true
	state.sourceLoading = false
	if msg.err != nil {
		if !msg.background {
			state.sourceError = msg.err
		}
		return state, effect{}
	}
	state.sourceError = nil
	state.sources = append([]repository.RefSource(nil), msg.sources...)
	state.ensureAllRefsSource()
	if firstLoad {
		state.chooseInitialSource()
	} else if !state.hasSource(state.selectedSource) {
		state.selectedSource = repository.AllRefsSource().ID.Key()
	}
	oldRows := append([]string(nil), state.sourcePlace.Items...)
	state.rebuildSourceRows()
	state.sourcePlace.Items = oldRows
	state.sourcePlace.Reconcile(historySourceRowIdentities(state.sourceRows))
	if firstLoad && (!hadRestoredCursor || state.sourceCursorDoesNotNameRestoredRow()) {
		state.selectSourceCursor(state.selectedSource, visibleRows)
	}
	state.sourcePlace.EnsureSelectionVisible(visibleRows)
	return state, state.requestCommits(state.traversal, msg.background)
}

func (state *historyState) ensureAllRefsSource() {
	all := repository.AllRefsSource()
	if !state.hasSource(all.ID.Key()) {
		state.sources = append([]repository.RefSource{all}, state.sources...)
	}
}

func (state *historyState) chooseInitialSource() {
	if state.selectedSource != "" && state.hasSource(state.selectedSource) {
		return
	}
	index := initialHistorySourceIndex(state.sources, state.preferredOID)
	if index >= 0 && index < len(state.sources) {
		state.selectedSource = state.sources[index].ID.Key()
		return
	}
	state.selectedSource = repository.AllRefsSource().ID.Key()
}

func (state historyState) sourceCursorDoesNotNameRestoredRow() bool {
	identity, ok := state.sourcePlace.SelectedIdentity()
	return !ok || !historySourceRowExists(state.sourceRows, identity)
}

func (state *historyState) selectSourceCursor(identity string, visibleRows int) {
	for index, row := range state.sourceRows {
		if row.identity == identity {
			state.sourcePlace.SelectIndex(index, visibleRows)
			return
		}
	}
}

func (state historyState) hasSource(identity string) bool {
	for _, source := range state.sources {
		if source.ID.Key() == identity {
			return true
		}
	}
	return false
}

func initialHistorySourceIndex(sources []repository.RefSource, preferredOID string) int {
	match := -1
	if preferredOID != "" {
		for index, source := range sources {
			if source.ID.Kind == repository.RefSourceAll || source.OID != preferredOID {
				continue
			}
			if match != -1 {
				match = -1
				break
			}
			match = index
		}
	}
	if match >= 0 {
		return match
	}
	for index, source := range sources {
		if source.ID.Kind == repository.RefSourceAll {
			return index
		}
	}
	return 0
}

func (state *historyState) requestCommits(traversal workspace.GitTraversal, background bool) effect {
	state.traversal = traversal
	state.listGeneration++
	state.listLoading = !background
	if !background {
		state.listError = nil
	}
	return effect{
		kind: effectLoadCommits, generation: state.listGeneration,
		query: state.commitQuery(traversal), background: background,
	}
}

func (state historyState) commitQuery(traversal workspace.GitTraversal) repository.CommitQuery {
	source, _ := state.selectedSourceValue()
	query := commitQuery(traversal, source.OID)
	if traversal == workspace.GitFirstParent && source.ID.Kind == repository.RefSourceAll {
		query.StartOID, _ = state.timelinePlace.SelectedIdentity()
	}
	return query
}

func (state historyState) landCommits(msg commitsLoadedMsg, visibleRows int) historyState {
	if msg.generation != state.listGeneration {
		return state
	}
	state.loaded = true
	state.listLoading = false
	state.traversal = traversalForQuery(msg.query)
	if msg.err != nil {
		if !msg.background {
			state.listError = msg.err
		}
		return state
	}
	state.listError = nil
	_, hadSelection := state.timelinePlace.SelectedIdentity()
	state.commits = append([]repository.Commit(nil), msg.commits...)
	state.rows = buildCommitRows(state.commits, state.traversal)
	state.timelinePlace.Reconcile(commitIdentities(state.commits))
	if !hadSelection {
		state.selectInitialCommit(visibleRows)
	}
	state.timelinePlace.EnsureSelectionVisible(visibleRows)
	return state
}

func commitIdentities(commits []repository.Commit) []string {
	identities := make([]string, len(commits))
	for index, commit := range commits {
		identities[index] = commit.OID
	}
	return identities
}

func (state *historyState) selectInitialCommit(visibleRows int) {
	for index, commit := range state.commits {
		if commit.Head {
			state.timelinePlace.SelectIndex(index, visibleRows)
			return
		}
	}
	state.timelinePlace.SelectIndex(0, visibleRows)
}

func (state *historyState) moveSource(delta, visibleRows int) effect {
	if !state.sourcePlace.SelectDelta(delta, visibleRows) {
		return effect{}
	}
	return state.activateSourceCursor()
}

func (state *historyState) selectSourceIndex(index, visibleRows int) effect {
	if !state.sourcePlace.SelectIndex(index, visibleRows) {
		return effect{}
	}
	return state.activateSourceCursor()
}

func (state *historyState) activateSourceCursor() effect {
	row, ok := state.selectedSourceRow()
	if !ok || row.source == nil || row.source.ID.Key() == state.selectedSource {
		return effect{}
	}
	state.selectedSource = row.source.ID.Key()
	return state.requestCommits(state.traversal, false)
}

func (state *historyState) moveTimeline(delta, visibleRows int) {
	state.timelinePlace.SelectDelta(delta, visibleRows)
}

func (state *historyState) selectTimelineIndex(index, visibleRows int) {
	state.timelinePlace.SelectIndex(index, visibleRows)
}

func (state *historyState) setSourceGroupExpanded(expanded bool, visibleRows int) {
	row, ok := state.selectedSourceRow()
	if !ok || state.sourceFolds[row.group] == !expanded {
		return
	}
	state.sourceFolds[row.group] = !expanded
	old := append([]string(nil), state.sourcePlace.Items...)
	state.rebuildSourceRows()
	state.sourcePlace.Items = old
	state.sourcePlace.Reconcile(historySourceRowIdentities(state.sourceRows))
	state.sourcePlace.EnsureSelectionVisible(visibleRows)
}

func (state historyState) selectedSourceRow() (historySourceRow, bool) {
	identity, ok := state.sourcePlace.SelectedIdentity()
	if !ok {
		return historySourceRow{}, false
	}
	for _, row := range state.sourceRows {
		if row.identity == identity {
			return row, true
		}
	}
	return historySourceRow{}, false
}

func (state historyState) selectedSourceValue() (repository.RefSource, bool) {
	for _, source := range state.sources {
		if source.ID.Key() == state.selectedSource {
			return source, true
		}
	}
	return repository.RefSource{}, false
}

func (state historyState) selectedCommit() (repository.Commit, bool) {
	oid, ok := state.timelinePlace.SelectedIdentity()
	if !ok {
		return repository.Commit{}, false
	}
	for _, commit := range state.commits {
		if commit.OID == oid {
			return commit, true
		}
	}
	return repository.Commit{}, false
}

func (state *historyState) enterInspection() effect {
	commit, ok := state.selectedCommit()
	if !ok {
		return effect{}
	}
	state.inspecting = true
	state.inspectionOID = commit.OID
	state.focus = workspace.GitFiles
	generation := state.inspection.beginFiles(commit.OID, false)
	return effect{kind: effectLoadCommitFiles, generation: generation, identity: commit.OID}
}

func (state *historyState) leaveInspection() {
	state.inspection.saveReaderPlace()
	state.inspecting = false
	state.focus = workspace.GitTimeline
}

func (state historyState) inspectionCommit() (repository.Commit, bool) {
	for _, commit := range state.commits {
		if commit.OID == state.inspectionOID {
			return commit, true
		}
	}
	return repository.Commit{OID: state.inspectionOID, ShortOID: abbreviateOID(state.inspectionOID)}, state.inspectionOID != ""
}

func (state *historyState) landInspectionFiles(msg commitFilesLoadedMsg) effect {
	accepted, loadReader := state.inspection.landFiles(msg.generation, msg.oid, msg.files, msg.err, msg.background)
	if !accepted || !loadReader {
		return effect{}
	}
	generation, file, ok := state.inspection.beginReader(msg.background)
	if !ok {
		return effect{}
	}
	return effect{
		kind: effectLoadCommitFile, generation: generation,
		identity: msg.oid, changedFile: file, background: msg.background,
	}
}

func (state *historyState) requestSelectedInspectionFile() effect {
	generation, file, ok := state.inspection.beginReader(false)
	if !ok {
		return effect{}
	}
	return effect{kind: effectLoadCommitFile, generation: generation, identity: state.inspectionOID, changedFile: file}
}
