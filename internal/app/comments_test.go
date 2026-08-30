package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/comments"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestVisualLineSelectionGrowsShrinksReversesAndCancelsToAnchor(t *testing.T) {
	t.Parallel()
	model := commentReaderModel(fileCommentDocument("one", "two", "three", "four", "five", "six"), workspace.FileReader)
	model.files.place.ReaderCursor = 2

	model.apply(Action{Kind: StartVisualLine})
	if selection := model.files.visualSelection; selection == nil || selection.Anchor.Number != 3 || selection.Active.Number != 3 {
		t.Fatalf("entered Visual line = %+v", selection)
	}
	model.apply(Action{Kind: MoveReaderSelection, Amount: 2})
	if got := model.files.visualSelection.Active.Number; got != 5 {
		t.Fatalf("grown active line = %d, want 5", got)
	}
	model.apply(Action{Kind: MoveReaderSelection, Amount: -1})
	if got := model.files.visualSelection.Active.Number; got != 4 {
		t.Fatalf("shrunk active line = %d, want 4", got)
	}
	model.apply(Action{Kind: MoveReaderSelection, Amount: -2})
	selection := model.files.visualSelection
	if selection.Anchor.Number != 3 || selection.Active.Number != 2 || model.files.place.ReaderCursor != 1 {
		t.Fatalf("reversed Visual line = %+v cursor=%d", selection, model.files.place.ReaderCursor)
	}
	selected := make([]uint64, 0)
	for _, row := range model.files.readerDocument().Rows {
		if row.VisualSelected {
			selected = append(selected, row.NewLine)
		}
	}
	if !reflect.DeepEqual(selected, []uint64{2, 3}) {
		t.Fatalf("painted inclusive range = %v", selected)
	}

	model.apply(Action{Kind: CancelVisualLine})
	if model.files.visualSelection != nil || model.files.place.ReaderCursor != 2 {
		t.Fatalf("cancel = selection %+v cursor %d, want anchor row 2", model.files.visualSelection, model.files.place.ReaderCursor)
	}
}

func TestVisualRangeCommentCapturesHybridAnchorOutsideHunksAndAcrossNewContext(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "context:1", Kind: ui.ReaderContext, Text: "before one", OldLine: 1, NewLine: 1},
		{Identity: "context:2", Kind: ui.ReaderContext, Text: "before two", OldLine: 2, NewLine: 2},
		{Identity: "add:3", Kind: ui.ReaderInsertion, Text: "changed", NewLine: 3},
		{Identity: "context:4", Kind: ui.ReaderContext, Text: "after", OldLine: 3, NewLine: 4},
	}}
	model := commentReaderModel(document, workspace.DiffReader)

	// Expanded unchanged context is a first-class anchor wholly before a hunk.
	model.files.place.ReaderCursor = 0
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: 1})
	model.apply(Action{Kind: ComposeComment})
	draft := model.files.commentDraft
	if draft == nil || draft.Range.Side != comments.NewSide || draft.Range.Start.Number != 1 || draft.Range.End.Number != 2 ||
		draft.PreferredLine != 2 || draft.Snippet != " before one\n before two" || draft.SourceIdentity == "" || draft.FileIdentity == "" {
		t.Fatalf("outside-hunk draft = %+v", draft)
	}
	model.apply(Action{Kind: CommentInsert, Text: "This context is important."})
	model.apply(Action{Kind: CommentSubmit})
	items := model.files.comments.Items()
	if len(items) != 1 || items[0].Range.Start.Number != 1 || items[0].Range.End.Number != 2 {
		t.Fatalf("saved outside-hunk comment = %+v", items)
	}
	wantExport := "main.go:1-2\n before one\n before two\nThis context is important."
	if got := comments.FormatAll(items); got != wantExport {
		t.Fatalf("self-contained export = %q, want %q", got, wantExport)
	}

	// A pure addition boundary remains one coherent new side.
	model.files.place.ReaderCursor = 1
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: 1})
	selection := model.files.visualSelection
	if selection.Side != comments.NewSide || selection.Anchor.Number != 2 || selection.Active.Number != 3 {
		t.Fatalf("context/addition range = %+v", selection)
	}
}

