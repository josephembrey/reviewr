package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	layout := layoutReaderPaneTitle(model.Geometry, model.Workspace, model.Controls)
	left := fit(renderTitle(title, focused), layout.leftWidth)
	if layout.control.rect.Width == 0 {
		return left
	}
	gap := max(0, layout.control.rect.X-model.Geometry.ReaderTitle.X-layout.leftWidth)
	return left + strings.Repeat(" ", gap) + renderHeaderControl(layout.control, true)
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
		return renderCommitRow(
			model.ReaderCommitRows[index], columns, width,
			index == model.ReaderCursor, model.Focus == navigation.FocusReader, now,
		)
	}
	if model.ReaderDocument.Kind != ReaderDocumentNone {
		wrapped, source, continuation := layout.RowWithSource(index)
		return renderReaderRowPartSelected(
			wrapped, geometry, highlight, continuation,
			source == model.ReaderCursor && !model.ReaderCharacterSelection,
			model.Focus == navigation.FocusReader,
		)
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
	return renderReaderRowPartSelected(row, geometry, highlight, continuation, false, false)
}

func renderReaderRowPartSelected(row ReaderRow, geometry ReaderGeometry, highlight workspace.DiffHighlight, continuation, selected, focused bool) string {
	width := geometry.Content.Width
	if width <= 0 {
		return ""
	}
	var line string
	var bar string
	var barStyle lipgloss.Style
	if row.Kind == ReaderFold {
		line = renderReaderFoldPayload(row.Text, width, row.FoldExpanded)
	} else if row.Kind == ReaderFoldEnd {
		line = renderReaderFoldEndPayload(row.Text, width)
	} else if row.Kind == ReaderCommentHeader {
		line = renderCommentHeader(row, width)
	} else if row.Kind == ReaderCommentBody {
		line = renderCommentBody(row.Text, width, false, -1)
	} else if row.Kind == ReaderCommentEnd {
		line = renderCommentEnd(width, false)
	} else if row.Kind == ReaderCommentComposerHeader {
		line = renderCommentComposerHeader(row.Text, width)
	} else if row.Kind == ReaderCommentComposerBody {
		cursor := -1
		if row.ComposerCursor {
			cursor = row.ComposerCursorColumn
		}
		line = renderCommentBody(row.Text, width, true, cursor)
	} else if row.Kind == ReaderCommentComposerEnd {
		line = renderCommentEnd(width, true)
	} else if row.Kind == ReaderMarkdown {
		line = fit(renderReaderPayload(row, nil), width)
	} else {
		bar, barStyle = readerChangeBar(row, continuation)
		number := readerLineNumber(row, geometry.Digits, continuation)
		numberStyle := mutedStyle
		if row.CommentHover && !continuation {
			numberStyle = commentTitleStyle
		}
		changed := row.Kind == ReaderInsertion || row.Kind == ReaderDeletion
		if changed && highlight == workspace.DiffHighlightBackground {
			line = renderReaderBackgroundRow(row, bar, number, width, row.VisualSelected)
		} else {
			line = barStyle.Render(bar) + numberStyle.Render(number) + renderReaderPayloadSelection(row, nil, focused)
			line = fit(line, width)
		}
	}
	if row.VisualCharacter {
		// Character selection owns payload cells only. The logical line cursor
		// must not turn it back into a full-row selection.
		return line
	}
	if row.VisualSelected && (row.Kind == ReaderInsertion || row.Kind == ReaderDeletion) && highlight == workspace.DiffHighlightBackground {
		// A background diff already owns the whole row's semantic green/red.
		// Bold underline supplies the Visual-line treatment without erasing it.
		return line
	}
	selected = selected || row.VisualSelected
	if !selected {
		return line
	}
	plain := ansi.Strip(fit(line, width))
	prefix := bar
	preserveChange := highlight == workspace.DiffHighlightSidebar && bar != "" && bar != " "
	if !preserveChange || !strings.HasPrefix(plain, prefix) {
		return selectionStyle(focused).Render(plain)
	}
	selection := selectionStyle(focused)
	selectedPrefix := selection.Render(bar)
	if preserveChange {
		selectedPrefix = selectedReaderBarStyle(barStyle, focused).Render(bar)
	}
	return selectedPrefix + selection.Render(strings.TrimPrefix(plain, prefix))
}

