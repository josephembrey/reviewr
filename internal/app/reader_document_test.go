package app

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestUnifiedDiffDocumentParsesMultipleHunksAndOmittedCounts(t *testing.T) {
	t.Parallel()
	document := unifiedDiffDocument("main.go", strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-const first = 1",
		"+const first = 2",
		"@@ -10,2 +20,3 @@ func second()",
		" keep",
		"-gone",
		"+addedOne",
		"+addedTwo",
		`\ No newline at end of file`,
	}, "\n"))

	var code []ui.ReaderRow
	for _, row := range document.Rows {
		switch row.Kind {
		case ui.ReaderFile, ui.ReaderContext, ui.ReaderInsertion, ui.ReaderDeletion:
			code = append(code, row)
		default:
			if row.OldLine != 0 || row.NewLine != 0 || row.DisplayLine() != 0 {
				t.Fatalf("metadata row fabricated a line identity: %+v", row)
			}
		}
	}
	want := []ui.ReaderRow{
		{Kind: ui.ReaderDeletion, Text: "const first = 1", OldLine: 1},
		{Kind: ui.ReaderInsertion, Text: "const first = 2", NewLine: 1},
		{Kind: ui.ReaderContext, Text: "keep", OldLine: 10, NewLine: 20},
		{Kind: ui.ReaderDeletion, Text: "gone", OldLine: 11},
		{Kind: ui.ReaderInsertion, Text: "addedOne", NewLine: 21},
		{Kind: ui.ReaderInsertion, Text: "addedTwo", NewLine: 22},
	}
	if len(code) != len(want) {
		t.Fatalf("code rows = %#v", code)
	}
	for index := range want {
		got := code[index]
		got.Identity = ""
		got.Spans = nil
		if !reflect.DeepEqual(got, want[index]) {
			t.Fatalf("code row %d = %+v, want %+v", index, got, want[index])
		}
	}
	if !document.DiffEligible() {
		t.Fatal("multi-hunk document is not diff-highlight eligible")
	}
}

func TestUnifiedDiffPayloadIsSafeAndNeverOwnsPresentationMarkers(t *testing.T) {
	t.Parallel()
	document := unifiedDiffDocument("main.go", "@@ -1 +1 @@\n-\x1b[31m--old\n+\x1b[32m++new")
	if len(document.Rows) != 3 {
		t.Fatalf("rows = %#v", document.Rows)
	}
	removed, added := document.Rows[1], document.Rows[2]
	if removed.Kind != ui.ReaderDeletion || added.Kind != ui.ReaderInsertion {
		t.Fatalf("semantic kinds = %v/%v", removed.Kind, added.Kind)
	}
	for _, row := range []ui.ReaderRow{removed, added} {
		if strings.ContainsRune(row.Text, '\x1b') || strings.Contains(readerSpanText(row), "\x1b") {
			t.Fatalf("hostile escape survived sanitization: %+v", row)
		}
		if len(row.Spans) != 0 && readerSpanText(row) != row.Text {
			t.Fatalf("syntax spans changed payload: %+v", row)
		}
	}
	if removed.Text != "␛[31m--old" || added.Text != "␛[32m++new" {
		t.Fatalf("marker stripping/safety = %q / %q", removed.Text, added.Text)
	}
}

