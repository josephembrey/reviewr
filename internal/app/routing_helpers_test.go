package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func routeNotesMessage(
	msg tea.Msg,
	geometry ui.Geometry,
	presentation notes.Presentation,
	selectionDragging, scrollbarDragging bool,
	scoped ...bool,
) (Action, bool) {
	hasWorktree := len(scoped) > 0 && scoped[0]
	return routeNotesInput(msg, notesRouteContext{
		geometry:          geometry,
		presentation:      presentation,
		selectionDragging: selectionDragging,
		scrollbarDragging: scrollbarDragging,
		hasWorktree:       hasWorktree,
	})
}

func routeMessage(
	msg tea.Msg,
	focus navigation.Focus,
	geometry ui.Geometry,
	active workspace.Kind,
	controls workspace.Controls,
	dividerDragging, scrollbarDragging bool,
	top, navigatorCount, readerOffset, readerLineCount int,
) (Action, bool) {
	return routeMessageWithRows(
		msg,
		focus,
		geometry,
		active,
		controls,
		dividerDragging,
		scrollbarDragging,
		top,
		navigatorCount,
		readerOffset,
		readerLineCount,
		nil,
	)
}

func routeMessageWithRows(
	msg tea.Msg,
	focus navigation.Focus,
	geometry ui.Geometry,
	active workspace.Kind,
	controls workspace.Controls,
	dividerDragging, scrollbarDragging bool,
	top, navigatorCount, readerOffset, readerLineCount int,
	rows []ui.NavigatorRow,
) (Action, bool) {
	return routeBrowserMessage(msg, browserRouteContext{
		focus:             focus,
		geometry:          geometry,
		active:            active,
		controls:          controls,
		dividerDragging:   dividerDragging,
		scrollbarDragging: scrollbarDragging,
		top:               top,
		navigatorCount:    navigatorCount,
		readerOffset:      readerOffset,
		readerLineCount:   readerLineCount,
		navigatorRows:     rows,
	})
}
