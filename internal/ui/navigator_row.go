package ui

import "strconv"

// NavigatorRowLayout is the shared paint and hit-test geometry for one row.
// Rectangles are relative to the navigator content row.
type NavigatorRowLayout struct {
	Label    Rect
	Progress Rect
	Changes  Rect
	Review   Rect
}

// LayoutNavigatorRow reserves independent right-side fields before the label
// is clipped. The review rectangle includes its separator and the complete
// fixed-width three-cell badge.
func LayoutNavigatorRow(row NavigatorRow, width int) NavigatorRowLayout {
	width = max(0, width)
	right := width
	layout := NavigatorRowLayout{}
	if row.Review != nil && right >= 4 {
		layout.Review = Rect{X: right - 4, Width: 4, Height: 1}
		right -= 4
	}
	if row.Changes != nil {
		additions, deletions := FormatLineChanges(*row.Changes)
		changesWidth := lineChangesWidth(additions, deletions)
		if changesWidth > 0 && changesWidth <= right {
			layout.Changes = Rect{X: right - changesWidth, Width: changesWidth, Height: 1}
			right -= changesWidth
		}
	}
	if row.Progress != "" {
		progressWidth := len(row.Progress) + 1
		if progressWidth <= right {
			layout.Progress = Rect{X: right - progressWidth, Width: progressWidth, Height: 1}
			right -= progressWidth
		}
	}
	layout.Label = Rect{Width: right, Height: 1}
	return layout
}

// FormatLineChanges returns only non-zero diff statistics. Every caller uses
// this shared rule so zero counts never consume space or appear in the UI.
func FormatLineChanges(changes LineChanges) (additions, deletions string) {
	if changes.Additions > 0 {
		additions = "+" + strconv.FormatUint(changes.Additions, 10)
	}
	if changes.Deletions > 0 {
		deletions = "-" + strconv.FormatUint(changes.Deletions, 10)
	}
	return additions, deletions
}

func lineChangesWidth(additions, deletions string) int {
	width := 0
	if additions != "" {
		width += len(additions) + 1
	}
	if deletions != "" {
		width += len(deletions) + 1
	}
	return width
}

// HitNavigatorReview resolves a click against the same content width and row
// layout used by rendering, including navigator-scrollbar reservation.
func (g Geometry) HitNavigatorReview(x, y, top int, rows []NavigatorRow) (int, bool) {
	if !g.NavigatorRows.Contains(x, y) {
		return 0, false
	}
	index := top + y - g.NavigatorRows.Y
	if index < 0 || index >= len(rows) {
		return 0, false
	}
	contentWidth := g.NavigatorRows.Width
	if _, ok := CalculateScrollbar(g.NavigatorRows, len(rows), top); ok {
		contentWidth--
	}
	layout := LayoutNavigatorRow(rows[index], contentWidth)
	review := layout.Review
	review.X += g.NavigatorRows.X
	review.Y = y
	if review.Contains(x, y) {
		return index, true
	}
	return 0, false
}