func TestEveryRichReaderSourceUsesSemanticRows(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/main.go b/main.go\n@@ -3 +7 @@\n-old\n+new"
	files := diffReaderDocument(repository.Diff{
		Entry: repository.Entry{Path: "main.go"}, Kind: repository.DiffReady, Content: patch,
	})
	stash := changeDiffDocument(repository.ChangeDocument{
		Change: repository.ChangedFile{Path: "main.go"},
		Patch:  repository.File{Path: "main.go", Kind: repository.FileReady, Content: patch},
	})
	incremental := reviewReaderDocument("main.go", review.Document{Exact: true, Lines: []review.Line{
		{Identity: "old", Text: "- old", Kind: review.RemovedLine, OldLine: 30},
		{Identity: "new", Text: "+ new", Kind: review.AddedLine, NewLine: 40},
	}})
	for name, document := range map[string]ui.ReaderDocument{
		"Files comparison":   files,
		"Stash":              stash,
		"review incremental": incremental,
	} {
		if !document.DiffEligible() {
			t.Fatalf("%s document is not eligible: %+v", name, document)
		}
		for _, row := range document.Rows {
			if row.Kind == ui.ReaderInsertion || row.Kind == ui.ReaderDeletion {
				if strings.HasPrefix(row.Text, "+") || strings.HasPrefix(row.Text, "-") {
					t.Fatalf("%s row retained marker: %+v", name, row)
				}
			}
		}
	}
}

func TestFileReaderNumbersNewSideFromOneAndNoticesStayUnnumbered(t *testing.T) {
	t.Parallel()
	document := fileReaderDocument(repository.File{
		Path: "main.go", Kind: repository.FileReady, Content: "package main\nfunc main() {}",
	}, repository.Entry{})
	if document.Kind != ui.ReaderFileDocument || len(document.Rows) != 2 ||
		document.Rows[0].Kind != ui.ReaderFile || document.Rows[0].NewLine != 1 ||
		document.Rows[1].NewLine != 2 {
		t.Fatalf("file reader document = %+v", document)
	}
	notice := fileReaderDocument(repository.File{Kind: repository.FileBinary, Size: 12}, repository.Entry{})
	if len(notice.Rows) != 1 || notice.Rows[0].Kind != ui.ReaderNotice || notice.Rows[0].DisplayLine() != 0 {
		t.Fatalf("notice document = %+v", notice)
	}
}

func TestDiffHighlightToggleIsGlobalRenderOnlyAndEligibilityIsVisibleDocument(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	presentation := unifiedDiffDocument("main.go", "@@ -1 +1 @@\n-old\n+new")
	model.files.readerPresentation = &presentation
	model.files.place = navigation.State{Items: []string{"main.go"}, Focus: navigation.FocusReader, ReaderOffset: 1}

	beforePlace := model.files.place
	beforeLayout := model.layout
	beforePresentation := model.files.readerPresentation
	beforeGeneration := model.files.contentGeneration
	if !model.diffHighlightEligible() || !model.presentationControls().RichDiff {
		t.Fatal("visible Files diff is not eligible")
	}
	pending := model.apply(Action{Kind: ToggleDiffHighlight})
	if pending.kind != effectNone || model.controls.DiffHighlight != workspace.DiffHighlightBackground {
		t.Fatalf("toggle = controls %+v effect %+v", model.controls, pending)
	}
	if !reflect.DeepEqual(model.files.place, beforePlace) || model.layout != beforeLayout ||
		model.files.readerPresentation != beforePresentation || model.files.contentGeneration != beforeGeneration {
		t.Fatalf("render-only toggle changed place/content: model=%+v", model)
	}

	model.controls.Reader = workspace.FileReader
	if model.diffHighlightEligible() {
		t.Fatal("File mode remained eligible")
	}
	model.apply(Action{Kind: ToggleDiffHighlight})
	if model.controls.DiffHighlight != workspace.DiffHighlightBackground {
		t.Fatal("ineligible semantic action changed preference")
	}
	model.controls.Reader = workspace.DiffReader
	model.active = workspace.Notes
	if model.diffHighlightEligible() {
		t.Fatal("Notes exposed diff highlight")
	}
	model.active = workspace.Files
	model.active = workspace.Git
	model.controls.Git = workspace.GitLog
	if model.diffHighlightEligible() {
		t.Fatal("Git Log exposed diff highlight")
	}
	model.controls.Git = workspace.GitStashes
	model.stashes.readerFileID = "main.go"
	model.stashes.reader = repository.ChangeDocument{Change: repository.ChangedFile{Path: "main.go"}}
	model.stashes.readerPresentation = &presentation
	if !model.diffHighlightEligible() {
		t.Fatal("Stash diff did not inherit global eligibility")
	}
}

