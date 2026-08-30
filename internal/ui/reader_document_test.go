package ui

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestReaderDocumentGutterIsStableAndAlignsEverySemanticKind(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument, Rows: []ReaderRow{
		{Kind: ReaderFile, Text: "fileCode", NewLine: 1},
		{Kind: ReaderContext, Text: "contextCode", OldLine: 12, NewLine: 22},
		{Kind: ReaderDeletion, Text: "deletedCode", OldLine: 9999},
		{Kind: ReaderInsertion, Text: "insertedCode", NewLine: 10000},
		{Kind: ReaderMetadata, Text: "metadataCode"},
		{Kind: ReaderNotice, Text: "noticeCode"},
		{Kind: ReaderFold, Text: "foldCode"},
		{Kind: ReaderFoldEnd, Text: "change resumes", FoldTarget: "fold:1"},
	}}
	if got := document.GutterDigits(); got != 5 {
		t.Fatalf("gutter digits = %d, want 5", got)
	}
	geometry := CalculateReaderGeometry(Rect{Width: 30, Height: len(document.Rows)}, document, false)
	if geometry.Prefix != 7 || geometry.Code.X != 7 || geometry.Code.Width != 23 {
		t.Fatalf("reader geometry = %+v", geometry)
	}
	for _, row := range document.Rows {
		plain := ansi.Strip(renderReaderRow(row, geometry, workspace.DiffHighlightSidebar))
		payload := row.Text
		if row.Kind == ReaderFold {
			payload = "── ▸ folded"
		} else if row.Kind == ReaderFoldEnd {
			payload = "── change resumes"
		}
		byteIndex := strings.Index(plain, payload)
		codeX := lipgloss.Width(plain[:byteIndex])
		wantX := geometry.Prefix
		if row.Kind == ReaderFold || row.Kind == ReaderFoldEnd {
			wantX = 0
		}
		if codeX != wantX {
			t.Fatalf("kind %d code x = %d, want %d: %q", row.Kind, codeX, wantX, plain)
		}
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[2], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "▌ 9999 ") {
		t.Fatalf("deletion gutter = %q", plain)
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[3], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "▌10000 ") {
		t.Fatalf("insertion gutter = %q", plain)
	}
	for _, index := range []int{4, 5} {
		plain := ansi.Strip(renderReaderRow(document.Rows[index], geometry, workspace.DiffHighlightSidebar))
		if !strings.HasPrefix(plain, strings.Repeat(" ", geometry.Prefix)) {
			t.Fatalf("metadata/notice fabricated gutter at row %d: %q", index, plain)
		}
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[6], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "── ▸ folded") || lipgloss.Width(plain) != geometry.Content.Width {
		t.Fatalf("fold did not replace the full gutter row: %q", plain)
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[7], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "── change resumes") || lipgloss.Width(plain) != geometry.Content.Width {
		t.Fatalf("fold end did not replace the full gutter row: %q", plain)
	}
}

func TestReaderCursorSelectionOwnsTheWholeVisualLine(t *testing.T) {
	t.Parallel()
	geometry := CalculateReaderGeometry(Rect{Width: 30, Height: 2}, ReaderDocument{
		Kind: ReaderDiffDocument,
		Rows: []ReaderRow{{Kind: ReaderInsertion, Text: "changed", NewLine: 12}},
	}, false)
	row := ReaderRow{
		Kind: ReaderInsertion, Text: "changed", NewLine: 12,
		Spans: []TextSpan{{Text: "changed", Tone: ToneSpecial}},
	}
	base := renderReaderRowPart(row, geometry, workspace.DiffHighlightBackground, false)
	selected := renderReaderRowPartSelected(row, geometry, workspace.DiffHighlightBackground, false, true, true)
	want := selectionStyle(true).Render(ansi.Strip(fit(base, geometry.Content.Width)))
	if selected != want || lipgloss.Width(selected) != geometry.Content.Width {
		t.Fatalf("selected reader row = %q, want %q", selected, want)
	}

	fold := ReaderRow{Kind: ReaderFold, Text: "12 unchanged lines"}
	selectedFold := renderReaderRowPartSelected(fold, geometry, workspace.DiffHighlightSidebar, false, true, true)
	wantFold := selectionStyle(true).Render(ansi.Strip(renderReaderFoldPayload(fold.Text, geometry.Content.Width, false)))
	if selectedFold != wantFold {
		t.Fatalf("selected fold = %q, want %q", selectedFold, wantFold)
	}

	end := ReaderRow{Kind: ReaderFoldEnd, Text: "change resumes", FoldTarget: "fold:1"}
	selectedEnd := renderReaderRowPartSelected(end, geometry, workspace.DiffHighlightSidebar, false, true, true)
	wantEnd := selectionStyle(true).Render(ansi.Strip(renderReaderFoldEndPayload(end.Text, geometry.Content.Width)))
	if selectedEnd != wantEnd {
		t.Fatalf("selected fold end = %q, want %q", selectedEnd, wantEnd)
	}
}

