package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/review"
)

func renderNavigator(model Model) string {
	g := model.Geometry
	rows := make([]string, 0, g.NavigatorRows.Height)
	title := model.NavigatorTitle
	visibleRows := g.NavigatorRows.Height
	bar, overflow := CalculateScrollbar(g.NavigatorRows, len(model.NavigatorRows), model.Top)
	contentWidth := g.NavigatorRows.Width
	var scrollbar []string
	if overflow {
		contentWidth = bar.Content.Width
		scrollbar = verticalScrollbar(bar, model.Focus == navigation.FocusNavigator)
	}
	commitRows := make([]commitrow.Row, 0, len(model.NavigatorRows))
	for _, row := range model.NavigatorRows {
		if row.Commit != nil {
			commitRows = append(commitRows, *row.Commit)
		}
	}
	commitColumns := commitrow.Measure(commitRows, contentWidth)
	now := time.Now()
	for row := 0; row < visibleRows; row++ {
		index := model.Top + row
		if index >= len(model.NavigatorRows) {
			if row == 0 && len(model.NavigatorRows) == 0 {
				rows = append(rows, renderLine(model.NavigatorEmpty))
			} else {
				rows = append(rows, "")
			}
			if overflow {
				rows[len(rows)-1] = fit(rows[len(rows)-1], contentWidth) + scrollbar[row]
			}
			continue
		}
		line := renderNavigatorPresentationRow(
			model.NavigatorRows[index],
			contentWidth,
			index == model.Selected,
			model.Focus == navigation.FocusNavigator,
			commitColumns,
			now,
		)
		if overflow {
			line += scrollbar[row]
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Navigator,
		g.NavigatorTitle,
		g.NavigatorRows,
		renderTitle(title, model.Focus == navigation.FocusNavigator),
		rows,
	)
}

func renderNavigatorPresentationRow(item NavigatorRow, width int, selected, focused bool, columns commitrow.Columns, now time.Time) string {
	if item.Commit != nil {
		return renderCommitRow(*item.Commit, columns, width, selected, focused, now)
	}
	if len(item.Prefix) != 0 || len(item.Suffix) != 0 {
		return renderCompactNavigatorRow(item, width, selected, focused)
	}
	if !item.Tree {
		return renderNavigatorRow(SafeSingleLine(item.Label), width, selected, focused)
	}
	marker, accent := treeNavigatorStatus(item.Status)
	return renderTreeNavigatorRow(item, width, treeRowStyleLayers{
		statusMarker: marker,
		statusAccent: accent,
		ignored:      item.Dimmed,
		selected:     selected,
		focused:      focused,
	})
}

func renderTreeNavigatorRow(item NavigatorRow, width int, layers treeRowStyleLayers) string {
	layout := LayoutNavigatorRow(item, width)
	depth := max(0, item.Depth)
	marker := " "
	icon := treeFileIcon(item.Label)
	label := SafeSingleLine(item.Label)
	if item.Directory {
		marker = "▸"
		if item.Expanded {
			marker = "▾"
		}
		icon = treeDirectoryIcon(item.Expanded)
		label += "/"
	} else if layers.statusMarker != "" {
		marker = fit(SafeSingleLine(layers.statusMarker), 1)
	}
	styles := resolveTreeRowStyles(item, icon, layers)
	selection := styles.row
	row := selection.Render(" "+strings.Repeat("  ", depth)) +
		styles.marker.Inherit(selection).Render(marker) + selection.Render(" ") +
		styles.icon.Inherit(selection).Render(icon.glyph) + selection.Render(" ") +
		styles.filename.Inherit(selection).Render(label)
	row = lipgloss.NewStyle().MaxWidth(layout.Label.Width).Render(row)
	row += selection.Render(strings.Repeat(" ", max(0, layout.Label.Width-lipgloss.Width(row))))
	if layout.Progress.Width > 0 {
		progress := " " + item.Progress
		row += chromeStyle.Inherit(selection).Render(fit(progress, layout.Progress.Width))
	}
	if layout.Changes.Width > 0 {
		additions, deletions := FormatLineChanges(*item.Changes)
		if additions != "" {
			row += selection.Render(" ") + addedStyle.Inherit(selection).Render(additions)
		}
		if deletions != "" {
			row += selection.Render(" ") + errorStyle.Inherit(selection).Render(deletions)
		}
	}
	if layout.Review.Width > 0 {
		badge := " " + item.Review.Badge()
		row += reviewBadgeStyle(*item.Review).Inherit(selection).Render(badge)
	}
	return row
}

func reviewBadgeStyle(state review.State) lipgloss.Style {
	switch state {
	case review.Reviewed:
		return addedStyle
	case review.Updated:
		return headerStyle
	case review.Partial:
		return yellowStyle
	case review.BasisChanged:
		return errorStyle
	default:
		return chromeStyle
	}
}

func treeNavigatorStatus(status NavigatorStatus) (string, treeStatusAccent) {
	switch status {
	case StatusModified:
		return "M", treeStatusModified
	case StatusAdded:
		return "A", treeStatusAdded
	case StatusDeleted:
		return "D", treeStatusDeleted
	case StatusRenamed:
		return "R", treeStatusRenamed
	case StatusUntracked:
		return "?", treeStatusUntracked
	case StatusIgnored:
		return "I", treeStatusNone
	default:
		return "", treeStatusNone
	}
}

func renderCompactNavigatorRow(item NavigatorRow, width int, selected, focused bool) string {
	prefix := renderSegments(item.Prefix)
	suffix := renderSegments(item.Suffix)
	label := SafeSingleLine(item.Label)
	row := prefix
	available := max(0, width-lipgloss.Width(prefix))
	labelWidth := lipgloss.Width(label)
	suffixWidth := lipgloss.Width(suffix)
	switch {
	case suffix == "":
		row += clip(label, available)
	case labelWidth+suffixWidth <= available:
		row += label + suffix
	case available < 28 || suffixWidth > available-12:
		row += clip(label, available)
	default:
		row += clip(label, available-suffixWidth) + suffix
	}
	row = fit(row, width)
	if !selected {
		return row
	}
	return selectionStyle(focused).Render(row)
}

func renderNavigatorRow(path string, width int, selected, focused bool) string {
	row := fit("  "+path, width)
	if !selected {
		return row
	}
	return selectionStyle(focused).Render(row)
}
