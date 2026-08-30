package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderPopupOverlay preserves the fixed application frame and centers one
// rounded card within the same screen geometry used by input routing.
func renderPopupOverlay(frame string, screen Rect, popup string) string {
	if !MeetsMinimumSize(screen.Width, screen.Height) || popup == "" {
		return frame
	}
	width, height := lipgloss.Size(popup)
	rect := centeredPopupRect(screen, width, height)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(frame),
		lipgloss.NewLayer(popup).X(rect.X).Y(rect.Y).Z(1),
	).Render()
}

func centeredPopupRect(screen Rect, width, height int) Rect {
	width = min(max(0, width), screen.Width)
	height = min(max(0, height), screen.Height)
	return Rect{
		X:      screen.X + max(0, (screen.Width-width)/2),
		Y:      screen.Y + max(0, (screen.Height-height)/2),
		Width:  width,
		Height: height,
	}
}

func renderPopupCard(width int, caption string, rows []string) string {
	if width <= 0 {
		return ""
	}
	if width < 3 {
		return fit(headerStyle.Render(caption), width)
	}

	innerWidth := width - 2
	caption = "─ " + caption + " "
	top := "╭" + caption + strings.Repeat("─", max(0, innerWidth-lipgloss.Width(caption))) + "╮"
	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, readerFoldStyle.Render(top))
	for _, row := range rows {
		lines = append(lines, readerFoldStyle.Render("│")+fit(row, innerWidth)+readerFoldStyle.Render("│"))
	}
	lines = append(lines, readerFoldStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}
