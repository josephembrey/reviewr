package app

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	reviewdomain "github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type fakeReviewSource struct {
	*fakeSource
	comparisons         map[string]reviewdomain.Snapshot
	contents            map[reviewdomain.Endpoint]reviewdomain.Content
	identity            reviewdomain.RepositoryID
	comparisonErr       map[string]error
	requestedCandidates [][]reviewdomain.Candidate
}

type commentEditedMsg struct{}
type herdrTurnMsg struct{}

func (source *fakeReviewSource) ReviewComparisons(scope string, candidates []reviewdomain.Candidate) (reviewdomain.Snapshot, error) {
	source.requestedCandidates = append(source.requestedCandidates, append([]reviewdomain.Candidate(nil), candidates...))
	if err := source.comparisonErr[scope]; err != nil {
		return reviewdomain.Snapshot{Scope: scope, Comparisons: map[string]reviewdomain.FileComparison{}}, err
	}
	return source.comparisons[scope], nil
}

func (source *fakeReviewSource) ReadReviewContent(_ reviewdomain.EndpointSource, endpoint reviewdomain.Endpoint) reviewdomain.Content {
	if content, ok := source.contents[endpoint]; ok {
		return content
	}
	return reviewdomain.UnavailableContent(endpoint.Path, endpoint.Kind, endpoint.Mode, errors.New("missing fake endpoint"))
}

func (source *fakeReviewSource) ReviewRepositoryID() (reviewdomain.RepositoryID, error) {
	return source.identity, nil
}

func testComparison(path, basis, oldID, newID string) reviewdomain.FileComparison {
	return reviewdomain.FileComparison{
		Identity:  reviewdomain.ComparisonIdentity{Scope: "uncommitted", Basis: basis},
		OldSource: reviewdomain.EndpointSource{Kind: reviewdomain.GitTreeSource, Value: basis},
		NewSource: reviewdomain.EndpointSource{Kind: reviewdomain.WorktreeSource},
		Action:    reviewdomain.Modified,
		Old:       reviewdomain.Endpoint{Path: path, Kind: reviewdomain.Regular, Mode: 0o100644, ContentID: oldID},
		New:       reviewdomain.Endpoint{Path: path, Kind: reviewdomain.Regular, Mode: 0o100644, ContentID: newID},
	}
}

func content(endpoint reviewdomain.Endpoint, text string) reviewdomain.Content {
	return reviewdomain.Content{Endpoint: endpoint, State: reviewdomain.ContentText, Text: text, Size: int64(len(text))}
}

