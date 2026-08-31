package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (m *Model) routeGitMessage(msg tea.Msg) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.routeGitKey(msg)
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		return m.routeGitClick(msg)
	case tea.MouseWheelMsg:
		return m.routeGitWheel(msg)
	case tea.MouseMotionMsg:
		mouse := msg.Mouse()
		if m.gitLayout.dragging != ui.GitDividerNone && mouse.Button == tea.MouseLeft {
			return Action{Kind: ResizePanes, X: mouse.X, Y: mouse.Y}, true
		}
		if m.scrollbar.active && mouse.Button == tea.MouseLeft {
			return Action{Kind: DragScrollbar, Position: mouse.Y}, true
		}
	case tea.MouseReleaseMsg:
		if m.gitLayout.dragging != ui.GitDividerNone {
			return Action{Kind: FinishPaneResize}, true
		}
		if m.scrollbar.active {
			return Action{Kind: FinishScrollbarDrag}, true
		}
	}
	return Action{}, false
}

func (m *Model) routeGitKey(msg tea.KeyPressMsg) (Action, bool) {
	key := msg.String()
	if action, ok := m.routeGitReaderJump(key); ok {
		return action, true
	}
	switch key {
	case "q", "ctrl+c":
		return Action{Kind: Quit}, true
	case workspace.SecondaryControlKey:
		return Action{Kind: ToggleSecondary}, true
	case workspace.TertiaryControlKey:
		if m.controls.Git == workspace.GitHistory {
			return Action{Kind: ToggleTertiary}, true
		}
	case workspace.DiffHighlightKey:
		if m.presentationControls().RichDiff {
			return Action{Kind: ToggleDiffHighlight}, true
		}
	case "g":
		return Action{Kind: ShowGit}, true
	case "n":
		return Action{Kind: ShowNotes}, true
	case "esc":
		if m.controls.Git == workspace.GitHistory && m.history.inspecting {
			return Action{Kind: BackGit}, true
		}
		return Action{Kind: ShowFiles}, true
	case "tab":
		return Action{Kind: FocusGitRegion, GitFocus: m.nextGitFocus()}, true
	case "r":
		return Action{Kind: Reload}, true
	case "j", "down":
		return Action{Kind: SelectNext}, true
	case "k", "up":
		return Action{Kind: SelectPrevious}, true
	case "enter":
		return Action{Kind: EnterGit}, true
	case "l", "right":
		return m.routeGitRight()
	case "h", "left":
		return m.routeGitLeft()
	case "]":
		if m.gitDiffFocused() {
			return Action{Kind: SelectNextLandmark}, true
		}
	case "[":
		if m.gitDiffFocused() {
			return Action{Kind: SelectPreviousLandmark}, true
		}
	}
	return Action{}, false
}

func (m *Model) routeGitReaderJump(key string) (Action, bool) {
	if !m.gitDiffFocused() {
		return Action{}, false
	}
	fullPage := max(1, m.gitGeometry.ContentRows.Height)
	switch key {
	case "home":
		return Action{Kind: SelectReaderBoundary, Amount: -1}, true
	case "end":
		return Action{Kind: SelectReaderBoundary, Amount: 1}, true
	case "H":
		return Action{Kind: SelectReaderViewport, Amount: -1}, true
	case "M":
		return Action{Kind: SelectReaderViewport}, true
	case "L":
		return Action{Kind: SelectReaderViewport, Amount: 1}, true
	case "pgup":
		return Action{Kind: MoveReaderPage, Amount: -fullPage}, true
	case "pgdown":
		return Action{Kind: MoveReaderPage, Amount: fullPage}, true
	}
	return Action{}, false
}

func (m *Model) routeGitRight() (Action, bool) {
	if m.controls.Git == workspace.GitStashes {
		switch m.stashes.focus {
		case workspace.GitStash:
			return Action{Kind: FocusGitRegion, GitFocus: workspace.GitFiles}, true
		case workspace.GitFiles:
			return Action{Kind: FocusGitRegion, GitFocus: workspace.GitDiff}, true
		case workspace.GitDiff:
			return Action{Kind: ExpandReaderFold}, true
		}
	}
	if !m.history.inspecting {
		if m.history.focus == workspace.GitSource {
			return Action{Kind: ExpandNavigatorSelection}, true
		}
		return Action{Kind: EnterGit}, true
	}
	if m.history.focus == workspace.GitFiles {
		return Action{Kind: FocusGitRegion, GitFocus: workspace.GitDiff}, true
	}
	return Action{Kind: ExpandReaderFold}, true
}

func (m *Model) routeGitLeft() (Action, bool) {
	if m.controls.Git == workspace.GitStashes {
		switch m.stashes.focus {
		case workspace.GitFiles:
			return Action{Kind: FocusGitRegion, GitFocus: workspace.GitStash}, true
		case workspace.GitDiff:
			if m.activeReaderFoldable() {
				return Action{Kind: CollapseReaderFold}, true
			}
			return Action{Kind: FocusGitRegion, GitFocus: workspace.GitFiles}, true
		}
		return Action{}, false
	}
	if !m.history.inspecting {
		if m.history.focus == workspace.GitSource {
			return Action{Kind: CollapseNavigatorSelection}, true
		}
		return Action{}, false
	}
	if m.history.focus == workspace.GitFiles {
		return Action{Kind: BackGit}, true
	}
	if m.activeReaderFoldable() {
		return Action{Kind: CollapseReaderFold}, true
	}
	return Action{Kind: FocusGitRegion, GitFocus: workspace.GitFiles}, true
}

