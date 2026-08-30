package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
)

type refsState struct {
	place    navigation.State
	sources  []repository.RefSource
	commits  []repository.RefCommit
	selected repository.RefSourceID

	sourceGeneration    uint64
	previewGeneration   uint64
	loaded              bool
	entered             bool
	sourceLoading       bool
	previewLoading      bool
	sourceError         error
	previewError        error
	preferredOID        string
	previewSource       repository.RefSourceID
	preservePreview     bool
	restoredPreviewRows []string
}

func newRefsState() refsState {
	return refsState{place: navigation.State{Focus: navigation.FocusNavigator}}
}

func (state *refsState) enter(preferredOID string) effect {
	if !state.entered {
		state.entered = true
		state.preferredOID = preferredOID
	}
	if !state.loaded && !state.sourceLoading {
		return state.reload()
	}
	return effect{}
}

func (state *refsState) reload() effect {
	state.sourceGeneration++
	state.sourceLoading = true
	state.sourceError = nil
	return effect{kind: effectLoadRefSources, generation: state.sourceGeneration}
}

func (state *refsState) poll() effect {
	state.sourceGeneration++
	return effect{kind: effectLoadRefSources, generation: state.sourceGeneration, background: true}
}

func (state refsState) landSources(msg refSourcesLoadedMsg, visibleRows int) (refsState, effect) {
	if msg.generation != state.sourceGeneration {
		return state, effect{}
	}
	firstLoad := !state.loaded
	state.loaded = true
	state.sourceLoading = false
	if msg.err != nil {
		if msg.background {
			return state, effect{}
		}
		state.sourceError = msg.err
		return state, effect{}
	}
	state.sourceError = nil
	oldSelected := state.selected
	restoredIdentity, hadRestoredSelection := state.place.SelectedIdentity()
	state.sources = append([]repository.RefSource(nil), msg.sources...)
	identities := make([]string, len(state.sources))
	for index, source := range state.sources {
		identities[index] = source.ID.Key()
	}
	state.place.Reconcile(identities)

	if len(state.sources) == 0 {
		state.clearSourceSelection()
		return state, effect{}
	}
	target, restorePreview := state.reconciledSourceTarget(firstLoad, oldSelected, restoredIdentity, hadRestoredSelection)
	target = min(max(0, target), len(state.sources)-1)
	state.place.Selected = target
	state.selected = state.sources[target].ID
	state.place.EnsureSelectionVisible(visibleRows)
	if restorePreview {
		return state, state.requestRestoredPreview(state.sources[target])
	}
	state.restoredPreviewRows = nil
	preserve := !firstLoad && state.selected == oldSelected
	if msg.background && preserve {
		return state, state.requestPreviewQuiet(state.sources[target])
	}
	return state, state.requestPreview(state.sources[target], preserve)
}

func (state refsState) reconciledSourceTarget(firstLoad bool, oldSelected repository.RefSourceID, restoredIdentity string, hadRestoredSelection bool) (int, bool) {
	if firstLoad && hadRestoredSelection {
		target := state.place.Selected
		restorePreview := target >= 0 && target < len(state.sources) && state.sources[target].ID.Key() == restoredIdentity
		return target, restorePreview
	}
	if firstLoad {
		return initialRefSourceIndex(state.sources, state.preferredOID), false
	}
	if index, ok := refSourceIndex(state.sources, oldSelected); ok {
		return index, false
	}
	target, _ := refSourceIndex(state.sources, repository.AllRefsSource().ID)
	return target, false
}

func (state *refsState) clearSourceSelection() {
	state.selected = repository.RefSourceID{}
	state.commits = nil
	state.previewSource = repository.RefSourceID{}
	state.previewLoading = false
	state.previewError = nil
	state.place.ReaderOffset = 0
}