func TestReviewMarkUnmarkIsExplicitVerifiedAndPersistent(t *testing.T) {
	root := t.TempDir()
	stateRoot := t.TempDir()
	comparison := testComparison("a.go", "head-1", "old", "new")
	source := &fakeReviewSource{
		fakeSource:  &fakeSource{snapshot: snapshotOf(repository.Entry{Path: "a.go", State: repository.FileModified})},
		comparisons: map[string]reviewdomain.Snapshot{"uncommitted": {Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": comparison}}},
		contents:    map[reviewdomain.Endpoint]reviewdomain.Content{comparison.Old: content(comparison.Old, "old\n"), comparison.New: content(comparison.New, "new\n")},
		identity:    reviewdomain.RepositoryID{CommonGitDir: root, Worktree: root},
	}
	model := NewWithReviewStateRoot(source, herdr.Context{}, stateRoot)
	model.geometry = ui.Calculate(80, 20)
	model.controls.Files = workspace.ChangedFiles

	next, _ := model.Update(model.command(effect{kind: effectLoadReviewState})())
	model = next.(Model)
	if !strings.Contains(model.View().Content, "review state missing; starting unreviewed") {
		t.Fatalf("missing-state warning was not rendered: %q", model.files.reviewWarning)
	}
	next, readerCommand := model.Update(model.command(effect{kind: effectLoadSnapshot, generation: model.files.listGeneration, reviewGeneration: model.files.reviewGeneration, scope: "uncommitted"})())
	model = next.(Model)
	if readerCommand == nil {
		t.Fatal("snapshot did not request the initial reader")
	}
	next, _ = model.Update(readerCommand())
	model = next.(Model)
	if got := model.files.ledger.Assess(comparison).State; got != reviewdomain.Unreviewed {
		t.Fatalf("observation created coverage: %v", got)
	}
	for _, observation := range []tea.Msg{commentEditedMsg{}, herdrTurnMsg{}} {
		next, command := model.Update(observation)
		model = next.(Model)
		if command != nil || len(model.files.ledger.Receipts()) != 0 {
			t.Fatalf("observation-only message %T changed coverage", observation)
		}
	}
	model.apply(Action{Kind: ScrollReader, Amount: 100})
	if len(model.files.ledger.Receipts()) != 0 {
		t.Fatal("scrolling created a receipt")
	}

	next, verifyCommand := model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = next.(Model)
	if verifyCommand == nil || len(model.files.ledger.Receipts()) != 0 {
		t.Fatal("x was not verification-gated")
	}
	next, persistCommand := model.Update(verifyCommand())
	model = next.(Model)
	if got := model.files.ledger.Assess(comparison).State; got != reviewdomain.Reviewed || persistCommand == nil {
		t.Fatalf("verified x state=%v persistence=%v", got, persistCommand != nil)
	}
	next, _ = model.Update(persistCommand())
	model = next.(Model)
	if model.files.reviewWarning != "" {
		t.Fatalf("successful persistence warning = %q", model.files.reviewWarning)
	}
	if _, err := os.Stat(model.files.store.Path()); err != nil {
		t.Fatalf("private review state was not written: %v", err)
	}
	restarted := NewWithReviewStateRoot(source, herdr.Context{}, stateRoot)
	next, _ = restarted.Update(restarted.command(effect{kind: effectLoadReviewState})())
	restarted = next.(Model)
	if restarted.files.ledger.Assess(comparison).State != reviewdomain.Reviewed {
		t.Fatal("process restart did not recover review coverage")
	}

	next, verifyCommand = model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = next.(Model)
	next, persistCommand = model.Update(verifyCommand())
	model = next.(Model)
	if got := model.files.ledger.Assess(comparison).State; got != reviewdomain.Unreviewed {
		t.Fatalf("second x did not clear applicable coverage: %v", got)
	}
	if persistCommand == nil {
		t.Fatal("clear did not schedule persistence")
	}
}

func TestUpdatedReaderBoundsToggleAndNarrowMark(t *testing.T) {
	comparison := testComparison("a.go", "head-1", "old", "new")
	middle := reviewdomain.Endpoint{Path: "a.go", Kind: reviewdomain.Regular, Mode: 0o100644, ContentID: "middle"}
	past := comparison
	past.New = middle
	middleText := "middle\n"
	ledger := reviewdomain.Ledger{}
	if !ledger.Mark(past, reviewdomain.Bounds{Old: past.Old, New: middle}, &middleText) {
		t.Fatal("failed to build reviewed frontier")
	}
	source := &fakeReviewSource{
		fakeSource:  &fakeSource{snapshot: snapshotOf(repository.Entry{Path: "a.go", State: repository.FileModified})},
		comparisons: map[string]reviewdomain.Snapshot{"uncommitted": {Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": comparison}}},
		contents:    map[reviewdomain.Endpoint]reviewdomain.Content{comparison.Old: content(comparison.Old, "old\n"), comparison.New: content(comparison.New, "new\n")},
	}
	model := New(source, herdr.Context{})
	model.geometry = ui.Calculate(80, 20)
	model.controls.Files = workspace.ChangedFiles
	model.controls.Reader = workspace.DiffReader
	model.files.ledger = ledger
	model.files.reviewLoaded = true

	next, command := model.Update(model.command(effect{kind: effectLoadSnapshot, generation: model.files.listGeneration, reviewGeneration: model.files.reviewGeneration, scope: "uncommitted"})())
	model = next.(Model)
	next, _ = model.Update(command())
	model = next.(Model)
	if model.files.displayedBounds == nil || model.files.displayedBounds.Old != middle || !strings.Contains(model.files.viewModel(model.geometry).ReaderTitle, "since reviewed") {
		t.Fatalf("updated reader bounds/title = %+v / %q", model.files.displayedBounds, model.files.viewModel(model.geometry).ReaderTitle)
	}
	before := model.files.ledger.Clone()
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'R', Text: "R"}))
	model = next.(Model)
	if command == nil || !reflect.DeepEqual(model.files.ledger, before) {
		t.Fatal("R changed coverage or did not load full bounds")
	}
	next, _ = model.Update(command())
	model = next.(Model)
	if model.files.displayedBounds.Old != comparison.Old || !strings.Contains(model.files.viewModel(model.geometry).ReaderTitle, "full comparison") {
		t.Fatalf("full reader bounds/title = %+v / %q", model.files.displayedBounds, model.files.viewModel(model.geometry).ReaderTitle)
	}

	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'R', Text: "R"}))
	model = next.(Model)
	next, _ = model.Update(command())
	model = next.(Model)
	model.files.place.Focus = navigation.FocusReader
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = next.(Model)
	next, _ = model.Update(command())
	model = next.(Model)
	receipts := model.files.ledger.Receipts()
	last := receipts[len(receipts)-1]
	if last.Old != middle || last.New != comparison.New || model.files.ledger.Assess(comparison).State != reviewdomain.Reviewed {
		t.Fatalf("incremental mark = %+v assessment=%v", last, model.files.ledger.Assess(comparison).State)
	}
}

