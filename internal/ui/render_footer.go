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
	}
	filesFooterEntries = []footerEntry{
		{key: "tab", label: "focus"},
		{key: "j/k", label: "move"},
		{key: "h/l", label: "less/more"},
		{key: "z", label: "swap"},
		{key: "x", label: "review"},
		{key: "R", label: "bounds"},
		{key: "X", label: "next gap"},
	}
)

func renderFooter(model Model) string {
	var content string
	if model.Workspace == workspace.Files && model.FooterWarning != "" {
		content = errorStyle.Render(SafeSingleLine(model.FooterWarning))
	} else {
		var entries []footerEntry
		switch {
		case model.Workspace == workspace.Notes:
			content = renderNotesFooter(model)
		case model.Workspace == workspace.Files:
			entries = filesFooterEntries
		case model.Workspace == workspace.Git && model.Controls.Git == workspace.GitStashes:
			entries = stashFooterEntries(model.ReaderContextFoldable)
		default:
			entries = standardFooterEntries
		}
		if content == "" {
			content = renderFooterEntries(entries)
		}
	}
	return renderFooterHelp(content, model.Geometry)
}

func renderFooterHelp(content string, geometry Geometry) string {
	footer := geometry.Footer
	help := geometry.FooterHelp
	if footer.Width <= 0 || footer.Height <= 0 {
		return ""
	}
	if help.Width == 0 {
		return fit(content, footer.Width)
	}
	contentWidth := max(0, help.X-footer.X-1)
	gap := max(0, help.X-footer.X-contentWidth)
	return fit(content, contentWidth) + strings.Repeat(" ", gap) + headerStyle.Render("?")
}

func renderNotesFooter(model Model) string {
	style := chromeStyle
	if model.NotesError {
		style = errorStyle
	}
	priorityStatus := model.NotesStatusPriority || model.NotesError
	status := style.Render(SafeSingleLine(model.NotesStatus))
	if priorityStatus {
		footer := status + renderFooterSeparator() + renderFooterEntry(footerEntry{key: "Esc", label: "Files"})
		if model.NotesHasWorktree {
			footer += renderFooterSeparator() + renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
		}
		return footer
	}
	footer := renderFooterEntry(footerEntry{key: "Esc", label: "Files"})
	if model.NotesHasWorktree {
		footer += renderFooterSeparator() + renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
	}
	return footer + renderFooterSeparator() + status
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
