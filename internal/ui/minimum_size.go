package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// MinimumWidth and MinimumHeight are the smallest dimensions that preserve
// the complete compact header and useful rows in both panes.
const (
	MinimumWidth  = 60
	MinimumHeight = 12
)

// MeetsMinimumSize reports whether the normal application surface fits.
func MeetsMinimumSize(width, height int) bool {
	return width >= MinimumWidth && height >= MinimumHeight
}

// RenderMinimumSize replaces the application surface while the terminal is
// too small, following btop's explicit current-versus-required presentation.
func RenderMinimumSize(width, height int) string {
	width = max(0, width)
	height = max(0, height)
	if width == 0 || height == 0 {
		return ""
	}
	message := []string{
		errorStyle.Bold(true).Render("terminal too small"),
		mutedStyle.Render("reviewr needs more room"),
		fmt.Sprintf("current  %d × %d", width, height),
		fmt.Sprintf("minimum  %d × %d", MinimumWidth, MinimumHeight),
		mutedStyle.Render("resize to continue  •  q quit"),
	}
	rows := make([]string, height)
	start := max(0, (height-len(message))/2)
	for row := range rows {
		line := ""
		if index := row - start; index >= 0 && index < len(message) {
			line = centerMinimumSizeLine(message[index], width)
		}
		rows[row] = fit(line, width)
	}
	return strings.Join(rows, "\n")
}

func centerMinimumSizeLine(line string, width int) string {
	line = ansi.Truncate(line, width, "")
	return strings.Repeat(" ", max(0, (width-lipgloss.Width(line))/2)) + line
}
