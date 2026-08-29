package ui

// Rect is a half-open terminal rectangle: [X, X+Width) x [Y, Y+Height).
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Contains reports whether a terminal cell belongs to the half-open rectangle.
func (r Rect) Contains(x, y int) bool {
	return r.Width > 0 && r.Height > 0 && x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// Geometry is the single source of pane bounds for render and mouse routing.
type Geometry struct {
	Screen        Rect
	Header        Rect
	Navigator     Rect
	NavigatorRows Rect
	Reader        Rect
	ReaderRows    Rect
	Footer        Rect
}

// Calculate returns responsive pane geometry for a terminal size.
func Calculate(width, height int) Geometry {
	width = max(0, width)
	height = max(0, height)
	headerHeight := min(1, height)
	footerHeight := 0
	if height-headerHeight > 0 {
		footerHeight = 1
	}
	bodyHeight := max(0, height-headerHeight-footerHeight)

	navigatorWidth := splitNavigator(width)
	g := Geometry{
		Screen:    Rect{Width: width, Height: height},
		Header:    Rect{Width: width, Height: headerHeight},
		Navigator: Rect{Y: headerHeight, Width: navigatorWidth, Height: bodyHeight},
		Reader: Rect{
			X:      navigatorWidth,
			Y:      headerHeight,
			Width:  width - navigatorWidth,
			Height: bodyHeight,
		},
		Footer: Rect{Y: headerHeight + bodyHeight, Width: width, Height: footerHeight},
	}
	g.NavigatorRows = paneRows(g.Navigator)
	g.ReaderRows = paneRows(g.Reader)
	return g
}

// HitKind identifies mouse targets calculated from Geometry.
type HitKind uint8

const (
	HitNone HitKind = iota
	HitNavigator
	HitNavigatorRow
	HitReader
)

// Hit is a mouse target. Index is meaningful only for HitNavigatorRow.
type Hit struct {
	Kind  HitKind
	Index int
}

// HitTest resolves a cell using visible Navigator state. A visible row takes
// precedence over its containing pane.
func (g Geometry) HitTest(x, y, top, fileCount int) Hit {
	if g.NavigatorRows.Contains(x, y) {
		index := top + y - g.NavigatorRows.Y
		if index >= 0 && index < fileCount {
			return Hit{Kind: HitNavigatorRow, Index: index}
		}
		return Hit{Kind: HitNavigator}
	}
	if g.Navigator.Contains(x, y) {
		return Hit{Kind: HitNavigator}
	}
	if g.Reader.Contains(x, y) {
		return Hit{Kind: HitReader}
	}
	return Hit{Kind: HitNone}
}

func splitNavigator(width int) int {
	if width <= 1 {
		return width
	}
	if width < 40 {
		return width / 2
	}
	navigatorWidth := clamp(width/3, 24, 40)
	return min(navigatorWidth, width-16)
}

func paneRows(pane Rect) Rect {
	if pane.Width <= 2 || pane.Height <= 3 {
		return Rect{X: pane.X + min(1, pane.Width), Y: pane.Y + min(2, pane.Height)}
	}
	return Rect{X: pane.X + 1, Y: pane.Y + 2, Width: pane.Width - 2, Height: pane.Height - 3}
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
