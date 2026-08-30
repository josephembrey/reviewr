package app

import (
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (state filesState) landFile(msg fileLoadedMsg) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.FileReader {
		return state
	}
	oldLines := state.previousReaderRows()
	oldOffset := state.place.ReaderOffset
	state.reader = msg.file
	state.diff = repository.Diff{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.readerContext.reconcile(presentation)
	state.readerLoadedKey = state.readerRequestKey
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	if msg.entry.State != repository.FileIgnored && msg.file.Kind != repository.FileUnreadable && msg.file.Kind != 0 {
		state.rememberReader(readerCacheEntry{
			key:  readerCacheKey{kind: effectLoadFile, entry: msg.entry},
			file: msg.file, presentation: msg.presentation,
		})
	}
	return state
}

func (state filesState) landDiff(msg diffLoadedMsg) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.DiffReader {
		return state
	}
	oldLines := state.previousReaderRows()
	oldOffset := state.place.ReaderOffset
	state.diff = msg.diff
	state.reader = repository.File{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.readerContext.reconcile(presentation)
	state.readerLoadedKey = state.readerRequestKey
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	if msg.diff.Kind == repository.DiffReady || msg.diff.Kind == repository.DiffTooLarge {
		state.rememberReader(readerCacheEntry{
			key:  readerCacheKey{kind: effectLoadDiff, entry: msg.entry},
			diff: msg.diff, presentation: msg.presentation,
		})
	}
	return state
}

func (state *filesState) requestReader(entry repository.Entry, mode workspace.ReaderMode) effect {
	return state.requestReaderWithLoading(entry, mode, true)
}

func (state *filesState) requestReaderQuiet(entry repository.Entry, mode workspace.ReaderMode) effect {
	return state.requestReaderWithLoading(entry, mode, false)
}

func (state *filesState) requestReaderWithLoading(entry repository.Entry, mode workspace.ReaderMode, loading bool) effect {
	state.contentGeneration++
	if state.readerEntry.Path != entry.Path || state.readerMode != mode {
		state.reader = repository.File{}
		state.diff = repository.Diff{}
		state.reviewDocument = review.Document{}
		state.reviewFile = review.Content{}
		state.reviewFileDiff = review.Document{}
		state.displayedComparison = nil
		state.displayedBounds = nil
		state.readerPresentation = nil
		state.resetReaderContext()
		state.restoredReaderRows = nil
	}
	state.readerEntry = entry
	state.readerMode = mode
	state.readerLoading = loading
	state.requestedComparison = nil
	state.requestedBounds = nil
	pending := effect{
		kind: effectLoadFile, generation: state.contentGeneration, entry: entry,
		fileComparison: state.snapshot.Comparison(),
	}
	var reviewComparison review.FileComparison
	var bounds review.Bounds
	if mode == workspace.FileReader {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			comparisonCopy := comparison
			boundsCopy := review.Bounds{Old: comparison.Old, New: comparison.New}
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			reviewComparison = comparison
			bounds = boundsCopy
			pending = effect{kind: effectLoadReviewFile, generation: state.contentGeneration, entry: entry, comparison: comparison}
		}
	} else {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			assessment := state.reviewAssessment(entry.Path, comparison)
			bounds = review.Bounds{Old: comparison.Old, New: comparison.New}
			var retained *string
			if assessment.State == review.Updated && !state.reviewFull[entry.Path] && assessment.Frontier != nil && assessment.Retained != nil {
				bounds.Old = *assessment.Frontier
				retained = assessment.Retained
			}
			comparisonCopy, boundsCopy := comparison, bounds
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			reviewComparison = comparison
			pending = effect{kind: effectLoadReviewDocument, generation: state.contentGeneration, entry: entry, comparison: comparison, bounds: bounds, retained: retained}
		} else {
			pending.kind = effectLoadDiff
		}
	}
	key := state.readerKey(pending.kind, entry, reviewComparison, bounds)
	state.readerRequestKey = key
	if state.restoreReader(key) {
		return effect{}
	}
	return pending
}

func (state *filesState) requestMode(mode workspace.ReaderMode) effect {
	if state.readerEntry.Path == "" || state.readerMode == mode {
		return effect{}
	}
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
	return state.requestReader(state.readerEntry, mode)
}

