package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type scrollbarDragState struct {
	active     bool
	pane       navigation.Focus
	grabOffset int
	git        bool
	gitFocus   workspace.GitFocus
}

func (state *scrollbarDragState) start(pane navigation.Focus, grabOffset int) {
	state.active = true
	state.pane = pane
	state.grabOffset = grabOffset
	state.git = false
	state.gitFocus = 0
}

func (state *scrollbarDragState) startGit(focus workspace.GitFocus, grabOffset int) {
	state.active = true
	state.git = true
	state.gitFocus = focus
	state.grabOffset = grabOffset
}

func (state *scrollbarDragState) finish() {
	state.active = false
	state.git = false
	state.gitFocus = 0
}

func (state scrollbarDragState) offsetAt(model *Model, y int) (int, bool) {
	if state.git {
		rows, total, offset := model.gitScrollbarState(state.gitFocus)
		bar, ok := ui.CalculateScrollbar(rows, total, offset)
		if !ok {
			return 0, false
		}
		return bar.OffsetAt(y, state.grabOffset), true
	}
	place := model.activePlace()
	rows := model.geometry.NavigatorRows
	total := len(place.Items)
	offset := place.Top
	if state.pane == navigation.FocusReader {
		rows = model.geometry.ReaderRows
		total = model.activeReaderLineCount()
		offset = model.activeReaderVisualOffset()
	}
	bar, ok := ui.CalculateScrollbar(rows, total, offset)
	if !ok {
		return 0, false
	}
	return bar.OffsetAt(y, state.grabOffset), true
}

func (model *Model) gitScrollbarState(focus workspace.GitFocus) (ui.Rect, int, int) {
	switch focus {
	case workspace.GitSource:
		return model.gitGeometry.RailRows, len(model.history.sourceRows), model.history.sourcePlace.Top
	case workspace.GitTimeline:
		return model.gitGeometry.ContentRows, len(model.history.commits), model.history.timelinePlace.Top
	case workspace.GitStash:
		return model.gitGeometry.RailRows, len(model.stashes.stashes), model.stashes.place.Top
	case workspace.GitFiles:
		if model.controls.Git == workspace.GitStashes {
			return model.gitGeometry.FilesRows, len(model.stashes.inspection.files), model.stashes.inspection.place.Top
		}
		return model.gitGeometry.FilesRows, len(model.history.inspection.files), model.history.inspection.place.Top
	case workspace.GitDiff:
		return model.gitGeometry.ContentRows, model.activeReaderLineCount(), model.activeReaderVisualOffset()
	default:
		return ui.Rect{}, 0, 0
	}
}

func (model *Model) dragGitScrollbar(focus workspace.GitFocus, offset int) {
	model.setGitFocus(focus)
	switch focus {
	case workspace.GitSource:
		model.history.sourcePlace.Top = offset
	case workspace.GitTimeline:
		model.history.timelinePlace.Top = offset
	case workspace.GitStash:
		model.stashes.place.Top = offset
	case workspace.GitFiles:
		if model.controls.Git == workspace.GitStashes {
			model.stashes.inspection.place.Top = offset
		} else {
			model.history.inspection.place.Top = offset
		}
	case workspace.GitDiff:
		model.setActiveReaderVisualOffset(offset)
	}
}

func (model *Model) dragScrollbarTo(y int) {
	if !model.scrollbar.active {
		return
	}
	offset, ok := model.scrollbar.offsetAt(model, y)
	if !ok {
		return
	}
	if model.scrollbar.git {
		model.dragGitScrollbar(model.scrollbar.gitFocus, offset)
		return
	}
	place := model.activePlace()
	place.Focus = model.scrollbar.pane
	if model.scrollbar.pane == navigation.FocusReader {
		model.setActiveReaderVisualOffset(offset)
		return
	}
	place.Top = offset
}
