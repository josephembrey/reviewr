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
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
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
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
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
	kind := effectLoadFile
	if mode == workspace.FileReader {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			comparisonCopy := comparison
			boundsCopy := review.Bounds{Old: comparison.Old, New: comparison.New}
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			return effect{kind: effectLoadReviewFile, generation: state.contentGeneration, entry: entry, comparison: comparison}
		}
	} else {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			assessment := state.ledger.Assess(comparison)
			bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
			var retained *string
			if assessment.State == review.Updated && !state.reviewFull[entry.Path] && assessment.Frontier != nil && assessment.Retained != nil {
				bounds.Old = *assessment.Frontier
				retained = assessment.Retained
			}
			comparisonCopy, boundsCopy := comparison, bounds
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			return effect{kind: effectLoadReviewDocument, generation: state.contentGeneration, entry: entry, comparison: comparison, bounds: bounds, retained: retained}
		}
		kind = effectLoadDiff
	}
	return effect{kind: kind, generation: state.contentGeneration, entry: entry}
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
	state.readerLoading = false
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
}

func (state filesState) readerDocument() ui.ReaderDocument {
	return state.rawReaderDocument().WithContextFoldProgress(state.readerContextProgress, readerContextAnimationSteps)
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
	if state.readerContextExpanded == expanded || !state.rawReaderDocument().ContextFoldable() {
		return false, false
	}
	state.readerContextExpanded = expanded
	state.readerContextGeneration++
	state.advanceReaderContextPresentation()
	return true, readerContextAnimating(state.readerContextProgress, expanded)
}

func (state *filesState) advanceReaderContext(generation uint64) (bool, bool) {
	if generation != state.readerContextGeneration || !readerContextAnimating(state.readerContextProgress, state.readerContextExpanded) {
		return false, false
	}
	state.advanceReaderContextPresentation()
	return true, readerContextAnimating(state.readerContextProgress, state.readerContextExpanded)
}

func (state *filesState) advanceReaderContextPresentation() {
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	state.readerContextProgress = stepReaderContext(state.readerContextProgress, state.readerContextExpanded)
	state.place.ReaderOffset = reconcileLogicalLine(oldRows, oldOffset, readerRowIdentities(state.readerRows()))
	if state.place.ReaderOffset != oldOffset {
		state.place.ReaderColumn = 0
	}
	state.place.ClampReaderSource(len(state.readerRows()))
}

func (state *filesState) resetReaderContext() {
	state.readerContextExpanded = false
	state.readerContextProgress = 0
	state.readerContextGeneration++
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
