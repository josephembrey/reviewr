package ui

// NavigatorRowLayout is the shared paint and hit-test geometry for one row.
// Rectangles are relative to the navigator content row.
type NavigatorRowLayout struct {
	Label    Rect
	Progress Rect
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