func TestPartialAndBasisChangedReadersExplainFullBounds(t *testing.T) {
	for _, test := range []struct {
		name  string
		scope string
		basis string
		want  string
	}{
		{name: "partial", scope: "last-turn", basis: "head", want: "older review gap; full comparison"},
		{name: "basis changed", scope: "uncommitted", basis: "old-head", want: "review basis changed; full comparison"},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := testComparison("a.go", "head", "current-old", "current-new")
			related := testComparison("a.go", test.basis, "related-old", "related-new")
			related.Identity.Scope = test.scope
			state := loadedFilesState(t, repository.Entry{Path: "a.go", State: repository.FileModified})
			_ = state.ledger.Mark(related, reviewdomain.Bounds{Old: related.Old, New: related.New}, nil)
			state.readerEntry = repository.Entry{Path: "a.go", State: repository.FileModified}
			state.readerMode = workspace.DiffReader
			state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": current}}
			bounds := reviewdomain.Bounds{Old: current.Old, New: current.New}
			state.displayedComparison = &current
			state.displayedBounds = &bounds
			state.reviewDocument = reviewdomain.Document{Bounds: bounds, Exact: true}
			if title := state.viewModel(ui.Calculate(80, 20)).ReaderTitle; !strings.Contains(title, test.want) {
				t.Fatalf("reader title = %q, want %q", title, test.want)
			}
		})
	}
}

func TestNextReviewGapUsesPriorityTreeOrderAndExpandsAncestors(t *testing.T) {
	entries := []repository.Entry{
		{Path: "a/unreviewed.go", State: repository.FileModified},
		{Path: "b/partial.go", State: repository.FileModified},
		{Path: "c/updated.go", State: repository.FileModified},
		{Path: "z/basis.go", State: repository.FileModified},
		{Path: "z/ordinary.go", State: repository.FileUnchanged},
	}
	state := loadedFilesState(t, entries...)
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{}}
	for _, entry := range entries {
		if !entry.Changed() {
			continue
		}
		state.reviewSnapshot.Comparisons[entry.Path] = testComparison(entry.Path, "basis-2", entry.Path+":old", entry.Path+":new")
	}

	basis := state.reviewSnapshot.Comparisons["z/basis.go"]
	priorBasis := basis
	priorBasis.Identity.Basis = "basis-1"
	priorBasis.Old.ContentID = "other-old"
	priorBasis.New.ContentID = "other-new"
	_ = state.ledger.Mark(priorBasis, reviewdomain.Bounds{Old: priorBasis.Old, New: priorBasis.New}, nil)
	updated := state.reviewSnapshot.Comparisons["c/updated.go"]
	middle := updated.New
	middle.ContentID = "middle"
	pastUpdated := updated
	pastUpdated.New = middle
	retained := "middle"
	_ = state.ledger.Mark(pastUpdated, reviewdomain.Bounds{Old: updated.Old, New: middle}, &retained)
	partial := state.reviewSnapshot.Comparisons["b/partial.go"]
	narrow := partial
	narrow.Identity.Scope = "last-turn"
	narrow.Old.ContentID = "narrow-old"
	narrow.New.ContentID = "narrow-new"
	_ = state.ledger.Mark(narrow, reviewdomain.Bounds{Old: narrow.Old, New: narrow.New}, nil)

	if row, ok := state.tree.Row(filetree.DirectoryIdentity("z")); !ok || row.Expanded {
		t.Fatalf("basis directory did not start collapsed: %+v, %v", row, ok)
	}
	state.reconcileVisibleRows(10)
	pending := state.selectNextReviewGap(10, workspace.FileReader)
	selected, _ := state.place.SelectedIdentity()
	if pending.entry.Path != "z/basis.go" || selected != filetree.FileIdentity("z/basis.go") {
		t.Fatalf("first gap = effect %+v selected %q", pending, selected)
	}
	row, _ := state.tree.Row(filetree.DirectoryIdentity("z"))
	if !row.Expanded {
		t.Fatal("hidden review gap ancestor remained collapsed")
	}
	pending = state.selectNextReviewGap(10, workspace.FileReader)
	if pending.entry.Path != "c/updated.go" {
		t.Fatalf("second gap = %+v, want Updated before Partial/Unreviewed", pending)
	}
}

func TestDirectoryRollupUsesHiddenChangedDescendantsAndUnchangedHasNoBadge(t *testing.T) {
	state := loadedFilesState(t,
		repository.Entry{Path: "src/a.go", State: repository.FileModified},
		repository.Entry{Path: "src/b.go", State: repository.FileModified},
		repository.Entry{Path: "plain.go", State: repository.FileUnchanged},
	)
	a := testComparison("src/a.go", "head", "a0", "a1")
	b := testComparison("src/b.go", "head", "b0", "b1")
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"src/a.go": a, "src/b.go": b}}
	_ = state.ledger.Mark(a, reviewdomain.Bounds{Old: a.Old, New: a.New}, nil)
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("src did not start collapsed: %+v, %v", row, ok)
	}
	state.reconcileVisibleRows(10)
	rows := state.viewModel(ui.Calculate(80, 20)).NavigatorRows
	for _, row := range rows {
		switch row.Identity {
		case filetree.DirectoryIdentity("src"):
			if row.Progress != "1/2" || row.Review != nil {
				t.Fatalf("directory rollup = %+v", row)
			}
		case filetree.FileIdentity("plain.go"):
			if row.Review != nil {
				t.Fatalf("unchanged row has review badge: %+v", row)
			}
		}
	}
}

