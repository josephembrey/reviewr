package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/review"
)

type treeStatusPresentation struct {
	marker string
	accent treeStatusAccent
}

var (
	reviewBadgeStyles = [...]lipgloss.Style{
		review.Reviewed:     addedStyle,
		review.Updated:      headerStyle,
		review.Partial:      warningStyle,
		review.BasisChanged: errorStyle,
	}
	treeStatusPresentations = [...]treeStatusPresentation{
		StatusModified:  {marker: "M", accent: treeStatusModified},
		StatusAdded:     {marker: "A", accent: treeStatusAdded},
		StatusDeleted:   {marker: "D", accent: treeStatusDeleted},
		StatusRenamed:   {marker: "R", accent: treeStatusRenamed},
		StatusUntracked: {marker: "?", accent: treeStatusUntracked},
		StatusIgnored:   {marker: "I", accent: treeStatusNone},
	}
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
	commitColumns, now := measureNavigatorCommits(model.NavigatorRows, model.Top, visibleRows, contentWidth)
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
		renderNavigatorTitle(model, title),
		rows,
	)
}

func renderNavigatorChangeSummary(summary ChangeSummary, focused bool) string {
	result := renderTitle(fmt.Sprintf("%d changes", summary.Files), focused)
	additions, deletions := FormatLineChanges(LineChanges{
		Additions: summary.Additions,
		Deletions: summary.Deletions,
	})
	if additions != "" {
		result += " " + addedStyle.Render(additions)
	}
	if deletions != "" {
		result += " " + errorStyle.Render(deletions)
	}
	return result
}

// measureNavigatorCommits avoids scanning large file trees and refs lists.
// Commit columns still measure the complete commit set whenever one is visible.
func measureNavigatorCommits(rows []NavigatorRow, top, height, width int) (commitrow.Columns, time.Time) {
	start := clamp(top, 0, len(rows))
	end := clamp(top+height, start, len(rows))
	visibleCommit := false
	for _, row := range rows[start:end] {
		if row.Commit != nil {
			visibleCommit = true
			break
		}
	}
	if !visibleCommit {
		return commitrow.Columns{}, time.Time{}
	}
	commits := make([]commitrow.Row, 0, len(rows))
	for _, row := range rows {
		if row.Commit != nil {
			commits = append(commits, *row.Commit)
		}
	}
	return commitrow.Measure(commits, width), time.Now()
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
		additions, deletions := FormatLineChanges(item.Changes)
		if additions != "" {
			row += selection.Render(" ") + addedStyle.Inherit(selection).Render(additions)
		}
		if deletions != "" {
			row += selection.Render(" ") + errorStyle.Inherit(selection).Render(deletions)
		}
	}
	if layout.Review.Width > 0 {
		badge := " " + item.Review.Badge()
		row += reviewBadgeStyle(item.Review).Inherit(selection).Render(badge)
	}
	return row
}

func reviewBadgeStyle(state review.State) lipgloss.Style {
	if state != review.Unreviewed && int(state) < len(reviewBadgeStyles) {
		return reviewBadgeStyles[state]
	}
	return chromeStyle
}

func treeNavigatorStatus(status NavigatorStatus) (string, treeStatusAccent) {
	if int(status) >= len(treeStatusPresentations) {
		return "", treeStatusNone
	}
	presentation := treeStatusPresentations[status]
	return presentation.marker, presentation.accent
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
