//go:build dev

package lab

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	fallbackTerminalBackground = color.RGBA{R: 0x1a, G: 0x1b, B: 0x26, A: 0xff}
	additionTintSource         = color.RGBA{R: 0x3f, G: 0xb9, B: 0x50, A: 0xff}
	removalTintSource          = color.RGBA{R: 0xf8, G: 0x51, B: 0x49, A: 0xff}
)

const tintAlpha = uint32(54) // approximately 21% semantic color over the terminal background

type tintSpan struct {
	text  string
	color color.Color
	bold  bool
}

func (model Model) viewDiffTints(width, height int) string {
	sampleWidth := min(max(0, width-4), 76)
	additionTint, removalTint := model.diffTints()
	backgroundSource := "terminal reported"
	if !model.backgroundReported {
		backgroundSource = "dark fallback"
	}
	lines := []string{
		title.Render("lab / diff background tints"),
		quiet.Render("tab next page  •  compare full-row backgrounds over source text  •  ctrl+l or esc close"),
		"",
		variant.Render("current ANSI blocks") + quiet.Render("   saturated background + forced black text"),
		"  " + ansiDiffRow(lipgloss.Green, "+ func summarize(files []File) int { return len(files) }", sampleWidth),
		"  " + ansiDiffRow(lipgloss.Red, "- func summarize(files []File) int { return 0 }", sampleWidth),
		"",
		variant.Render("background-blended truecolor") + quiet.Render("   preserves syntax foregrounds"),
		quiet.Render(fmt.Sprintf("  background %s · %s", colorHex(model.terminalBackground), backgroundSource)),
		quiet.Render("  addition " + colorHex(additionTint)),
		"  " + syntaxTintRow(additionTint, "+", "len(files)", sampleWidth),
		quiet.Render("  removal  " + colorHex(removalTint)),
		"  " + syntaxTintRow(removalTint, "-", "0", sampleWidth),
		"",
		quiet.Render("Opaque RGB output = 79% terminal background + 21% semantic green/red."),
	}
	return fitPage(lines, max(0, width), max(0, height))
}

func (model *Model) setTerminalBackground(background color.Color) {
	if background == nil {
		return
	}
	converted, ok := color.RGBAModel.Convert(background).(color.RGBA)
	if !ok {
		return
	}
	converted.A = 0xff
	model.terminalBackground = converted
	model.backgroundReported = true
}

func (model Model) diffTints() (color.RGBA, color.RGBA) {
	return blendColor(model.terminalBackground, additionTintSource, tintAlpha),
		blendColor(model.terminalBackground, removalTintSource, tintAlpha)
}

func blendColor(background, foreground color.RGBA, alpha uint32) color.RGBA {
	alpha = min(alpha, 255)
	blend := func(background, foreground uint8) uint8 {
		return uint8((uint32(background)*(255-alpha) + uint32(foreground)*alpha + 127) / 255)
	}
	return color.RGBA{
		R: blend(background.R, foreground.R),
		G: blend(background.G, foreground.G),
		B: blend(background.B, foreground.B),
		A: 0xff,
	}
}

func colorHex(value color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", value.R, value.G, value.B)
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