func (state refsState) landPreview(msg refCommitsLoadedMsg, visibleRows int) refsState {
	if msg.generation != state.previewGeneration || msg.sourceID != state.previewSource || msg.sourceID != state.selected {
		return state
	}
	state.previewLoading = false
	if msg.err != nil {
		if msg.background {
			return state
		}
		state.previewError = msg.err
		return state
	}
	state.previewError = nil
	oldCommitIDs := refCommitIdentities(state.commits)
	oldOffset := state.place.ReaderOffset
	state.commits = append([]repository.RefCommit(nil), msg.commits...)
	if len(state.restoredPreviewRows) != 0 {
		state.place.ReaderOffset = reconcilePreviewIdentities(state.restoredPreviewRows, oldOffset, state.commits)
		state.restoredPreviewRows = nil
	} else if state.preservePreview {
		state.place.ReaderOffset = reconcilePreviewIdentities(oldCommitIDs, oldOffset, state.commits)
	} else {
		state.place.ReaderOffset = 0
	}
	state.place.ClampReader(len(state.commits), visibleRows)
	return state
}

func (state *refsState) selectDelta(delta, visibleRows int) effect {
	if !state.place.SelectDelta(delta, visibleRows) {
		return effect{}
	}
	return state.selectCurrent()
}

func (state *refsState) selectIndex(index, visibleRows int) effect {
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	return state.selectCurrent()
}

func (state *refsState) selectCurrent() effect {
	if state.place.Selected < 0 || state.place.Selected >= len(state.sources) {
		return effect{}
	}
	source := state.sources[state.place.Selected]
	state.selected = source.ID
	return state.requestPreview(source, false)
}

func (state *refsState) requestPreview(source repository.RefSource, preserve bool) effect {
	state.previewGeneration++
	state.previewSource = source.ID
	state.previewLoading = true
	state.previewError = nil
	state.preservePreview = preserve
	if !preserve {
		state.commits = nil
		state.place.ReaderOffset = 0
	}
	return effect{
		kind:       effectLoadRefCommits,
		generation: state.previewGeneration,
		refSource:  source,
	}
}

func (state *refsState) requestPreviewQuiet(source repository.RefSource) effect {
	state.previewGeneration++
	state.previewSource = source.ID
	state.preservePreview = true
	return effect{
		kind: effectLoadRefCommits, generation: state.previewGeneration,
		refSource: source, background: true,
	}
}

func (state *refsState) requestRestoredPreview(source repository.RefSource) effect {
	state.previewGeneration++
	state.previewSource = source.ID
	state.previewLoading = true
	state.previewError = nil
	state.preservePreview = false
	return effect{
		kind: effectLoadRefCommits, generation: state.previewGeneration,
		refSource: source,
	}
}

func (state refsState) selectedSource() (repository.RefSource, bool) {
	if index, ok := refSourceIndex(state.sources, state.selected); ok {
		return state.sources[index], true
	}
	return repository.RefSource{}, false
}

func initialRefSourceIndex(sources []repository.RefSource, preferredOID string) int {
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
		if source.ID.Kind == repository.RefSourceCurrentWorktree {
			return index
		}
	}
	if index, ok := refSourceIndex(sources, repository.AllRefsSource().ID); ok {
		return index
	}
	return 0
}

func refSourceIndex(sources []repository.RefSource, id repository.RefSourceID) (int, bool) {
	for index, source := range sources {
		if source.ID == id {
			return index, true
		}
	}
	return 0, false
}

func reconcilePreviewIdentities(old []string, oldOffset int, current []repository.RefCommit) int {
	currentIDs := refCommitIdentities(current)
	place := navigation.State{Items: append([]string(nil), old...), Selected: oldOffset, Top: oldOffset}
	place.Reconcile(currentIDs)
	return place.Selected
}

func refCommitIdentities(commits []repository.RefCommit) []string {
	identities := make([]string, len(commits))
	for index, commit := range commits {
		identities[index] = commit.OID
	}
	return identities
}
