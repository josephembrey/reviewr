//go:build dev

package lab

import (
	"fmt"
	"strconv"

	"charm.land/lipgloss/v2"
)

var ansiColorNames = [...]string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"bright black", "bright red", "bright green", "bright yellow",
	"bright blue", "bright magenta", "bright cyan", "bright white",
}

func (model Model) viewANSIPalette(width, height int) string {
	lines := []string{
		title.Render("lab / ANSI palette"),
		quiet.Render("tab next page  •  terminal-defined 16-color slots  •  ctrl+l or esc close"),
		"",
		variant.Render("foreground") + quiet.Render("   normal slots 0–7 / bright slots 8–15"),
	}
	for index := 0; index < 8; index++ {
		lines = append(lines, renderANSIPair(index, false))
	}
	lines = append(lines,
		"",
		variant.Render("background")+quiet.Render("   solid swatches use the same slots"),
	)
	for index := 0; index < 8; index++ {
		lines = append(lines, renderANSIPair(index, true))
	}
	return fitPage(lines, max(0, width), max(0, height))
}

func renderANSIPair(normal int, background bool) string {
	return renderANSISlot(normal, background) + "    " + renderANSISlot(normal+8, background)
}

func renderANSISlot(index int, background bool) string {
	label := fmt.Sprintf("%2d %-14s ", index, ansiColorNames[index])
	style := lipgloss.NewStyle()
	sample := "████ sample"
	if background {
		style = style.Background(lipgloss.Color(strconv.Itoa(index)))
		sample = "          "
	} else {
		style = style.Foreground(lipgloss.Color(strconv.Itoa(index)))
	}
	return label + style.Render(sample)
}
