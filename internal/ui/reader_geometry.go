package ui

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