func TestInitialNestedCollapseCoexistsWithReviewBadgesAndRollups(t *testing.T) {
	reviewed := testComparison("src/reviewed.go", "head", "reviewed-old", "reviewed-new")
	nestedGap := testComparison("src/nested/gap.go", "head", "gap-old", "gap-new")
	nestedGapTwo := testComparison("src/nested/other.go", "head", "other-old", "other-new")
	rootGap := testComparison("root.go", "head", "root-old", "root-new")
	state := newFilesState()
	if !state.ledger.Mark(reviewed, reviewdomain.Bounds{Old: reviewed.Old, New: reviewed.New}, nil) {
		t.Fatal("failed to seed reviewed descendant")
	}
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation:       state.listGeneration,
		reviewGeneration: state.reviewGeneration,
		reviewCapable:    true,
		snapshot: snapshotOf(
			repository.Entry{Path: reviewed.New.Path, State: repository.FileModified},
			repository.Entry{Path: nestedGap.New.Path, State: repository.FileModified},
			repository.Entry{Path: nestedGapTwo.New.Path, State: repository.FileModified},
			repository.Entry{Path: rootGap.New.Path, State: repository.FileModified},
		),
		reviewSnapshot: reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{
			reviewed.New.Path:     reviewed,
			nestedGap.New.Path:    nestedGap,
			nestedGapTwo.New.Path: nestedGapTwo,
			rootGap.New.Path:      rootGap,
		}},
	}, workspace.ChangedFiles, workspace.FileReader, 10)

	for _, path := range []string{"src", "src/nested"} {
		row, ok := state.tree.Row(filetree.DirectoryIdentity(path))
		if !ok || row.Expanded {
			t.Fatalf("initial directory %q = %+v, %v; want collapsed", path, row, ok)
		}
	}
	view := state.viewModel(ui.Calculate(80, 24))
	if got, want := len(view.NavigatorRows), 2; got != want {
		t.Fatalf("initial visible rows = %d, want %d: %+v", got, want, view.NavigatorRows)
	}
	for _, row := range view.NavigatorRows {
		switch row.Identity {
		case filetree.DirectoryIdentity("src"):
			if row.Progress != "1/3" || row.Review != nil {
				t.Fatalf("collapsed directory review presentation = %+v", row)
			}
		case filetree.FileIdentity("root.go"):
			if row.Review == nil || *row.Review != reviewdomain.Unreviewed || row.Progress != "" {
				t.Fatalf("visible changed-file review presentation = %+v", row)
			}
		default:
			t.Fatalf("collapsed descendant leaked into initial rows: %+v", row)
		}
	}
}

