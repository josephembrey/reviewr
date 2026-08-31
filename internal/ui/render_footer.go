package ui

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/workspace"
)

type footerEntry struct {
	key   string
	label string
}

const hunkNavigationKey = "[/]"

func renderFooter(model Model) string {
	if model.Settings.Open {
		return fit(renderFooterEntry(footerEntry{key: ",/Esc", label: "close"}), model.Geometry.Footer.Width)
	}
	var content string
	if model.Workspace == workspace.Files && model.FooterWarning != "" {
		content = errorStyle.Render(SafeSingleLine(model.FooterWarning))
	} else {
		var entries []footerEntry
		switch {
		case model.Workspace == workspace.Notes:
			content = renderNotesFooter(model)
		case model.Workspace == workspace.Files:
			entries = fileFooterEntries(model)
		}
		if content == "" {
			content = renderFooterEntries(entries)
		}
	}
	return renderFooterHelp(content, model.Geometry)
}

func fileFooterEntries(model Model) []footerEntry {
	controls := model.Controls
	if model.ReaderComposingComment {
		return []footerEntry{
			{key: "enter", label: "save comment"},
			{key: "esc", label: "cancel"},
			{key: "alt+enter", label: "newline"},
		}
	}
	if model.ReaderVisualSelection {
		entries := []footerEntry{
			{key: "c", label: "comment range"},
			{key: "y", label: "copy"},
			{key: "esc", label: "cancel selection"},
		}
		if !model.ReaderCharacterSelection {
			entries = append(entries, footerEntry{key: "j/k", label: "extend"})
		}
		return append(entries, availableFileFooterEntries(controls, model.FileActions)...)
	}
	entries := make([]footerEntry, 0, 6)
	if model.ReaderCommentHeader {
		fold := footerEntry{key: "l", label: "expand comment"}
		if model.ReaderCommentExpanded {
			fold = footerEntry{key: "h", label: "collapse comment"}
		}
		entries = append(entries, fold)
	} else if model.ReaderCommentable {
		entries = append(entries,
			footerEntry{key: "V", label: "select lines"},
			footerEntry{key: "c", label: "comment"},
		)
	}
	return append(entries, availableFileFooterEntries(controls, model.FileActions)...)
}

func availableFileFooterEntries(controls workspace.Controls, actions FileFooterActions) []footerEntry {
	entries := make([]footerEntry, 0, 4)
	if controls.MarkdownPreviewEligible {
		label := "preview"
		if controls.MarkdownPreview {
			label = "source"
		}
		entries = append(entries, footerEntry{key: "m", label: label})
	}
	if actions.Review {
		entries = append(entries, footerEntry{key: "x", label: "review"})
	}
	if actions.ReviewBounds {
		entries = append(entries, footerEntry{key: "R", label: "bounds"})
	}
	if actions.NextGap {
		entries = append(entries, footerEntry{key: "X", label: "next gap"})
	}
	return entries
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
	if !model.NotesHasWorktree {
		return status
	}
	scope := renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
	if status == "" {
		return scope
	}
	if priorityStatus {
		return status + renderFooterSeparator() + scope
	}
	return scope + renderFooterSeparator() + status
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
	key := renderFooterKey(entry.key)
	if entry.label == "" {
		return key
	}
	return key + chromeStyle.Render(" "+SafeSingleLine(entry.label))
}

func renderFooterKey(key string) string {
	parts := strings.Split(SafeSingleLine(key), "/")
	var rendered strings.Builder
	for index, part := range parts {
		if index > 0 {
			rendered.WriteString(chromeStyle.Render("/"))
		}
		rendered.WriteString(headerStyle.Render(part))
	}
	return rendered.String()
}

func renderFooterSeparator() string {
	return mutedStyle.Render(" • ")
}
