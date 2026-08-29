package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
)

type scrollbarDragState struct {
	active     bool
	pane       navigation.Focus
	grabOffset int
}

func (state *scrollbarDragState) start(pane navigation.Focus, grabOffset int) {
	state.active = true
	state.pane = pane
	state.grabOffset = grabOffset
}

func (state *scrollbarDragState) finish() {
	state.active = false
}

func (state scrollbarDragState) offsetAt(model *Model, y int) (int, bool) {
	place := model.activePlace()
	rows := model.geometry.NavigatorRows
	total := len(place.Items)
	offset := place.Top
	if state.pane == navigation.FocusReader {
		rows = model.geometry.ReaderRows
		total = model.activeReaderLineCount()
		offset = place.ReaderOffset
	}
	bar, ok := ui.CalculateScrollbar(rows, total, offset)
	if !ok {
		return 0, false
	}
	return bar.OffsetAt(y, state.grabOffset), true
}

func (model *Model) dragScrollbarTo(y int) {
	if !model.scrollbar.active {
		return
	}
	offset, ok := model.scrollbar.offsetAt(model, y)
	if !ok {
		return
	}
	place := model.activePlace()
	place.Focus = model.scrollbar.pane
	if model.scrollbar.pane == navigation.FocusReader {
		place.ReaderOffset = offset
		return
	}
	place.Top = offset
}
