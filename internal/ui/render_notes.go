package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/notes"
)

func renderNotes(model Model) string {
	g := model.Geometry
	presentation := model.Notes
	document := presentation.Document
	rows := make([]string, 0, g.NotesRows.Height)
	bar, overflow := CalculateScrollbar(g.NotesRows, len(document.Rows), presentation.Top)
	textRows := g.NotesRows
	var scrollbar []string
	if overflow {
		textRows = bar.Content
		scrollbar = verticalScrollbar(bar, true)
	}
	cursorRow := document.RowForIndex(presentation.Cursor)
	for visible := 0; visible < g.NotesRows.Height; visible++ {
		rowIndex := presentation.Top + visible
		line := ""
		if rowIndex < len(document.Rows) {
			line = renderNotesRow(document.Rows[rowIndex], rowIndex == cursorRow, presentation, textRows.Width)
		}
		line = fit(line, textRows.Width)
		if overflow {
			line += scrollbar[visible]
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Body,
		g.NotesTitle,
		g.NotesRows,
		renderNotesTitle(g, model.NotesScope, model.NotesHasWorktree),
		rows,
	)
}

func renderNotesTitle(g Geometry, scope notes.Scope, hasWorktree bool) string {
	if !hasWorktree {
		return renderTitle("Notes", true)
	}
	width := min(g.NotesTitle.Width, g.NotesWorktreeScope.X+g.NotesWorktreeScope.Width-g.NotesTitle.X)
	value := []byte(strings.Repeat(" ", max(0, width)))
	paint := func(x int, label string) {
		for index := 0; index < len(label); index++ {
			position := x - g.NotesTitle.X + index
			if position >= 0 && position < len(value) {
				value[position] = label[index]
			}
		}
	}
	paint(g.NotesTitle.X, "Notes")
	paint(g.NotesProjectScope.X+1, "project")
	paint(g.NotesWorktreeScope.X+1, "worktree")
	selected := g.NotesProjectScope
	if scope == notes.Worktree {
		selected = g.NotesWorktreeScope
	}
	if selected.Width > 0 {
		value[selected.X-g.NotesTitle.X] = '['
		if selected.Width > 1 {
			value[selected.X-g.NotesTitle.X+selected.Width-1] = ']'
		}
	}
	var rendered strings.Builder
	isFocused := func(index int) bool {
		x := g.NotesTitle.X + index
		return x < g.NotesTitle.X+len("Notes") || selected.Contains(x, g.NotesTitle.Y)
	}
	for index := 0; index < len(value); {
		focused := isFocused(index)
		style := chromeStyle
		if focused {
			style = focusedTitleStyle
		}
		end := index + 1
		for end < len(value) && isFocused(end) == focused {
			end++
		}
		rendered.WriteString(style.Render(string(value[index:end])))
		index = end
	}
	return rendered.String()
}

func renderNotesRow(row notes.Row, cursorRow bool, presentation notes.Presentation, width int) string {
	var rendered strings.Builder
	for _, cell := range row.Cells {
		value := cell.Display
		selected := presentation.HasSelection && cell.Index >= presentation.SelectionStart && cell.Index < presentation.SelectionEnd
		cursor := cursorRow && cell.Index == presentation.Cursor
		switch {
		case cursor:
			value = headerStyle.Reverse(true).Render(value)
		case selected:
			value = selectionStyle(true).Render(value)
		case cell.Index >= 0 && cell.Index < len(presentation.Styles):
			style := presentation.Styles[cell.Index]
			value = renderTextStyle(value, TextStyle{
				Foreground: style.Foreground,
				Bold:       style.Bold,
				Italic:     style.Italic,
				Underline:  style.Underline,
			})
		}
		rendered.WriteString(value)
	}
	if cursorRow && presentation.Cursor == row.End && lipgloss.Width(rendered.String()) < width {
		rendered.WriteString(headerStyle.Reverse(true).Render(" "))
	}
	return rendered.String()
}