func TestReviewActivityLeavesGitAndScratchPlaceUntouched(t *testing.T) {
	comparison := testComparison("a.go", "head", "old", "new")
	gap := testComparison("b.go", "head", "old-b", "new-b")
	source := &fakeReviewSource{
		fakeSource: &fakeSource{snapshot: snapshotOf(
			repository.Entry{Path: "a.go", State: repository.FileModified},
			repository.Entry{Path: "b.go", State: repository.FileModified},
		)},
		comparisons: map[string]reviewdomain.Snapshot{"uncommitted": {Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{
			"a.go": comparison,
			"b.go": gap,
		}}},
		contents: map[reviewdomain.Endpoint]reviewdomain.Content{
			comparison.Old: content(comparison.Old, "old\n"),
			comparison.New: content(comparison.New, "new\n"),
			gap.Old:        content(gap.Old, "old-b\n"),
			gap.New:        content(gap.New, "new-b\n"),
		},
	}
	model := New(source, herdr.Context{})
	model.geometry = ui.Calculate(80, 24)
	model.controls.Files = workspace.ChangedFiles
	next, readerCommand := model.Update(model.command(effect{
		kind: effectLoadSnapshot, generation: model.files.listGeneration,
		reviewGeneration: model.files.reviewGeneration, scope: "uncommitted",
	})())
	model = next.(Model)
	if readerCommand == nil {
		t.Fatal("review snapshot did not request the visible Files reader")
	}
	next, _ = model.Update(readerCommand())
	model = next.(Model)

	model.history.place = navigation.State{Items: []string{"commit-1", "commit-2"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 7}
	model.refs.place = navigation.State{Items: []string{"all", "branch"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 5}
	model.stashes.place = navigation.State{Items: []string{"stash-1", "stash-2"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 3}
	model.note.editor.Load("first\nsecond\nthird")
	model.note.editor.MoveEnd(false)
	model.note.editor.Resize(12, 1)
	model.note.editor.SetScroll(1)
	model.note.loaded = true
	model.note.generation = 4
	model.note.savedGeneration = 4
	historyPlace := model.history.place
	refsPlace := model.refs.place
	stashesPlace := model.stashes.place
	scratchPlace := model.note.editor.Presentation()
	scratchGeneration := model.note.generation

	next, verifyCommand := model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = next.(Model)
	if verifyCommand == nil {
		t.Fatal("Files x did not request exact endpoint verification")
	}
	next, _ = model.Update(verifyCommand())
	model = next.(Model)
	if model.files.ledger.Assess(comparison).State != reviewdomain.Reviewed {
		t.Fatal("Files review activity did not create exact coverage")
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'X', Text: "X"}))
	model = next.(Model)

	model.active = workspace.Git
	beforeReceipts := model.files.ledger.Receipts()
	for _, action := range []Action{{Kind: ToggleReview}, {Kind: ToggleReviewBounds}, {Kind: NextReviewGap}, {Kind: ActivateReviewBadge, Index: 0}} {
		if pending := model.apply(action); pending.kind != effectNone {
			t.Fatalf("Git accepted review action %+v as effect %+v", action, pending)
		}
	}
	model.active = workspace.Files
	model.scratch = true
	for _, action := range []Action{{Kind: ToggleReview}, {Kind: ToggleReviewBounds}, {Kind: NextReviewGap}, {Kind: ActivateReviewBadge, Index: 0}} {
		if pending := model.apply(action); pending.kind != effectNone {
			t.Fatalf("Scratch accepted review action %+v as effect %+v", action, pending)
		}
	}

	if !reflect.DeepEqual(model.history.place, historyPlace) || !reflect.DeepEqual(model.refs.place, refsPlace) || !reflect.DeepEqual(model.stashes.place, stashesPlace) {
		t.Fatalf("review activity changed Git place: log=%+v refs=%+v stashes=%+v", model.history.place, model.refs.place, model.stashes.place)
	}
	if !reflect.DeepEqual(model.note.editor.Presentation(), scratchPlace) || model.note.generation != scratchGeneration {
		t.Fatalf("review activity changed Scratch place: presentation=%+v generation=%d", model.note.editor.Presentation(), model.note.generation)
	}
	if !reflect.DeepEqual(model.files.ledger.Receipts(), beforeReceipts) {
		t.Fatal("review-inert workspaces mutated Files receipts")
	}
}

func TestNextReviewGapReportsCompletionWithoutMovingPlace(t *testing.T) {
	state := loadedFilesState(t, repository.Entry{Path: "done.go", State: repository.FileModified})
	comparison := testComparison("done.go", "head", "old", "new")
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"done.go": comparison}}
	_ = state.ledger.Mark(comparison, reviewdomain.Bounds{Old: comparison.Old, New: comparison.New}, nil)
	before := state.place
	if pending := state.selectNextReviewGap(10, workspace.FileReader); pending.kind != effectNone {
		t.Fatalf("completed gaps scheduled %+v", pending)
	}
	if !reflect.DeepEqual(state.place, before) || !strings.Contains(state.comparisonWarning, "reviewed") {
		t.Fatalf("completion changed place or omitted status: place=%+v warning=%q", state.place, state.comparisonWarning)
	}
}

func TestReviewDocumentLandingReconcilesLogicalPlaceAndPreservesOtherPlace(t *testing.T) {
	comparison := testComparison("src/a.go", "head", "old", "new")
	bounds := reviewdomain.Bounds{Old: comparison.Old, New: comparison.New}
	state := loadedFilesState(t, repository.Entry{Path: "src/a.go", State: repository.FileModified}, repository.Entry{Path: "src/b.go", State: repository.FileModified})
	state.readerEntry = repository.Entry{Path: "src/a.go", State: repository.FileModified}
	state.readerMode = workspace.DiffReader
	state.contentGeneration = 2
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"src/a.go": comparison}}
	state.requestedComparison = &comparison
	state.requestedBounds = &bounds
	first := reviewdomain.BuildDocument(bounds, content(comparison.Old, "keep\nold\n"), content(comparison.New, "keep\nnew\n"))
	state = state.landReviewDocument(reviewDocumentLoadedMsg{generation: 2, entry: state.readerEntry, comparison: comparison, bounds: bounds, document: first}, 1)
	keepIdentity := first.Lines[0].Identity
	state.place.ReaderOffset = 0
	state.reviewCursor = 0
	state.reviewSelectionAnchor = 0
	state.place.Focus = navigation.FocusReader
	state.reviewFull["src/a.go"] = true
	if row, ok := state.tree.Row(filetree.DirectoryIdentity("src")); !ok || row.Expanded {
		t.Fatalf("src did not start collapsed: %+v, %v", row, ok)
	}
	state.contentGeneration = 3
	second := reviewdomain.BuildDocument(bounds, content(comparison.Old, "prefix\nkeep\nold\n"), content(comparison.New, "prefix\nkeep\nnew\n"))
	state = state.landReviewDocument(reviewDocumentLoadedMsg{generation: 3, entry: state.readerEntry, comparison: comparison, bounds: bounds, document: second}, 1)
	if second.Lines[state.reviewCursor].Identity != keepIdentity || second.Lines[state.reviewSelectionAnchor].Identity != keepIdentity || second.Lines[state.place.ReaderOffset].Identity != keepIdentity {
		t.Fatalf("logical place did not follow identity: offset=%d cursor=%d anchor=%d", state.place.ReaderOffset, state.reviewCursor, state.reviewSelectionAnchor)
	}
	row, _ := state.tree.Row(filetree.DirectoryIdentity("src"))
	if state.place.Focus != navigation.FocusReader || row.Expanded || !state.reviewFull["src/a.go"] {
		t.Fatalf("unrelated Continuity changed: focus=%v row=%+v full=%v", state.place.Focus, row, state.reviewFull["src/a.go"])
	}
}

