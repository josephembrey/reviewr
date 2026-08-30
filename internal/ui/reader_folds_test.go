package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestReaderContextFoldsHideOnlyLongUnchangedGaps(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument}
	appendContext := func(start, count int) {
		for line := start; line < start+count; line++ {
			document.Rows = append(document.Rows, ReaderRow{
				Identity: fmt.Sprintf("context:%d", line), Kind: ReaderContext,
				Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
			})
		}
	}
	appendContext(1, 10)
	document.Rows = append(document.Rows,
		ReaderRow{Identity: "removed", Kind: ReaderDeletion, Text: "old", OldLine: 11},
		ReaderRow{Identity: "added", Kind: ReaderInsertion, Text: "new", NewLine: 11},
	)
	appendContext(12, 12)
	document.Rows = append(document.Rows, ReaderRow{Identity: "added-2", Kind: ReaderInsertion, Text: "again", NewLine: 24})
	appendContext(25, 10)

	if !document.ContextFoldable() {
		t.Fatal("long unchanged gaps are not foldable")
	}
	compact := document.WithContextFolds(false)
	if len(compact.Rows) >= len(document.Rows) {
		t.Fatalf("compact rows = %d, source rows = %d", len(compact.Rows), len(document.Rows))
	}
	folds := 0
	var text strings.Builder
	visible := make(map[string]bool)
	for _, row := range compact.Rows {
		if row.Kind == ReaderFold {
			folds++
			if row.DisplayLine() != 0 || row.Tone != ToneDefault || row.FoldExpanded || !strings.Contains(row.Text, "unchanged lines") {
				t.Fatalf("fold row = %+v", row)
			}
		}
		text.WriteString(row.Text)
		visible[row.Text] = true
	}
	if folds != 3 || !strings.Contains(text.String(), "old") || !strings.Contains(text.String(), "new") || !strings.Contains(text.String(), "again") {
		t.Fatalf("compact document folds=%d text=%q", folds, text.String())
	}
	if visible["context 1"] || visible["context 17"] || visible["context 34"] {
		t.Fatalf("hidden context remains visible: %q", text.String())
	}
	expanded := document.WithContextFolds(true)
	if len(expanded.Rows) != len(document.Rows)+folds {
		t.Fatalf("expanded rows = %d, want %d source rows plus %d controls", len(expanded.Rows), len(document.Rows), folds)
	}
	restored := make([]ReaderRow, 0, len(document.Rows))
	for _, row := range expanded.Rows {
		if row.Kind == ReaderFold {
			if !row.FoldExpanded {
				t.Fatalf("expanded control lacks state: %+v", row)
			}
			continue
		}
		restored = append(restored, row)
	}
	if !reflect.DeepEqual(restored, document.Rows) {
		t.Fatal("expanded controls changed the semantic document rows")
	}

	half := document.WithContextFoldProgress(4, 8)
	if len(half.Rows) <= len(compact.Rows) || len(half.Rows) >= len(expanded.Rows) {
		t.Fatalf("half-expanded rows = %d, compact=%d expanded=%d", len(half.Rows), len(compact.Rows), len(expanded.Rows))
	}
	visibleContext := 0
	for _, row := range half.Rows {
		if row.Kind == ReaderFold && !row.FoldExpanded {
			t.Fatalf("partially expanded control lacks state: %+v", row)
		}
		if row.Kind == ReaderContext {
			visibleContext++
		}
	}
	if visibleContext <= 18 || visibleContext >= 32 {
		t.Fatalf("half-expanded visible context = %d, want an intermediate presentation", visibleContext)
	}
}

func TestReaderContextFoldsLeaveSmallRunsAlone(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument, Rows: []ReaderRow{
		{Kind: ReaderDeletion, Text: "old", OldLine: 1},
		{Kind: ReaderContext, Text: "one", OldLine: 2, NewLine: 2},
		{Kind: ReaderContext, Text: "two", OldLine: 3, NewLine: 3},
		{Kind: ReaderInsertion, Text: "new", NewLine: 4},
	}}
	if document.ContextFoldable() || !reflect.DeepEqual(document.WithContextFolds(false).Rows, document.Rows) {
		t.Fatal("small context run was folded")
	}
}

func TestReaderContextGapsExpandIndependentlyAndDefineHunks(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument}
	appendContext := func(start, count int) {
		for line := start; line < start+count; line++ {
			document.Rows = append(document.Rows, ReaderRow{
				Identity: fmt.Sprintf("context:%d", line), Kind: ReaderContext,
				Text: fmt.Sprintf("context %d", line), OldLine: uint64(line), NewLine: uint64(line),
			})
		}
	}
	appendContext(1, 10)
	document.Rows = append(document.Rows, ReaderRow{Identity: "change:1", Kind: ReaderInsertion, Text: "first", NewLine: 11})
	appendContext(12, 20)
	document.Rows = append(document.Rows, ReaderRow{Identity: "change:2", Kind: ReaderInsertion, Text: "second", NewLine: 32})
	appendContext(33, 10)

	identities := document.ContextFoldIdentities()
	if len(identities) != 3 {
		t.Fatalf("fold identities = %#v, want leading, inter-hunk, and trailing gaps", identities)
	}
	compact := document.WithContextFolds(false)
	starts := compact.HunkStarts()
	if len(starts) != 2 || starts[0] >= starts[1] {
		t.Fatalf("compact hunk starts = %#v", starts)
	}
	expanded := document.WithContextFoldProgresses(map[string]int{identities[1]: 8}, 0, 8)
	states := make(map[string]bool)
	for _, row := range expanded.Rows {
		if row.Kind == ReaderFold {
			states[row.Identity] = row.FoldExpanded
		}
	}
	if states[identities[0]] || !states[identities[1]] || states[identities[2]] {
		t.Fatalf("independent fold states = %#v", states)
	}
	if near, ok := expanded.ContextFoldNear(expanded.HunkStarts()[1]); !ok || near != identities[1] {
		t.Fatalf("second hunk context = %q, %v; want %q", near, ok, identities[1])
	}
}

