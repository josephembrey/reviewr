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

type readerBarPresentation struct {
	glyph string
	style lipgloss.Style
}

var readerChangeBars = [...]readerBarPresentation{
	ReaderInsertion: {glyph: "▌", style: lipgloss.NewStyle().Foreground(addedColor).Bold(true)},
	ReaderDeletion:  {glyph: "▌", style: lipgloss.NewStyle().Foreground(errorColor).Bold(true)},
}

func renderReader(model Model) string {
	g := model.Geometry
	title := SafeSingleLine(model.ReaderTitle)
	rows := make([]string, 0, g.ReaderRows.Height)
	document := model.ReaderDocument
	commitRows := model.ReaderCommitRows
	content, readerLayout, total, readerOffset := readerContent(model, g.ReaderRows)
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
	var commitColumns commitrow.Columns
	var now time.Time
	if len(commitRows) != 0 {
		commitColumns = commitrow.Measure(commitRows, contentWidth)
		now = time.Now()
	}
	highlight := readerDiffHighlight(document, model.Controls.DiffHighlight)
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := readerOffset + row
		line := ""
		if index < total {
			line = renderReaderContentLine(index, contentWidth, &model, content, readerLayout, readerGeometry, commitColumns, now, highlight)
		}
		if overflow {
			if index >= total {
				line = fit(line, contentWidth)
			}
			line += scrollbar[row]
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Reader,
		g.ReaderTitle,
		g.ReaderRows,
		renderReaderTitle(model, title),
		rows,
	)
}

func renderReaderTitle(model Model, title string) string {
	focused := model.Focus == navigation.FocusReader
	control := model.Geometry.ReaderContextFold
	if !model.ReaderContextFoldable || control.Width == 0 {
		return renderTitle(title, focused)
	}

	label := "▸ all context"
	if model.ReaderContextExpanded {
		label = "▾ all context"
	}
	leftWidth := max(0, control.X-model.Geometry.ReaderTitle.X-1)
	return fit(renderTitle(title, focused), leftWidth) + " " + readerFoldStyle.Render(label)
}

func readerContent(model Model, rows Rect) ([]Line, ReaderLayout, int, int) {
	content := model.ReaderLines
	document := model.ReaderDocument
	if document.Kind == ReaderDocumentNone && len(content) == 0 && len(model.ReaderCommitRows) == 0 && model.ReaderEmpty.Text != "" {
		content = []Line{model.ReaderEmpty}
	}
	total := len(content)
	offset := model.ReaderOffset
	layout := ReaderLayout{}
	if document.Kind != ReaderDocumentNone {
		if model.ReaderLayout != nil {
			layout = *model.ReaderLayout
		} else {
			layout = CalculateReaderLayout(rows, document)
		}
		total = layout.Total
		offset = layout.VisualOffset(model.ReaderOffset, model.ReaderColumn)
	}
	if len(model.ReaderCommitRows) != 0 {
		total = len(model.ReaderCommitRows)
		offset = model.ReaderOffset
	}
	return content, layout, total, offset
}

func renderReaderContentLine(index, width int, model *Model, content []Line, layout ReaderLayout, geometry ReaderGeometry, columns commitrow.Columns, now time.Time, highlight workspace.DiffHighlight) string {
	if len(model.ReaderCommitRows) != 0 {
		return renderCommitRow(model.ReaderCommitRows[index], columns, width, false, false, now)
	}
	if model.ReaderDocument.Kind != ReaderDocumentNone {
		wrapped, continuation := layout.Row(index)
		return renderReaderRowPart(wrapped, geometry, highlight, continuation)
	}
	return fit(renderLine(content[index]), width)
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
	bar, barStyle := readerChangeBar(row, continuation)
	number := readerLineNumber(row, geometry.Digits, continuation)
	changed := row.Kind == ReaderInsertion || row.Kind == ReaderDeletion
	if changed && highlight == workspace.DiffHighlightBackground {
		return renderReaderBackgroundRow(row, bar, number, width)
	}
	line := barStyle.Render(bar) + mutedStyle.Render(number) + renderReaderPayload(row, nil)
	return fit(line, width)
}

func readerChangeBar(row ReaderRow, continuation bool) (string, lipgloss.Style) {
	if !continuation && (row.RemovedBefore > 0 || row.RemovedAfter > 0) {
		return readerBoundaryBar(row)
	}
	if int(row.Kind) < len(readerChangeBars) && readerChangeBars[row.Kind].glyph != "" {
		presentation := readerChangeBars[row.Kind]
		return presentation.glyph, presentation.style
	}
	return " ", lipgloss.NewStyle()
}

func readerBoundaryBar(row ReaderRow) (string, lipgloss.Style) {
	if row.Kind == ReaderInsertion {
		// One terminal cell carries both halves of a replacement boundary:
		// removed above in red, current addition below in green.
		style := lipgloss.NewStyle().Foreground(errorColor).Background(addedColor).Bold(true)
		return "▀", style
	}
	bar := "▴"
	if row.RemovedBefore > 0 && row.RemovedAfter > 0 {
		bar = "◆"
	} else if row.RemovedAfter > 0 {
		bar = "▾"
	}
	return bar, lipgloss.NewStyle().Foreground(errorColor).Bold(true)
}

func readerLineNumber(row ReaderRow, digits int, continuation bool) string {
	number := ""
	if line := row.DisplayLine(); line > 0 && !continuation {
		number = strconv.FormatUint(line, 10)
	}
	return fmt.Sprintf("%*s ", digits, number)
}

func renderReaderBackgroundRow(row ReaderRow, bar, number string, width int) string {
	backgroundColor := lipgloss.Green
	barColor := lipgloss.BrightGreen
	if row.Kind == ReaderDeletion {
		backgroundColor = lipgloss.Red
		barColor = lipgloss.BrightRed
	}
	base := lipgloss.NewStyle().Background(backgroundColor).Foreground(lipgloss.Black)
	barStyle := base.Foreground(barColor).Bold(true)
	line := barStyle.Render(bar) + base.Render(number) + renderReaderPayload(row, backgroundColor)
	line = clip(line, width)
	if padding := width - lipgloss.Width(line); padding > 0 {
		line += base.Render(strings.Repeat(" ", padding))
	}
	return line
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