func TestRichReaderScrollTraversesWrappedVisualRows(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 10})
	model.active = workspace.Files
	model.files.readerEntry = repository.Entry{Path: "long.go"}
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: []ui.ReaderRow{{
		Identity: "line:1", Kind: ui.ReaderFile, Text: strings.Repeat("x", 1_000), NewLine: 1,
	}}}
	model.files.readerPresentation = &document
	layout := ui.CalculateReaderLayout(model.geometry.ReaderRows, document)
	if layout.Total <= model.geometry.ReaderRows.Height {
		t.Fatalf("fixture did not overflow: total %d height %d", layout.Total, model.geometry.ReaderRows.Height)
	}

	model.apply(Action{Kind: ScrollReader, Amount: 1})
	if model.files.place.ReaderOffset != 0 || model.files.place.ReaderColumn != layout.Geometry.Code.Width || model.activeReaderVisualOffset() != 1 {
		t.Fatalf("first wrapped scroll = %+v visual=%d", model.files.place, model.activeReaderVisualOffset())
	}
	model.apply(Action{Kind: ScrollReader, Amount: 10_000})
	want := layout.Total - model.geometry.ReaderRows.Height
	if got := model.activeReaderVisualOffset(); got != want {
		t.Fatalf("wrapped scroll bottom = %d, want %d; place=%+v", got, want, model.files.place)
	}
}

func TestReaderViewportLayoutSurvivesScrollAndInvalidatesOnResize(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 24})
	model.active = workspace.Files
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	for line := uint64(1); line <= 200; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Kind: ui.ReaderFile, NewLine: line,
			Text: "a reader line long enough to exercise the shared wrapped layout",
		})
	}
	model.files.readerPresentation = &document
	model.files.readerEntry = repository.Entry{Path: "large.go"}

	presented := model.files.readerDocument()
	model.clampDocumentReader(&model.files.place, presented)
	before, ok := model.cachedActiveReaderViewport()
	if !ok {
		t.Fatal("reader layout was not retained after clamping")
	}
	model.apply(Action{Kind: ScrollReader, Amount: 3})
	after, ok := model.cachedActiveReaderViewport()
	if !ok || after.key != before.key || model.files.place.ReaderOffset == 0 {
		t.Fatalf("scroll invalidated viewport: cached=%v offset=%d", ok, model.files.place.ReaderOffset)
	}

	model.apply(Action{Kind: Resize, Width: 120, Height: 24})
	resized, ok := model.cachedActiveReaderViewport()
	if !ok || resized.key == before.key || resized.layout.Geometry.Rows != model.geometry.ReaderRows {
		t.Fatalf("reader layout was not rebuilt at the new width: cached=%v geometry=%+v", ok, resized.layout.Geometry.Rows)
	}
}

func TestReaderSelectionMovesByLogicalLineAndWheelScrollStaysIndependent(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 12})
	model.active = workspace.Files
	model.files.readerEntry = repository.Entry{Path: "large.go"}
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	for line := 1; line <= 80; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("line:%d", line), Kind: ui.ReaderFile,
			Text: fmt.Sprintf("line %d", line), NewLine: uint64(line),
		})
	}
	model.files.readerPresentation = &document
	model.files.place.Focus = navigation.FocusReader

	model.apply(Action{Kind: MoveReaderSelection, Amount: 30})
	if model.files.place.ReaderCursor != 30 || model.activeReaderVisualOffset() == 0 {
		t.Fatalf("selection did not move into view: place=%+v", model.files.place)
	}
	top := model.activeReaderVisualOffset()
	model.apply(Action{Kind: ScrollReader, Amount: 3})
	if model.files.place.ReaderCursor != 30 || model.activeReaderVisualOffset() != top+3 {
		t.Fatalf("wheel-style scroll moved selection or wrong viewport: place=%+v visual=%d", model.files.place, model.activeReaderVisualOffset())
	}
	model.apply(Action{Kind: MoveReaderSelection, Amount: -1})
	if model.files.place.ReaderCursor != 29 {
		t.Fatalf("reverse selection = %d, want 29", model.files.place.ReaderCursor)
	}
}

