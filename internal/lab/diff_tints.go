//go:build dev

package lab

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	additionTint = lipgloss.Color("#173D2B")
	removalTint  = lipgloss.Color("#48242A")
)

type tintSpan struct {
	text  string
	color color.Color
	bold  bool
}

func (model Model) viewDiffTints(width, height int) string {
	sampleWidth := min(max(0, width-4), 76)
	lines := []string{
		title.Render("lab / diff background tints"),
		quiet.Render("tab next page  •  compare full-row backgrounds over source text  •  ctrl+l or esc close"),
		"",
		variant.Render("current ANSI blocks") + quiet.Render("   saturated background + forced black text"),
		"  " + ansiDiffRow(lipgloss.Green, "+ func summarize(files []File) int { return len(files) }", sampleWidth),
		"  " + ansiDiffRow(lipgloss.Red, "- func summarize(files []File) int { return 0 }", sampleWidth),
		"",
		variant.Render("proposed truecolor tint") + quiet.Render("   preserves syntax foregrounds"),
		quiet.Render("  addition #173D2B"),
		"  " + syntaxTintRow(additionTint, "+", "len(files)", sampleWidth),
		quiet.Render("  removal  #48242A"),
		"  " + syntaxTintRow(removalTint, "-", "0", sampleWidth),
		"",
		quiet.Render("The RGB color is an opaque, pre-blended tint; terminals do not composite alpha backgrounds."),
	}
	return fitPage(lines, max(0, width), max(0, height))
}

func ansiDiffRow(background color.Color, text string, width int) string {
	return lipgloss.NewStyle().
		Background(background).
		Foreground(lipgloss.Black).
		Render(padTintRow(text, width))
}

func syntaxTintRow(background color.Color, marker, result string, width int) string {
	markerColor := lipgloss.Red
	if marker == "+" {
		markerColor = lipgloss.Green
	}
	resultColor := lipgloss.Cyan
	if result == "0" {
		resultColor = lipgloss.Yellow
	}
	spans := []tintSpan{
		{text: marker + " ", color: markerColor, bold: true},
		{text: "func", color: lipgloss.Blue, bold: true},
		{text: " summarize", color: lipgloss.Cyan},
		{text: "(files []"},
		{text: "File", color: lipgloss.Green},
		{text: ") "},
		{text: "int", color: lipgloss.Green},
		{text: " { "},
		{text: "return", color: lipgloss.Blue, bold: true},
		{text: " " + result, color: resultColor},
		{text: " }  "},
		{text: "// syntax remains visible", color: lipgloss.BrightBlack},
	}
	var rendered strings.Builder
	used := 0
	for _, span := range spans {
		if used >= width {
			break
		}
		text := clipTintText(span.text, width-used)
		style := lipgloss.NewStyle().Background(background).Bold(span.bold)
		if span.color != nil {
			style = style.Foreground(span.color)
		}
		rendered.WriteString(style.Render(text))
		used += lipgloss.Width(text)
	}
	if used < width {
		rendered.WriteString(lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", width-used)))
	}
	return rendered.String()
}

func padTintRow(text string, width int) string {
	text = clipTintText(text, width)
	return text + strings.Repeat(" ", max(0, width-lipgloss.Width(text)))
}

func clipTintText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}