func TestSelectedSidebarDiffBarsKeepTheirSemanticColor(t *testing.T) {
	t.Parallel()
	geometry := CalculateReaderGeometry(Rect{Width: 30, Height: 1}, ReaderDocument{
		Kind: ReaderDiffDocument,
	}, false)
	for _, test := range []struct {
		name string
		row  ReaderRow
	}{
		{name: "addition", row: ReaderRow{Kind: ReaderInsertion, Text: "added", NewLine: 1}},
		{name: "removal", row: ReaderRow{Kind: ReaderDeletion, Text: "removed", OldLine: 1}},
		{name: "replacement boundary", row: ReaderRow{Kind: ReaderInsertion, Text: "replaced", NewLine: 1, RemovedBefore: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bar, style := readerChangeBar(test.row, false)
			selectedStyle := selectedReaderBarStyle(style, true)
			if !selectedStyle.GetReverse() {
				t.Fatal("selected bar is not part of the reversed selection")
			}
			assertSameColor(t, selectedStyle.GetBackground(), style.GetForeground())
			assertSameColor(t, selectedStyle.GetForeground(), style.GetBackground())

			rendered := renderReaderRowPartSelected(
				test.row, geometry, workspace.DiffHighlightSidebar, false, true, true,
			)
			if !strings.HasPrefix(rendered, selectedStyle.Render(bar)) {
				t.Fatalf("selected row lost semantic bar: %q", rendered)
			}
			if got := lipgloss.Width(rendered); got != geometry.Content.Width {
				t.Fatalf("selected row width = %d, want %d", got, geometry.Content.Width)
			}
			if plain := ansi.Strip(rendered); plain != ansi.Strip(renderReaderRow(test.row, geometry, workspace.DiffHighlightSidebar)) {
				t.Fatalf("selection changed row content: %q", plain)
			}
		})
	}
}

func TestReaderLayoutWrapsStyledSourceRowsInsideCodeWidth(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderFileDocument, Rows: []ReaderRow{{
		Kind: ReaderFile, Text: "abcdefghij", NewLine: 1,
		Spans: []TextSpan{
			{Text: "abcde", Style: TextStyle{Foreground: "4"}},
			{Text: "fghij", Style: TextStyle{Foreground: "3"}},
		},
	}}}
	layout := CalculateReaderLayout(Rect{Width: 10, Height: 2}, document)
	if layout.Geometry.Code.Width != 5 || layout.Total != 2 {
		t.Fatalf("reader layout = %+v", layout)
	}
	first, firstContinuation := layout.Row(0)
	second, secondContinuation := layout.Row(1)
	if firstContinuation || !secondContinuation || first.Text != "abcde" || second.Text != "fghij" {
		t.Fatalf("wrapped rows = (%+v,%v), (%+v,%v)", first, firstContinuation, second, secondContinuation)
	}
	firstRendered := renderReaderRowPart(first, layout.Geometry, workspace.DiffHighlightSidebar, firstContinuation)
	secondRendered := renderReaderRowPart(second, layout.Geometry, workspace.DiffHighlightSidebar, secondContinuation)
	if got := ansi.Strip(firstRendered); got != "   1 abcde" {
		t.Fatalf("first wrapped row = %q", got)
	}
	if got := ansi.Strip(secondRendered); got != "     fghij" {
		t.Fatalf("continuation row = %q", got)
	}
	if !strings.Contains(firstRendered, "34m") || !strings.Contains(secondRendered, "33m") {
		t.Fatalf("wrapped syntax styles were lost: %q / %q", firstRendered, secondRendered)
	}
}