func TestReaderVimJumpsMoveCursorAndViewport(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 16})
	model.active = workspace.Files
	model.files.readerEntry = repository.Entry{Path: "large.go"}
	model.files.place.Focus = navigation.FocusReader
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	for line := 1; line <= 80; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("line:%d", line), Kind: ui.ReaderFile,
			Text: fmt.Sprintf("line %d", line), NewLine: uint64(line),
		})
	}
	model.files.readerPresentation = &document

	model.apply(Action{Kind: SelectReaderBoundary, Amount: 1})
	if model.files.place.ReaderCursor != len(document.Rows)-1 || model.activeReaderVisualOffset() == 0 {
		t.Fatalf("G place = %+v", model.files.place)
	}
	model.apply(Action{Kind: SelectReaderBoundary, Amount: -1})
	if model.files.place.ReaderCursor != 0 || model.activeReaderVisualOffset() != 0 {
		t.Fatalf("gg place = %+v", model.files.place)
	}

	height := model.geometry.ReaderRows.Height
	model.setActiveReaderVisualOffset(20)
	model.apply(Action{Kind: SelectReaderViewport, Amount: -1})
	if model.files.place.ReaderCursor != 20 {
		t.Fatalf("H cursor = %d, want 20", model.files.place.ReaderCursor)
	}
	model.apply(Action{Kind: SelectReaderViewport})
	if want := 20 + (height-1)/2; model.files.place.ReaderCursor != want {
		t.Fatalf("M cursor = %d, want %d", model.files.place.ReaderCursor, want)
	}
	model.apply(Action{Kind: SelectReaderViewport, Amount: 1})
	if want := 20 + height - 1; model.files.place.ReaderCursor != want {
		t.Fatalf("L cursor = %d, want %d", model.files.place.ReaderCursor, want)
	}

	model.files.place.ReaderCursor = 30
	model.setActiveReaderVisualOffset(25)
	delta := max(1, height/2)
	model.apply(Action{Kind: MoveReaderPage, Amount: delta})
	if model.files.place.ReaderCursor != 30+delta || model.activeReaderVisualOffset() != 25+delta {
		t.Fatalf("page down place = %+v visual=%d", model.files.place, model.activeReaderVisualOffset())
	}
	model.apply(Action{Kind: MoveReaderPage, Amount: -delta})
	if model.files.place.ReaderCursor != 30 || model.activeReaderVisualOffset() != 25 {
		t.Fatalf("page up place = %+v visual=%d", model.files.place, model.activeReaderVisualOffset())
	}

}

