package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ReaderGeometry is the shared measurement for rich-reader paint, truncation,
// scrollbar coexistence, and future gutter/code mouse targets.
type ReaderGeometry struct {
	Rows       Rect
	Content    Rect
	ChangeBar  Rect
	LineNumber Rect
	Code       Rect
	Scrollbar  Rect
	Digits     int
	Prefix     int
}

// ReaderLayout maps semantic source rows onto the visual rows produced by
// wrapping them to the reader's current code width. Source starts are retained
// so application place state can stay anchored to a logical line while the
// pane width changes.
type ReaderLayout struct {
	Geometry ReaderGeometry
	Total    int
	document ReaderDocument
	starts   []int
}

// CalculateReaderLayout derives wrapping and scrollbar reservation together.
// A scrollbar can only narrow the code region, so one second pass is enough
// once wrapping makes the document taller than the viewport.
func CalculateReaderLayout(rows Rect, document ReaderDocument) ReaderLayout {
	geometry := CalculateReaderGeometry(rows, document, false)
	layout := calculateReaderLayout(geometry, document)
	if _, overflow := CalculateScrollbar(rows, layout.Total, 0); overflow {
		layout = calculateReaderLayout(CalculateReaderGeometry(rows, document, true), document)
	}
	return layout
}

func calculateReaderLayout(geometry ReaderGeometry, document ReaderDocument) ReaderLayout {
	starts := make([]int, len(document.Rows)+1)
	total := 0
	for index, row := range document.Rows {
		starts[index] = total
		total += readerWrapCount(row, geometry.Code.Width)
	}
	starts[len(document.Rows)] = total
	return ReaderLayout{Geometry: geometry, Total: total, document: document, starts: starts}
}

// VisualOffset maps a logical source row and source-cell column to the
// corresponding visual row, clamping stale place after content or width changes.
func (layout ReaderLayout) VisualOffset(source, column int) int {
	if len(layout.document.Rows) == 0 || layout.Total == 0 {
		return 0
	}
	source = clamp(source, 0, len(layout.document.Rows)-1)
	continuations := layout.starts[source+1] - layout.starts[source]
	continuation := 0
	if layout.Geometry.Code.Width > 0 {
		continuation = max(0, column) / layout.Geometry.Code.Width
	}
	continuation = clamp(continuation, 0, max(0, continuations-1))
	return layout.starts[source] + continuation
}

// SourceOffset maps a visual row back to stable logical place state.
func (layout ReaderLayout) SourceOffset(visual int) (source, column int) {
	if len(layout.document.Rows) == 0 || layout.Total == 0 {
		return 0, 0
	}
	visual = clamp(visual, 0, layout.Total-1)
	source = sort.Search(len(layout.document.Rows), func(index int) bool {
		return layout.starts[index+1] > visual
	})
	return source, (visual - layout.starts[source]) * layout.Geometry.Code.Width
}

// Row returns one wrapped visual segment and whether it continues its source
// row. Styling is sliced semantically so syntax colors survive wrapping.
func (layout ReaderLayout) Row(visual int) (ReaderRow, bool) {
	source, _ := layout.SourceOffset(visual)
	if source < 0 || source >= len(layout.document.Rows) {
		return ReaderRow{}, false
	}
	continuation := visual - layout.starts[source]
	row := sliceReaderRow(layout.document.Rows[source], continuation, layout.Geometry.Code.Width)
	return row, continuation > 0
}

func readerWrapCount(row ReaderRow, width int) int {
	if width <= 0 {
		return 1
	}
	payloadWidth := readerPayloadWidth(row)
	return max(1, (payloadWidth+width-1)/width)
}

func readerPayloadWidth(row ReaderRow) int {
	// Text is the semantic payload and spans are only its presentation. Measuring
	// it once avoids walking every syntax token on each frame.
	return ansi.StringWidth(SafeSingleLine(row.Text))
}

func sliceReaderRow(row ReaderRow, continuation, width int) ReaderRow {
	if width <= 0 {
		row.Text = ""
		row.Spans = nil
		return row
	}
	left := continuation * width
	right := left + width
	if len(row.Spans) == 0 {
		row.Text = ansi.Cut(SafeSingleLine(row.Text), left, right)
		return row
	}
	spans := make([]TextSpan, 0, len(row.Spans))
	position := 0
	var text strings.Builder
	for _, span := range row.Spans {
		value := SafeSingleLine(span.Text)
		spanWidth := ansi.StringWidth(value)
		start := max(0, left-position)
		end := min(spanWidth, right-position)
		if start < end {
			piece := span
			piece.Text = ansi.Cut(value, start, end)
			if piece.Text != "" {
				spans = append(spans, piece)
				text.WriteString(piece.Text)
			}
		}
		position += spanWidth
		if position >= right {
			break
		}
	}
	row.Text = text.String()
	row.Spans = spans
	return row
}

// CalculateReaderGeometry partitions a reader viewport into its semantic
// gutter, invariant code origin, and optional existing one-cell scrollbar
// lane. Tiny widths clip each rectangle without producing negative sizes.
func CalculateReaderGeometry(rows Rect, document ReaderDocument, scrollbar bool) ReaderGeometry {
	geometry := ReaderGeometry{Rows: rows, Digits: document.GutterDigits()}
	geometry.Prefix = 1 + geometry.Digits + 1
	geometry.Content = rows
	if scrollbar && rows.Width > 0 {
		geometry.Content.Width--
		geometry.Scrollbar = Rect{
			X: rows.X + rows.Width - 1, Y: rows.Y, Width: 1, Height: rows.Height,
		}
	}
	barWidth := min(1, geometry.Content.Width)
	geometry.ChangeBar = Rect{
		X: geometry.Content.X, Y: geometry.Content.Y,
		Width: barWidth, Height: geometry.Content.Height,
	}
	numberWidth := min(geometry.Digits, max(0, geometry.Content.Width-barWidth))
	geometry.LineNumber = Rect{
		X: geometry.ChangeBar.X + geometry.ChangeBar.Width, Y: geometry.Content.Y,
		Width: numberWidth, Height: geometry.Content.Height,
	}
	codeX := geometry.Content.X + min(geometry.Prefix, geometry.Content.Width)
	geometry.Code = Rect{
		X: codeX, Y: geometry.Content.Y,
		Width: max(0, geometry.Content.X+geometry.Content.Width-codeX), Height: geometry.Content.Height,
	}
	return geometry
}
