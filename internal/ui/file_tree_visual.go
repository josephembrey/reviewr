package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// treeRowStyleLayers is the narrow merge seam for later status and ignored metadata. Status owns
// only the reserved marker and an optional filename accent; it never replaces the filetype icon.
// Ignored is the stronger outer content layer, while selection remains a terminal background layer.
type treeRowStyleLayers struct {
	statusMarker string
	statusAccent treeStatusAccent
	ignored      bool
	selected     bool
	focused      bool
}

type treeStatusAccent uint8

const (
	treeStatusNone treeStatusAccent = iota
	treeStatusAdded
	treeStatusModified
	treeStatusDeleted
	treeStatusRenamed
	treeStatusUntracked
)

var treeStatusColors = [...]color.Color{
	treeStatusAdded:     lipgloss.BrightGreen,
	treeStatusModified:  lipgloss.BrightBlue,
	treeStatusDeleted:   lipgloss.BrightRed,
	treeStatusRenamed:   lipgloss.BrightMagenta,
	treeStatusUntracked: lipgloss.BrightGreen,
}

type resolvedTreeRowStyles struct {
	marker   lipgloss.Style
	icon     lipgloss.Style
	filename lipgloss.Style
	row      lipgloss.Style
}

func resolveTreeRowStyles(item NavigatorRow, icon fileTreeIcon, layers treeRowStyleLayers) resolvedTreeRowStyles {
	styles := resolvedTreeRowStyles{
		marker: mutedStyle,
		icon:   lipgloss.NewStyle().Foreground(fileTreeIconColor(icon.tone)),
	}
	if item.Directory {
		styles.marker = lipgloss.NewStyle().Foreground(directoryTreeColor)
		styles.filename = lipgloss.NewStyle().Foreground(directoryTreeColor).Bold(true)
	}
	if color, ok := treeStatusColor(layers.statusAccent); ok {
		styles.marker = lipgloss.NewStyle().Foreground(color)
		styles.filename = lipgloss.NewStyle().Foreground(color)
	}
	if layers.ignored {
		styles.marker = ignoredTreeStyle
		styles.icon = ignoredTreeStyle
		styles.filename = ignoredTreeStyle
	}
	if layers.selected {
		styles.row = treeSelectionStyle(layers.focused)
	}
	return styles
}

func treeSelectionStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Black).
		Background(lipgloss.White).
		Bold(focused)
}

func treeStatusColor(accent treeStatusAccent) (color.Color, bool) {
	if int(accent) >= len(treeStatusColors) || treeStatusColors[accent] == nil {
		return nil, false
	}
	return treeStatusColors[accent], true
}
