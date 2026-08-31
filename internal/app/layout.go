package app

import (
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// layoutState owns user-controlled pane geometry. Once customized, each pane
// keeps its semantic width even when the panes swap sides.
type layoutState struct {
	navigatorWidth int
	customized     bool
	dragging       bool
	swapped        bool
}

func (state *layoutState) resize(width, height int) ui.Geometry {
	if !state.customized {
		return state.order(ui.Calculate(width, height))
	}
	geometry := ui.CalculateWithNavigatorWidth(width, height, state.navigatorWidth)
	state.navigatorWidth = geometry.Navigator.Width
	return state.order(geometry)
}

func (state *layoutState) startDrag() {
	state.dragging = true
}

func (state *layoutState) dragTo(column, width, height int) ui.Geometry {
	requestedNavigatorWidth := column
	if state.swapped {
		defaultGeometry := ui.Calculate(width, height)
		contentWidth := defaultGeometry.Navigator.Width + defaultGeometry.Reader.Width
		requestedNavigatorWidth = contentWidth - column
	}
	geometry := ui.CalculateWithNavigatorWidth(width, height, requestedNavigatorWidth)
	state.navigatorWidth = geometry.Navigator.Width
	state.customized = true
	return state.order(geometry)
}

func (state *layoutState) finishDrag() {
	state.dragging = false
}

func (state *layoutState) swap(width, height int) ui.Geometry {
	state.finishDrag()
	state.swapped = !state.swapped
	return state.resize(width, height)
}

func (state layoutState) order(geometry ui.Geometry) ui.Geometry {
	if state.swapped {
		return geometry.SwapPanes()
	}
	return geometry
}

func (m *Model) applyLayoutAction(action Action) {
	switch action.Kind {
	case Resize:
		m.geometry = m.layout.resize(action.Width, action.Height)
		m.updateGitGeometry()
		if !ui.MeetsMinimumSize(action.Width, action.Height) {
			m.layout.finishDrag()
			m.gitLayout.finish()
			m.scrollbar.finish()
			m.note.finishPointers()
		}
		m.resizeWorkspaceState()
		m.note.resize(m.geometry)
	case StartPaneResize:
		m.scrollbar.finish()
		if m.active == workspace.Git && action.GitDivider != ui.GitDividerNone {
			m.layout.finishDrag()
			m.gitLayout.start(action.GitDivider)
		} else {
			m.gitLayout.finish()
			m.layout.startDrag()
		}
	case ResizePanes:
		if m.active == workspace.Git && m.gitLayout.dragging != ui.GitDividerNone {
			m.dragGitDivider(action.X, action.Y)
		} else if m.layout.dragging {
			m.geometry = m.layout.dragTo(action.Position, m.geometry.Screen.Width, m.geometry.Screen.Height)
			m.resizeWorkspaceState()
		}
	case FinishPaneResize:
		m.layout.finishDrag()
		m.gitLayout.finish()
	case SwapPanes:
		if m.active == workspace.Git {
			return
		}
		m.scrollbar.finish()
		m.geometry = m.layout.swap(m.geometry.Screen.Width, m.geometry.Screen.Height)
		m.resizeWorkspaceState()
	case StartScrollbarDrag:
		m.layout.finishDrag()
		m.gitLayout.finish()
		if m.active == workspace.Git && action.GitFocus != 0 {
			m.scrollbar.startGit(action.GitFocus, action.Grab)
		} else {
			m.scrollbar.start(action.Pane, action.Grab)
		}
		m.dragScrollbarTo(action.Position)
	case DragScrollbar:
		m.dragScrollbarTo(action.Position)
	case FinishScrollbarDrag:
		m.scrollbar.finish()
	}
}

// resizeWorkspaceState preserves each workspace's semantic place while
// clamping only the coordinates made invalid by the new shared geometry.
func (m *Model) resizeWorkspaceState() {
	m.files.resizeMarkdownPreview(m.geometry.ReaderRows)
	m.files.resizeCommentComposer(m.geometry)
	m.files.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	if m.active == workspace.Git {
		switch {
		case m.controls.Git == workspace.GitStashes:
			m.stashes.place.EnsureSelectionVisible(m.gitGeometry.RailRows.Height)
			m.stashes.ensureFileVisible(m.gitGeometry.FilesRows.Height)
		case m.history.inspecting:
			m.history.inspection.place.EnsureSelectionVisible(m.gitGeometry.FilesRows.Height)
		default:
			m.history.sourcePlace.EnsureSelectionVisible(m.gitGeometry.RailRows.Height)
			m.history.timelinePlace.EnsureSelectionVisible(m.gitGeometry.ContentRows.Height)
		}
	}
	if len(m.files.restoredReaderRows) == 0 &&
		(m.files.reader.Kind != 0 || m.files.diff.Kind != 0 || m.files.displayedBounds != nil || m.files.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
	}
	if m.active == workspace.Git && m.controls.Git == workspace.GitHistory && m.history.inspecting &&
		(m.history.inspection.readerFileID != "" || m.history.inspection.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.history.inspection.place, m.history.inspection.readerDocument())
	}
	if m.gitStashesActive() && len(m.stashes.inspection.restoredReaderRows) == 0 &&
		(m.stashes.inspection.readerFileID != "" || m.stashes.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.stashes.inspection.place, m.stashes.readerDocument())
	}
}
