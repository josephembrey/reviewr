package ui

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type readerPaneTitleLayout struct {
	control   headerControl
	leftWidth int
}

func renderNavigatorTitle(model Model, title string) string {
	control := layoutPaneHeaderControls(model.Geometry, model.Workspace, model.Controls).navigator
	focused := model.Focus == navigation.FocusNavigator
	content := renderTitle(title, focused)
	if model.Workspace == workspace.Files && model.Controls.Files == workspace.ChangedFiles && model.Changes.Ready {
		content = renderNavigatorChangeSummary(model.Changes, focused)
	}
	return renderPaneTitleContent(model.Geometry.NavigatorTitle, content, control)
}

func renderPaneTitle(titleRect Rect, title string, focused bool, control headerControl) string {
	return renderPaneTitleContent(titleRect, renderTitle(title, focused), control)
}

func renderPaneTitleContent(titleRect Rect, content string, control headerControl) string {
	leftWidth := titleRect.Width
	if control.rect.Width > 0 {
		leftWidth = max(0, control.rect.X-titleRect.X-1)
	}
	left := fit(content, leftWidth)
	if control.rect.Width == 0 {
		return left
	}
	gap := max(0, control.rect.X-titleRect.X-leftWidth)
	return left + strings.Repeat(" ", gap) + renderHeaderControl(control, true)
}

func layoutReaderPaneTitle(geometry Geometry, active workspace.Kind, controls workspace.Controls) readerPaneTitleLayout {
	control := layoutPaneHeaderControls(geometry, active, controls).reader
	leftWidth := geometry.ReaderTitle.Width
	if control.rect.Width > 0 {
		leftWidth = max(0, control.rect.X-geometry.ReaderTitle.X-1)
	}
	return readerPaneTitleLayout{control: control, leftWidth: leftWidth}
}
