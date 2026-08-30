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
	destinationScratch
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
	{name: "numbered auxiliary", description: "Scratch is global but remains in the numbered control family.", render: renderNumbered},
	{name: "drawer key", description: "Backtick treats Scratch like a temporary drawer instead of a workspace.", render: renderDrawer},
	{name: "notes key", description: "A semantic n binding favors discoverability over symmetry.", render: renderNotes},
	{name: "minimal rail", description: "No container; active state is typography plus a small marker.", render: renderRail},
}

// View renders a fixed-size development page.
func (model Model) View(width, height int) string {
	if model.page == labPageFolds {
		return model.viewFolds(width, height)
	}
	return model.viewSwitchers(width, height)
}

func (model Model) viewSwitchers(width, height int) string {
	width = max(0, width)
	height = max(0, height)
	lines := []string{
		title.Render("lab / switchers"),
		quiet.Render("j/k choose  •  h/l preview destination  •  1/2/3/4 change sample  •  0, `, or n opens Scratch"),
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

func renderNumbered(model Model) string {
	primary := "1 " + group(
		option("files", model.destination == destinationFiles, underlineSelection),
		option("git", model.destination == destinationGit, underlineSelection),
	)
	scratch := "0 " + group(option("scratch", model.destination == destinationScratch, underlineSelection))
	return primary + "  " + scratch + "  " + secondary(model)
}

func renderDrawer(model Model) string {
	primary := "1  " + option("files", model.destination == destinationFiles, bracketSelection) + "  " +
		option("git", model.destination == destinationGit, bracketSelection)
	scratch := quiet.Render("| ` ") + option("scratch", model.destination == destinationScratch, bracketSelection)
	return primary + "  " + scratch + "  " + secondary(model)
}

func renderNotes(model Model) string {
	primary := option("1 files", model.destination == destinationFiles, foregroundSelection) + "  " +
		option("git", model.destination == destinationGit, foregroundSelection)
	scratch := quiet.Render("| ") + option("n notes", model.destination == destinationScratch, foregroundSelection)
	return primary + "  " + scratch + "  " + secondary(model)
}

func renderRail(model Model) string {
	labels := []string{"files", "git", "scratch"}
	parts := make([]string, len(labels))
	for index, label := range labels {
		marker := "  "
		style := quiet
		if index == model.destination {
			marker = title.Render("> ")
			style = lipgloss.NewStyle().Bold(true).Foreground(accent)
		}
		parts[index] = marker + style.Render(label)
	}
	return strings.Join(parts, quiet.Render("  /  ")) + "  " + secondary(model)
}

type selectionKind uint8

const (
	underlineSelection selectionKind = iota
	bracketSelection
	foregroundSelection
)

func option(label string, active bool, kind selectionKind) string {
	if !active {
		return quiet.Render(label)
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(accent)
	switch kind {
	case underlineSelection:
		return style.Underline(true).Render(label)
	case bracketSelection:
		return title.Render("[" + label + "]")
	default:
		return style.Render(label)
	}
}

func group(options ...string) string {
	return quiet.Render("[ ") + strings.Join(options, "  ") + quiet.Render(" ]")
}

func secondary(model Model) string {
	sets := []string{"all", "changed"}
	readers := []string{"file", "diff"}
	return neutralControl("2", sets[model.fileSet], green) + "  " +
		neutralControl("3", readers[model.reader], purple) + "  " +
		neutralControl("4", comparisonLabels[model.comparison], yellow)
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
