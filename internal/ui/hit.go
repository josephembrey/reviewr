package ui

import "github.com/josephembrey/reviewr/internal/workspace"

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

type rectHitTarget struct {
	rect Rect
	kind HitKind
}

// NotesHitTest resolves the full-width editor using the same explicit
// rectangles and scrollbar calculation used to paint it.
func (g Geometry) NotesHitTest(x, y, totalRows, offset int) Hit {
	return g.NotesHitTestWithScopes(x, y, totalRows, offset, false)
}

// NotesHitTestWithScopes adds the optional scope labels to the shared title
// geometry without making an absent primary-checkout switcher interactive.
func (g Geometry) NotesHitTestWithScopes(x, y, totalRows, offset int, hasWorktree bool) Hit {
	if hit, ok := g.workspaceHit(x, y); ok {
		return hit
	}
	if hasWorktree {
		targets := [...]rectHitTarget{
			{rect: g.NotesProjectScope, kind: HitNotesProjectScope},
			{rect: g.NotesWorktreeScope, kind: HitNotesWorktreeScope},
		}
		if hit, ok := firstRectHit(x, y, targets[:]); ok {
			return hit
		}
	}
	if g.Header.Contains(x, y) || g.NotesTitle.Contains(x, y) {
		return Hit{Kind: HitNone}
	}
	if hit, ok := scrollbarHit(g.NotesRows, totalRows, offset, x, y, HitNotesScrollbar); ok {
		return hit
	}
	if g.NotesText.Contains(x, y) {
		return Hit{Kind: HitNotesText}
	}
	return Hit{Kind: HitNone}
}

// HitTest resolves a cell using visible Navigator state. A visible row takes
// precedence over its containing pane.
func (g Geometry) HitTest(x, y int, active workspace.Kind, controls workspace.Controls, top, fileCount, readerOffset, readerLineCount int) Hit {
	if hit, ok := g.headerHit(x, y, active, controls); ok {
		return hit
	}
	if g.Divider.Contains(x, y) {
		if active != workspace.Notes {
			return Hit{Kind: HitDivider}
		}
		return Hit{Kind: HitNone}
	}
	if active == workspace.Files {
		paneControls := layoutPaneHeaderControls(g, active, controls)
		for _, control := range [...]headerControl{paneControls.navigator, paneControls.reader} {
			if control.rect.Contains(x, y) {
				return Hit{Kind: control.hit}
			}
		}
	}
	if active != workspace.Notes {
		if hit, ok := scrollbarHit(g.NavigatorRows, fileCount, top, x, y, HitNavigatorScrollbar); ok {
			return hit
		}
		if hit, ok := scrollbarHit(g.ReaderRows, readerLineCount, readerOffset, x, y, HitReaderScrollbar); ok {
			return hit
		}
	}
	return g.surfaceHit(x, y, top, fileCount)
}

func (g Geometry) workspaceHit(x, y int) (Hit, bool) {
	targets := [...]rectHitTarget{
		{rect: g.HeaderFiles, kind: HitFilesWorkspace},
		{rect: g.HeaderGit, kind: HitGitWorkspace},
		{rect: g.HeaderNotes, kind: HitNotesWorkspace},
	}
	return firstRectHit(x, y, targets[:])
}

func (g Geometry) headerHit(x, y int, active workspace.Kind, controls workspace.Controls) (Hit, bool) {
	if hit, ok := g.workspaceHit(x, y); ok {
		return hit, true
	}
	for _, control := range layoutHeaderControls(g, active, controls) {
		if control.rect.Contains(x, y) {
			return Hit{Kind: control.hit}, true
		}
	}
	if g.Header.Contains(x, y) {
		return Hit{Kind: HitNone}, true
	}
	return Hit{}, false
}

func (g Geometry) surfaceHit(x, y, top, fileCount int) Hit {
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

func firstRectHit(x, y int, targets []rectHitTarget) (Hit, bool) {
	for _, target := range targets {
		if target.rect.Contains(x, y) {
			return Hit{Kind: target.kind}, true
		}
	}
	return Hit{}, false
}

func scrollbarHit(rows Rect, total, offset, x, y int, kind HitKind) (Hit, bool) {
	bar, ok := CalculateScrollbar(rows, total, offset)
	if !ok || !bar.Track.Contains(x, y) {
		return Hit{}, false
	}
	return Hit{Kind: kind, GrabOffset: bar.GrabOffset(y)}, true
}