func (state *filesState) clearReader() {
	state.contentGeneration++
	state.readerEntry = repository.Entry{}
	state.reader = repository.File{}
	state.diff = repository.Diff{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerPresentation = nil
	state.resetReaderContext()
	state.restoredReaderRows = nil
	state.requestedComparison = nil
	state.requestedBounds = nil
	state.readerRequestKey = readerCacheKey{}
	state.readerLoadedKey = readerCacheKey{}
	state.readerLoading = false
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
}

func (state filesState) readerDocument() ui.ReaderDocument {
	return state.readerContext.document(state.rawReaderDocument())
}

func (state filesState) rawReaderDocument() ui.ReaderDocument {
	if state.readerPresentation != nil {
		return *state.readerPresentation
	}
	if state.readerEntry.Path == "" || state.readerLoading {
		return ui.ReaderDocument{}
	}
	return state.deriveReaderDocument()
}

func (state *filesState) setReaderContextExpanded(expanded bool) (bool, bool) {
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	changed, animating := state.readerContext.setAll(state.rawReaderDocument(), expanded)
	if changed {
		state.reconcileReaderContextPlace(oldRows, oldOffset)
	}
	return changed, animating
}

func (state *filesState) toggleReaderContextFold(identity string) (bool, bool) {
	return state.changeReaderContextFold(identity, nil)
}

func (state *filesState) setReaderContextFold(identity string, expanded bool) (bool, bool) {
	return state.changeReaderContextFold(identity, &expanded)
}

func (state *filesState) changeReaderContextFold(identity string, expanded *bool) (bool, bool) {
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	var changed, animating bool
	if expanded == nil {
		changed, animating = state.readerContext.toggleFold(state.rawReaderDocument(), identity)
	} else {
		changed, animating = state.readerContext.setFold(state.rawReaderDocument(), identity, *expanded)
	}
	if changed {
		state.reconcileReaderContextPlace(oldRows, oldOffset)
	}
	return changed, animating
}

func (state *filesState) advanceReaderContext(generation uint64) (bool, bool) {
	if generation != state.readerContext.generation || !state.readerContext.animating(state.rawReaderDocument()) {
		return false, false
	}
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	if !state.readerContext.advance(state.rawReaderDocument()) {
		return false, false
	}
	state.reconcileReaderContextPlace(oldRows, oldOffset)
	return true, state.readerContext.animating(state.rawReaderDocument())
}

func (state *filesState) reconcileReaderContextPlace(oldRows []string, oldOffset int) {
	state.place.ReaderOffset = reconcileLogicalLine(oldRows, oldOffset, readerRowIdentities(state.readerRows()))
	if state.place.ReaderOffset != oldOffset {
		state.place.ReaderColumn = 0
	}
	state.place.ClampReaderSource(len(state.readerRows()))
}

func (state *filesState) resetReaderContext() {
	state.readerContext.reset()
}

func (state filesState) readerRows() []ui.ReaderRow {
	return state.readerDocument().Rows
}

func (state filesState) previousReaderRows() []string {
	current := readerRowIdentities(state.readerRows())
	if len(current) == 0 && len(state.restoredReaderRows) != 0 {
		return append([]string(nil), state.restoredReaderRows...)
	}
	return current
}

func (state filesState) deriveReaderDocument() ui.ReaderDocument {
	if state.readerMode == workspace.DiffReader {
		if state.displayedBounds != nil {
			return reviewReaderDocument(state.readerEntry.Path, state.reviewDocument)
		}
		return (readerDocument{Diff: state.diff, Mode: state.readerMode}).build()
	}
	if state.displayedComparison != nil {
		if state.reviewFile.Endpoint != state.displayedComparison.New {
			return ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: noticeRows("File changed; refresh before marking reviewed.", ui.ToneError)}
		}
		return annotatedReviewFileReaderDocument(
			state.reviewFile,
			state.readerEntry,
			*state.displayedComparison,
			state.reviewFileDiff,
		)
	}
	return (readerDocument{
		File: state.reader, Entry: state.readerEntry, Diff: state.diff, Mode: state.readerMode,
	}).build()
}