func TestDiffContextFoldActionsPreservePlaceAndSurviveRefresh(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.active = workspace.Files
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	model.files.place.Focus = navigation.FocusReader
	document := foldableDiffDocument()
	model.files.readerPresentation = &document

	compact := model.files.readerDocument()
	if !document.ContextFoldable() || len(compact.Rows) >= len(document.Rows) {
		t.Fatalf("initial diff is not compact: source=%d compact=%d", len(document.Rows), len(compact.Rows))
	}
	removed := readerIdentityIndex(compact.Rows, "removed")
	model.files.place.ReaderOffset = removed
	model.files.place.ReaderCursor = removed
	firstFold := compact.Rows[0].Identity
	if pending := model.apply(Action{Kind: ExpandReaderFold}); pending.kind != effectNone || model.files.readerContext.target(firstFold) {
		t.Fatalf("non-fold selection expanded context: effect=%+v target=%v", pending, model.files.readerContext.target(firstFold))
	}
	model.files.place.ReaderCursor = 0
	pending := model.apply(Action{Kind: ExpandReaderFold})
	if model.files.readerContext.target(firstFold) != true || model.files.readerContext.progress(firstFold) != 1 || pending.kind != effectAnimateReaderContext ||
		model.files.readerRows()[model.files.place.ReaderOffset].Identity != "removed" {
		t.Fatalf("expand did not start in place: target=%v progress=%d offset=%d rows=%+v", model.files.readerContext.target(firstFold), model.files.readerContext.progress(firstFold), model.files.place.ReaderOffset, model.files.readerRows())
	}
	finishReaderContextAnimation(&model, pending)
	if len(model.files.readerRows()) >= len(document.Rows)+2 || model.files.readerContext.allExpanded(document) {
		t.Fatalf("local expansion opened unrelated gaps: rows=%d source=%d all=%v", len(model.files.readerRows()), len(document.Rows), model.files.readerContext.allExpanded(document))
	}

	model.files.place.ReaderOffset = readerIdentityIndex(model.files.readerRows(), "context:4")
	model.files.place.ReaderCursor = model.files.place.ReaderOffset
	if pending = model.apply(Action{Kind: CollapseReaderFold}); pending.kind != effectNone {
		t.Fatalf("context line selection collapsed a fold: %+v", pending)
	}
	model.files.place.ReaderCursor = readerIdentityIndex(model.files.readerRows(), firstFold)
	pending = model.apply(Action{Kind: CollapseReaderFold})
	finishReaderContextAnimation(&model, pending)
	if model.files.readerContext.target(firstFold) || model.files.readerRows()[model.files.place.ReaderOffset].Identity != "context:8" ||
		model.files.readerRows()[model.files.place.ReaderCursor].Identity != firstFold {
		t.Fatalf("collapse lost viewport or cursor identity: target=%v place=%+v", model.files.readerContext.target(firstFold), model.files.place)
	}

	model.files.place.ReaderCursor = readerIdentityIndex(model.files.readerRows(), firstFold)
	pending = model.apply(Action{Kind: ExpandReaderFold})
	finishReaderContextAnimation(&model, pending)
	model.files.contentGeneration = 9
	model.files = model.files.landDiff(diffLoadedMsg{
		generation:   9,
		entry:        model.files.readerEntry,
		presentation: document,
	})
	if !model.files.readerContext.target(firstFold) || model.files.readerContext.allExpanded(document) {
		t.Fatalf("same-identity refresh reset local fold state: target=%v all=%v", model.files.readerContext.target(firstFold), model.files.readerContext.allExpanded(document))
	}
}

func TestReaderFoldClickExpandsAndRefoldsPersistentControl(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	model.files.place.Focus = navigation.FocusNavigator
	document := foldableDiffDocument()
	model.files.readerPresentation = &document

	click := tea.MouseClickMsg(tea.Mouse{
		X: model.geometry.ReaderRows.X, Y: model.geometry.ReaderRows.Y, Button: tea.MouseLeft,
	})
	for _, test := range []struct {
		expanded bool
		label    string
	}{{expanded: true, label: "expand"}, {expanded: false, label: "refold"}} {
		next, command := model.Update(click)
		model = next.(Model)
		firstFold := model.files.readerDocument().Rows[0].Identity
		if command == nil || model.files.readerContext.target(firstFold) != test.expanded || model.files.place.Focus != navigation.FocusReader ||
			model.files.readerRows()[model.files.place.ReaderCursor].Identity != firstFold {
			t.Fatalf("%s click = expanded %v focus %v cursor %d command=%v", test.label, model.files.readerContext.target(firstFold), model.files.place.Focus, model.files.place.ReaderCursor, command != nil)
		}
		finishReaderContextAnimation(&model, readerContextAnimationEffect(readerContextFiles, model.files.readerContext.generation, true))
		if row := model.files.readerRows()[0]; row.Kind != ui.ReaderFold || row.FoldExpanded != test.expanded {
			t.Fatalf("%s control = %+v", test.label, row)
		}
		for _, row := range model.files.readerRows()[1:] {
			if row.Kind == ui.ReaderFold && row.FoldExpanded {
				t.Fatalf("%s changed an unrelated fold: %+v", test.label, row)
			}
		}
	}
}

