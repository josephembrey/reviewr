package ui

import (
	"github.com/josephembrey/reviewr/internal/workspace"
)

// GitLayoutKind selects the body arrangement without leaking application
// state into rendering or hit testing.
type GitLayoutKind uint8

const (
	GitHistoryLayout GitLayoutKind = iota
	GitCommitLayout
	GitStashesLayout
)

// GitDividerKind identifies one authored split in Git-specific geometry.
type GitDividerKind uint8

const (
	GitDividerNone GitDividerKind = iota
	GitPrimaryDivider
	GitSecondaryDivider
)

// GitWidths contains user-authored split widths. Zero selects responsive
// defaults; calculations clamp every value to usable terminal bounds.
type GitWidths struct {
	Rail  int
	Files int
}

// GitGeometry is the single source of truth for every Git surface and mouse
// target. Rail is sources in History and stashes in Stashes. Files is visible
// only while inspecting a commit or stash. Content is timeline or diff.
type GitGeometry struct {
	Base             Geometry
	Kind             GitLayoutKind
	Rail             Rect
	RailTitle        Rect
	RailRows         Rect
	PrimaryDivider   Rect
	Files            Rect
	FilesTitle       Rect
	FilesRows        Rect
	SecondaryDivider Rect
	Content          Rect
	ContentTitle     Rect
	ContentRows      Rect
	Status           Rect
	FilesStacked     bool
}

const gitMinimumSurface = 16

// CalculateGitGeometry derives responsive Git regions from the application's
// stable outer chrome.
func CalculateGitGeometry(base Geometry, kind GitLayoutKind, widths GitWidths) GitGeometry {
	g := GitGeometry{Base: base, Kind: kind}
	switch kind {
	case GitHistoryLayout:
		g.layoutHistory(widths)
	case GitCommitLayout:
		g.layoutCommit(widths)
	case GitStashesLayout:
		g.layoutStashes(widths)
	}
	return g
}

func (g *GitGeometry) layoutHistory(widths GitWidths) {
	body := g.Base.Body
	g.Rail, g.PrimaryDivider, g.Content = verticalGitSplit(body, widths.Rail, splitGitRail(body.Width))
	if body.Height >= 3 {
		g.Content.Height--
		g.Status = Rect{X: g.Content.X, Y: body.Y + body.Height - 1, Width: g.Content.Width, Height: 1}
	}
	g.RailTitle, g.RailRows = surfaceRows(g.Rail)
	g.ContentTitle, g.ContentRows = surfaceRows(g.Content)
}

func (g *GitGeometry) layoutCommit(widths GitWidths) {
	body := g.Base.Body
	if body.Width >= 72 {
		g.Files, g.SecondaryDivider, g.Content = verticalGitSplit(body, widths.Files, splitGitFiles(body.Width))
	} else {
		g.FilesStacked = true
		g.Files, g.SecondaryDivider, g.Content = horizontalGitSplit(body, widths.Files)
	}
	g.FilesTitle, g.FilesRows = surfaceRows(g.Files)
	g.ContentTitle, g.ContentRows = surfaceRows(g.Content)
}

func (g *GitGeometry) layoutStashes(widths GitWidths) {
	body := g.Base.Body
	var right Rect
	g.Rail, g.PrimaryDivider, right = verticalGitSplit(body, widths.Rail, splitGitRail(body.Width))
	g.RailTitle, g.RailRows = surfaceRows(g.Rail)
	if right.Width >= 56 {
		g.Files, g.SecondaryDivider, g.Content = verticalGitSplit(right, widths.Files, splitGitFiles(right.Width))
	} else {
		g.FilesStacked = true
		g.Files, g.SecondaryDivider, g.Content = horizontalGitSplit(right, widths.Files)
	}
	g.FilesTitle, g.FilesRows = surfaceRows(g.Files)
	g.ContentTitle, g.ContentRows = surfaceRows(g.Content)
}

func verticalGitSplit(bounds Rect, requested, fallback int) (Rect, Rect, Rect) {
	if bounds.Width <= 1 || bounds.Height <= 0 {
		return bounds, Rect{}, Rect{}
	}
	dividerWidth := 1
	available := bounds.Width - dividerWidth
	leftWidth := fallback
	if requested > 0 {
		leftWidth = requested
	}
	minimum := min(gitMinimumSurface, available/2)
	leftWidth = clamp(leftWidth, minimum, available-minimum)
	left := Rect{X: bounds.X, Y: bounds.Y, Width: leftWidth, Height: bounds.Height}
	divider := Rect{X: left.X + left.Width, Y: bounds.Y, Width: dividerWidth, Height: bounds.Height}
	right := Rect{X: divider.X + divider.Width, Y: bounds.Y, Width: available - leftWidth, Height: bounds.Height}
	return left, divider, right
}

