package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestComparisonCacheRestoresExactSnapshotAndReviewMetadata(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	entry := repository.Entry{Path: "a.go", State: repository.FileModified}
	uncommitted := repository.NewComparisonSnapshot(
		[]repository.Entry{entry},
		repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: "head"},
	)
	uncommittedReview := testComparison("a.go", "head", "old-head", "new")
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration, reviewGeneration: state.reviewGeneration,
		snapshot: uncommitted, reviewCapable: true,
		reviewSnapshot: review.Snapshot{
			Scope:       repository.ComparisonUncommitted,
			Comparisons: map[string]review.FileComparison{"a.go": uncommittedReview},
		},
	}, workspace.ChangedFiles, workspace.DiffReader, 10)

	branchLoad := state.reload(repository.ComparisonBranch)
	branch := repository.NewComparisonSnapshot(
		[]repository.Entry{entry},
		repository.Comparison{Scope: repository.ComparisonBranch, Basis: "merge-base"},
	)
	branchReview := testComparison("a.go", "merge-base", "old-base", "new")
	branchReview.Identity.Scope = repository.ComparisonBranch
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation: branchLoad.generation, reviewGeneration: branchLoad.reviewGeneration,
		snapshot: branch, reviewCapable: true,
		reviewSnapshot: review.Snapshot{
			Scope:       repository.ComparisonBranch,
			Comparisons: map[string]review.FileComparison{"a.go": branchReview},
		},
	}, workspace.ChangedFiles, workspace.DiffReader, 10)

	pending, cached := state.activateComparison(
		repository.ComparisonUncommitted,
		workspace.ChangedFiles,
		workspace.DiffReader,
		10,
	)
	if !cached || pending.kind == effectLoadSnapshot {
		t.Fatalf("cached activation = cached %v effect %+v", cached, pending)
	}
	if comparison := state.snapshot.Comparison(); comparison.Scope != repository.ComparisonUncommitted || comparison.Basis != "head" {
		t.Fatalf("restored comparison = %+v", comparison)
	}
	if got := state.reviewSnapshot.Comparisons["a.go"]; got != uncommittedReview {
		t.Fatalf("restored review comparison = %+v, want %+v", got, uncommittedReview)
	}
	selected, _ := state.place.SelectedIdentity()
	if selected == "" || state.listLoading {
		t.Fatalf("cached activation lost place or exposed loading: selected=%q loading=%v", selected, state.listLoading)
	}
}

func TestReaderCacheRestoresTheLastExactDiffPerComparison(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	entry := repository.Entry{Path: "a.go", State: repository.FileModified}
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{entry},
			repository.Comparison{Scope: repository.ComparisonBranch, Basis: "merge-base"},
		),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	if pending.kind != effectLoadDiff {
		t.Fatalf("initial diff effect = %+v", pending)
	}
	diff := repository.Diff{Entry: entry, Kind: repository.DiffReady, Content: "cached diff"}
	presentation := ui.ReaderDocument{
		Kind: ui.ReaderDiffDocument,
		Rows: []ui.ReaderRow{{Identity: "cached", Text: "cached diff"}},
	}
	state = state.landDiff(diffLoadedMsg{
		generation: pending.generation, entry: entry, diff: diff, presentation: presentation,
	})
	state.diff = repository.Diff{}
	state.readerPresentation = nil

	if next := state.requestReader(entry, workspace.DiffReader); next.kind != effectNone {
		t.Fatalf("cached reader started effect %+v", next)
	}
	if state.diff.Content != "cached diff" || state.readerLoading || len(state.readerRows()) != 1 {
		t.Fatalf("restored reader = diff %+v loading=%v rows=%#v", state.diff, state.readerLoading, state.readerRows())
	}

	state.invalidateComparison(repository.ComparisonBranch)
	if next := state.requestReader(entry, workspace.DiffReader); next.kind != effectLoadDiff {
		t.Fatalf("invalidated reader effect = %+v", next)
	}
}

