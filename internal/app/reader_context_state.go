package app

import (
	"slices"

	"github.com/josephembrey/reviewr/internal/ui"
)

type readerContextFold struct {
	expanded bool
	progress int
}

// readerContextState owns authored context density independently for every
// stable gap. defaultExpanded records the last bulk choice; folds contains
// only local overrides or gaps still moving toward that default.
type readerContextState struct {
	startExpanded   bool
	defaultExpanded bool
	folds           map[string]readerContextFold
	order           []string
	generation      uint64
	revision        uint64
}

// setStartExpanded changes the policy for future reader identities. It does
// not touch the current document: authored fold state is place state and must
// move only under a direct fold action.
func (state *readerContextState) setStartExpanded(expanded bool) {
	state.startExpanded = expanded
}

func (state readerContextState) document(source ui.ReaderDocument) ui.ReaderDocument {
	var progresses map[string]int
	if len(state.folds) != 0 {
		progresses = make(map[string]int, len(state.folds))
		for identity, fold := range state.folds {
			progresses[identity] = fold.progress
		}
	}
	return source.WithContextFoldProgresses(
		progresses,
		readerContextTarget(state.defaultExpanded),
		readerContextAnimationSteps,
	)
}

func (state readerContextState) allExpanded(source ui.ReaderDocument) bool {
	identities := source.ContextFoldIdentities()
	if len(identities) == 0 {
		return false
	}
	for _, identity := range identities {
		if !state.target(identity) {
			return false
		}
	}
	return true
}

func (state *readerContextState) setAll(source ui.ReaderDocument, expanded bool) (bool, bool) {
	identities := source.ContextFoldIdentities()
	if len(identities) == 0 {
		return false, false
	}
	changed := false
	progresses := make(map[string]int, len(identities))
	for _, identity := range identities {
		progresses[identity] = state.progress(identity)
		changed = changed || state.target(identity) != expanded
	}
	if !changed {
		return false, false
	}
	state.defaultExpanded = expanded
	state.order = append(state.order[:0], identities...)
	state.folds = make(map[string]readerContextFold, len(identities))
	for _, identity := range identities {
		state.folds[identity] = readerContextFold{expanded: expanded, progress: progresses[identity]}
	}
	state.generation++
	state.advance(source)
	return true, state.animating(source)
}

func (state *readerContextState) setFold(source ui.ReaderDocument, identity string, expanded bool) (bool, bool) {
	identities := source.ContextFoldIdentities()
	if identity == "" || !slices.Contains(identities, identity) || state.target(identity) == expanded {
		return false, false
	}
	state.order = append(state.order[:0], identities...)
	if state.folds == nil {
		state.folds = make(map[string]readerContextFold)
	}
	state.folds[identity] = readerContextFold{expanded: expanded, progress: state.progress(identity)}
	state.generation++
	state.advance(source)
	return true, state.animating(source)
}

func (state *readerContextState) toggleFold(source ui.ReaderDocument, identity string) (bool, bool) {
	return state.setFold(source, identity, !state.target(identity))
}

func (state *readerContextState) advance(source ui.ReaderDocument) bool {
	identities := source.ContextFoldIdentities()
	known := make(map[string]struct{}, len(identities))
	changed := false
	for _, identity := range identities {
		known[identity] = struct{}{}
		fold, overridden := state.folds[identity]
		if !overridden {
			continue
		}
		next := stepReaderContext(fold.progress, fold.expanded)
		if next != fold.progress {
			changed = true
			fold.progress = next
		}
		if fold.expanded == state.defaultExpanded && fold.progress == readerContextTarget(fold.expanded) {
			delete(state.folds, identity)
		} else {
			state.folds[identity] = fold
		}
	}
	for identity := range state.folds {
		if _, ok := known[identity]; !ok {
			delete(state.folds, identity)
		}
	}
	if changed {
		state.revision++
	}
	return changed
}

func (state readerContextState) animating(source ui.ReaderDocument) bool {
	for _, identity := range source.ContextFoldIdentities() {
		if readerContextAnimating(state.progress(identity), state.target(identity)) {
			return true
		}
	}
	return false
}

func (state readerContextState) target(identity string) bool {
	if fold, ok := state.folds[identity]; ok {
		return fold.expanded
	}
	return state.defaultExpanded
}

func (state readerContextState) progress(identity string) int {
	if fold, ok := state.folds[identity]; ok {
		return fold.progress
	}
	return readerContextTarget(state.defaultExpanded)
}

func (state *readerContextState) reset() {
	state.defaultExpanded = state.startExpanded
	state.folds = nil
	state.order = nil
	state.generation++
	state.revision++
}

func (state readerContextState) overrides() map[string]bool {
	result := make(map[string]bool, len(state.folds))
	for identity, fold := range state.folds {
		if fold.expanded != state.defaultExpanded {
			result[identity] = fold.expanded
		}
	}
	return result
}

func (state *readerContextState) restore(defaultExpanded bool, overrides map[string]bool) {
	state.defaultExpanded = defaultExpanded
	state.order = nil
	state.folds = make(map[string]readerContextFold, len(overrides))
	for identity, expanded := range overrides {
		state.folds[identity] = readerContextFold{
			expanded: expanded,
			progress: readerContextTarget(expanded),
		}
	}
	state.generation++
	state.revision++
}

// reconcile preserves exact gap identities across world refreshes, then moves
// an authored override to the nearest surviving gap when its old gap vanished.
// This is the fold-specific form of Continuity: world events reconcile place;
// only user input chooses expanded versus collapsed.
func (state *readerContextState) reconcile(source ui.ReaderDocument) {
	current := source.ContextFoldIdentities()
	if len(state.folds) == 0 {
		state.order = append(state.order[:0], current...)
		return
	}
	currentSet := make(map[string]struct{}, len(current))
	claimed := make(map[string]struct{}, len(state.folds))
	next := make(map[string]readerContextFold, len(state.folds))
	for _, identity := range current {
		currentSet[identity] = struct{}{}
		if fold, ok := state.folds[identity]; ok {
			next[identity] = fold
			claimed[identity] = struct{}{}
		}
	}
	for oldIndex, identity := range state.order {
		fold, ok := state.folds[identity]
		if !ok {
			continue
		}
		if _, survives := currentSet[identity]; survives {
			continue
		}
		if replacement, ok := nearestUnclaimed(current, claimed, oldIndex); ok {
			next[replacement] = fold
			claimed[replacement] = struct{}{}
		}
	}
	changed := !sameReaderContextFolds(state.folds, next)
	state.folds = next
	state.order = append(state.order[:0], current...)
	if changed {
		state.revision++
	}
}

func nearestUnclaimed(identities []string, claimed map[string]struct{}, target int) (string, bool) {
	if len(identities) == 0 {
		return "", false
	}
	target = max(0, min(target, len(identities)-1))
	for distance := 0; distance < len(identities); distance++ {
		indexes := []int{target - distance}
		if distance != 0 {
			indexes = append(indexes, target+distance)
		}
		for _, index := range indexes {
			if index < 0 || index >= len(identities) {
				continue
			}
			identity := identities[index]
			if _, used := claimed[identity]; !used {
				return identity, true
			}
		}
	}
	return "", false
}

func sameReaderContextFolds(left, right map[string]readerContextFold) bool {
	if len(left) != len(right) {
		return false
	}
	for identity, fold := range left {
		if right[identity] != fold {
			return false
		}
	}
	return true
}
