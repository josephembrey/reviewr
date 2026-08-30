package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

const readerContextFoldWidth = 13

type readerPaneTitleLayout struct {
	control    headerControl
	leftWidth  int
	titleWidth int
	fold       Rect
}

func renderNavigatorTitle(model Model, title string) string {
	control := layoutPaneHeaderControls(model.Geometry, model.Workspace, model.Controls).navigator
	return renderPaneTitle(model.Geometry.NavigatorTitle, title, model.Focus == navigation.FocusNavigator, control)
}

func renderPaneTitle(titleRect Rect, title string, focused bool, control headerControl) string {
	leftWidth := titleRect.Width
	if control.rect.Width > 0 {
		leftWidth = max(0, control.rect.X-titleRect.X-1)
	}
	left := fit(renderTitle(title, focused), leftWidth)
	if control.rect.Width == 0 {
		return left
	}
	gap := max(0, control.rect.X-titleRect.X-leftWidth)
	return left + strings.Repeat(" ", gap) + renderHeaderControl(control, true)
}

func layoutReaderPaneTitle(geometry Geometry, title string, foldable bool, active workspace.Kind, controls workspace.Controls) readerPaneTitleLayout {
	control := layoutPaneHeaderControls(geometry, active, controls).reader
	leftWidth := geometry.ReaderTitle.Width
	if control.rect.Width > 0 {
		leftWidth = max(0, control.rect.X-geometry.ReaderTitle.X-1)
	}
	layout := readerPaneTitleLayout{control: control, leftWidth: leftWidth, titleWidth: leftWidth}
	if !foldable || leftWidth < readerContextFoldWidth+2 {
		return layout
	}
	layout.titleWidth = min(lipgloss.Width(SafeSingleLine(title)), leftWidth-readerContextFoldWidth-1)
	layout.fold = Rect{
		X:      geometry.ReaderTitle.X + layout.titleWidth + 1,
		Y:      geometry.ReaderTitle.Y,
		Width:  readerContextFoldWidth,
		Height: geometry.ReaderTitle.Height,
	}
	return layout
}

// LayoutReaderContextFold returns the exact global context target shared by
// the reader-title renderer and app mouse routing.
func LayoutReaderContextFold(geometry Geometry, title string, foldable bool, active workspace.Kind, controls workspace.Controls) Rect {
	return layoutReaderPaneTitle(geometry, title, foldable, active, controls).fold
}