func TestVisualDiffSideStopsAtMixedDeletionAdditionBoundary(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "delete", Kind: ui.ReaderDeletion, Text: "old", OldLine: 8},
		{Identity: "insert", Kind: ui.ReaderInsertion, Text: "new", NewLine: 9},
		{Identity: "context", Kind: ui.ReaderContext, Text: "tail", OldLine: 9, NewLine: 10},
	}}
	model := commentReaderModel(document, workspace.DiffReader)
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: 2})
	if selection := model.files.visualSelection; selection.Side != comments.OldSide || selection.Active.Number != 8 || model.files.place.ReaderCursor != 0 {
		t.Fatalf("old-side selection crossed replacement boundary: %+v cursor=%d", selection, model.files.place.ReaderCursor)
	}
	model.apply(Action{Kind: ComposeComment})
	if draft := model.files.commentDraft; draft == nil || draft.Range.Side != comments.OldSide || draft.Snippet != "-old" {
		t.Fatalf("old-side draft = %+v", draft)
	}
	model.apply(Action{Kind: CommentCancel})

	model.files.place.ReaderCursor = 1
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: -1})
	if selection := model.files.visualSelection; selection.Side != comments.NewSide || selection.Active.Number != 9 || model.files.place.ReaderCursor != 1 {
		t.Fatalf("new-side selection crossed replacement boundary: %+v cursor=%d", selection, model.files.place.ReaderCursor)
	}
}

func TestVisualHunkJumpsMoveTheActiveEndpoint(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "hunk:1", Kind: ui.ReaderMetadata, Text: "@@ -1 +1 @@"},
		{Identity: "add:1", Kind: ui.ReaderInsertion, Text: "one", NewLine: 1},
		{Identity: "context:2", Kind: ui.ReaderContext, Text: "two", OldLine: 2, NewLine: 2},
		{Identity: "hunk:2", Kind: ui.ReaderMetadata, Text: "@@ -3 +3 @@"},
		{Identity: "add:3", Kind: ui.ReaderInsertion, Text: "three", NewLine: 3},
	}}
	model := commentReaderModel(document, workspace.DiffReader)
	model.files.place.ReaderCursor = 1
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: SelectNextLandmark})
	if selection := model.files.visualSelection; selection.Active.Number != 3 || model.files.place.ReaderCursor != 4 {
		t.Fatalf("next hunk Visual endpoint = %+v cursor=%d", selection, model.files.place.ReaderCursor)
	}
	model.apply(Action{Kind: SelectPreviousLandmark})
	if selection := model.files.visualSelection; selection.Active.Number != 1 || model.files.place.ReaderCursor != 1 {
		t.Fatalf("previous hunk Visual endpoint = %+v cursor=%d", selection, model.files.place.ReaderCursor)
	}
}

func TestVisualAnchorSurvivesWrappingAndCollapsedContext(t *testing.T) {
	t.Parallel()
	model := commentReaderModel(foldableDiffDocument(), workspace.DiffReader)
	changed, animating := model.files.setReaderContextExpanded(true)
	if !changed {
		t.Fatal("context did not expand")
	}
	for animating {
		_, animating = model.files.advanceReaderContext(model.files.readerContext.generation)
	}
	expanded := model.files.readerDocument()
	anchor := readerIdentityIndex(expanded.Rows, "context:20")
	if anchor < 0 {
		t.Fatal("expanded context line is not visible")
	}
	model.files.place.ReaderCursor = anchor
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: 1})
	want := *model.files.visualSelection
	_ = ui.CalculateReaderLayout(ui.Rect{Width: 18, Height: 5}, model.files.readerDocument())
	_ = ui.CalculateReaderLayout(ui.Rect{Width: 90, Height: 20}, model.files.readerDocument())

	_, animating = model.files.setReaderContextExpanded(false)
	for animating {
		_, animating = model.files.advanceReaderContext(model.files.readerContext.generation)
	}
	if got := model.files.visualSelection; got == nil || *got != want {
		t.Fatalf("fold/wrap changed stable Visual anchor: got %+v want %+v", got, want)
	}
	model.apply(Action{Kind: ComposeComment})
	if draft := model.files.commentDraft; draft == nil || draft.Range.Start.Number != 20 || draft.Range.End.Number != 21 ||
		draft.Snippet != " context 20\n context 21" {
		t.Fatalf("comment from folded selection = %+v", draft)
	}
}

