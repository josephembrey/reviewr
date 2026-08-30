package ui

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/workspace"
)

type footerEntry struct {
	key   string
	label string
}

var (
	standardFooterEntries = []footerEntry{
		{key: "tab", label: "focus"},
		{key: "j/k or ↑/↓", label: "navigate"},
		{key: "z", label: "swap"},
		{key: "r", label: "refresh"},
		{key: "q", label: "quit"},
	}
	filesFooterEntries = []footerEntry{
		{key: "tab", label: "focus"},
		{key: "j/k", label: "move"},
		{key: "h/l", label: "less/more"},
		{key: "z", label: "swap"},
		{key: "x", label: "review"},
		{key: "R", label: "bounds"},
		{key: "X", label: "next gap"},
		{key: "r", label: "refresh"},
		{key: "q", label: "quit"},
	}
)

func renderFooter(model Model) string {
	width := model.Geometry.Footer.Width
	if model.Workspace == workspace.Files && model.FooterWarning != "" {
		return fit(errorStyle.Render(SafeSingleLine(model.FooterWarning)), width)
	}
	var entries []footerEntry
	switch {
	case model.Workspace == workspace.Notes:
		return fit(renderNotesFooter(model), width)
	case model.Workspace == workspace.Files:
		entries = filesFooterEntries
	case model.Workspace == workspace.Git && model.Controls.Git == workspace.GitStashes:
		entries = stashFooterEntries(model.ReaderContextFoldable)
	default:
		entries = standardFooterEntries
	}
	return fit(renderFooterEntries(entries), width)
}

func renderNotesFooter(model Model) string {
	style := chromeStyle
	if model.NotesError {
		style = errorStyle
	}
	priorityStatus := model.NotesStatusPriority || model.NotesError
	status := style.Render(SafeSingleLine(model.NotesStatus))
	footer := renderFooterEntry(footerEntry{key: "Esc", label: "Files"})
	if priorityStatus {
		footer += renderFooterSeparator() + status
	}
	if model.NotesHasWorktree {
		footer += renderFooterSeparator() + renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
	}
	if !priorityStatus {
		footer += renderFooterSeparator() + status
	}
	return footer
}

func stashFooterEntries(contextFoldable bool) []footerEntry {
	entries := []footerEntry{
		{key: "tab", label: "focus"},
		{key: "j/k", label: "move stashes"},
		{key: "f/F", label: "move files"},
	}
	if contextFoldable {
		entries = append(entries, footerEntry{key: "h/l", label: "context"})
	}
	return append(entries,
		footerEntry{key: "z", label: "swap"},
		footerEntry{key: "r", label: "refresh"},
		footerEntry{key: "q", label: "quit"},
	)
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