func horizontalGitSplit(bounds Rect, requested int) (Rect, Rect, Rect) {
	if bounds.Height <= 2 || bounds.Width <= 0 {
		return bounds, Rect{}, Rect{}
	}
	available := bounds.Height - 1
	topHeight := requested
	if topHeight <= 0 {
		topHeight = max(3, available/3)
	}
	minimum := min(3, available/2)
	topHeight = clamp(topHeight, minimum, available-minimum)
	top := Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: topHeight}
	divider := Rect{X: bounds.X, Y: top.Y + top.Height, Width: bounds.Width, Height: 1}
	bottom := Rect{X: bounds.X, Y: divider.Y + 1, Width: bounds.Width, Height: available - topHeight}
	return top, divider, bottom
}

func splitGitRail(width int) int {
	if width < 64 {
		return max(1, width/3)
	}
	return clamp(width/4, 20, 30)
}

func splitGitFiles(width int) int {
	return clamp(width/3, 22, 36)
}

// GitHit carries both the generic target kind and the exact Git region that
// owns it. Region makes identical row/scrollbar kinds unambiguous.
type GitHit struct {
	Kind       HitKind
	Region     workspace.GitFocus
	Index      int
	GrabOffset int
	Divider    GitDividerKind
}

// GitHitState supplies only disposable list lengths and offsets needed to
// resolve visible rows and scrollbars.
type GitHitState struct {
	RailTop      int
	RailCount    int
	FilesTop     int
	FilesCount   int
	ContentTop   int
	ContentCount int
	ReaderOffset int
	ReaderRows   int
}

// HitTest resolves Git controls, dividers, scrollbars, rows, and surfaces in
// paint precedence order from this same geometry.
func (g GitGeometry) HitTest(x, y int, active workspace.Kind, controls workspace.Controls, state GitHitState) GitHit {
	if hit, ok := g.Base.headerHit(x, y, active, controls); ok {
		return GitHit{Kind: hit.Kind}
	}
	if g.PrimaryDivider.Contains(x, y) {
		return GitHit{Kind: HitDivider, Divider: GitPrimaryDivider}
	}
	if g.SecondaryDivider.Contains(x, y) {
		return GitHit{Kind: HitDivider, Divider: GitSecondaryDivider}
	}
	regions := g.hitRegions(state)
	for _, region := range regions {
		if hit, ok := scrollbarHit(region.rows, region.total, region.offset, x, y, HitNavigatorScrollbar); ok {
			return GitHit{Kind: hit.Kind, Region: region.focus, GrabOffset: hit.GrabOffset}
		}
	}
	for _, region := range regions {
		if region.rows.Contains(x, y) {
			index := region.offset + y - region.rows.Y
			if index >= 0 && index < region.total {
				return GitHit{Kind: HitNavigatorRow, Region: region.focus, Index: index}
			}
			return GitHit{Kind: HitNavigator, Region: region.focus}
		}
		if region.surface.Contains(x, y) {
			return GitHit{Kind: HitNavigator, Region: region.focus}
		}
	}
	return GitHit{Kind: HitNone}
}

type gitHitRegion struct {
	focus   workspace.GitFocus
	surface Rect
	rows    Rect
	offset  int
	total   int
}

func (g GitGeometry) hitRegions(state GitHitState) []gitHitRegion {
	switch g.Kind {
	case GitHistoryLayout:
		return []gitHitRegion{
			{focus: workspace.GitSource, surface: g.Rail, rows: g.RailRows, offset: state.RailTop, total: state.RailCount},
			{focus: workspace.GitTimeline, surface: g.Content, rows: g.ContentRows, offset: state.ContentTop, total: state.ContentCount},
		}
	case GitCommitLayout:
		return []gitHitRegion{
			{focus: workspace.GitFiles, surface: g.Files, rows: g.FilesRows, offset: state.FilesTop, total: state.FilesCount},
			{focus: workspace.GitDiff, surface: g.Content, rows: g.ContentRows, offset: state.ReaderOffset, total: state.ReaderRows},
		}
	default:
		return []gitHitRegion{
			{focus: workspace.GitStash, surface: g.Rail, rows: g.RailRows, offset: state.RailTop, total: state.RailCount},
			{focus: workspace.GitFiles, surface: g.Files, rows: g.FilesRows, offset: state.FilesTop, total: state.FilesCount},
			{focus: workspace.GitDiff, surface: g.Content, rows: g.ContentRows, offset: state.ReaderOffset, total: state.ReaderRows},
		}
	}
}
