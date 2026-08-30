package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func renderReader(model Model) string {
	g := model.Geometry
	title := SafeSingleLine(model.ReaderTitle)
	rows := make([]string, 0, g.ReaderRows.Height)
	content := model.ReaderLines
	document := model.ReaderDocument
	commitRows := model.ReaderCommitRows
	if document.Kind == ReaderDocumentNone && len(content) == 0 && len(commitRows) == 0 && model.ReaderEmpty.Text != "" {
		content = []Line{model.ReaderEmpty}
	}
	total := len(content)
	readerOffset := model.ReaderOffset
	readerLayout := ReaderLayout{}
	if document.Kind != ReaderDocumentNone {
		if model.ReaderLayout != nil {
			readerLayout = *model.ReaderLayout
		} else {
			readerLayout = CalculateReaderLayout(g.ReaderRows, document)
		}
		total = readerLayout.Total
		readerOffset = readerLayout.VisualOffset(model.ReaderOffset, model.ReaderColumn)
	}
	if len(commitRows) != 0 {
		total = len(commitRows)
		readerOffset = model.ReaderOffset
	}
	bar, overflow := CalculateScrollbar(g.ReaderRows, total, readerOffset)
	contentWidth := g.ReaderRows.Width
	var scrollbar []string
	if overflow {
		contentWidth = bar.Content.Width
		scrollbar = verticalScrollbar(bar, model.Focus == navigation.FocusReader)
	}
	readerGeometry := readerLayout.Geometry
	if document.Kind == ReaderDocumentNone {
		readerGeometry = CalculateReaderGeometry(g.ReaderRows, document, scrollbar != nil)
	}
	commitColumns := commitrow.Measure(commitRows, contentWidth)
	highlight := readerDiffHighlight(document, model.Controls.DiffHighlight)
	now := time.Now()
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := readerOffset + row
		if index < total {
			line := ""
			if len(commitRows) != 0 {
				line = renderCommitRow(commitRows[index], commitColumns, contentWidth, false, false, now)
			} else if document.Kind != ReaderDocumentNone {
				wrapped, continuation := readerLayout.Row(index)
				line = renderReaderRowPart(wrapped, readerGeometry, highlight, continuation)
			} else {
				line = fit(renderLine(content[index]), contentWidth)
			}
			if overflow {
				line += scrollbar[row]
			}
			rows = append(rows, line)
		} else {
			line := ""
			if overflow {
				line = fit(line, contentWidth) + scrollbar[row]
			}
			rows = append(rows, line)
		}
	}
	return renderSurface(
		g.Reader,
		g.ReaderTitle,
		g.ReaderRows,
		renderTitle(title, model.Focus == navigation.FocusReader),
		rows,
	)
}

func readerDiffHighlight(document ReaderDocument, requested workspace.DiffHighlight) workspace.DiffHighlight {
	if document.Kind == ReaderFileDocument {
		return workspace.DiffHighlightSidebar
	}
	return requested
}

func renderReaderRow(row ReaderRow, geometry ReaderGeometry, highlight workspace.DiffHighlight) string {
	return renderReaderRowPart(row, geometry, highlight, false)
}

func renderReaderRowPart(row ReaderRow, geometry ReaderGeometry, highlight workspace.DiffHighlight, continuation bool) string {
	width := geometry.Content.Width
	if width <= 0 {
		return ""
	}
	if row.Kind == ReaderFold {
		return renderReaderFoldPayload(row.Text, width, row.FoldExpanded)
	}
	changed := row.Kind == ReaderInsertion || row.Kind == ReaderDeletion
	background := changed && highlight == workspace.DiffHighlightBackground

	bar := " "
	barStyle := lipgloss.NewStyle()
	removedBoundary := !continuation && (row.RemovedBefore > 0 || row.RemovedAfter > 0)
	switch {
	case removedBoundary && row.Kind == ReaderInsertion:
		// One terminal cell carries both halves of a replacement boundary:
		// removed above in red, current addition below in green.
		bar = "▀"
		barStyle = barStyle.Foreground(errorColor).Background(addedColor).Bold(true)
	case removedBoundary:
		bar = "▴"
		if row.RemovedBefore > 0 && row.RemovedAfter > 0 {
			bar = "◆"
		} else if row.RemovedAfter > 0 {
			bar = "▾"
		}
		barStyle = barStyle.Foreground(errorColor).Bold(true)
	case row.Kind == ReaderInsertion:
		bar = "▌"
		barStyle = barStyle.Foreground(addedColor).Bold(true)
	case row.Kind == ReaderDeletion:
		bar = "▌"
		barStyle = barStyle.Foreground(errorColor).Bold(true)
	}
	number := ""
	if line := row.DisplayLine(); line > 0 && !continuation {
		number = strconv.FormatUint(line, 10)
	}
	number = fmt.Sprintf("%*s ", geometry.Digits, number)

	if background {
		backgroundColor := lipgloss.Green
		barColor := lipgloss.BrightGreen
		if row.Kind == ReaderDeletion {
			backgroundColor = lipgloss.Red
			barColor = lipgloss.BrightRed
		}
		base := lipgloss.NewStyle().Background(backgroundColor).Foreground(lipgloss.Black)
		barStyle = base.Foreground(barColor).Bold(true)
		line := barStyle.Render(bar) + base.Render(number) + renderReaderPayload(row, backgroundColor)
		line = clip(line, width)
		if padding := width - lipgloss.Width(line); padding > 0 {
			line += base.Render(strings.Repeat(" ", padding))
		}
		return line
	}

	line := barStyle.Render(bar) + mutedStyle.Render(number) + renderReaderPayload(row, nil)
	return fit(line, width)
}

func renderReaderFoldPayload(text string, width int, expanded bool) string {
	if width <= 0 {
		return ""
	}
	state := "▸ folded"
	if expanded {
		state = "▾ expanded"
	}
	label := "── " + state + " · " + SafeSingleLine(text) + " "
	label = clip(label, width)
	if remaining := width - lipgloss.Width(label); remaining > 0 {
		label += strings.Repeat("─", remaining)
	}
	return readerFoldStyle.Render(label)
}

func renderReaderPayload(row ReaderRow, background color.Color) string {
	if len(row.Spans) == 0 {
		text := SafeSingleLine(row.Text)
		if background != nil {
			return lipgloss.NewStyle().Background(background).Foreground(lipgloss.Black).Render(text)
		}
		return renderToneText(text, row.Tone)
	}
	var rendered strings.Builder
	for _, span := range row.Spans {
		text := SafeSingleLine(span.Text)
		if background != nil {
			style := lipgloss.NewStyle().
				Background(background).
				Foreground(lipgloss.Black).
				Bold(span.Style.Bold).
				Italic(span.Style.Italic).
				Underline(span.Style.Underline)
			rendered.WriteString(style.Render(text))
			continue
		}
		tone := span.Tone
		if tone == ToneDefault {
			tone = row.Tone
		}
		if tone != ToneDefault {
			rendered.WriteString(renderToneText(text, tone))
		} else {
			rendered.WriteString(renderTextStyle(text, span.Style))
		}
	}
	return rendered.String()
}
