package ui

import "github.com/josephembrey/reviewr/internal/workspace"

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
	Screen             Rect
	Header             Rect
	HeaderSwitcher     Rect
	HeaderFiles        Rect
	HeaderGit          Rect
	HeaderNotes        Rect
	Body               Rect
	Navigator          Rect
	NavigatorTitle     Rect
	NavigatorRows      Rect
	Divider            Rect
	Reader             Rect
	ReaderTitle        Rect
	ReaderContextFold  Rect
	ReaderRows         Rect
	NotesTitle         Rect
	NotesProjectScope  Rect
	NotesWorktreeScope Rect
	NotesRows          Rect
	NotesText          Rect
	NotesBar           Rect
	Footer             Rect
}

// MinimumPaneWidth is the draggable split's preferred lower bound. Geometry
// only relaxes it for tiny component-test surfaces below the app minimum.
const MinimumPaneWidth = 16

// readerContextFoldWidth is the terminal-cell width of both global context
// controls: "▸ all context" and "▾ all context".
const readerContextFoldWidth = 13

const workspaceSwitcher = "[files | g git | n notes]"

var workspaceSwitcherItems = [...]struct {
	kind  workspace.Kind
	key   string
	label string
}{
	{kind: workspace.Files, label: "files"},
	{kind: workspace.Git, key: "g", label: "git"},
	{kind: workspace.Notes, key: "n", label: "notes"},
}

// Calculate returns responsive pane geometry with the default split.
func Calculate(width, height int) Geometry {
	return calculate(width, height, 0, false)
}

// CalculateWithNavigatorWidth returns geometry using a user-selected split,
// clamped so both panes remain usable.
func CalculateWithNavigatorWidth(width, height, navigatorWidth int) Geometry {
	return calculate(width, height, navigatorWidth, true)
}

// SwapPanes moves Navigator and Reader to the opposite sides while preserving
// each surface's width. Calling it twice restores the original geometry.
func (g Geometry) SwapPanes() Geometry {
	if g.Navigator.X <= g.Reader.X {
		g.Reader.X = g.Body.X
		g.Divider.X = g.Reader.X + g.Reader.Width
		g.Navigator.X = g.Divider.X + g.Divider.Width
	} else {
		g.Navigator.X = g.Body.X
		g.Divider.X = g.Navigator.X + g.Navigator.Width
		g.Reader.X = g.Divider.X + g.Divider.Width
	}
	g.NavigatorTitle, g.NavigatorRows = surfaceRows(g.Navigator)
	g.ReaderTitle, g.ReaderRows = surfaceRows(g.Reader)
	g.ReaderContextFold = readerContextFoldRect(g.ReaderTitle)
	return g
}

