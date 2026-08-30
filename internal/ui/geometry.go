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

const workspaceSwitcher = "files | git | notes"

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
	g.NotesTitle, g.NotesRows = surfaceRows(g.Body)
	g.NotesProjectScope = clipTo(g.NotesTitle, Rect{X: g.NotesTitle.X + 7, Y: g.NotesTitle.Y, Width: 9, Height: 1})
	g.NotesWorktreeScope = clipTo(g.NotesTitle, Rect{X: g.NotesTitle.X + 16, Y: g.NotesTitle.Y, Width: 10, Height: 1})
	// Notes uses the full row width while content fits. NotesBar is only
	// the potential lane; CalculateScrollbar decides whether it is reserved.
	g.NotesText = g.NotesRows
	g.NotesBar = scrollbarLane(g.NotesRows)
	return g
}

// workspaceSwitcherRect is the exact label-only paint and hit target inside
// the stable "files | git | notes" tab group.
func workspaceSwitcherRect(kind workspace.Kind) Rect {
	switch kind {
	case workspace.Git:
		return Rect{X: 8, Width: 3, Height: 1}
	case workspace.Notes:
		return Rect{X: 14, Width: 5, Height: 1}
	default:
		return Rect{Width: 5, Height: 1}
	}
}

// HitKind identifies mouse targets calculated from Geometry.
type HitKind uint8

const (
	HitNone HitKind = iota
	HitFilesWorkspace
	HitGitWorkspace
	HitNotesWorkspace
	HitSecondaryControl
	HitTertiaryControl
	HitComparisonControl
	HitDiffHighlightControl
	HitDivider
	HitNavigatorScrollbar
	HitReaderScrollbar
	HitNavigator
	HitNavigatorRow
	HitReader
	HitNotesProjectScope
	HitNotesWorktreeScope
	HitNotesText
	HitNotesScrollbar
)

// Hit is a mouse target. Index is meaningful only for HitNavigatorRow.
type Hit struct {
	Kind       HitKind
	Index      int
	GrabOffset int
}

// NotesHitTest resolves the full-width editor using the same explicit
// rectangles and scrollbar calculation used to paint it.
func (g Geometry) NotesHitTest(x, y, totalRows, offset int) Hit {
	return g.NotesHitTestWithScopes(x, y, totalRows, offset, false)
}

// NotesHitTestWithScopes adds the optional scope labels to the shared title
// geometry without making an absent primary-checkout switcher interactive.
func (g Geometry) NotesHitTestWithScopes(x, y, totalRows, offset int, hasWorktree bool) Hit {
	if g.HeaderFiles.Contains(x, y) {
		return Hit{Kind: HitFilesWorkspace}
	}
	if g.HeaderGit.Contains(x, y) {
		return Hit{Kind: HitGitWorkspace}
	}
	if g.HeaderNotes.Contains(x, y) {
		return Hit{Kind: HitNotesWorkspace}
	}
	if hasWorktree && g.NotesProjectScope.Contains(x, y) {
		return Hit{Kind: HitNotesProjectScope}
	}
	if hasWorktree && g.NotesWorktreeScope.Contains(x, y) {
		return Hit{Kind: HitNotesWorktreeScope}
	}
	if g.Header.Contains(x, y) || g.NotesTitle.Contains(x, y) {
		return Hit{Kind: HitNone}
	}
	if bar, ok := CalculateScrollbar(g.NotesRows, totalRows, offset); ok && bar.Track.Contains(x, y) {
		return Hit{Kind: HitNotesScrollbar, GrabOffset: bar.GrabOffset(y)}
	}
	if g.NotesText.Contains(x, y) {
		return Hit{Kind: HitNotesText}
	}
	return Hit{Kind: HitNone}
}

// HitTest resolves a cell using visible Navigator state. A visible row takes
// precedence over its containing pane.
func (g Geometry) HitTest(x, y int, active workspace.Kind, controls workspace.Controls, top, fileCount, readerOffset, readerLineCount int) Hit {
	if g.HeaderFiles.Contains(x, y) {
		return Hit{Kind: HitFilesWorkspace}
	}
	if g.HeaderGit.Contains(x, y) {
		return Hit{Kind: HitGitWorkspace}
	}
	if g.HeaderNotes.Contains(x, y) {
		return Hit{Kind: HitNotesWorkspace}
	}
	for _, control := range layoutHeaderControls(g, active, controls) {
		if control.rect.Contains(x, y) {
			return Hit{Kind: control.hit}
		}
	}
	if g.Header.Contains(x, y) {
		return Hit{Kind: HitNone}
	}
	if g.Divider.Contains(x, y) {
		if active != workspace.Notes {
			return Hit{Kind: HitDivider}
		}
		return Hit{Kind: HitNone}
	}
	if active != workspace.Notes {
		if bar, ok := CalculateScrollbar(g.NavigatorRows, fileCount, top); ok && bar.Track.Contains(x, y) {
			return Hit{Kind: HitNavigatorScrollbar, GrabOffset: bar.GrabOffset(y)}
		}
		if bar, ok := CalculateScrollbar(g.ReaderRows, readerLineCount, readerOffset); ok && bar.Track.Contains(x, y) {
			return Hit{Kind: HitReaderScrollbar, GrabOffset: bar.GrabOffset(y)}
		}
	}
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

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
