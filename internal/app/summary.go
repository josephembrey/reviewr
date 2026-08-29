package app

import "github.com/josephembrey/reviewr/internal/repository"

type summaryState struct {
	value      repository.ChangeSummary
	generation uint64
	loaded     bool
	loading    bool
	err        error
}

func newSummaryState() summaryState {
	return summaryState{generation: 1, loading: true}
}

func (state *summaryState) reload() effect {
	state.generation++
	state.loading = true
	state.err = nil
	return effect{kind: effectLoadSummary, generation: state.generation}
}

func (state summaryState) land(msg summaryLoadedMsg) summaryState {
	if msg.generation != state.generation {
		return state
	}
	state.loading = false
	state.err = msg.err
	if msg.err == nil {
		state.value = msg.summary
		state.loaded = true
	}
	return state
}
