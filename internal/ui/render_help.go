package ui

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
			{key: ",", label: "settings"},
			{key: "q/ctrl+c", label: "quit"},
			{key: "r", label: "refresh"},
		},
	},
	{
		entries: []footerEntry{
			{key: "g", label: "git"},
			{key: "n", label: "notes"},
			{key: "esc", label: "files"},
			{key: "tab", label: "pane"},
			{key: "z", label: "swap"},
			{key: "e", label: "edit"},
		},
	},
	{
		section: "Files",
		entries: []footerEntry{
			{key: "1/2/3/4", label: "views"},
			{key: "m", label: "render"},
			{key: "x", label: "review"},
			{key: "R", label: "compare"},
			{key: "X", label: "gap"},
		},
	},
	{
		entries: []footerEntry{
			{key: "j/k/↑↓", label: "nav"},
			{key: "h/l", label: "fold"},
			{key: hunkNavigationKey, label: "marks"},
			{key: "V", label: "lines"},
			{key: "c", label: "comment"},
		},
	},
	{
		entries: []footerEntry{
			{key: "home/end"},
			{key: "H/M/L", label: "view"},
			{key: "pgup/dn", label: "page"},
			{key: "alt+[/]", label: "scroll"},
		},
	},
	{
		section: "Git",
		entries: []footerEntry{
			{key: "1", label: "history/stashes"},
			{key: "2", label: "graph/parent"},
			{key: "tab", label: "focus"},
		},
	},
	{
		entries: []footerEntry{
			{key: "j/k/↑↓", label: "nav"},
			{key: "enter/l", label: "inspect"},
			{key: "h/esc", label: "back/fold"},
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
}

func renderHelpOverlay(frame string, screen Rect) string {
	width := min(helpPopupWidth, screen.Width)
	popup := renderHelpPopup(width)
	return renderPopupOverlay(frame, screen, popup)
}

func renderHelpPopup(width int) string {
	if width <= 0 {
		return ""
	}
	lines := make([]string, 0, len(helpRows))
	for _, row := range helpRows {
		lines = append(lines, renderHelpRow(row))
	}
	return renderPopupCard(width, "hotkeys · ?/esc close", lines)
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
