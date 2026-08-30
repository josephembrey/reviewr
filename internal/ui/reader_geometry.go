package ui

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
	wraps    []readerRange
}

type readerRange struct {
	left  int
	right int
}

// CalculateReaderLayout derives wrapping and scrollbar reservation together.
// A scrollbar can only narrow the code region, so one second pass is enough
// once wrapping makes the document taller than the viewport.
func CalculateReaderLayout(rows Rect, document ReaderDocument) ReaderLayout {
	// Every source row occupies at least one visual row. Reserve the scrollbar
	// immediately when source height already proves overflow, avoiding a full
	// throwaway wrap pass for ordinary large files.
	if rows.Width > 1 && rows.Height > 0 && len(document.Rows) > rows.Height {
		return calculateReaderLayout(CalculateReaderGeometry(rows, document, true), document)
	}
	geometry := CalculateReaderGeometry(rows, document, false)
	layout := calculateReaderLayout(geometry, document)
	if _, overflow := CalculateScrollbar(rows, layout.Total, 0); overflow {
		layout = calculateReaderLayout(CalculateReaderGeometry(rows, document, true), document)
	}
	return layout
}

func calculateReaderLayout(geometry ReaderGeometry, document ReaderDocument) ReaderLayout {
	starts := make([]int, len(document.Rows)+1)
	wraps := make([]readerRange, 0, len(document.Rows))
	for index, row := range document.Rows {
		starts[index] = len(wraps)
		value := SafeSingleLine(row.Text)
		if row.Kind == ReaderFold || row.Kind == ReaderFoldEnd {
			// Fold controls are painted across the full content row and clipped
			// there, so they always occupy exactly one visual row.
			wraps = append(wraps, readerRange{right: ansi.StringWidth(value)})
		} else {
			wraps = appendReaderWrapRanges(wraps, value, geometry.Code.Width)
		}
	}
	starts[len(document.Rows)] = len(wraps)
	return ReaderLayout{Geometry: geometry, Total: len(wraps), document: document, starts: starts, wraps: wraps}
}

// VisualOffset maps a logical source row and source-cell column to the
// corresponding visual row, clamping stale place after content or width changes.
func (layout ReaderLayout) VisualOffset(source, column int) int {
	if len(layout.document.Rows) == 0 || layout.Total == 0 {
		return 0
	}
	source = clamp(source, 0, len(layout.document.Rows)-1)
	ranges := layout.wraps[layout.starts[source]:layout.starts[source+1]]
	continuation := sort.Search(len(ranges), func(index int) bool {
		return ranges[index].right > max(0, column)
	})
	continuation = clamp(continuation, 0, max(0, len(ranges)-1))
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
	return source, layout.wraps[visual].left
}

// Row returns one wrapped visual segment and whether it continues its source
// row. Styling is sliced semantically so syntax colors survive wrapping.
func (layout ReaderLayout) Row(visual int) (ReaderRow, bool) {
	row, _, continuation := layout.RowWithSource(visual)
	return row, continuation
}

// RowWithSource returns one wrapped segment together with its logical row.
func (layout ReaderLayout) RowWithSource(visual int) (ReaderRow, int, bool) {
	source, _ := layout.SourceOffset(visual)
	if source < 0 || source >= len(layout.document.Rows) {
		return ReaderRow{}, -1, false
	}
	continuation := visual - layout.starts[source]
	wrapped := layout.wraps[visual]
	row := sliceReaderRow(layout.document.Rows[source], wrapped.left, wrapped.right)
	return row, source, continuation > 0
}

// SourceAt maps one painted terminal cell back to its logical document row.
// visualOffset is the same viewport offset used by renderReader.
func (layout ReaderLayout) SourceAt(x, y, visualOffset int) (int, bool) {
	if !layout.Geometry.Content.Contains(x, y) || layout.Total == 0 {
		return 0, false
	}
	visual := visualOffset + y - layout.Geometry.Rows.Y
	if visual < 0 || visual >= layout.Total {
		return 0, false
	}
	source, _ := layout.SourceOffset(visual)
	return source, true
}

// readerWrapRanges prefers whitespace and common code punctuation, then falls
// back to a hard cell boundary only for a single chunk wider than the pane.
// Every source cell remains represented exactly once.
func appendReaderWrapRanges(ranges []readerRange, value string, width int) []readerRange {
	if width <= 0 {
		return append(ranges, readerRange{})
	}
	if value == "" {
		return append(ranges, readerRange{})
	}
	segmentByte, segmentCell := 0, 0
	for segmentByte < len(value) {
		limit := segmentCell + width
		position, index := segmentCell, segmentByte
		endByte, endCell := segmentByte, segmentCell
		breakByte, breakCell := -1, -1
		for index < len(value) {
			cluster, clusterWidth := firstReaderCluster(value[index:])
			if position+clusterWidth > limit && endByte > segmentByte {
				break
			}
			index += len(cluster)
			position += clusterWidth
			endByte, endCell = index, position
			if readerBreakAfter(cluster) {
				breakByte, breakCell = endByte, endCell
			}
			if position >= limit {
				break
			}
		}
		if endByte == len(value) {
			ranges = append(ranges, readerRange{left: segmentCell, right: endCell})
			break
		}
		if breakByte > segmentByte {
			endByte, endCell = breakByte, breakCell
		}
		if endByte == segmentByte {
			cluster, clusterWidth := firstReaderCluster(value[segmentByte:])
			endByte += len(cluster)
			endCell += clusterWidth
		}
		ranges = append(ranges, readerRange{left: segmentCell, right: endCell})
		segmentByte, segmentCell = endByte, endCell
	}
	return ranges
}

func firstReaderCluster(value string) (string, int) {
	if value[0] < utf8.RuneSelf && (len(value) == 1 || value[1] < utf8.RuneSelf) {
		return value[:1], 1
	}
	return ansi.FirstGraphemeCluster(value, ansi.GraphemeWidth)
}

func readerBreakAfter(cluster string) bool {
	r, _ := utf8.DecodeRuneInString(cluster)
	return unicode.IsSpace(r) || strings.ContainsRune(".,;:/\\|=+-_*&?!%#@()[]{}<>", r)
}

func sliceReaderRow(row ReaderRow, left, right int) ReaderRow {
	if right <= left {
		row.Text = ""
		row.Spans = nil
		return row
	}
	if len(row.Spans) == 0 {
		if row.Styled != "" {
			row.Styled = ansi.Cut(row.Styled, left, right)
			row.Text = ansi.Strip(row.Styled)
			return row
		}
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
	geometry.Content = rows
	if scrollbar && rows.Width > 0 {
		geometry.Content.Width--
		geometry.Scrollbar = Rect{
			X: rows.X + rows.Width - 1, Y: rows.Y, Width: 1, Height: rows.Height,
		}
	}
	if document.Kind == ReaderMarkdownDocument {
		geometry.Code = geometry.Content
		return geometry
	}
	geometry.Prefix = 1 + geometry.Digits + 1
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