func TestReviewRoutingIsFilesOnlyAndBadgeUsesSemanticAction(t *testing.T) {
	g := ui.Calculate(80, 20)
	state := reviewdomain.Unreviewed
	rows := []ui.NavigatorRow{{Label: "a.go", Tree: true, Review: &state}}
	for _, key := range []tea.Key{{Code: 'x', Text: "x"}, {Code: 'R', Text: "R"}, {Code: 'X', Text: "X"}} {
		if _, ok := routeMessageWithRows(tea.KeyPressMsg(key), navigation.FocusNavigator, g, workspace.Git, workspace.Controls{}, false, false, 0, 1, 0, 0, rows); ok {
			t.Fatalf("Git consumed review key %q", key.Text)
		}
	}
	layout := ui.LayoutNavigatorRow(rows[0], g.NavigatorRows.Width)
	for x := layout.Review.X; x < layout.Review.X+layout.Review.Width; x++ {
		action, ok := routeMessageWithRows(tea.MouseClickMsg(tea.Mouse{X: g.NavigatorRows.X + x, Y: g.NavigatorRows.Y, Button: tea.MouseLeft}), navigation.FocusNavigator, g, workspace.Files, workspace.Controls{}, false, false, 0, 1, 0, 0, rows)
		if !ok || action.Kind != ActivateReviewBadge || action.Index != 0 {
			t.Fatalf("badge cell %d routed as (%+v,%v)", x, action, ok)
		}
	}
}

func TestReviewSnapshotsAreGenerationScopedAndRefreshNeverMutatesLedger(t *testing.T) {
	one := testComparison("changed.go", "head-1", "old", "one")
	two := testComparison("changed.go", "head-2", "old", "two")
	state := newFilesState()
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation:       state.listGeneration,
		reviewGeneration: state.reviewGeneration,
		reviewCapable:    true,
		snapshot: snapshotOf(
			repository.Entry{Path: "changed.go", State: repository.FileModified},
			repository.Entry{Path: "plain.go", State: repository.FileUnchanged},
		),
		reviewSnapshot: reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"changed.go": one}},
	}, workspace.AllFiles, workspace.FileReader, 10)
	_ = state.ledger.Mark(one, reviewdomain.Bounds{Old: one.Old, New: one.New}, nil)
	before := state.ledger.Clone()
	if effect := state.switchScope(workspace.ChangedFiles, workspace.FileReader, 10); effect.kind != effectNone {
		t.Fatalf("scope switch started a comparison enumeration: %+v", effect)
	}
	if !reflect.DeepEqual(state.ledger, before) || state.ledger.Assess(one).State != reviewdomain.Reviewed {
		t.Fatal("All/Changed projection changed review coverage")
	}

	branch := state.requestComparison("branch")
	lastTurn := state.requestComparison("last-turn")
	state, _ = state.landReviewSnapshot(reviewSnapshotLoadedMsg{
		listGeneration: branch.generation, reviewGeneration: branch.reviewGeneration, scope: "branch",
		snapshot: reviewdomain.Snapshot{Scope: "branch", Comparisons: map[string]reviewdomain.FileComparison{"changed.go": two}},
	}, workspace.FileReader, 10)
	if state.reviewScope != "last-turn" || len(state.reviewSnapshot.Comparisons) != 0 {
		t.Fatal("stale comparison scope landed")
	}
	state, _ = state.landReviewSnapshot(reviewSnapshotLoadedMsg{
		listGeneration: lastTurn.generation, reviewGeneration: lastTurn.reviewGeneration, scope: "last-turn",
		snapshot: reviewdomain.Snapshot{Scope: "last-turn", Comparisons: map[string]reviewdomain.FileComparison{"changed.go": two}},
	}, workspace.FileReader, 10)
	if state.reviewSnapshot.Comparisons["changed.go"] != two || !reflect.DeepEqual(state.ledger, before) {
		t.Fatal("current scope failed to land or mutated receipts")
	}

	refresh := state.reload()
	state, _ = state.landSnapshot(snapshotLoadedMsg{
		generation:       refresh.generation,
		reviewGeneration: refresh.reviewGeneration,
		reviewCapable:    true,
		snapshot:         snapshotOf(repository.Entry{Path: "changed.go", State: repository.FileModified}),
		reviewSnapshot:   reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"changed.go": two}},
	}, workspace.ChangedFiles, workspace.FileReader, 10)
	if !reflect.DeepEqual(state.ledger, before) {
		t.Fatal("world refresh advanced or removed coverage")
	}
}

