package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/workspace"
)

var (
	headerControlStyles = map[HitKind]lipgloss.Style{
		HitTertiaryControl:      specialStyle,
		HitComparisonControl:    warningStyle,
		HitDiffHighlightControl: headerStyle,
	}
)

func renderHeader(model Model) string {
	g := model.Geometry
	switcher := renderWorkspaceSwitcher(g.HeaderSwitcher.Width, model.Workspace)
	left := switcher
	for _, control := range layoutHeaderControls(g, model.Workspace, model.Controls) {
		padding := strings.Repeat(" ", max(0, control.rect.X-lipgloss.Width(left)))
		left += padding + renderHeaderControl(control, g.Header.Width >= wideHeaderControls)
	}
	return fit(left, g.Header.Width)
}

func renderHeaderControl(control headerControl, wide bool) string {
	style := addedStyle
	if semanticStyle, ok := headerControlStyles[control.hit]; ok {
		style = semanticStyle
	}
	key := ""
	if wide {
		key = style.Bold(true).Render(control.key) + " "
	}
	return key + chromeStyle.Render("[") + style.Bold(true).Render(control.value) + chromeStyle.Render("]")
}

func renderWorkspaceSwitcher(width int, activeWorkspace workspace.Kind) string {
	var rendered strings.Builder
	rendered.WriteString(chromeStyle.Render("["))
	for index, item := range workspaceSwitcherItems {
		if index > 0 {
			rendered.WriteString(mutedStyle.Render(" | "))
		}
		if item.key != "" {
			rendered.WriteString(headerStyle.Render(item.key + " "))
		}
		if item.kind == activeWorkspace {
			rendered.WriteString(headerStyle.Render(item.label))
		} else {
			rendered.WriteString(chromeStyle.Render(item.label))
		}
	}
	rendered.WriteString(chromeStyle.Render("]"))
	return fit(rendered.String(), min(max(0, width), len(workspaceSwitcher)))
}
