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

func (state *summaryState) poll() effect {
	state.generation++
	return effect{kind: effectLoadSummary, generation: state.generation, background: true}
}

func (state summaryState) land(msg summaryLoadedMsg) summaryState {
	if msg.generation != state.generation {
		return state
	}
	state.loading = false
	if !msg.background {
		state.err = msg.err
	}
	if msg.background && msg.err != nil {
		return state
	}
	if msg.err == nil {
		state.err = nil
		state.value = msg.summary
		state.loaded = true
	}
	return state
}