func TestExplicitUnifiedDiffHeadersRemainHunkNavigationTargets(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument, Rows: []ReaderRow{
		{Kind: ReaderMetadata, Text: "@@ -1,2 +1,2 @@"},
		{Kind: ReaderDeletion, Text: "old", OldLine: 1},
		{Kind: ReaderInsertion, Text: "new", NewLine: 1},
		{Kind: ReaderMetadata, Text: "@@ -20,2 +20,2 @@"},
		{Kind: ReaderDeletion, Text: "old again", OldLine: 20},
		{Kind: ReaderInsertion, Text: "new again", NewLine: 20},
	}}
	if starts := document.HunkStarts(); !reflect.DeepEqual(starts, []int{0, 3}) {
		t.Fatalf("explicit hunk starts = %#v", starts)
	}
}

func TestReaderFoldUsesNormalWeightAccent(t *testing.T) {
	t.Parallel()
	const width = 44
	row := ReaderRow{Kind: ReaderFold, Text: "12 unchanged lines"}
	rendered := renderReaderFoldPayload(row.Text, width, false)
	plain := ansi.Strip(rendered)
	if !strings.HasPrefix(plain, "── ▸ folded · 12 unchanged lines ") || !strings.HasSuffix(plain, "─") || lipgloss.Width(plain) != width {
		t.Fatalf("fold payload = %q, want full-width structural control", plain)
	}
	if rendered != readerFoldStyle.Render(plain) {
		t.Fatalf("fold payload = %q, want normal-weight accent %q", rendered, readerFoldStyle.Render(plain))
	}
	expanded := ansi.Strip(renderReaderFoldPayload(row.Text, width, true))
	if !strings.HasPrefix(expanded, "── ▾ expanded · 12 unchanged lines ") {
		t.Fatalf("expanded fold payload = %q", expanded)
	}
}

func TestReaderHeaderShowsClickableGlobalContextState(t *testing.T) {
	t.Parallel()
	geometry := Calculate(80, 12)
	model := Model{
		Geometry:              geometry,
		Workspace:             workspace.Files,
		Controls:              workspace.Controls{Reader: workspace.DiffReader},
		ReaderTitle:           "main.go  [diff]",
		ReaderContextFoldable: true,
	}

	collapsed := ansi.Strip(renderReaderTitle(model, model.ReaderTitle))
	if !strings.HasPrefix(collapsed, model.ReaderTitle+" ▸") || strings.Contains(collapsed, "all context") || !strings.HasSuffix(collapsed, "2 [diff]") || lipgloss.Width(collapsed) != geometry.ReaderTitle.Width {
		t.Fatalf("collapsed reader title = %q", collapsed)
	}
	model.ReaderContextExpanded = true
	expanded := ansi.Strip(renderReaderTitle(model, model.ReaderTitle))
	if !strings.HasPrefix(expanded, model.ReaderTitle+" ▾") || strings.Contains(expanded, "all context") || !strings.HasSuffix(expanded, "2 [diff]") || lipgloss.Width(expanded) != geometry.ReaderTitle.Width {
		t.Fatalf("expanded reader title = %q", expanded)
	}

	target := LayoutReaderContextFold(geometry, model.ReaderTitle, true, model.Workspace, model.Controls)
	if !target.Contains(target.X, target.Y) || target.Contains(target.X-1, target.Y) ||
		LayoutReaderContextFold(geometry, model.ReaderTitle, false, model.Workspace, model.Controls) != (Rect{}) {
		t.Fatalf("global context hit target disagrees with painted control: %+v", target)
	}
	wantX := geometry.ReaderTitle.X + lipgloss.Width(model.ReaderTitle) + 1
	if target.X != wantX {
		t.Fatalf("global context target x=%d, want left-cluster position %d", target.X, wantX)
	}
	model.ReaderContextFoldable = false
	if title := ansi.Strip(renderReaderTitle(model, model.ReaderTitle)); strings.Contains(title, "▸") || strings.Contains(title, "▾") {
		t.Fatalf("non-foldable reader title exposed a global control: %q", title)
	}
}

func TestReaderLayoutHitFoldUsesPaintedRowsAndExcludesScrollbar(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument, Rows: []ReaderRow{
		{Identity: "fold:1", Kind: ReaderFold, Text: "12 unchanged lines"},
		{Kind: ReaderInsertion, Text: "changed", NewLine: 20},
	}}
	layout := CalculateReaderLayout(Rect{X: 30, Y: 4, Width: 20, Height: 1}, document)
	if identity, ok := layout.FoldAt(layout.Geometry.Content.X, layout.Geometry.Rows.Y, 0); !ok || identity != "fold:1" {
		t.Fatalf("fold target = %q, %v; want stable row identity", identity, ok)
	}
	if _, ok := layout.FoldAt(layout.Geometry.Scrollbar.X, layout.Geometry.Rows.Y, 0); ok {
		t.Fatal("fold target claimed the scrollbar lane")
	}
	if _, ok := layout.FoldAt(layout.Geometry.Content.X, layout.Geometry.Rows.Y, 1); ok {
		t.Fatal("non-fold row was clickable after scrolling")
	}
}