func TestVisualSelectionReconcilesStableSourceIdentitiesAfterAgentEdit(t *testing.T) {
	t.Parallel()
	oldDocument := fileCommentDocument("before", "first", "second", "after")
	model := commentReaderModel(oldDocument, workspace.FileReader)
	model.files.place.ReaderCursor = 1
	model.apply(Action{Kind: StartVisualLine})
	model.apply(Action{Kind: MoveReaderSelection, Amount: 1})
	oldRaw := model.files.rawReaderDocument()

	newDocument := fileCommentDocument("inserted", "before", "first", "second", "after")
	model.files.readerPresentation = &newDocument
	model.files.reconcileCommentInteraction(oldRaw)
	selection := model.files.visualSelection
	if selection == nil || selection.Anchor.Number != 3 || selection.Active.Number != 4 ||
		selection.Anchor.Identity == "" || selection.Active.Identity == "" {
		t.Fatalf("agent-edit Visual reconciliation = %+v", selection)
	}
	model.apply(Action{Kind: ComposeComment})
	if draft := model.files.commentDraft; draft == nil || draft.Range.Start.Number != 3 || draft.Range.End.Number != 4 ||
		draft.Snippet != " first\n second" {
		t.Fatalf("reconciled Visual comment = %+v", draft)
	}
}

func TestInlineCommentHeaderFoldAndRefreshContinuityByIdentity(t *testing.T) {
	t.Parallel()
	oldDocument := fileCommentDocument("before", "target", "after")
	model := commentReaderModel(oldDocument, workspace.FileReader)
	model.files.snapshot = repository.NewComparisonSnapshot(
		[]repository.Entry{{Path: "main.go"}},
		repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: "base", Target: "tree-before"},
	)
	model.files.place.ReaderCursor = 1
	model.apply(Action{Kind: ComposeComment})
	model.apply(Action{Kind: CommentInsert, Text: "line one\nline two"})
	model.apply(Action{Kind: CommentSubmit})
	comment := model.files.comments.Items()[0]
	header := readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":header")
	if header < 0 || !model.files.readerDocument().Rows[header].Selectable() {
		t.Fatalf("saved comment header is not a selectable row: %d", header)
	}
	model.files.place.ReaderCursor = header
	model.files.place.ReaderOffset = header
	model.apply(Action{Kind: CollapseReaderFold})
	if model.files.commentExpanded(comment.ID) || model.files.place.ReaderCursor != readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":header") {
		t.Fatalf("collapsed card lost header place: cursor=%d rows=%+v", model.files.place.ReaderCursor, model.files.readerDocument().Rows)
	}

	oldRaw := model.files.rawReaderDocument()
	oldRows := readerRowIdentities(model.files.readerRows())
	oldOffset, oldCursor := model.files.place.ReaderOffset, model.files.place.ReaderCursor
	newDocument := fileCommentDocument("inserted", "before", "target", "after")
	model.files.snapshot = repository.NewComparisonSnapshot(
		[]repository.Entry{{Path: "main.go"}},
		repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: "base", Target: "tree-after"},
	)
	model.files.readerPresentation = &newDocument
	model.files.reconcileCommentInteraction(oldRaw)
	model.files.reconcileReaderPlace(oldRows, oldOffset, oldCursor)

	item := model.files.comments.Items()[0]
	header = readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":header")
	if item.Stale || item.Range.Start.Number != 3 || item.Range.End.Number != 3 ||
		model.files.place.ReaderCursor != header || model.files.place.ReaderOffset != header || model.files.commentExpanded(comment.ID) {
		t.Fatalf("refresh continuity = item %+v header=%d place=%+v expanded=%v", item, header, model.files.place, model.files.commentExpanded(comment.ID))
	}

	model.apply(Action{Kind: ExpandReaderFold})
	if !model.files.commentExpanded(comment.ID) || readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":body:1") < 0 {
		t.Fatalf("expanded refreshed card rows = %+v", model.files.readerDocument().Rows)
	}

	missing := ui.ReaderDocument{}
	oldRaw = model.files.rawReaderDocument()
	model.files.readerPresentation = &missing
	model.files.reconcileCommentInteraction(oldRaw)
	items := model.files.comments.Items()
	if len(items) != 1 || !items[0].Stale {
		t.Fatalf("missing file discarded or silently resolved comment: %+v", items)
	}
}