func TestStaleReaderAndVerificationCannotPaintOrMarkCurrent(t *testing.T) {
	one := testComparison("a.go", "head-1", "old", "one")
	two := testComparison("a.go", "head-1", "old", "two")
	state := loadedFilesState(t, repository.Entry{Path: "a.go", State: repository.FileModified})
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": one}}
	state.readerEntry = repository.Entry{Path: "a.go", State: repository.FileModified}
	state.readerMode = workspace.DiffReader
	state.contentGeneration = 5
	bounds := reviewdomain.Bounds{Old: one.Old, New: one.New}
	state.requestedComparison = &one
	state.requestedBounds = &bounds
	state.reviewSnapshot.Comparisons["a.go"] = two
	document := reviewdomain.BuildDocument(bounds, content(one.Old, "old"), content(one.New, "one"))
	landed := state.landReviewDocument(reviewDocumentLoadedMsg{generation: 5, entry: state.readerEntry, comparison: one, bounds: bounds, document: document}, 10)
	if landed.displayedComparison != nil || len(landed.reviewDocument.Lines) != 0 {
		t.Fatal("stale comparison document painted as current")
	}

	delta := reviewdomain.Delta{Kind: reviewdomain.MarkDelta, Comparison: one, Bounds: bounds}
	verified, _ := state.landReviewVerified(reviewVerifiedMsg{
		generation: state.listGeneration, entry: state.readerEntry, comparison: one, delta: delta, content: content(one.New, "one"),
	})
	if len(verified.ledger.Receipts()) != 0 || !strings.Contains(verified.comparisonWarning, "refresh") {
		t.Fatalf("stale verification = receipts %#v warning %q", verified.ledger.Receipts(), verified.comparisonWarning)
	}

	fileState := loadedFilesState(t, repository.Entry{Path: "a.go", State: repository.FileModified})
	fileState.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": one}}
	fileState.readerEntry = repository.Entry{Path: "a.go", State: repository.FileModified}
	fileState.readerMode = workspace.FileReader
	fileState.contentGeneration = 7
	fileState.requestedComparison = &one
	fileState.requestedBounds = &bounds
	fileState = fileState.landReviewFile(reviewFileLoadedMsg{generation: 7, entry: fileState.readerEntry, comparison: one, content: content(two.New, "two")}, 10)
	fileState.place.Focus = navigation.FocusReader
	if pending := fileState.requestReviewToggle(navigation.FocusReader, -1); pending.kind != effectNone || !strings.Contains(fileState.readerLines()[0].Text, "changed") {
		t.Fatalf("stale File reader was reviewable or painted current: pending=%+v lines=%#v", pending, fileState.readerLines())
	}
	fileState.contentGeneration = 8
	fileState.requestedComparison = &one
	fileState.requestedBounds = &bounds
	fileState = fileState.landReviewFile(reviewFileLoadedMsg{generation: 8, entry: fileState.readerEntry, comparison: one, content: content(one.New, "one")}, 10)
	if pending := fileState.requestReviewToggle(navigation.FocusReader, -1); pending.kind != effectVerifyReview {
		t.Fatalf("exact File reader did not verify review: %+v", pending)
	}
}

func TestPersistenceFailureKeepsLocalQueuedActionAndWarns(t *testing.T) {
	comparison := testComparison("a.go", "head", "old", "new")
	state := newFilesState()
	state.reviewLoaded = true
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": comparison}}
	state.snapshot = snapshotOf(repository.Entry{Path: "a.go", State: repository.FileModified})
	state.entries = state.snapshot.Changed()
	delta := reviewdomain.Delta{Kind: reviewdomain.MarkDelta, Comparison: comparison, Bounds: reviewdomain.Bounds{Old: comparison.Old, New: comparison.New}}
	state, pending := state.landReviewVerified(reviewVerifiedMsg{
		generation: state.listGeneration,
		entry:      state.entries[0],
		comparison: comparison,
		delta:      delta,
		content:    content(comparison.New, "new"),
	})
	if pending.kind != effectPersistReview || state.ledger.Assess(comparison).State != reviewdomain.Reviewed {
		t.Fatalf("local apply = pending %+v state %v", pending, state.ledger.Assess(comparison).State)
	}
	state, _ = state.landReviewPersisted(reviewPersistedMsg{delta: delta, err: errors.New("disk full")})
	if state.ledger.Assess(comparison).State != reviewdomain.Reviewed || !strings.Contains(state.reviewWarning, "will not survive restart") {
		t.Fatalf("failure lost local mark or warning: state=%v warning=%q", state.ledger.Assess(comparison).State, state.reviewWarning)
	}
}

