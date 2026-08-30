//go:build dev

package lab

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	destinationFiles = iota
	destinationGit
	destinationNotes
)

const (
	fileSetAll = iota
	fileSetChanged
)

const (
	readerFile = iota
	readerDiff
)

const (
	labPageSwitchers = iota
	labPageFolds
	labPageANSIPalette
	labPageCount
)

var (
	accent  = lipgloss.Color("#7AA2F7")
	green   = lipgloss.Color("#9ECE6A")
	purple  = lipgloss.Color("#BB9AF7")
	yellow  = lipgloss.Color("#E0AF68")
	dim     = lipgloss.Color("#777777")
	quiet   = lipgloss.NewStyle().Foreground(dim)
	title   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	variant = lipgloss.NewStyle().Bold(true)
)

var comparisonLabels = []string{"uncommitted", "branch", "last-turn"}

type variantSpec struct {
	name        string
	description string
	render      func(Model) string
}

var variants = []variantSpec{
	{name: "top-level destinations", description: "The stable tab group keeps every destination visible and clickable.", render: renderDestinations},
	{name: "Files controls", description: "Files owns 4/5/6; key 7 remains reserved for eligible rich diffs.", render: renderFilesControls},
	{name: "Git controls", description: "Git owns 4 and conditionally 5 without changing destination keys.", render: renderGitControls},
	{name: "Notes help", description: "Notes advertises Esc home and scope help, never printable digit shortcuts.", render: renderNotesHelp},
}

// View renders a fixed-size development page.
func (model Model) View(width, height int) string {
	switch model.page {
	case labPageFolds:
		return model.viewFolds(width, height)
	case labPageANSIPalette:
		return model.viewANSIPalette(width, height)
	default:
		return model.viewSwitchers(width, height)
	}
}

func (model Model) viewSwitchers(width, height int) string {
	width = max(0, width)
	height = max(0, height)
	lines := []string{
		title.Render("lab / switchers"),
		quiet.Render("tab next page  •  j/k choose  •  h/l preview destination  •  1/2/3 destination  •  4/5/6 Files controls"),
		"",
	}
	for index, spec := range variants {
		marker := "  "
		nameStyle := quiet
		if index == model.selected {
			marker = title.Render("> ")
			nameStyle = variant
		}
		lines = append(lines,
			marker+nameStyle.Render(fmt.Sprintf("%c  %s", 'A'+index, spec.name)),
			"   "+spec.render(model),
			"   "+quiet.Render(spec.description),
			"",
		)
	}
	lines = append(lines, quiet.Render("ctrl+l or esc close  •  q quit"))

	return fitPage(lines, width, height)
}

func fitPage(lines []string, width, height int) string {
	fitted := make([]string, max(0, height))
	for row := range fitted {
		if row < len(lines) {
			fitted[row] = fit(lines[row], width)
		} else {
			fitted[row] = strings.Repeat(" ", width)
		}
	}
	return strings.Join(fitted, "\n")
}

func renderDestinations(model Model) string {
	return topTabs(model.destination)
}

func renderFilesControls(model Model) string {
	return topTabs(model.destination) + "  " + secondary(model) + "  " + neutralControl("7", "sidebar", accent)
}

func renderGitControls(model Model) string {
	return topTabs(model.destination) + "  " + neutralControl("4", "log", green) + "  " + neutralControl("5", "graph", purple)
}

func renderNotesHelp(model Model) string {
	return topTabs(model.destination) + "  " + title.Render("Esc") + quiet.Render(" Files • ") + title.Render("ctrl+t") + quiet.Render(" scope")
}

func option(label string, active bool) string {
	if !active {
		return quiet.Render(label)
	}
	return lipgloss.NewStyle().Reverse(true).Bold(true).Render(label)
}

func topTabs(destination int) string {
	return quiet.Render("[ ") + strings.Join([]string{
		option("files", destination == destinationFiles),
		option("git", destination == destinationGit),
		option("notes", destination == destinationNotes),
	}, quiet.Render(" | ")) + quiet.Render(" ]")
}

func secondary(model Model) string {
	sets := []string{"all", "changed"}
	readers := []string{"file", "diff"}
	return neutralControl("4", sets[model.fileSet], green) + "  " +
		neutralControl("5", readers[model.reader], purple) + "  " +
		neutralControl("6", comparisonLabels[model.comparison], yellow)
}

func neutralControl(key, value string, foreground color.Color) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(foreground)
	return style.Render(key) + quiet.Render(" [") + style.Render(value) + quiet.Render("]")
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "")
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}