func TestOldSideCommentRemainsBoundToItsImmutableBasis(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{{
		Identity: "removed", Kind: ui.ReaderDeletion, Text: "gone", OldLine: 8,
	}}}
	model := commentReaderModel(document, workspace.DiffReader)
	setComparison := func(basis, target string) {
		model.files.snapshot = repository.NewComparisonSnapshot(
			[]repository.Entry{{Path: "main.go"}},
			repository.Comparison{Scope: repository.ComparisonUncommitted, Basis: basis, Target: target},
		)
	}
	setComparison("base-one", "target-one")
	model.apply(Action{Kind: ComposeComment})
	model.apply(Action{Kind: CommentInsert, Text: "still required"})
	model.apply(Action{Kind: CommentSubmit})
	comment := model.files.comments.Items()[0]

	setComparison("base-one", "target-two")
	if readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":header") < 0 {
		t.Fatal("new-side refresh hid an old-side comment from the same basis")
	}
	setComparison("base-two", "target-two")
	if readerIdentityIndex(model.files.readerDocument().Rows, comment.ID+":header") >= 0 || model.files.comments.Len() != 1 {
		t.Fatalf("basis change retargeted or discarded old comment: rows=%+v comments=%+v", model.files.readerDocument().Rows, model.files.comments.Items())
	}
}

func TestSettingsPolicyNavigatesMixedHunkAndCommentLandmarks(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "hunk:1", Kind: ui.ReaderMetadata, Text: "@@ -1 +1 @@"},
		{Identity: "add:1", Kind: ui.ReaderInsertion, Text: "one", NewLine: 1},
		{Identity: "comment:1:header", Kind: ui.ReaderCommentHeader, Text: "main.go:1", CommentID: "comment:1"},
		{Identity: "context", Kind: ui.ReaderContext, Text: "middle", OldLine: 2, NewLine: 2},
		{Identity: "hunk:2", Kind: ui.ReaderMetadata, Text: "@@ -3 +3 @@"},
		{Identity: "add:3", Kind: ui.ReaderInsertion, Text: "three", NewLine: 3},
		{Identity: "comment:2:header", Kind: ui.ReaderCommentHeader, Text: "main.go:3", CommentID: "comment:2"},
	}}
	model := commentReaderModel(document, workspace.DiffReader)
	landmarks := readerNavigationLandmarks(model.files.readerDocument())
	if len(landmarks) != 4 || landmarks[1].kind != readerCommentLandmark || landmarks[1].identity != "comment:1" {
		t.Fatalf("mixed landmark stream = %+v", landmarks)
	}

	model.files.place.ReaderCursor = 0
	model.apply(Action{Kind: SelectNextLandmark})
	if model.files.place.ReaderCursor != 2 {
		t.Fatalf("enabled next landmark = %d, want comment header 2", model.files.place.ReaderCursor)
	}
	model.apply(Action{Kind: SelectNextLandmark})
	if model.files.place.ReaderCursor != 4 {
		t.Fatalf("enabled second landmark = %d, want hunk 4", model.files.place.ReaderCursor)
	}

	model.settings.includeCommentsInHunkNavigation = false
	model.files.place.ReaderCursor = 0
	model.apply(Action{Kind: SelectNextLandmark})
	if model.files.place.ReaderCursor != 4 {
		t.Fatalf("disabled next landmark = %d, want unchanged hunk 4", model.files.place.ReaderCursor)
	}
	model.files.place.ReaderCursor = 6
	model.apply(Action{Kind: SelectPreviousLandmark})
	if model.files.place.ReaderCursor != 4 {
		t.Fatalf("disabled previous landmark = %d, want hunk 4", model.files.place.ReaderCursor)
	}
}

