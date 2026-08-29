package app

import "github.com/josephembrey/reviewr/internal/ui"

// layoutState owns user-controlled pane geometry. Once customized, the split
// remains an absolute terminal column across ordinary redraws and resizes.
type layoutState struct {
	navigatorWidth int
	customized     bool
	dragging       bool
}

func (state *layoutState) resize(width, height int) ui.Geometry {
	if !state.customized {
		return ui.Calculate(width, height)
	}
	geometry := ui.CalculateWithNavigatorWidth(width, height, state.navigatorWidth)
	state.navigatorWidth = geometry.Navigator.Width
	return geometry
}

func (state *layoutState) startDrag() {
	state.dragging = true
}

func (state *layoutState) dragTo(column, width, height int) ui.Geometry {
	geometry := ui.CalculateWithNavigatorWidth(width, height, column)
	state.navigatorWidth = geometry.Navigator.Width
	state.customized = true
	return geometry
}

func (state *layoutState) finishDrag() {
	state.dragging = false
}
