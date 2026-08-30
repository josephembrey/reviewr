package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func renderHeader(model Model) string {
	g := model.Geometry
	switcher := renderWorkspaceSwitcher(g.HeaderSwitcher.Width, model.Workspace)
	left := switcher
	for _, control := range layoutHeaderControls(g, model.Workspace, model.Controls) {
		padding := strings.Repeat(" ", max(0, control.rect.X-lipgloss.Width(left)))
		left += padding + renderHeaderControl(control, g.Header.Width >= wideHeaderControls)
	}
	if model.Workspace != workspace.Files || !model.Changes.Ready {
		return fit(left, g.Header.Width)
	}
	for _, summary := range []string{renderChangeSummary(model.Changes), renderChangeTotals(model.Changes)} {
		if summary == "" {
			continue
		}
		summaryX := g.Header.Width - lipgloss.Width(summary)
		minimumSummaryX := lipgloss.Width(left) + 2
		if summaryX >= minimumSummaryX {
			padding := strings.Repeat(" ", max(0, summaryX-lipgloss.Width(left)))
			return fit(left+padding+summary, g.Header.Width)
		}
	}
	return fit(left, g.Header.Width)
}

func renderHeaderControl(control headerControl, wide bool) string {
	style := addedStyle
	switch control.hit {
	case HitTertiaryControl:
		style = purpleStyle
	case HitComparisonControl:
		style = yellowStyle
	case HitDiffHighlightControl:
		style = headerStyle
	}
	key := ""
	if wide {
		key = style.Bold(true).Render(control.key) + " "
	}
	return key + chromeStyle.Render("[") + style.Bold(true).Render(control.value) + chromeStyle.Render("]")
}

func renderChangeSummary(summary ChangeSummary) string {
	result := chromeStyle.Render(fmt.Sprintf("%d changes", summary.Files))
	if totals := renderChangeTotals(summary); totals != "" {
		result += " " + totals
	}
	return result
}

func renderChangeTotals(summary ChangeSummary) string {
	additions, deletions := FormatLineChanges(LineChanges{
		Additions: summary.Additions,
		Deletions: summary.Deletions,
	})
	parts := make([]string, 0, 2)
	if additions != "" {
		parts = append(parts, addedStyle.Render(additions))
	}
	if deletions != "" {
		parts = append(parts, errorStyle.Render(deletions))
	}
	return strings.Join(parts, " ")
}

func renderWorkspaceSwitcher(width int, activeWorkspace workspace.Kind) string {
	labels := []struct {
		kind  workspace.Kind
		label string
	}{
		{workspace.Files, "files"},
		{workspace.Git, "git"},
		{workspace.Notes, "notes"},
	}
	var rendered strings.Builder
	rendered.WriteString(headerStyle.Render("tab"))
	rendered.WriteString(chromeStyle.Render(" ["))
	for index, item := range labels {
		if index > 0 {
			rendered.WriteString(mutedStyle.Render("|"))
		}
		if item.kind == activeWorkspace {
			rendered.WriteString(headerStyle.Render(item.label))
		} else {
			rendered.WriteString(chromeStyle.Render(item.label))
		}
	}
	rendered.WriteString(chromeStyle.Render("]"))
	return fit(rendered.String(), min(max(0, width), len(workspaceSwitcher)))
}
