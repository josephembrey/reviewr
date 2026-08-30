package ui

import (
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
)

var graphPalette = []color.Color{
	lipgloss.Blue,
	lipgloss.Magenta,
	lipgloss.Green,
	lipgloss.Yellow,
	lipgloss.Cyan,
	lipgloss.Red,
}

func renderCommitRow(row commitrow.Row, columns commitrow.Columns, width int, selected, focused bool, now time.Time) string {
	if width <= 0 {
		return ""
	}
	fixed := columns.Graph + commitrow.SHAWidth + 2
	if columns.Author > 0 {
		fixed += 2 + columns.Author
	}
	if columns.Age > 0 {
		fixed += 2 + columns.Age
	}
	contentWidth := max(0, width-fixed)
	trailWidth := commitrow.TrailReserve(row, contentWidth)
	subjectWidth := max(0, contentWidth-trailWidth)

	var rendered strings.Builder
	rendered.WriteString(renderCommitGraph(row.Graph, columns.Graph, selected, focused))
	sha := truncatePlain(SafeSingleLine(row.ShortOID), commitrow.SHAWidth)
	rendered.WriteString(commitCellStyle(lipgloss.Blue, selected, focused).Render(padRight(sha, commitrow.SHAWidth)))
	rendered.WriteString(commitCellStyle(nil, selected, focused).Render("  "))
	subject := truncatePlain(SafeSingleLine(row.Subject), subjectWidth)
	rendered.WriteString(commitCellStyle(nil, selected, focused).Render(padRight(subject, subjectWidth)))
	rendered.WriteString(renderCommitTrail(row, trailWidth, selected, focused))

	used := columns.Graph + commitrow.SHAWidth + 2 + subjectWidth + trailWidth
	trailingWidth := width - used
	if columns.Author > 0 {
		author := truncatePlain(SafeSingleLine(row.Author), columns.Author)
		gap := max(2, trailingWidth-(columns.Author+2+columns.Age))
		rendered.WriteString(commitCellStyle(nil, selected, focused).Render(strings.Repeat(" ", gap)))
		rendered.WriteString(commitCellStyle(dimColor, selected, focused).Render(padRight(author, columns.Author)))
		trailingWidth -= gap + columns.Author
	}
	if columns.Age > 0 {
		gap := max(2, trailingWidth-columns.Age)
		rendered.WriteString(commitCellStyle(nil, selected, focused).Render(strings.Repeat(" ", gap)))
		age := truncatePlain(commitrow.AgeLabel(now, row.AuthoredUnix), columns.Age)
		rendered.WriteString(commitCellStyle(dimColor, selected, focused).Render(padLeft(age, columns.Age)))
	}
	remaining := width - lipgloss.Width(rendered.String())
	if remaining > 0 {
		rendered.WriteString(commitCellStyle(nil, selected, focused).Render(strings.Repeat(" ", remaining)))
	}
	return ansi.Truncate(rendered.String(), width, "")
}

func renderCommitGraph(graph commitgraph.Row, width int, selected, focused bool) string {
	if width <= 0 {
		return ""
	}
	var rendered strings.Builder
	used := 0
	write := func(glyph rune, laneColor commitgraph.Color, colored bool) {
		if used == width {
			return
		}
		var foreground color.Color
		if colored {
			foreground = graphPalette[int(laneColor)%len(graphPalette)]
		}
		rendered.WriteString(commitCellStyle(foreground, selected, focused).Render(string(glyph)))
		used++
	}
	for _, cell := range graph.Cells {
		write(cell.Glyph, cell.GlyphColor, cell.GlyphColored)
		write(cell.Horizontal, cell.HorizontalColor, cell.HorizontalColored)
		if used == width {
			break
		}
	}
	if used < width {
		rendered.WriteString(commitCellStyle(nil, selected, focused).Render(strings.Repeat(" ", width-used)))
	}
	return rendered.String()
}

func renderCommitTrail(row commitrow.Row, width int, selected, focused bool) string {
	if width <= 0 {
		return ""
	}
	var rendered strings.Builder
	remaining := width
	write := func(value string, foreground color.Color) {
		value = truncatePlain(value, remaining)
		rendered.WriteString(commitCellStyle(foreground, selected, focused).Render(value))
		remaining -= lipgloss.Width(value)
	}
	write("  ", nil)
	first := true
	for _, reference := range row.Refs {
		if remaining == 0 {
			break
		}
		if !first {
			write(" · ", dimColor)
		}
		icon, semanticColor := commitRefStyle(reference.Kind)
		write(icon+" "+SafeSingleLine(reference.Name), semanticColor)
		first = false
	}
	if row.Merge && remaining > 0 {
		if !first {
			write(" · ", dimColor)
		}
		write("merge", dimColor)
	}
	if remaining > 0 {
		write(strings.Repeat(" ", remaining), nil)
	}
	return rendered.String()
}

func commitRefStyle(kind commitrow.RefKind) (string, color.Color) {
	switch kind {
	case commitrow.Remote:
		return "", lipgloss.Blue
	case commitrow.Tag:
		return "", lipgloss.Yellow
	default:
		return "", lipgloss.Green
	}
}

func commitCellStyle(foreground color.Color, selected, focused bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if foreground != nil {
		style = style.Foreground(foreground)
	}
	if selected {
		style = style.Reverse(true).Bold(focused)
	}
	return style
}

func truncatePlain(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "")
}

func padRight(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func padLeft(value string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(value))) + value
}