func calculate(width, height, requestedNavigatorWidth int, customized bool) Geometry {
	width = max(0, width)
	height = max(0, height)
	headerHeight := min(1, height)
	footerHeight := 0
	if height-headerHeight > 0 {
		footerHeight = 1
	}
	bodyHeight := max(0, height-headerHeight-footerHeight)

	body := Rect{Y: headerHeight, Width: width, Height: bodyHeight}
	dividerWidth := 0
	contentWidth := width
	if width >= 3 && bodyHeight > 0 {
		dividerWidth = 1
		contentWidth--
	}
	navigatorWidth := splitNavigator(contentWidth)
	if customized {
		navigatorWidth = clampNavigatorWidth(contentWidth, requestedNavigatorWidth)
	}
	readerWidth := contentWidth - navigatorWidth
	g := Geometry{
		Screen:    Rect{Width: width, Height: height},
		Header:    Rect{Width: width, Height: headerHeight},
		Body:      body,
		Navigator: Rect{Y: body.Y, Width: navigatorWidth, Height: body.Height},
		Divider: Rect{
			X:      navigatorWidth,
			Y:      body.Y,
			Width:  dividerWidth,
			Height: body.Height,
		},
		Reader: Rect{
			X:      navigatorWidth + dividerWidth,
			Y:      body.Y,
			Width:  readerWidth,
			Height: body.Height,
		},
		Footer: Rect{Y: body.Y + body.Height, Width: width, Height: footerHeight},
	}
	g.HeaderSwitcher = clipTo(g.Header, Rect{Width: len(workspaceSwitcher), Height: 1})
	g.HeaderFiles = clipTo(g.Header, workspaceSwitcherRect(workspace.Files))
	g.HeaderGit = clipTo(g.Header, workspaceSwitcherRect(workspace.Git))
	g.HeaderNotes = clipTo(g.Header, workspaceSwitcherRect(workspace.Notes))
	g.NavigatorTitle, g.NavigatorRows = surfaceRows(g.Navigator)
	g.ReaderTitle, g.ReaderRows = surfaceRows(g.Reader)
	g.ReaderContextFold = readerContextFoldRect(g.ReaderTitle)
	g.NotesTitle, g.NotesRows = surfaceRows(g.Body)
	g.NotesProjectScope = clipTo(g.NotesTitle, Rect{X: g.NotesTitle.X + 7, Y: g.NotesTitle.Y, Width: 9, Height: 1})
	g.NotesWorktreeScope = clipTo(g.NotesTitle, Rect{X: g.NotesTitle.X + 16, Y: g.NotesTitle.Y, Width: 10, Height: 1})
	// Notes uses the full row width while content fits. NotesBar is only
	// the potential lane; CalculateScrollbar decides whether it is reserved.
	g.NotesText = g.NotesRows
	g.NotesBar = scrollbarLane(g.NotesRows)
	return g
}

// workspaceSwitcherRect is the exact item paint and hit target inside the
// stable workspace control. Shortcut prefixes belong to their item target.
func workspaceSwitcherRect(kind workspace.Kind) Rect {
	position := 1 // opening bracket
	for index, item := range workspaceSwitcherItems {
		if index > 0 {
			position += len(" | ")
		}
		width := len(item.label)
		if item.key != "" {
			width += len(item.key) + 1
		}
		if item.kind == kind {
			return Rect{X: position, Width: width, Height: 1}
		}
		position += width
	}
	return Rect{}
}

func clipTo(bounds, rect Rect) Rect {
	rightBound := bounds.X + bounds.Width
	bottomBound := bounds.Y + bounds.Height
	left := min(max(bounds.X, rect.X), rightBound)
	top := min(max(bounds.Y, rect.Y), bottomBound)
	right := max(left, min(rightBound, rect.X+rect.Width))
	bottom := max(top, min(bottomBound, rect.Y+rect.Height))
	return Rect{X: left, Y: top, Width: max(0, right-left), Height: max(0, bottom-top)}
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

func clampNavigatorWidth(width, requested int) int {
	if width <= 1 {
		return width
	}
	minimum := min(MinimumPaneWidth, width/2)
	return clamp(requested, minimum, width-minimum)
}

func surfaceRows(surface Rect) (Rect, Rect) {
	titleHeight := min(1, surface.Height)
	title := Rect{X: surface.X, Y: surface.Y, Width: surface.Width, Height: titleHeight}
	rows := Rect{
		X:      surface.X,
		Y:      surface.Y + titleHeight,
		Width:  surface.Width,
		Height: surface.Height - titleHeight,
	}
	return title, rows
}

func readerContextFoldRect(title Rect) Rect {
	// Keep one cell of title and one separating space at the minimum visible
	// width. Normal application panes are wider than this; tiny test surfaces
	// simply omit the optional control.
	if title.Height == 0 || title.Width < readerContextFoldWidth+2 {
		return Rect{}
	}
	return Rect{
		X:      title.X + title.Width - readerContextFoldWidth,
		Y:      title.Y,
		Width:  readerContextFoldWidth,
		Height: title.Height,
	}
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