func TestGutterHoverAndClickUseSharedGeometryWithoutMovingPlace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		row  ui.ReaderRow
		side comments.Side
		line uint64
	}{
		{name: "removed old side", row: ui.ReaderRow{Identity: "old", Kind: ui.ReaderDeletion, Text: "removed", OldLine: 8}, side: comments.OldSide, line: 8},
		{name: "added new side", row: ui.ReaderRow{Identity: "new", Kind: ui.ReaderInsertion, Text: "added", NewLine: 9}, side: comments.NewSide, line: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
				{Identity: "other", Kind: ui.ReaderContext, Text: "other", OldLine: 1, NewLine: 1}, test.row,
			}}
			model := commentReaderModel(document, workspace.DiffReader)
			model.files.place.ReaderCursor = 0
			model.files.place.ReaderOffset = 0
			layout, ok := model.activeReaderLayout()
			if !ok {
				t.Fatal("reader has no layout")
			}
			y := layout.Geometry.Rows.Y + layout.VisualOffset(1, 0)
			x := layout.Geometry.LineNumber.X
			before := model.files.place
			next, command := model.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y}))
			model = next.(Model)
			if command != nil || !reflect.DeepEqual(model.files.place, before) || model.files.commentHover == nil ||
				model.files.commentHover.Side != test.side || model.files.commentHover.Line.Number != test.line {
				t.Fatalf("hover = place %+v command=%v hover=%+v", model.files.place, command != nil, model.files.commentHover)
			}
			if !strings.Contains(ansi.Strip(model.View().Content), "[+]") {
				t.Fatal("hovered gutter did not render [+]")
			}

			// A refresh inserts presentation chrome, then scroll/swap/resize all
			// recompute hit geometry while the semantic hover identity survives.
			oldRaw := model.files.rawReaderDocument()
			refreshed := document
			refreshed.Rows = append([]ui.ReaderRow{{Identity: "notice", Kind: ui.ReaderNotice, Text: "refreshed"}}, refreshed.Rows...)
			model.files.readerPresentation = &refreshed
			model.files.reconcileCommentInteraction(oldRaw)
			model.files.place.ReaderOffset = 1
			model.apply(Action{Kind: SwapPanes})
			model.apply(Action{Kind: Resize, Width: 112, Height: 18})
			if model.files.commentHover == nil || model.files.commentHover.Line.Number != test.line {
				t.Fatalf("layout change lost hover identity: %+v", model.files.commentHover)
			}
			layout, _ = model.activeReaderLayout()
			target := 2
			y = layout.Geometry.Rows.Y + layout.VisualOffset(target, 0) - model.activeReaderVisualOffset()
			x = layout.Geometry.LineNumber.X
			next, _ = model.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
			model = next.(Model)
			if draft := model.files.commentDraft; draft == nil || draft.Range.Side != test.side || draft.Range.Start.Number != test.line ||
				draft.Range.End.Number != test.line || model.files.place.ReaderCursor != target || model.files.place.Focus != navigation.FocusReader {
				t.Fatalf("gutter click = draft %+v place %+v", draft, model.files.place)
			}
		})
	}
}

func TestCommentKeyRoutesThroughSemanticActionsAndComposerOwnsEscape(t *testing.T) {
	t.Parallel()
	context := browserRouteContext{
		active: workspace.Files, focus: navigation.FocusReader, readerCommentable: true,
	}
	for key, want := range map[string]ActionKind{"V": StartVisualLine, "c": ComposeComment} {
		action, ok := routeBrowserMessage(tea.KeyPressMsg(tea.Key{Code: rune(key[0]), Text: key}), context)
		if !ok || action.Kind != want {
			t.Fatalf("%q route = (%+v, %v), want %v", key, action, ok, want)
		}
	}
	context.visualSelecting = true
	if action, ok := routeBrowserMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), context); !ok || action.Kind != CancelVisualLine {
		t.Fatalf("Visual Escape = (%+v, %v)", action, ok)
	}
	if action, ok := routeCommentInput(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})); !ok || action.Kind != CommentCancel {
		t.Fatalf("composer Escape = (%+v, %v)", action, ok)
	}
	if action, ok := routeCommentInput(tea.WindowSizeMsg{Width: 88, Height: 22}); !ok || action != (Action{Kind: Resize, Width: 88, Height: 22}) {
		t.Fatalf("composer resize = (%+v, %v)", action, ok)
	}
}

func commentReaderModel(document ui.ReaderDocument, mode workspace.ReaderMode) Model {
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.active = workspace.Files
	model.controls.Reader = mode
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = mode
	model.files.readerPresentation = &document
	model.files.place.Focus = navigation.FocusReader
	return model
}

func fileCommentDocument(lines ...string) ui.ReaderDocument {
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: make([]ui.ReaderRow, len(lines))}
	for index, line := range lines {
		document.Rows[index] = ui.ReaderRow{
			Identity: line, Kind: ui.ReaderFile, Text: line, NewLine: uint64(index + 1),
		}
	}
	return document
}