func TestExpandedFoldEndCollapsesFromKeyboardAndMouse(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 80})
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	model.files.place.Focus = navigation.FocusReader
	document := foldableDiffDocument()
	model.files.readerPresentation = &document
	identities := document.ContextFoldIdentities()
	if len(identities) < 2 {
		t.Fatalf("fold identities = %#v, want an inter-hunk gap", identities)
	}
	identity := identities[1]

	expand := func() {
		changed, animating := model.files.setReaderContextFold(identity, true)
		if !changed {
			t.Fatal("inter-hunk fold did not expand")
		}
		finishReaderContextAnimation(&model, readerContextAnimationEffect(readerContextFiles, model.files.readerContext.generation, animating))
	}
	endIndex := func() int {
		for index, row := range model.files.readerRows() {
			if row.Kind == ui.ReaderFoldEnd && row.FoldTarget == identity {
				return index
			}
		}
		return -1
	}

	expand()
	marker := endIndex()
	if marker < 0 {
		t.Fatal("expanded inter-hunk fold has no end marker")
	}
	model.files.place.ReaderCursor = marker
	pending := model.apply(Action{Kind: CollapseReaderFold})
	if model.files.readerContext.target(identity) || pending.kind != effectAnimateReaderContext {
		t.Fatalf("h on fold end = target %v effect %+v", model.files.readerContext.target(identity), pending)
	}
	finishReaderContextAnimation(&model, pending)
	if endIndex() >= 0 {
		t.Fatal("collapsed fold retained its end marker")
	}

	expand()
	marker = endIndex()
	layout := ui.CalculateReaderLayout(model.geometry.ReaderRows, model.files.readerDocument())
	visual := layout.VisualOffset(marker, 0) - model.activeReaderVisualOffset()
	click := tea.MouseClickMsg(tea.Mouse{
		X: model.geometry.ReaderRows.X, Y: model.geometry.ReaderRows.Y + visual, Button: tea.MouseLeft,
	})
	next, command := model.Update(click)
	model = next.(Model)
	if model.files.readerContext.target(identity) || command == nil || model.files.place.Focus != navigation.FocusReader {
		t.Fatalf("fold end click = target %v focus %v command=%v", model.files.readerContext.target(identity), model.files.place.Focus, command != nil)
	}
}

func TestReaderHunkNavigationMovesThroughTheContinuousDiff(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	model.files.place.Focus = navigation.FocusNavigator
	document := foldableDiffDocument()
	model.files.readerPresentation = &document

	presented := model.files.readerDocument()
	starts := presented.HunkStarts()
	targets := presented.HunkNavigationTargets()
	if len(starts) != 2 || len(targets) != 2 || starts[0] <= 0 || starts[1] <= starts[0] {
		t.Fatalf("hunk starts = %#v targets = %#v, want two ordered compact groups", starts, targets)
	}
	for index, target := range targets {
		if target != starts[index]-1 || presented.Rows[target].Kind != ui.ReaderFold {
			t.Fatalf("hunk target %d = %d (%v), want leading fold before %d", index, target, presented.Rows[target].Kind, starts[index])
		}
	}
	model.files.place.ReaderCursor = targets[0]
	if model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("initial hunk focus = %v, want navigator", model.files.place.Focus)
	}
	model.apply(Action{Kind: ExpandReaderFold})
	if !model.files.readerContext.target(presented.Rows[targets[0]].Identity) {
		t.Fatal("right from hunk target did not expand its leading fold")
	}
	expandedTargets := model.files.readerDocument().HunkNavigationTargets()
	model.apply(Action{Kind: SelectNextLandmark})
	if model.files.place.ReaderCursor != expandedTargets[1] || model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("second hunk = cursor %d focus %v, want %d without focus change", model.files.place.ReaderCursor, model.files.place.Focus, expandedTargets[1])
	}
	model.apply(Action{Kind: SelectPreviousLandmark})
	if model.files.place.ReaderCursor != expandedTargets[0] {
		t.Fatalf("previous hunk cursor = %d, want %d", model.files.place.ReaderCursor, expandedTargets[0])
	}
}

