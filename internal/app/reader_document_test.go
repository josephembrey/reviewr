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
	model.files.place.ReaderOffset = readerIdentityIndex(compact.Rows, "removed")
	model.apply(Action{Kind: ExpandReaderContext})
	if !model.files.readerContextExpanded || len(model.files.readerRows()) != len(document.Rows) ||
		model.files.readerRows()[model.files.place.ReaderOffset].Identity != "removed" {
		t.Fatalf("expand lost place: expanded=%v offset=%d rows=%+v", model.files.readerContextExpanded, model.files.place.ReaderOffset, model.files.readerRows())
	}

	model.files.place.ReaderOffset = readerIdentityIndex(model.files.readerRows(), "context:4")
	model.apply(Action{Kind: CollapseReaderContext})
	if model.files.readerContextExpanded || model.files.readerRows()[model.files.place.ReaderOffset].Identity != "context:8" {
		t.Fatalf("collapse did not choose nearest surviving identity: expanded=%v offset=%d row=%+v", model.files.readerContextExpanded, model.files.place.ReaderOffset, model.files.readerRows()[model.files.place.ReaderOffset])
	}

	model.apply(Action{Kind: ExpandReaderContext})
	model.files.contentGeneration = 9
	model.files = model.files.landDiff(diffLoadedMsg{
		generation:   9,
		entry:        model.files.readerEntry,
		presentation: document,
	}, model.geometry.ReaderRows.Height)
	if !model.files.readerContextExpanded || len(model.files.readerRows()) != len(document.Rows) {
		t.Fatalf("same-identity refresh reset authored fold state: expanded=%v rows=%d", model.files.readerContextExpanded, len(model.files.readerRows()))
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
		if command != nil {
			t.Fatalf("horizontal detail key %q produced a command", key.String())
		}
	}
	press(tea.Key{Code: 'l', Text: "l"})
	if !model.files.readerContextExpanded || model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("navigator l = expanded %v focus %v", model.files.readerContextExpanded, model.files.place.Focus)
	}
	selected, _ := model.files.place.SelectedIdentity()
	if selected != filetree.FileIdentity("main.go") {
		t.Fatalf("navigator l moved selection to %q", selected)
	}

	press(tea.Key{Code: tea.KeyLeft})
	if model.files.readerContextExpanded || model.files.place.Focus != navigation.FocusNavigator {
		t.Fatalf("navigator left = expanded %v focus %v", model.files.readerContextExpanded, model.files.place.Focus)
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
