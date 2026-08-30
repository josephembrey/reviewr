package ui

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/workspace"
)

type footerEntry struct {
	key   string
	label string
}

func renderFooter(model Model) string {
	width := model.Geometry.Footer.Width
	if model.Workspace == workspace.Files && model.FooterWarning != "" {
		return fit(errorStyle.Render(SafeSingleLine(model.FooterWarning)), width)
	}
	if model.Workspace == workspace.Notes {
		style := chromeStyle
		if model.NotesError {
			style = errorStyle
		}
		priorityStatus := model.NotesStatusPriority || model.NotesError
		footer := renderFooterEntry(footerEntry{key: "Esc", label: "Files"})
		if priorityStatus {
			footer += renderFooterSeparator() + style.Render(SafeSingleLine(model.NotesStatus))
		}
		if model.NotesHasWorktree {
			footer += renderFooterSeparator() + renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
		}
		if !priorityStatus {
			footer += renderFooterSeparator() + style.Render(SafeSingleLine(model.NotesStatus))
		}
		return fit(footer, width)
	}

	entries := []footerEntry{
		{key: "j/k or ↑/↓", label: "navigate"},
		{key: "z", label: "swap"},
		{key: "r", label: "refresh"},
		{key: "q", label: "quit"},
	}
	if model.Workspace == workspace.Files {
		entries = []footerEntry{
			{key: "j/k", label: "move"},
			{key: "h/l", label: "less/more"},
			{key: "z", label: "swap"},
			{key: "x", label: "review"},
			{key: "R", label: "bounds"},
			{key: "X", label: "next gap"},
			{key: "r", label: "refresh"},
			{key: "q", label: "quit"},
		}
	}
	if model.Workspace == workspace.Git && model.Controls.Git == workspace.GitStashes {
		stashEntries := []footerEntry{
			{key: "j/k", label: "move stashes"},
			{key: "f/F", label: "move files"},
		}
		if model.ReaderContextFoldable {
			stashEntries = append(stashEntries, footerEntry{key: "h/l", label: "context"})
		}
		stashEntries = append(stashEntries,
			footerEntry{key: "z", label: "swap"},
			footerEntry{key: "r", label: "refresh"},
			footerEntry{key: "q", label: "quit"},
		)
		entries = stashEntries
	} else if model.Workspace == workspace.Git {
		entries = []footerEntry{
			{key: "j/k or ↑/↓", label: "navigate"},
			{key: "z", label: "swap"},
			{key: "r", label: "refresh"},
			{key: "q", label: "quit"},
		}
	}
	return fit(renderFooterEntries(entries), width)
}

func renderFooterEntries(entries []footerEntry) string {
	var rendered strings.Builder
	for index, entry := range entries {
		if index > 0 {
			rendered.WriteString(renderFooterSeparator())
		}
		rendered.WriteString(renderFooterEntry(entry))
	}
	return rendered.String()
}

func renderFooterEntry(entry footerEntry) string {
	key := headerStyle.Render(SafeSingleLine(entry.key))
	if entry.label == "" {
		return key
	}
	return key + chromeStyle.Render(" "+SafeSingleLine(entry.label))
}

func renderFooterSeparator() string {
	return mutedStyle.Render(" • ")
}