func TestReaderLandmarkNavigationReachesTrailingContextFold(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument}
	for line := 1; line <= 10; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("context:%d", line), Kind: ui.ReaderContext,
			Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
		})
	}
	document.Rows = append(document.Rows, ui.ReaderRow{
		Identity: "added", Kind: ui.ReaderInsertion, Text: "changed", NewLine: 11,
	})
	for line := 12; line <= 21; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("context:%d", line), Kind: ui.ReaderContext,
			Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
		})
	}
	model.files.readerPresentation = &document

	presented := model.files.readerDocument()
	targets := model.settings.hunkNavigationTargets(readerNavigationLandmarks(presented))
	if len(targets) != 2 || presented.Rows[targets[0]].Kind != ui.ReaderFold || presented.Rows[targets[1]].Kind != ui.ReaderFold {
		t.Fatalf("landmarks = %v in %+v, want leading and trailing folds", targets, presented.Rows)
	}
	model.files.place.ReaderCursor = targets[0]
	model.apply(Action{Kind: SelectNextLandmark})
	if model.files.place.ReaderCursor != targets[1] {
		t.Fatalf("next landmark = %d, want trailing fold %d", model.files.place.ReaderCursor, targets[1])
	}
}

func TestReaderClickSelectsLogicalLine(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 16})
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	for line := 1; line <= 20; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{Identity: fmt.Sprintf("line:%d", line), Kind: ui.ReaderFile, Text: "code", NewLine: uint64(line)})
	}
	model.files.readerPresentation = &document

	click := tea.MouseClickMsg(tea.Mouse{
		X:      model.geometry.ReaderRows.X + 2,
		Y:      model.geometry.ReaderRows.Y + 4,
		Button: tea.MouseLeft,
	})
	next, _ := model.Update(click)
	model = next.(Model)
	if model.files.place.Focus != navigation.FocusReader || model.files.place.ReaderCursor != 4 {
		t.Fatalf("reader click place = %+v, want focused cursor 4", model.files.place)
	}
}

func TestNavigatorFileHorizontalKeysChangeDiffDetailWithoutMovingFocus(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.controls.Reader = workspace.DiffReader
	model.files.tree = filetree.New([]string{"main.go"})
	model.files.place.Reconcile(model.files.tree.Identities())
	model.files.place.Focus = navigation.FocusNavigator
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	document := foldableDiffDocument()
	model.files.readerPresentation = &document

	press := func(key tea.Key) {
		next, command := model.Update(tea.KeyPressMsg(key))
		model = next.(Model)
		if command == nil {
			t.Fatalf("horizontal detail key %q did not schedule animation", key.String())
		}
		finishReaderContextAnimation(&model, readerContextAnimationEffect(readerContextFiles, model.files.readerContext.generation, true))
	}
	press(tea.Key{Code: 'l', Text: "l"})
	if !model.files.readerContext.allExpanded(document) || model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("navigator l = expanded %v focus %v", model.files.readerContext.allExpanded(document), model.files.place.Focus)
	}
	selected, _ := model.files.place.SelectedIdentity()
	if selected != filetree.FileIdentity("main.go") {
		t.Fatalf("navigator l moved selection to %q", selected)
	}

	press(tea.Key{Code: tea.KeyLeft})
	if model.files.readerContext.allExpanded(document) || model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("navigator left = expanded %v focus %v", model.files.readerContext.allExpanded(document), model.files.place.Focus)
	}
}