func (m *Model) routeGitClick(msg tea.MouseClickMsg) (Action, bool) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return Action{}, false
	}
	if m.gitDiffVisible() && m.gitGeometry.ContentRows.Contains(mouse.X, mouse.Y) {
		if layout, ok := m.activeReaderLayout(); ok {
			offset := m.activeReaderVisualOffset()
			if source, hit := layout.SourceAt(mouse.X, mouse.Y, offset); hit {
				row, _ := layout.Row(offset + mouse.Y - layout.Geometry.Rows.Y)
				if identity, fold := row.ContextFoldIdentity(); fold {
					return Action{Kind: ToggleReaderFold, Identity: identity, Index: source, GitFocus: workspace.GitDiff}, true
				}
				return Action{Kind: SelectReaderLine, Index: source, GitFocus: workspace.GitDiff}, true
			}
		}
	}
	hit := m.gitGeometry.HitTest(mouse.X, mouse.Y, workspace.Git, m.presentationControls(), m.gitHitState())
	switch hit.Kind {
	case ui.HitFilesWorkspace:
		return Action{Kind: ShowFiles}, true
	case ui.HitGitWorkspace:
		return Action{Kind: ShowGit}, true
	case ui.HitNotesWorkspace:
		return Action{Kind: ShowNotes}, true
	case ui.HitSecondaryControl:
		return Action{Kind: ToggleSecondary}, true
	case ui.HitTertiaryControl:
		return Action{Kind: ToggleTertiary}, true
	case ui.HitDiffHighlightControl:
		return Action{Kind: ToggleDiffHighlight}, true
	case ui.HitDivider:
		return Action{Kind: StartPaneResize, GitDivider: hit.Divider}, true
	case ui.HitNavigatorScrollbar:
		return Action{Kind: StartScrollbarDrag, GitFocus: hit.Region, Position: mouse.Y, Grab: hit.GrabOffset}, true
	case ui.HitNavigatorRow:
		return Action{Kind: ActivateGitRow, GitFocus: hit.Region, Index: hit.Index}, true
	case ui.HitNavigator:
		return Action{Kind: FocusGitRegion, GitFocus: hit.Region}, true
	}
	return Action{}, false
}

func (m *Model) routeGitWheel(msg tea.MouseWheelMsg) (Action, bool) {
	mouse := msg.Mouse()
	hit := m.gitGeometry.HitTest(mouse.X, mouse.Y, workspace.Git, m.presentationControls(), m.gitHitState())
	if hit.Kind == ui.HitNone || hit.Kind == ui.HitDivider || hit.Region == 0 && !m.gitGeometry.Rail.Contains(mouse.X, mouse.Y) {
		return Action{}, false
	}
	delta := 0
	if mouse.Button == tea.MouseWheelUp {
		delta = -1
	} else if mouse.Button == tea.MouseWheelDown {
		delta = 1
	} else {
		return Action{}, false
	}
	if hit.Region == workspace.GitDiff {
		return Action{Kind: ScrollReader, Amount: delta * 3}, true
	}
	kind := SelectNext
	if delta < 0 {
		kind = SelectPrevious
	}
	return Action{Kind: kind, GitFocus: hit.Region}, true
}

func (m *Model) gitHitState() ui.GitHitState {
	state := ui.GitHitState{ReaderOffset: m.activeReaderVisualOffset(), ReaderRows: m.activeReaderLineCount()}
	if m.controls.Git == workspace.GitStashes {
		state.RailTop, state.RailCount = m.stashes.place.Top, len(m.stashes.stashes)
		state.FilesTop, state.FilesCount = m.stashes.inspection.place.Top, len(m.stashes.inspection.files)
		return state
	}
	if m.history.inspecting {
		state.FilesTop, state.FilesCount = m.history.inspection.place.Top, len(m.history.inspection.files)
		return state
	}
	state.RailTop, state.RailCount = m.history.sourcePlace.Top, len(m.history.sourceRows)
	state.ContentTop, state.ContentCount = m.history.timelinePlace.Top, len(m.history.commits)
	return state
}

func (m *Model) nextGitFocus() workspace.GitFocus {
	if m.controls.Git == workspace.GitStashes {
		switch m.stashes.focus {
		case workspace.GitStash:
			return workspace.GitFiles
		case workspace.GitFiles:
			return workspace.GitDiff
		default:
			return workspace.GitStash
		}
	}
	if m.history.inspecting {
		if m.history.focus == workspace.GitFiles {
			return workspace.GitDiff
		}
		return workspace.GitFiles
	}
	if m.history.focus == workspace.GitSource {
		return workspace.GitTimeline
	}
	return workspace.GitSource
}

func (m *Model) gitDiffVisible() bool {
	return m.controls.Git == workspace.GitStashes || m.history.inspecting
}

func (m *Model) gitDiffFocused() bool {
	if m.controls.Git == workspace.GitStashes {
		return m.stashes.focus == workspace.GitDiff
	}
	return m.history.inspecting && m.history.focus == workspace.GitDiff
}

func (m *Model) activeReaderFoldable() bool {
	document, ok := m.activeReaderDocument()
	if !ok || len(document.Rows) == 0 {
		return false
	}
	cursor := max(0, min(m.activePlace().ReaderCursor, len(document.Rows)-1))
	_, foldable := document.Rows[cursor].ContextFoldIdentity()
	return foldable
}
