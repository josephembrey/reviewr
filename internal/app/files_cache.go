package app

import (
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// comparisonCacheEntry is one exact world snapshot. Scope switches may reuse
// it until the repository poll invalidates the state that produced it.
type comparisonCacheEntry struct {
	snapshot       repository.Snapshot
	reviewSnapshot review.Snapshot
	reviewCapable  bool
	warning        string
}

// readerCache keeps the last file and diff reader per comparison. Six bounded
// slots make repeated scope switches instant without retaining every file the
// reviewer has opened.
type readerCacheSlot struct {
	scope string
	mode  workspace.ReaderMode
}

type readerCacheKey struct {
	slot             readerCacheSlot
	kind             effectKind
	entry            repository.Entry
	comparison       repository.Comparison
	reviewComparison review.FileComparison
	bounds           review.Bounds
}

type readerCacheEntry struct {
	key          readerCacheKey
	file         repository.File
	diff         repository.Diff
	content      review.Content
	document     review.Document
	presentation ui.ReaderDocument
}

func (state *filesState) rememberComparison(msg snapshotLoadedMsg) {
	comparison := msg.snapshot.Comparison()
	if comparison.Scope == "" || msg.err != nil {
		return
	}
	entry := comparisonCacheEntry{snapshot: msg.snapshot}
	if msg.reviewCapable && msg.reviewGeneration == state.reviewGeneration {
		entry.reviewCapable = true
		entry.reviewSnapshot = msg.reviewSnapshot
		entry.warning = reviewLoadWarning(msg.reviewErr)
	}
	state.comparisonCache[comparison.Scope] = entry
}

func (state *filesState) activateComparison(
	scope string,
	fileSet workspace.FileSet,
	mode workspace.ReaderMode,
	visibleRows int,
) (effect, bool) {
	entry, ok := state.comparisonCache[scope]
	if !ok {
		return effect{}, false
	}
	state.listGeneration++
	state.reviewGeneration++
	state.listLoading = false
	state.listError = nil
	state.loaded = true
	state.snapshot = entry.snapshot
	state.reviewScope = scope
	state.reviewSnapshot = review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	state.comparisonWarning = ""
	if entry.reviewCapable {
		state.reviewSnapshot = entry.reviewSnapshot
		state.comparisonWarning = entry.warning
	}
	state.rederiveReviews()
	return state.project(fileSet, mode, visibleRows, false, true, false), true
}

func (state *filesState) invalidateComparison(scope string) {
	delete(state.comparisonCache, scope)
	for slot := range state.readerCache {
		if slot.scope == scope {
			delete(state.readerCache, slot)
		}
	}
}

func (state *filesState) invalidateComparisons() {
	clear(state.comparisonCache)
	clear(state.readerCache)
}

func (state filesState) readerKey(
	kind effectKind,
	entry repository.Entry,
	comparison review.FileComparison,
	bounds review.Bounds,
) readerCacheKey {
	repositoryComparison := state.snapshot.Comparison()
	scope := repositoryComparison.Scope
	if scope == "" {
		scope = state.reviewScope
	}
	return readerCacheKey{
		slot:             readerCacheSlot{scope: scope, mode: state.readerMode},
		kind:             kind,
		entry:            entry,
		comparison:       repositoryComparison,
		reviewComparison: comparison,
		bounds:           bounds,
	}
}

func (state *filesState) restoreReader(key readerCacheKey) bool {
	cached, ok := state.readerCache[key.slot]
	if !ok || cached.key != key {
		return false
	}
	switch key.kind {
	case effectLoadFile:
		*state = state.landFile(fileLoadedMsg{
			generation: state.contentGeneration, entry: key.entry,
			file: cached.file, presentation: cached.presentation,
		})
	case effectLoadDiff:
		*state = state.landDiff(diffLoadedMsg{
			generation: state.contentGeneration, entry: key.entry,
			diff: cached.diff, presentation: cached.presentation,
		})
	case effectLoadReviewDocument:
		*state = state.landReviewDocument(reviewDocumentLoadedMsg{
			generation: state.contentGeneration, entry: key.entry,
			comparison: key.reviewComparison, bounds: key.bounds,
			document: cached.document, presentation: cached.presentation,
		})
	case effectLoadReviewFile:
		*state = state.landReviewFile(reviewFileLoadedMsg{
			generation: state.contentGeneration, entry: key.entry,
			comparison: key.reviewComparison, content: cached.content,
			document: cached.document, presentation: cached.presentation,
		})
	default:
		return false
	}
	return !state.readerLoading
}

func (state filesState) readerComparisonPending() bool {
	return state.readerLoading &&
		state.readerLoadedKey.slot.scope != "" &&
		state.readerRequestKey.slot.scope != "" &&
		state.readerLoadedKey.slot.scope != state.readerRequestKey.slot.scope
}

func (state *filesState) rememberReader(entry readerCacheEntry) {
	key := state.readerRequestKey
	if key.kind == effectNone || key.kind != entry.key.kind || key.entry != entry.key.entry {
		return
	}
	entry.key = key
	state.readerCache[key.slot] = entry
}