func TestReaderLayoutPrefersWordAndCodeChunkBoundaries(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderFileDocument, Rows: []ReaderRow{{
		Kind: ReaderFile, Text: "call(first, second)", NewLine: 1,
	}}}
	layout := CalculateReaderLayout(Rect{Width: 15, Height: 3}, document)
	if layout.Geometry.Code.Width != 10 || layout.Total != 3 {
		t.Fatalf("reader layout = %+v", layout)
	}
	want := []string{"call(", "first, ", "second)"}
	var joined strings.Builder
	for index, expected := range want {
		row, continuation := layout.Row(index)
		if row.Text != expected || continuation != (index > 0) {
			t.Fatalf("wrapped row %d = %q continuation=%v, want %q", index, row.Text, continuation, expected)
		}
		joined.WriteString(row.Text)
	}
	if joined.String() != document.Rows[0].Text {
		t.Fatalf("wrapping changed source text: %q", joined.String())
	}
}

func TestReaderLayoutMapsWrappedVisualScrollToLogicalPlace(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument, Rows: []ReaderRow{
		{Kind: ReaderInsertion, Text: "abcdefghijkl", NewLine: 10},
		{Kind: ReaderContext, Text: "tail", OldLine: 11, NewLine: 11},
	}}
	layout := CalculateReaderLayout(Rect{Width: 10, Height: 2}, document)
	if layout.Total < 4 {
		t.Fatalf("wrapped total = %d, want at least 4", layout.Total)
	}
	for visual := 0; visual < layout.Total; visual++ {
		source, column := layout.SourceOffset(visual)
		if roundTrip := layout.VisualOffset(source, column); roundTrip != visual {
			t.Fatalf("visual %d -> (%d,%d) -> %d", visual, source, column, roundTrip)
		}
		row, continued := layout.Row(visual)
		if source == 0 && continued && row.NewLine != 10 {
			t.Fatalf("continuation lost source identity: %+v", row)
		}
	}
}

func TestReaderGutterMinimumNarrowWidthsAndScrollbarLane(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderFileDocument}
	for line := uint64(1); line <= 40; line++ {
		document.Rows = append(document.Rows, ReaderRow{Kind: ReaderFile, Text: "code", NewLine: line})
	}
	if document.GutterDigits() != 3 {
		t.Fatalf("minimum gutter = %d", document.GutterDigits())
	}
	rows := Rect{X: 10, Y: 3, Width: 12, Height: 5}
	geometry := CalculateReaderGeometry(rows, document, true)
	if geometry.Prefix != 5 || geometry.Content.Width != 11 || geometry.Code != (Rect{X: 15, Y: 3, Width: 6, Height: 5}) ||
		geometry.Scrollbar != (Rect{X: 21, Y: 3, Width: 1, Height: 5}) {
		t.Fatalf("gutter + scrollbar geometry = %+v", geometry)
	}
	for width := 0; width <= geometry.Prefix+2; width++ {
		narrow := CalculateReaderGeometry(Rect{Width: width, Height: 1}, document, width > 0)
		for _, highlight := range []workspace.DiffHighlight{workspace.DiffHighlightSidebar, workspace.DiffHighlightBackground} {
			line := renderReaderRow(ReaderRow{Kind: ReaderInsertion, Text: "hostile\tcode", NewLine: 40}, narrow, highlight)
			if got := lipgloss.Width(line); got != narrow.Content.Width {
				t.Fatalf("width %d highlight %d rendered %d cells, want %d: %q", width, highlight, got, narrow.Content.Width, line)
			}
		}
	}
}