func TestRapidReviewDeltasAdoptMergedStateWithoutLosingQueuedClear(t *testing.T) {
	comparison := testComparison("a.go", "head", "old", "new")
	entry := repository.Entry{Path: "a.go", State: repository.FileModified}
	state := newFilesState()
	state.reviewLoaded = true
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: map[string]reviewdomain.FileComparison{"a.go": comparison}}
	mark := reviewdomain.Delta{Kind: reviewdomain.MarkDelta, Comparison: comparison, Bounds: reviewdomain.Bounds{Old: comparison.Old, New: comparison.New}}
	state, firstPersist := state.landReviewVerified(reviewVerifiedMsg{generation: state.listGeneration, entry: entry, comparison: comparison, delta: mark, content: content(comparison.New, "new")})
	clear := reviewdomain.Delta{Kind: reviewdomain.ClearDelta, Comparison: comparison, Bounds: mark.Bounds}
	state, secondPersist := state.landReviewVerified(reviewVerifiedMsg{generation: state.listGeneration, entry: entry, comparison: comparison, delta: clear, content: content(comparison.New, "new")})
	if firstPersist.kind != effectPersistReview || secondPersist.kind != effectNone || state.ledger.Assess(comparison).State != reviewdomain.Unreviewed || len(state.reviewQueue) != 2 {
		t.Fatalf("rapid local queue = first %+v second %+v state=%v queue=%d", firstPersist, secondPersist, state.ledger.Assess(comparison).State, len(state.reviewQueue))
	}
	merged := reviewdomain.Ledger{}
	_ = mark.Apply(&merged)
	state, nextPersist := state.landReviewPersisted(reviewPersistedMsg{delta: mark, ledger: merged})
	if state.ledger.Assess(comparison).State != reviewdomain.Unreviewed || nextPersist.kind != effectPersistReview || nextPersist.delta.Kind != reviewdomain.ClearDelta {
		t.Fatalf("merged adoption lost queued clear: state=%v next=%+v", state.ledger.Assess(comparison).State, nextPersist)
	}
	cleared := merged.Clone()
	_ = clear.Apply(&cleared)
	state, _ = state.landReviewPersisted(reviewPersistedMsg{delta: clear, ledger: cleared})
	if state.ledger.Assess(comparison).State != reviewdomain.Unreviewed || len(state.reviewQueue) != 0 || state.reviewPersisting {
		t.Fatalf("final queued state = %v queue=%d persisting=%v", state.ledger.Assess(comparison).State, len(state.reviewQueue), state.reviewPersisting)
	}
}

func TestMissingReviewProviderLeavesRowsAndActionsReviewInert(t *testing.T) {
	model := newTestModel(&fakeSource{snapshot: snapshotOf(repository.Entry{Path: "a.go", State: repository.FileModified})})
	model.geometry = ui.Calculate(80, 20)
	next, command := model.Update(model.command(effect{kind: effectLoadSnapshot, generation: model.files.listGeneration, scope: "uncommitted"})())
	model = next.(Model)
	if command == nil {
		t.Fatal("ordinary reader did not load")
	}
	rows := model.files.viewModel(model.geometry).NavigatorRows
	if len(rows) != 1 || rows[0].Review != nil {
		t.Fatalf("missing provider advertised review: %#v", rows)
	}
	if pending := model.apply(Action{Kind: ToggleReview, Index: -1}); pending.kind != effectNone || len(model.files.ledger.Receipts()) != 0 {
		t.Fatalf("missing provider review action = %+v receipts=%#v", pending, model.files.ledger.Receipts())
	}
}

func BenchmarkReviewRefreshDerivation1000(b *testing.B) {
	state := newFilesState()
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: make(map[string]reviewdomain.FileComparison, 1000)}
	for index := 0; index < 1000; index++ {
		path := fmt.Sprintf("src/file-%04d.go", index)
		comparison := testComparison(path, "head", path+":old", path+":new")
		state.reviewSnapshot.Comparisons[path] = comparison
		if index%2 == 0 {
			_ = state.ledger.Mark(comparison, reviewdomain.Bounds{Old: comparison.Old, New: comparison.New}, nil)
		}
	}
	b.ResetTimer()
	for range b.N {
		state.rederiveReviews()
	}
}

func BenchmarkReviewNavigatorSteadyState1000(b *testing.B) {
	entries := make([]repository.Entry, 1000)
	for index := range entries {
		entries[index] = repository.Entry{Path: fmt.Sprintf("src/file-%04d.go", index), State: repository.FileModified}
	}
	state := newFilesState()
	state, _ = state.landSnapshot(snapshotLoadedMsg{generation: state.listGeneration, snapshot: snapshotOf(entries...)}, workspace.AllFiles, workspace.FileReader, 40)
	if !state.tree.Expand(filetree.DirectoryIdentity("src")) {
		b.Fatal("benchmark fixture did not expand its collapsed 1,000-file directory")
	}
	state.reconcileVisibleRows(40)
	state.reviewSnapshot = reviewdomain.Snapshot{Scope: "uncommitted", Comparisons: make(map[string]reviewdomain.FileComparison, len(entries))}
	for _, entry := range entries {
		state.reviewSnapshot.Comparisons[entry.Path] = testComparison(entry.Path, "head", entry.Path+":old", entry.Path+":new")
	}
	state.rederiveReviews()
	geometry := ui.Calculate(120, 40)
	b.ResetTimer()
	for range b.N {
		_ = state.viewModel(geometry)
	}
}
