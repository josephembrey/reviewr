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