func TestReaderContextAnimationReversesImmediatelyAndRejectsStaleFrames(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.active = workspace.Files
	model.controls.Reader = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "main.go"}
	model.files.readerMode = workspace.DiffReader
	document := foldableDiffDocument()
	model.files.readerPresentation = &document
	firstFold := model.files.readerDocument().Rows[0].Identity

	pending := model.apply(Action{Kind: ExpandReaderFold})
	oldGeneration := pending.generation
	for range 3 {
		pending = model.landReaderContextFrame(readerContextFrameMsg{owner: readerContextFiles, generation: oldGeneration})
	}
	if model.files.readerContext.progress(firstFold) != 4 {
		t.Fatalf("expanded progress = %d, want 4", model.files.readerContext.progress(firstFold))
	}

	pending = model.apply(Action{Kind: CollapseReaderFold})
	if model.files.readerContext.progress(firstFold) != 3 || model.files.readerContext.target(firstFold) || pending.generation == oldGeneration {
		t.Fatalf("reverse = target %v progress %d generation %d", model.files.readerContext.target(firstFold), model.files.readerContext.progress(firstFold), pending.generation)
	}
	model.landReaderContextFrame(readerContextFrameMsg{owner: readerContextFiles, generation: oldGeneration})
	if model.files.readerContext.progress(firstFold) != 3 {
		t.Fatalf("stale expansion frame changed reversed progress to %d", model.files.readerContext.progress(firstFold))
	}
	finishReaderContextAnimation(&model, pending)
	if model.files.readerContext.progress(firstFold) != 0 || len(model.files.readerRows()) >= len(document.Rows) {
		t.Fatalf("collapse finished at progress %d with %d rows", model.files.readerContext.progress(firstFold), len(model.files.readerRows()))
	}
}

func TestStashReaderUsesSharedContextAnimation(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 20})
	model.active = workspace.Git
	model.controls.Git = workspace.GitStashes
	document := foldableDiffDocument()
	model.stashes.readerPresentation = &document
	firstFold := model.stashes.readerDocument().Rows[0].Identity

	pending := model.apply(Action{Kind: ExpandReaderFold})
	if pending.readerContextOwner != readerContextStashes || model.stashes.readerContext.progress(firstFold) != 1 {
		t.Fatalf("stash animation = effect %+v progress %d", pending, model.stashes.readerContext.progress(firstFold))
	}
	finishReaderContextAnimation(&model, pending)
	if !model.stashes.readerContext.target(firstFold) || model.stashes.readerContext.progress(firstFold) != readerContextAnimationSteps {
		t.Fatalf("stash expansion finished at target %v progress %d", model.stashes.readerContext.target(firstFold), model.stashes.readerContext.progress(firstFold))
	}
}

func finishReaderContextAnimation(model *Model, pending effect) {
	for pending.kind == effectAnimateReaderContext {
		pending = model.landReaderContextFrame(readerContextFrameMsg{
			owner: pending.readerContextOwner, generation: pending.generation,
		})
	}
}

func foldableDiffDocument() ui.ReaderDocument {
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument}
	for line := 1; line <= 10; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("context:%d", line), Kind: ui.ReaderContext,
			Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
		})
	}
	document.Rows = append(document.Rows,
		ui.ReaderRow{Identity: "removed", Kind: ui.ReaderDeletion, Text: "old", OldLine: 11},
		ui.ReaderRow{Identity: "added", Kind: ui.ReaderInsertion, Text: "new", NewLine: 11},
	)
	for line := 12; line <= 50; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("context:%d", line), Kind: ui.ReaderContext,
			Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
		})
	}
	for line := 51; line <= 70; line++ {
		document.Rows = append(document.Rows, ui.ReaderRow{
			Identity: fmt.Sprintf("added:%d", line), Kind: ui.ReaderInsertion,
			Text: fmt.Sprintf("added %d", line), NewLine: uint64(line),
		})
	}
	return document
}

func readerIdentityIndex(rows []ui.ReaderRow, identity string) int {
	for index, row := range rows {
		if row.Identity == identity {
			return index
		}
	}
	return -1
}
