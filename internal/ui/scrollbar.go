package ui

// Scrollbar is the shared paint and hit-test geometry for one vertical bar.
type Scrollbar struct {
	Content   Rect
	Track     Rect
	Thumb     Rect
	MaxOffset int
}

// CalculateScrollbar derives a proportional scrollbar for a row viewport.
// The boolean is false when all content fits or no lane is available.
func CalculateScrollbar(rows Rect, total, offset int) (Scrollbar, bool) {
	viewport := rows.Height
	track := scrollbarLane(rows)
	if track.Width == 0 || viewport <= 0 || total <= viewport {
		return Scrollbar{}, false
	}
	offset = clamp(offset, 0, total-viewport)
	thumbHeight := clamp(roundedScale(viewport, viewport, total), 1, viewport)
	travel := viewport - thumbHeight
	maxOffset := total - viewport
	thumbStart := roundedScale(offset, travel, maxOffset)
	content := rows
	content.Width--
	return Scrollbar{
		Content:   content,
		Track:     track,
		Thumb:     Rect{X: track.X, Y: track.Y + thumbStart, Width: 1, Height: thumbHeight},
		MaxOffset: maxOffset,
	}, true
}

func scrollbarLane(rows Rect) Rect {
	if rows.Width <= 1 || rows.Height <= 0 {
		return Rect{}
	}
	return Rect{X: rows.X + rows.Width - 1, Y: rows.Y, Width: 1, Height: rows.Height}
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
	return roundedScale(thumbTop, bar.MaxOffset, travel)
}

// roundedScale matches Herdr's nearest proportional rounding for non-negative
// scrollbar values.
func roundedScale(value, scale, denominator int) int {
	if denominator <= 0 {
		return 0
	}
	return (value*scale + denominator/2) / denominator
}

// verticalScrollbar paints one cell for every row in an already-calculated
// track. Paint and hit testing therefore consume the same Scrollbar geometry.
func verticalScrollbar(bar Scrollbar, focused bool) []string {
	thumbGlyph := "▕"
	thumbStyle := scrollbarUnfocusedThumbStyle
	if focused {
		thumbGlyph = "▐"
		thumbStyle = scrollbarFocusedThumbStyle
	}
	cells := make([]string, bar.Track.Height)
	for row := range cells {
		cells[row] = scrollbarTrackStyle.Render("▕")
		if bar.Thumb.Contains(bar.Thumb.X, bar.Track.Y+row) {
			cells[row] = thumbStyle.Render(thumbGlyph)
		}
	}
	return cells
}
