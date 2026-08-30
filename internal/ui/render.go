package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Render paints one fixed-size frame from the shared Geometry.
func Render(model Model) string {
	g := model.Geometry
	blocks := make([]string, 0, 3)
	if g.Header.Height > 0 {
		blocks = append(blocks, renderHeader(model))
	}
	if g.Body.Height > 0 {
		if model.Workspace == workspace.Notes {
			blocks = append(blocks, renderNotes(model))
		} else {
			navigator := renderNavigator(model)
			divider := renderDivider(g.Divider, model.DividerDragging)
			reader := renderReader(model)
			if g.Navigator.X < g.Reader.X {
				blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, navigator, divider, reader))
			} else {
				blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, reader, divider, navigator))
			}
		}
	}
	if g.Footer.Height > 0 {
		blocks = append(blocks, renderFooter(model))
	}
	if len(blocks) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func renderTitle(title string, focused bool) string {
	if focused {
		return focusedTitleStyle.Render(title)
	}
	return chromeStyle.Render(title)
}

func selectionStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().Reverse(true).Bold(focused)
}

func renderDivider(rect Rect, dragging bool) string {
	if rect.Width <= 0 || rect.Height <= 0 {
		return ""
	}
	style := chromeStyle
	if dragging {
		style = headerStyle
	}
	line := fit(style.Render("│"), rect.Width)
	return strings.Repeat(line+"\n", rect.Height-1) + line
}

func renderSurface(surface, titleRect, rowsRect Rect, title string, rows []string) string {
	if surface.Width <= 0 || surface.Height <= 0 {
		return ""
	}
	lines := make([]string, surface.Height)
	for index := range lines {
		lines[index] = strings.Repeat(" ", surface.Width)
	}
	if titleRect.Width > 0 && titleRect.Height > 0 {
		lines[titleRect.Y-surface.Y] = fit(title, titleRect.Width)
	}
	for index := 0; index < rowsRect.Height; index++ {
		row := ""
		if index < len(rows) {
			row = rows[index]
		}
		lines[rowsRect.Y-surface.Y+index] = fit(row, rowsRect.Width)
	}
	return strings.Join(lines, "\n")
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = clip(value, width)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func clip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(value)
}
