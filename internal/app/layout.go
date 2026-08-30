package app

import "github.com/josephembrey/reviewr/internal/ui"

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
		if !ui.MeetsMinimumSize(action.Width, action.Height) {
			m.layout.finishDrag()
			m.scrollbar.finish()
			m.note.finishPointers()
		}
		m.resizeWorkspaceState()
		m.note.resize(m.geometry)
	case StartPaneResize:
		m.scrollbar.finish()
		m.layout.startDrag()
	case ResizePanes:
		if m.layout.dragging {
			m.geometry = m.layout.dragTo(action.Position, m.geometry.Screen.Width, m.geometry.Screen.Height)
			m.resizeWorkspaceState()
		}
	case FinishPaneResize:
		m.layout.finishDrag()
	case SwapPanes:
		m.scrollbar.finish()
		m.geometry = m.layout.swap(m.geometry.Screen.Width, m.geometry.Screen.Height)
		m.resizeWorkspaceState()
	case StartScrollbarDrag:
		m.layout.finishDrag()
		m.scrollbar.start(action.Pane, action.Grab)
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
	m.files.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.history.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.refs.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.stashes.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	if len(m.files.restoredReaderRows) == 0 &&
		(m.files.reader.Kind != 0 || m.files.diff.Kind != 0 || m.files.displayedBounds != nil || m.files.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
	}
	if m.history.summary.OID != "" {
		m.history.place.ClampReader(len(commitSummaryLines(m.history.summary)), m.geometry.ReaderRows.Height)
	}
	m.refs.place.ClampReader(len(m.refs.commits), m.geometry.ReaderRows.Height)
	if len(m.stashes.restoredReaderRows) == 0 &&
		(m.stashes.readerFileID != "" || m.stashes.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
	}
}
