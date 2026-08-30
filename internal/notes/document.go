package notes

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/highlight"
	"github.com/rivo/uniseg"
)

// Cell is one rendered grapheme span in a wrapped row. Index identifies the
// original grapheme; Display is safe terminal text with tabs expanded.
type Cell struct {
	Index   int
	Column  int
	Width   int
	Display string
}

// Row is a half-open grapheme interval in one visual terminal row.
type Row struct {
	Start     int
	End       int
	Cells     []Cell
	HardBreak bool
}

// BoundaryAt maps a display column to the nearest insertion boundary.
func (r Row) BoundaryAt(column int) int {
	column = max(0, column)
	for _, cell := range r.Cells {
		midpoint := cell.Column + (cell.Width+1)/2
		if column < midpoint {
			return cell.Index
		}
		if column < cell.Column+cell.Width {
			return cell.Index + 1
		}
	}
	return r.End
}

// ColumnAt returns the display column for a grapheme boundary in this row.
func (r Row) ColumnAt(index int) int {
	for _, cell := range r.Cells {
		if index <= cell.Index {
			return cell.Column
		}
	}
	if len(r.Cells) == 0 {
		return 0
	}
	last := r.Cells[len(r.Cells)-1]
	return last.Column + last.Width
}

// Document is the shared soft-wrap result used for painting and every form
// of navigation and hit testing.
type Document struct {
	Width int
	Rows  []Row
}

// Document builds the editor's current shared wrap document.
func (e Editor) Document() Document {
	return Wrap(e.graphemes, e.width)
}

// Wrap creates visual rows without splitting grapheme clusters.
func Wrap(graphemes []string, width int) Document {
	document := Document{Width: max(0, width)}
	limit := max(1, width)
	row := Row{Start: 0}
	column := 0
	appendRow := func(end int, hardBreak bool) {
		row.End = end
		row.HardBreak = hardBreak
		document.Rows = append(document.Rows, row)
	}
	for index, value := range graphemes {
		if value == "\n" {
			appendRow(index, true)
			row = Row{Start: index + 1}
			column = 0
			continue
		}
		cellWidth := displayWidth(value, column, limit)
		if column > 0 && column+cellWidth > limit {
			appendRow(index, false)
			row = Row{Start: index}
			column = 0
			cellWidth = displayWidth(value, column, limit)
		}
		display := value
		if value == "\t" {
			display = strings.Repeat(" ", cellWidth)
		} else if uniseg.StringWidth(value) <= 0 {
			display = "◌" + value
		} else if uniseg.StringWidth(value) > limit {
			display = "�"
			cellWidth = 1
		}
		row.Cells = append(row.Cells, Cell{Index: index, Column: column, Width: cellWidth, Display: display})
		column += cellWidth
	}
	appendRow(len(graphemes), false)
	return document
}

func displayWidth(value string, column, limit int) int {
	if value == "\t" {
		width := TabWidth - column%TabWidth
		return min(max(1, width), max(1, limit))
	}
	width := uniseg.StringWidth(value)
	if width <= 0 {
		return 1
	}
	return width
}

// RowForIndex finds the visual row for a grapheme boundary. At a soft-wrap
// boundary, the following row owns the cursor.
func (d Document) RowForIndex(index int) int {
	if len(d.Rows) == 0 {
		return 0
	}
	index = max(0, index)
	for rowIndex, row := range d.Rows {
		if index < row.End {
			return rowIndex
		}
		if index == row.End {
			if rowIndex+1 < len(d.Rows) && d.Rows[rowIndex+1].Start == index {
				return rowIndex + 1
			}
			return rowIndex
		}
	}
	return len(d.Rows) - 1
}

// Position returns visual row and display column for a grapheme boundary.
func (d Document) Position(index int) (int, int) {
	rowIndex := d.RowForIndex(index)
	return rowIndex, d.Rows[rowIndex].ColumnAt(index)
}

// Point resolves viewport-relative cell coordinates to the nearest grapheme
// boundary using the same rows painting consumes.
func (d Document) Point(x, y, scroll, viewportHeight int) int {
	if len(d.Rows) == 0 {
		return 0
	}
	visibleHeight := max(1, viewportHeight)
	y = clamp(y, 0, visibleHeight-1)
	rowIndex := clamp(scroll+y, 0, len(d.Rows)-1)
	return d.Rows[rowIndex].BoundaryAt(x)
}

// Presentation is an immutable snapshot consumed by UI rendering.
type Presentation struct {
	Document       Document
	Top            int
	Height         int
	Cursor         int
	SelectionStart int
	SelectionEnd   int
	HasSelection   bool
	// Styles is indexed by the same grapheme identities as the editor. It is
	// presentation-only: Document remains the sole wrapping and hit authority.
	Styles []highlight.Style
}

func (e Editor) Presentation() Presentation {
	start, end, selected := e.Selection()
	return Presentation{
		Document:       e.Document(),
		Top:            e.scroll,
		Height:         e.height,
		Cursor:         e.cursor,
		SelectionStart: start,
		SelectionEnd:   end,
		HasSelection:   selected,
	}
}
