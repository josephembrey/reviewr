package ui

// Scrollbar is the shared paint and hit-test geometry for one vertical bar.
type Scrollbar struct {
	Track     Rect
	Thumb     Rect
	MaxOffset int
}

// CalculateScrollbar derives a proportional scrollbar for a row viewport.
// The boolean is false when all content fits or no lane is available.
func CalculateScrollbar(rows Rect, total, offset int) (Scrollbar, bool) {
	viewport := rows.Height
	if rows.Width <= 0 || viewport <= 0 || total <= viewport {
		return Scrollbar{}, false
	}
	offset = clamp(offset, 0, total-viewport)
	thumbHeight := max(1, viewport*viewport/total)
	travel := viewport - thumbHeight
	maxOffset := total - viewport
	thumbStart := 0
	if travel > 0 && maxOffset > 0 {
		thumbStart = (offset*travel + maxOffset/2) / maxOffset
	}
	track := Rect{X: rows.X + rows.Width - 1, Y: rows.Y, Width: 1, Height: viewport}
	return Scrollbar{
		Track:     track,
		Thumb:     Rect{X: track.X, Y: track.Y + thumbStart, Width: 1, Height: thumbHeight},
		MaxOffset: maxOffset,
	}, true
}

// GrabOffset preserves the pointer's position within the thumb. A track click
// grabs the thumb at its center so the view jumps toward the pointer.
func (bar Scrollbar) GrabOffset(y int) int {
	if bar.Thumb.Contains(bar.Thumb.X, y) {
		return y - bar.Thumb.Y
	}
	return bar.Thumb.Height / 2
}

// OffsetAt maps a dragged thumb position back into content coordinates.
func (bar Scrollbar) OffsetAt(y, grabOffset int) int {
	travel := bar.Track.Height - bar.Thumb.Height
	if travel <= 0 || bar.MaxOffset <= 0 {
		return 0
	}
	thumbTop := clamp(y-bar.Track.Y-grabOffset, 0, travel)
	return (thumbTop*bar.MaxOffset + travel/2) / travel
}

// verticalScrollbar returns one cell per viewport row. A nil slice means all
// content fits and no lane should be reserved.
func verticalScrollbar(viewport, total, offset int, focused bool) []string {
	barGeometry, ok := CalculateScrollbar(Rect{Width: 1, Height: viewport}, total, offset)
	if !ok {
		return nil
	}

	thumbStyle := dimStyle.Bold(true)
	if focused {
		thumbStyle = headerStyle
	}
	bar := make([]string, viewport)
	for row := range bar {
		bar[row] = dimStyle.Render("▕")
		if barGeometry.Thumb.Contains(barGeometry.Thumb.X, row) {
			bar[row] = thumbStyle.Render("▐")
		}
	}
	return bar
}