func TestSidebarAndBackgroundDiffTreatmentsUseTerminalANSIRoles(t *testing.T) {
	t.Parallel()
	geometry := CalculateReaderGeometry(Rect{Width: 24, Height: 1}, ReaderDocument{Kind: ReaderDiffDocument}, false)
	rows := []ReaderRow{
		{Kind: ReaderDeletion, Text: `const answer = 41`, OldLine: 1, Spans: []TextSpan{
			{Text: "const", Style: TextStyle{Foreground: "4", Bold: true}}, {Text: " answer = 41", Style: TextStyle{Foreground: "3"}},
		}},
		{Kind: ReaderInsertion, Text: `const answer = 42`, NewLine: 1, Spans: []TextSpan{
			{Text: "const", Style: TextStyle{Foreground: "4", Bold: true}}, {Text: " answer = 42", Style: TextStyle{Foreground: "3"}},
		}},
	}
	for _, row := range rows {
		sidebar := renderReaderRow(row, geometry, workspace.DiffHighlightSidebar)
		if strings.Contains(sidebar, "\x1b[41") || strings.Contains(sidebar, "\x1b[42") {
			t.Fatalf("sidebar row has background fill: %q", sidebar)
		}
		wantForeground := "32m"
		if row.Kind == ReaderDeletion {
			wantForeground = "31m"
		}
		if !strings.Contains(sidebar, wantForeground) || !strings.HasPrefix(ansi.Strip(sidebar), "▌") {
			t.Fatalf("sidebar row lacks semantic bar: %q", sidebar)
		}

		background := renderReaderRow(row, geometry, workspace.DiffHighlightBackground)
		if got := lipgloss.Width(background); got != geometry.Content.Width {
			t.Fatalf("background width = %d, want %d", got, geometry.Content.Width)
		}
		if strings.Contains(background, "38;2") || strings.Contains(background, "48;2") || strings.Contains(background, "38;5") || strings.Contains(background, "48;5") {
			t.Fatalf("background row used fixed/indexed color: %q", background)
		}
		assertEveryPrintableHasChangeBackground(t, background, row.Kind)
		if !strings.HasPrefix(ansi.Strip(background), "▌") || !strings.Contains(background, "30;") {
			t.Fatalf("background row lacks visible bar/readable normalized ink: %q", background)
		}
	}
	for _, row := range []ReaderRow{
		{Kind: ReaderContext, Text: "context", NewLine: 1},
		{Kind: ReaderFile, Text: "file", NewLine: 1},
		{Kind: ReaderMetadata, Text: "metadata"},
		{Kind: ReaderNotice, Text: "notice"},
		{Kind: ReaderFold, Text: "future fold"},
	} {
		rendered := renderReaderRow(row, geometry, workspace.DiffHighlightBackground)
		if strings.Contains(rendered, "\x1b[41") || strings.Contains(rendered, "\x1b[42") {
			t.Fatalf("unchanged kind %d received fill: %q", row.Kind, rendered)
		}
	}
}

func TestFileDocumentChangeGutterStaysSidebarAndShowsDeletionBoundaries(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderFileDocument}
	if got := readerDiffHighlight(document, workspace.DiffHighlightBackground); got != workspace.DiffHighlightSidebar {
		t.Fatalf("file highlight = %v, want sidebar", got)
	}
	if got := readerDiffHighlight(ReaderDocument{Kind: ReaderDiffDocument}, workspace.DiffHighlightBackground); got != workspace.DiffHighlightBackground {
		t.Fatalf("diff highlight = %v, want requested background", got)
	}

	geometry := CalculateReaderGeometry(Rect{Width: 24, Height: 1}, document, false)
	tests := []struct {
		name string
		row  ReaderRow
		bar  string
	}{
		{name: "removed before", row: ReaderRow{Kind: ReaderFile, Text: "keep", NewLine: 2, RemovedBefore: 3}, bar: "▴"},
		{name: "removed after", row: ReaderRow{Kind: ReaderFile, Text: "keep", NewLine: 2, RemovedAfter: 3}, bar: "▾"},
		{name: "removed around", row: ReaderRow{Kind: ReaderFile, Text: "keep", NewLine: 2, RemovedBefore: 1, RemovedAfter: 1}, bar: "◆"},
		{name: "replacement", row: ReaderRow{Kind: ReaderInsertion, Text: "new", NewLine: 2, RemovedBefore: 1}, bar: "▀"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rendered := renderReaderRow(test.row, geometry, readerDiffHighlight(document, workspace.DiffHighlightBackground))
			if plain := ansi.Strip(rendered); !strings.HasPrefix(plain, test.bar) {
				t.Fatalf("rendered boundary = %q, want %q", plain, test.bar)
			}
			if !strings.Contains(rendered, "31") {
				t.Fatalf("deletion boundary is not ANSI red: %q", rendered)
			}
			if test.row.Kind == ReaderInsertion && !strings.Contains(rendered, "42") {
				t.Fatalf("replacement boundary does not retain ANSI green: %q", rendered)
			}
			if strings.Contains(rendered, "48;2") || strings.Contains(rendered, "48;5") {
				t.Fatalf("file marker used a fixed/indexed background: %q", rendered)
			}
		})
	}

	wrapped := renderReaderRowPart(
		ReaderRow{Kind: ReaderInsertion, Text: "continued", NewLine: 2, RemovedBefore: 1},
		geometry,
		workspace.DiffHighlightSidebar,
		true,
	)
	if strings.HasPrefix(ansi.Strip(wrapped), "▀") || strings.HasPrefix(ansi.Strip(wrapped), "▴") {
		t.Fatalf("wrapped continuation repeated deletion boundary: %q", wrapped)
	}
}