// selectedReaderBarStyle swaps the authored colors before applying reverse.
// The terminal swaps them back while retaining the selection background, so
// the one-cell diff marker keeps its semantic red/green foreground.
func selectedReaderBarStyle(style lipgloss.Style, focused bool) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(style.GetBackground()).
		Background(style.GetForeground()).
		Bold(focused || style.GetBold()).
		Reverse(true)
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
	if row.CommentHover && !continuation && row.Commentable() {
		number = "[+]"
	} else if line := row.DisplayLine(); line > 0 && !continuation {
		number = strconv.FormatUint(line, 10)
	}
	return fmt.Sprintf("%*s ", digits, number)
}

const commentCardIndent = 2

func renderCommentHeader(row ReaderRow, width int) string {
	boxWidth := max(0, width-commentCardIndent)
	if boxWidth <= 0 {
		return ""
	}
	state := ""
	if row.CommentStale {
		state = " · stale"
	}
	label := " ▸ comment · " + SafeSingleLine(row.Text) + state + " "
	if row.FoldExpanded {
		label = " ▾ comment · " + SafeSingleLine(row.Text) + state + " "
	}
	label = clip(label, max(0, boxWidth-3))
	fill := strings.Repeat("─", max(0, boxWidth-3-lipgloss.Width(label)))
	return strings.Repeat(" ", min(commentCardIndent, width)) +
		commentBorderStyle.Render("╭─") + commentTitleStyle.Render(label) + commentBorderStyle.Render(fill+"╮")
}

func renderCommentBody(text string, width int, composing bool, cursor int) string {
	boxWidth := max(0, width-commentCardIndent)
	if boxWidth <= 0 {
		return ""
	}
	inner := max(0, boxWidth-4)
	body := fit(SafeSingleLine(text), inner)
	if cursor >= 0 {
		plain := ansi.Strip(body)
		cursor = max(0, min(cursor, lipgloss.Width(plain)))
		left := ansi.Cut(plain, 0, cursor)
		right := ansi.Cut(plain, cursor, cursor+1)
		if right == "" {
			right = " "
		}
		tail := ansi.Cut(plain, cursor+1, inner)
		body = commentBodyStyle.Render(left) + commentTitleStyle.Reverse(true).Render(right) + commentBodyStyle.Render(tail)
	} else {
		body = commentBodyStyle.Render(body)
	}
	border := commentBorderStyle
	if composing {
		border = composerBorderStyle
	}
	return strings.Repeat(" ", min(commentCardIndent, width)) + border.Render("│ ") + body + border.Render(" │")
}

func renderCommentEnd(width int, composing bool) string {
	boxWidth := max(0, width-commentCardIndent)
	if boxWidth <= 0 {
		return ""
	}
	border := commentBorderStyle
	if composing {
		border = composerBorderStyle
	}
	return strings.Repeat(" ", min(commentCardIndent, width)) + border.Render("╰"+strings.Repeat("─", max(0, boxWidth-2))+"╯")
}

func renderCommentComposerHeader(text string, width int) string {
	boxWidth := max(0, width-commentCardIndent)
	if boxWidth <= 0 {
		return ""
	}
	label := clip(" comment · "+SafeSingleLine(text)+" ", max(0, boxWidth-3))
	fill := strings.Repeat("─", max(0, boxWidth-3-lipgloss.Width(label)))
	return strings.Repeat(" ", min(commentCardIndent, width)) +
		composerBorderStyle.Render("╭─") + commentTitleStyle.Render(label) + composerBorderStyle.Render(fill+"╮")
}

