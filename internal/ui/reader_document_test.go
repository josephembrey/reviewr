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
		}
		byteIndex := strings.Index(plain, payload)
		codeX := lipgloss.Width(plain[:byteIndex])
		if codeX != geometry.Prefix {
			t.Fatalf("kind %d code x = %d, want %d: %q", row.Kind, codeX, geometry.Prefix, plain)
		}
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[2], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "▌ 9999 ") {
		t.Fatalf("deletion gutter = %q", plain)
	}
	if plain := ansi.Strip(renderReaderRow(document.Rows[3], geometry, workspace.DiffHighlightSidebar)); !strings.HasPrefix(plain, "▌10000 ") {
		t.Fatalf("insertion gutter = %q", plain)
	}
	for _, index := range []int{4, 5, 6} {
		plain := ansi.Strip(renderReaderRow(document.Rows[index], geometry, workspace.DiffHighlightSidebar))
		if !strings.HasPrefix(plain, strings.Repeat(" ", geometry.Prefix)) {
			t.Fatalf("metadata/notice/fold fabricated gutter at row %d: %q", index, plain)
		}
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