func TestRichReaderRenderMatrixAndScrollbarCoexistWithoutPanics(t *testing.T) {
	t.Parallel()
	document := ReaderDocument{Kind: ReaderDiffDocument}
	for index := 0; index < 40; index++ {
		kind := ReaderContext
		if index%3 == 1 {
			kind = ReaderDeletion
		} else if index%3 == 2 {
			kind = ReaderInsertion
		}
		document.Rows = append(document.Rows, ReaderRow{
			Kind: kind, Text: "row " + strconv.Itoa(index),
			OldLine: uint64(12_300 + index), NewLine: uint64(98_700 + index),
		})
	}
	for width := 0; width <= 100; width++ {
		for height := 0; height <= 16; height++ {
			for _, highlight := range []workspace.DiffHighlight{workspace.DiffHighlightSidebar, workspace.DiffHighlightBackground} {
				frame := Render(Model{
					Geometry: Calculate(width, height), Workspace: workspace.Files,
					ReaderDocument: document, ReaderOffset: 17, Focus: navigation.FocusReader,
					Controls: workspace.Controls{DiffHighlight: highlight, RichDiff: true},
				})
				gotWidth, gotHeight := lipgloss.Size(frame)
				if width > 0 && height > 0 && (gotWidth != width || gotHeight != height) {
					t.Fatalf("size %dx%d highlight %d rendered %dx%d", width, height, highlight, gotWidth, gotHeight)
				}
			}
		}
	}
	g := Calculate(80, 12)
	frame := ansi.Strip(Render(Model{
		Geometry: g, Workspace: workspace.Files, ReaderDocument: document,
		ReaderOffset: 17, Focus: navigation.FocusReader,
	}))
	for row := g.ReaderRows.Y; row < g.ReaderRows.Y+g.ReaderRows.Height; row++ {
		cells := []rune(strings.Split(frame, "\n")[row])
		if cell := cells[g.ReaderRows.X+g.ReaderRows.Width-1]; cell != '▕' && cell != '▐' {
			t.Fatalf("reader row %d scrollbar cell = %q", row, cell)
		}
	}
}

func BenchmarkReaderLayoutLargeFile(b *testing.B) {
	document := benchmarkReaderDocument(10_000)
	rows := Rect{Width: 100, Height: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = CalculateReaderLayout(rows, document)
	}
}

func BenchmarkRenderScrolledLargeFile(b *testing.B) {
	document := benchmarkReaderDocument(10_000)
	geometry := Calculate(160, 50)
	layout := CalculateReaderLayout(geometry.ReaderRows, document)
	model := Model{
		Geometry: geometry, Workspace: workspace.Files, Focus: navigation.FocusReader,
		ReaderDocument: document, ReaderLayout: &layout,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		model.ReaderOffset = index % (len(document.Rows) - geometry.ReaderRows.Height)
		_ = Render(model)
	}
}

func benchmarkReaderDocument(lines int) ReaderDocument {
	document := ReaderDocument{Kind: ReaderFileDocument, Rows: make([]ReaderRow, lines)}
	for index := range document.Rows {
		document.Rows[index] = ReaderRow{
			Kind:    ReaderFile,
			Text:    "func renderOneLargeSourceLine(value string) string { return strings.TrimSpace(value) }",
			NewLine: uint64(index + 1),
		}
	}
	return document
}

func assertEveryPrintableHasChangeBackground(t *testing.T, rendered string, kind ReaderRowKind) {
	t.Helper()
	want := 42
	if kind == ReaderDeletion {
		want = 41
	}
	background := 0
	for index := 0; index < len(rendered); {
		if rendered[index] == '\x1b' && index+2 < len(rendered) && rendered[index+1] == '[' {
			end := strings.IndexByte(rendered[index+2:], 'm')
			if end < 0 {
				t.Fatalf("unterminated SGR: %q", rendered[index:])
			}
			parameters := strings.Split(rendered[index+2:index+2+end], ";")
			for _, parameter := range parameters {
				value, _ := strconv.Atoi(parameter)
				switch value {
				case 0, 49:
					background = 0
				case 41, 42:
					background = value
				}
			}
			index += end + 3
			continue
		}
		_, size := utf8.DecodeRuneInString(rendered[index:])
		if background != want {
			t.Fatalf("printable cell at byte %d has background %d, want %d: %q", index, background, want, rendered)
		}
		index += size
	}
}