func renderReaderBackgroundRow(row ReaderRow, bar, number string, width int, visualSelected bool) string {
	backgroundColor := addedColor
	barColor := lipgloss.BrightGreen
	if row.Kind == ReaderDeletion {
		backgroundColor = errorColor
		barColor = lipgloss.BrightRed
	}
	base := lipgloss.NewStyle().Background(backgroundColor).Foreground(lipgloss.Black).
		Bold(visualSelected).Underline(visualSelected)
	barStyle := base.Foreground(barColor).Bold(true)
	numberStyle := base
	if row.CommentHover {
		// Match the legacy affordance while retaining the changed-row fill.
		numberStyle = numberStyle.Bold(true).Underline(true)
	}
	payload := renderReaderPayloadSelection(row, backgroundColor, true)
	if visualSelected && !row.VisualCharacter {
		payload = renderReaderBackgroundPayload(row, backgroundColor, true)
	}
	line := barStyle.Render(bar) + numberStyle.Render(number) + payload
	line = clip(line, width)
	if padding := width - lipgloss.Width(line); padding > 0 {
		line += base.Render(strings.Repeat(" ", padding))
	}
	return line
}

func renderReaderBackgroundPayload(row ReaderRow, background color.Color, visualSelected bool) string {
	if len(row.Spans) == 0 {
		return lipgloss.NewStyle().Background(background).Foreground(lipgloss.Black).
			Bold(visualSelected).Underline(visualSelected).Render(SafeSingleLine(row.Text))
	}
	var rendered strings.Builder
	for _, span := range row.Spans {
		style := lipgloss.NewStyle().
			Background(background).
			Foreground(lipgloss.Black).
			Bold(span.Style.Bold || visualSelected).
			Italic(span.Style.Italic).
			Underline(span.Style.Underline || visualSelected)
		rendered.WriteString(style.Render(SafeSingleLine(span.Text)))
	}
	return rendered.String()
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

func renderReaderFoldEndPayload(text string, width int) string {
	if width <= 0 {
		return ""
	}
	label := "── " + SafeSingleLine(text) + " "
	label = clip(label, width)
	if remaining := width - lipgloss.Width(label); remaining > 0 {
		label += strings.Repeat("─", remaining)
	}
	return readerFoldEndStyle.Render(label)
}

func renderReaderPayload(row ReaderRow, background color.Color) string {
	if row.Styled != "" && background == nil {
		return row.Styled
	}
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

// renderReaderPayloadSelection preserves syntax outside a mouse-authored
// character range. On ordinary rows selected cells use the terminal's native
// reverse-video selection; background diff rows retain their red/green fill
// and add bold underline, matching Visual-line semantics.
func renderReaderPayloadSelection(row ReaderRow, background color.Color, focused bool) string {
	if !row.VisualCharacter {
		return renderReaderPayload(row, background)
	}
	if len(row.Spans) == 0 {
		return renderReaderPayloadSelectionPiece(
			SafeSingleLine(row.Text), row.Tone, TextStyle{}, background,
			row.VisualStart, row.VisualEnd, focused,
		)
	}
	var rendered strings.Builder
	position := 0
	for _, span := range row.Spans {
		text := SafeSingleLine(span.Text)
		width := ansi.StringWidth(text)
		start := max(0, row.VisualStart-position)
		end := min(width, row.VisualEnd-position)
		tone := span.Tone
		if tone == ToneDefault {
			tone = row.Tone
		}
		rendered.WriteString(renderReaderPayloadSelectionPiece(text, tone, span.Style, background, start, end, focused))
		position += width
	}
	return rendered.String()
}

func renderReaderPayloadSelectionPiece(
	text string,
	tone Tone,
	textStyle TextStyle,
	background color.Color,
	start, end int,
	focused bool,
) string {
	width := ansi.StringWidth(text)
	start = max(0, min(start, width))
	end = max(start, min(end, width))
	render := func(value string, selected bool) string {
		if value == "" {
			return ""
		}
		if selected {
			if background != nil {
				return lipgloss.NewStyle().Background(background).Foreground(lipgloss.Black).
					Bold(true).Underline(true).Render(value)
			}
			return selectionStyle(focused).Render(value)
		}
		if background != nil {
			return lipgloss.NewStyle().Background(background).Foreground(lipgloss.Black).
				Bold(textStyle.Bold).Italic(textStyle.Italic).Underline(textStyle.Underline).Render(value)
		}
		if tone != ToneDefault {
			return renderToneText(value, tone)
		}
		return renderTextStyle(value, textStyle)
	}
	return render(ansi.Cut(text, 0, start), false) +
		render(ansi.Cut(text, start, end), true) +
		render(ansi.Cut(text, end, width), false)
}
