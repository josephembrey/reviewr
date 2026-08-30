package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const helpPopupWidth = 60

type helpRow struct {
	section string
	entries []footerEntry
}

var helpRows = []helpRow{
	{
		section: "Browser",
		entries: []footerEntry{
			{key: "?", label: "help"},
			{key: "q/ctrl+c", label: "quit"},
			{key: "r", label: "refresh"},
		},
	},
	{
		entries: []footerEntry{
			{key: "g", label: "git"},
			{key: "n", label: "notes"},
			{key: "esc", label: "files"},
			{key: "tab", label: "focus"},
			{key: "z", label: "swap"},
		},
	},
	{
		section: "Files",
		entries: []footerEntry{
			{key: "1", label: "scope"},
			{key: "2", label: "file"},
			{key: "3", label: "base"},
			{key: "4", label: "diff"},
			{key: "m", label: "render"},
			{key: "x", label: "review"},
		},
	},
	{
		entries: []footerEntry{
			{key: "j/k/↑↓", label: "nav"},
			{key: "h/l/←→", label: "fold"},
			{key: hunkNavigationKey, label: "hunks"},
			{key: "R", label: "bounds"},
			{key: "X", label: "gap"},
		},
	},
	{
		entries: []footerEntry{
			{key: "home/end", label: "ends"},
			{key: "H/M/L", label: "view"},
			{key: "pgup/dn", label: "page"},
			{key: "e", label: "edit"},
		},
	},
	{
		section: "Git",
		entries: []footerEntry{
			{key: "1", label: "view"},
			{key: "2", label: "mode"},
			{key: "j/k/↑↓", label: "nav"},
			{key: "f/F", label: "files"},
			{key: "h/l", label: "fold"},
		},
	},
	{
		section: "Notes",
		entries: []footerEntry{
			{key: "esc", label: "files"},
			{key: "ctrl+t", label: "scope"},
			{key: "arrows", label: "move"},
			{key: "tab", label: "indent"},
		},
	},
	{
		entries: []footerEntry{
			{key: "shift+move", label: "select"},
			{key: "ctrl+a", label: "all"},
			{key: "ctrl+z/y", label: "undo/redo"},
		},
	},
	{
		entries: []footerEntry{
			{key: "ctrl+←/→", label: "word"},
			{key: "home/end", label: "line"},
			{key: "pgup/dn", label: "page"},
		},
	},
	{
		entries: []footerEntry{
			{key: "backspace/delete", label: "edit"},
			{key: "enter", label: "newline"},
		},
	},
}

func renderHelpOverlay(frame string, screen Rect) string {
	if !MeetsMinimumSize(screen.Width, screen.Height) {
		return frame
	}
	width := min(helpPopupWidth, screen.Width)
	popup := renderHelpPopup(width)
	popupWidth, popupHeight := lipgloss.Size(popup)
	x := max(0, (screen.Width-popupWidth)/2)
	y := max(0, (screen.Height-popupHeight)/2)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(frame),
		lipgloss.NewLayer(popup).X(x).Y(y).Z(1),
	).Render()
}

func renderHelpPopup(width int) string {
	if width <= 0 {
		return ""
	}
	if width < 3 {
		return fit(headerStyle.Render("?"), width)
	}

	innerWidth := width - 2
	caption := "─ hotkeys · ?/esc close "
	top := "╭" + caption + strings.Repeat("─", max(0, innerWidth-lipgloss.Width(caption))) + "╮"
	lines := make([]string, 0, len(helpRows)+2)
	lines = append(lines, readerFoldStyle.Render(top))
	for _, row := range helpRows {
		lines = append(lines,
			readerFoldStyle.Render("│")+fit(renderHelpRow(row), innerWidth)+readerFoldStyle.Render("│"),
		)
	}
	lines = append(lines, readerFoldStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

func renderHelpRow(row helpRow) string {
	line := fit(headerStyle.Render(row.section), 7)
	for index, entry := range row.entries {
		if index > 0 {
			line += "  "
		}
		line += renderFooterEntry(entry)
	}
	return line
}
