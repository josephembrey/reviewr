//go:build dev

package lab

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type foldVariantSpec struct {
	name        string
	description string
	compact     []foldPreviewRow
	expanded    []foldPreviewRow
}

type foldPreviewTone uint8

const (
	foldContext foldPreviewTone = iota
	foldAdded
	foldRemoved
	foldControl
)

type foldPreviewRow struct {
	line uint64
	text string
	tone foldPreviewTone
}

var foldVariants = []foldVariantSpec{
	{
		name:        "unchanged gaps",
		description: "Only context is folded; every changed line remains visible.",
		compact: []foldPreviewRow{
			{line: 38, text: "func before() {}"},
			{line: 39, text: ""},
			{line: 0, text: "... 6 unchanged lines (40-45) ...", tone: foldControl},
			{line: 46, text: "if ready {"},
			{line: 47, text: "return previous", tone: foldRemoved},
			{line: 47, text: "return current", tone: foldAdded},
			{line: 48, text: "}"},
			{line: 0, text: "... 10 unchanged lines (49-58) ...", tone: foldControl},
			{line: 59, text: "func after() error {"},
			{line: 60, text: "return nil"},
			{line: 61, text: "}"},
		},
		expanded: []foldPreviewRow{
			{line: 38, text: "func before() {}"},
			{line: 39, text: ""},
			{line: 40, text: "func prepare() error {"},
			{line: 41, text: "if err := validate(); err != nil {"},
			{line: 42, text: "return err"},
			{line: 43, text: "}"},
			{line: 44, text: "return nil"},
			{line: 45, text: "}"},
			{line: 46, text: "if ready {"},
			{line: 47, text: "return previous", tone: foldRemoved},
			{line: 47, text: "return current", tone: foldAdded},
			{line: 48, text: "}"},
		},
	},
	{
		name:        "hunk accordion",
		description: "Whole change blocks collapse into summaries, including their edits.",
		compact: []foldPreviewRow{
			{line: 0, text: "@@ 61-65   +1 -1   return value", tone: foldControl},
			{line: 0, text: "@@ 108-116 +3 -2   error path", tone: foldControl},
			{line: 0, text: "@@ 144-149 +2 -0   new helper", tone: foldControl},
			{line: 0, text: "", tone: foldControl},
			{line: 0, text: "3 collapsed hunks", tone: foldControl},
		},
		expanded: []foldPreviewRow{
			{line: 0, text: "@@ 61-65   +1 -1", tone: foldControl},
			{line: 61, text: "func value() string {"},
			{line: 62, text: "if ready {"},
			{line: 63, text: "return previous", tone: foldRemoved},
			{line: 63, text: "return current", tone: foldAdded},
			{line: 64, text: "}"},
			{line: 65, text: "}"},
			{line: 0, text: "", tone: foldControl},
			{line: 0, text: "@@ 108-116 +3 -2   collapsed", tone: foldControl},
			{line: 0, text: "@@ 144-149 +2 -0   collapsed", tone: foldControl},
		},
	},
	{
		name:        "whole-file context",
		description: "One binary control switches the document between compact and complete.",
		compact: []foldPreviewRow{
			{line: 0, text: "context: compact   2 hunks", tone: foldControl},
			{line: 0, text: "", tone: foldControl},
			{line: 45, text: "func value() string {"},
			{line: 46, text: "if ready {"},
			{line: 47, text: "return previous", tone: foldRemoved},
			{line: 47, text: "return current", tone: foldAdded},
			{line: 48, text: "}"},
			{line: 49, text: "}"},
			{line: 0, text: "", tone: foldControl},
			{line: 0, text: "... next hunk ...", tone: foldControl},
		},
		expanded: []foldPreviewRow{
			{line: 0, text: "context: full   149 file lines", tone: foldControl},
			{line: 1, text: "package reviewr"},
			{line: 2, text: ""},
			{line: 3, text: "import ("},
			{line: 4, text: "\"errors\""},
			{line: 5, text: "\"fmt\""},
			{line: 6, text: ")"},
			{line: 7, text: ""},
			{line: 8, text: "func New() Model {"},
			{line: 9, text: "return Model{}"},
			{line: 10, text: "}"},
		},
	},
}

func (model Model) viewFolds(width, height int) string {
	width = max(0, width)
	height = max(0, height)
	lines := []string{
		title.Render("lab / diff folds") + quiet.Render("   [tab: switchers]"),
		quiet.Render("j/k choose solution  •  h/l collapse/expand  •  enter toggle  •  ctrl+l or esc close"),
		renderFoldSelector(model),
		"",
	}
	index := model.foldSelected
	spec := foldVariants[index]
	expanded := model.foldExpanded[index]
	state := "compact"
	preview := spec.compact
	if expanded {
		state = "expanded"
		preview = spec.expanded
	}
	lines = append(lines,
		variant.Render(fmt.Sprintf("%c  %s", 'A'+index, spec.name))+quiet.Render("  ["+state+"]"),
		quiet.Render(spec.description),
		"",
	)
	for _, row := range preview {
		lines = append(lines, renderFoldPreviewRow(row))
	}
	lines = append(lines, "", quiet.Render("This is a visual/interaction stub; no production diff behavior is active."))
	return fitPage(lines, width, height)
}

func renderFoldSelector(model Model) string {
	parts := make([]string, len(foldVariants))
	for index, spec := range foldVariants {
		label := fmt.Sprintf("%c %s", 'A'+index, spec.name)
		style := quiet
		if index == model.foldSelected {
			style = title
			label = "[" + label + "]"
		}
		parts[index] = style.Render(label)
	}
	return strings.Join(parts, quiet.Render("  |  "))
}

func renderFoldPreviewRow(row foldPreviewRow) string {
	bar := " "
	barStyle := lipgloss.NewStyle()
	textStyle := lipgloss.NewStyle()
	switch row.tone {
	case foldAdded:
		bar = "▌"
		barStyle = lipgloss.NewStyle().Foreground(green).Bold(true)
		textStyle = lipgloss.NewStyle().Foreground(green)
	case foldRemoved:
		bar = "▌"
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E")).Bold(true)
		textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
	case foldControl:
		bar = "…"
		barStyle = quiet
		textStyle = quiet.Italic(true)
	}
	number := ""
	if row.line > 0 {
		number = fmt.Sprintf("%4d", row.line)
	} else {
		number = strings.Repeat(" ", 4)
	}
	return barStyle.Render(bar) + quiet.Render(number+" ") + textStyle.Render(row.text)
}