func TestReaderCacheRestoresTheExactReviewDocument(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	entry := repository.Entry{Path: "a.go", State: repository.FileModified}
	comparison := testComparison("a.go", "merge-base", "old", "new")
	comparison.Identity.Scope = repository.ComparisonBranch
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration, reviewGeneration: state.reviewGeneration,
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{entry},
			repository.Comparison{Scope: repository.ComparisonBranch, Basis: "merge-base"},
		),
		reviewCapable: true,
		reviewSnapshot: review.Snapshot{
			Scope:       repository.ComparisonBranch,
			Comparisons: map[string]review.FileComparison{"a.go": comparison},
		},
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	if pending.kind != effectLoadReviewDocument {
		t.Fatalf("initial review effect = %+v", pending)
	}
	bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
	document := review.Document{
		Bounds: bounds, Exact: true,
		Lines: []review.Line{{Identity: "cached-review", Text: "+ new", Kind: review.AddedLine, NewLine: 1}},
	}
	presentation := ui.ReaderDocument{
		Kind: ui.ReaderDiffDocument,
		Rows: []ui.ReaderRow{{Identity: "cached-review", Text: "new", Kind: ui.ReaderInsertion}},
	}
	state = state.landReviewDocument(reviewDocumentLoadedMsg{
		generation: pending.generation, entry: entry, comparison: comparison, bounds: bounds,
		document: document, presentation: presentation,
	})
	state.reviewDocument = review.Document{}
	state.readerPresentation = nil
	state.displayedComparison = nil
	state.displayedBounds = nil

	if next := state.requestReader(entry, workspace.DiffReader); next.kind != effectNone {
		t.Fatalf("cached review reader started effect %+v", next)
	}
	if !state.reviewDocument.Exact || state.displayedComparison == nil || *state.displayedComparison != comparison ||
		state.readerLoading || len(state.readerRows()) != 1 {
		t.Fatalf("restored review reader = document %+v comparison %+v loading=%v rows=%#v", state.reviewDocument, state.displayedComparison, state.readerLoading, state.readerRows())
	}
}

func TestCachedComparisonBlanksOnlyAnUncachedReaderFromAnotherScope(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	entry := repository.Entry{Path: "a.go", State: repository.FileModified}
	state, pending := state.landSnapshot(snapshotLoadedMsg{
		generation: state.listGeneration,
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{entry},
			repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: "head"},
		),
	}, workspace.ChangedFiles, workspace.DiffReader, 10)
	state = state.landDiff(diffLoadedMsg{
		generation: pending.generation, entry: entry,
		diff: repository.Diff{Entry: entry, Kind: repository.DiffReady, Content: "old scope"},
		presentation: ui.ReaderDocument{
			Kind: ui.ReaderDiffDocument,
			Rows: []ui.ReaderRow{{Identity: "old", Text: "old scope"}},
		},
	})
	state.comparisonCache[repository.ComparisonBranch] = comparisonCacheEntry{
		snapshot: repository.NewComparisonSnapshot(
			[]repository.Entry{entry},
			repository.Comparison{Scope: repository.ComparisonBranch, Basis: "merge-base"},
		),
	}

	pending, cached := state.activateComparison(
		repository.ComparisonBranch, workspace.ChangedFiles, workspace.DiffReader, 10,
	)
	if !cached || pending.kind != effectLoadDiff || !state.readerComparisonPending() {
		t.Fatalf("cached comparison with reader miss = cached %v effect %+v state %+v", cached, pending, state)
	}
	view := state.viewModel(ui.Calculate(80, 20))
	if len(view.NavigatorRows) != 1 || len(view.ReaderDocument.Rows) != 0 || view.ReaderEmpty.Text != "Loading comparison…" {
		t.Fatalf("reader miss view = navigator %#v reader %#v empty %+v", view.NavigatorRows, view.ReaderDocument.Rows, view.ReaderEmpty)
	}
}

func TestComparisonInvalidationMatchesRepositorySignals(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	for _, scope := range []string{
		repository.ComparisonUncommitted,
		repository.ComparisonBranch,
		repository.ComparisonLastTurn,
	} {
		model.files.comparisonCache[scope] = comparisonCacheEntry{
			snapshot: repository.NewComparisonSnapshot(nil, repository.Comparison{Scope: scope, Basis: scope}),
		}
		model.files.readerCache[readerCacheSlot{scope: scope, mode: workspace.DiffReader}] = readerCacheEntry{}
	}
	model.poll = repositoryPollState{
		generation: 1, ready: true,
		fingerprint: repository.StateFingerprint{Worktree: "worktree", Refs: "refs"},
	}

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 1,
		state:      repository.StateFingerprint{Worktree: "worktree", Refs: "new-refs"},
	})
	if _, ok := model.files.comparisonCache[repository.ComparisonBranch]; ok {
		t.Fatal("public ref change retained Branch cache")
	}
	for _, scope := range []string{repository.ComparisonUncommitted, repository.ComparisonLastTurn} {
		if _, ok := model.files.comparisonCache[scope]; !ok {
			t.Fatalf("public ref change invalidated %s cache", scope)
		}
	}

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation:          1,
		state:               repository.StateFingerprint{Worktree: "worktree", Refs: "new-refs"},
		turnBaselineChanged: true,
	})
	if _, ok := model.files.comparisonCache[repository.ComparisonLastTurn]; ok {
		t.Fatal("turn baseline change retained Last-turn cache")
	}
	if _, ok := model.files.comparisonCache[repository.ComparisonUncommitted]; !ok {
		t.Fatal("turn baseline change invalidated Uncommitted cache")
	}

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 1,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "new-refs"},
	})
	if len(model.files.comparisonCache) != 0 || len(model.files.readerCache) != 0 {
		t.Fatalf("worktree change retained caches: comparisons=%d readers=%d", len(model.files.comparisonCache), len(model.files.readerCache))
	}
}
