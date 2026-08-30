package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/scratch"
	"github.com/josephembrey/reviewr/internal/workspace"
)

var (
	// Structural and semantic roles use the terminal's basic palette. This
	// keeps reviewr coherent with both generated palettes and conventional
	// ANSI themes; file-type icons are the deliberate truecolor exception.
	accentColor = lipgloss.Blue
	dimColor    = lipgloss.BrightBlack
	errorColor  = lipgloss.Red
	addedColor  = lipgloss.Green
	purpleColor = lipgloss.Magenta
	yellowColor = lipgloss.Yellow

	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	focusedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	quietTitleStyle   = lipgloss.NewStyle().Foreground(dimColor)
	dimStyle          = lipgloss.NewStyle().Foreground(dimColor)
	errorStyle        = lipgloss.NewStyle().Foreground(errorColor)
	addedStyle        = lipgloss.NewStyle().Foreground(addedColor)
	purpleStyle       = lipgloss.NewStyle().Foreground(purpleColor)
	yellowStyle       = lipgloss.NewStyle().Foreground(yellowColor)
)

// Render paints one fixed-size frame from the shared Geometry.
func Render(model Model) string {
	g := model.Geometry
	blocks := make([]string, 0, 3)
	if g.Header.Height > 0 {
		blocks = append(blocks, renderHeader(model))
	}
	if g.Body.Height > 0 {
		if model.Workspace == workspace.Scratch {
			blocks = append(blocks, renderScratch(model))
		} else {
			navigator := renderNavigator(model)
			divider := renderDivider(g.Divider, model.DividerDragging)
			reader := renderReader(model)
			if g.Navigator.X < g.Reader.X {
				blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, navigator, divider, reader))
			} else {
				blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, reader, divider, navigator))
			}
		}
	}
	if g.Footer.Height > 0 {
		footer := "j/k or ↑/↓ navigate  •  tab focus  •  z swap  •  r refresh  •  q quit"
		if model.Workspace == workspace.Files {
			footer = "j/k move • h/l fold • z swap • x review • R bounds • X next gap • r refresh • q quit"
		}
		if model.Workspace == workspace.Git && model.Controls.Git == workspace.GitStashes {
			footer = "j/k move stashes • f/F move files • tab focus • z swap • r refresh • q quit"
		}
		if model.Workspace == workspace.Scratch {
			footer = SafeSingleLine(model.ScratchStatus)
		}
		style := dimStyle
		if model.Workspace == workspace.Files && model.FooterWarning != "" {
			footer = SafeSingleLine(model.FooterWarning)
			style = errorStyle
		} else if model.Workspace == workspace.Scratch && model.ScratchError {
			style = errorStyle
		}
		blocks = append(blocks, fit(style.Render(footer), g.Footer.Width))
	}
	if len(blocks) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func renderHeader(model Model) string {
	g := model.Geometry
	switcher := renderWorkspaceSwitcher(g.HeaderSwitcher.Width, model.Workspace, model.PrimaryWorkspace)
	left := switcher
	for _, control := range layoutHeaderControls(g, model.Workspace, model.Controls) {
		padding := strings.Repeat(" ", max(0, control.rect.X-lipgloss.Width(left)))
		left += padding + renderHeaderControl(control, g.Header.Width >= wideHeaderControls)
	}
	if model.Workspace != workspace.Files || !model.Changes.Ready {
		return fit(left, g.Header.Width)
	}
	for _, summary := range []string{renderChangeSummary(model.Changes), renderChangeTotals(model.Changes)} {
		summaryX := g.Header.Width - lipgloss.Width(summary)
		minimumSummaryX := lipgloss.Width(left) + 2
		if summaryX >= minimumSummaryX {
			padding := strings.Repeat(" ", max(0, summaryX-lipgloss.Width(left)))
			return fit(left+padding+summary, g.Header.Width)
		}
	}
	return fit(left, g.Header.Width)
}

func renderHeaderControl(control headerControl, wide bool) string {
	style := addedStyle
	switch control.hit {
	case HitTertiaryControl:
		style = purpleStyle
	case HitComparisonControl:
		style = yellowStyle
	}
	key := ""
	if wide {
		key = style.Bold(true).Render(control.key) + " "
	}
	return key + quietTitleStyle.Render("[") + style.Bold(true).Render(control.value) + quietTitleStyle.Render("]")
}

func renderChangeSummary(summary ChangeSummary) string {
	return quietTitleStyle.Render(fmt.Sprintf("%d changes ", summary.Files)) + renderChangeTotals(summary)
}

func renderChangeTotals(summary ChangeSummary) string {
	return addedStyle.Render(fmt.Sprintf("+%d", summary.Additions)) + " " +
		errorStyle.Render(fmt.Sprintf("-%d", summary.Deletions))
}

func renderWorkspaceSwitcher(width int, activeWorkspace, primaryWorkspace workspace.Kind) string {
	value := []byte("1  files  git  | esc  scratch ")
	active := workspaceSwitcherRect(activeWorkspace)
	bracket := func(rect Rect) {
		value[rect.X] = '['
		value[rect.X+rect.Width-1] = ']'
	}
	bracket(active)
	if activeWorkspace == workspace.Scratch {
		bracket(workspaceSwitcherRect(primaryWorkspace))
	}
	width = min(max(0, width), len(value))
	var rendered strings.Builder
	for index := 0; index < width; {
		style := workspaceSwitcherCellStyle(index, active)
		end := index + 1
		for end < width && workspaceSwitcherCellStyle(end, active) == style {
			end++
		}
		segment := string(value[index:end])
		switch style {
		case switcherKey:
			rendered.WriteString(headerStyle.Render(segment))
		default:
			rendered.WriteString(quietTitleStyle.Render(segment))
		}
		index = end
	}
	return fit(rendered.String(), width)
}

type switcherCellStyle uint8

const (
	switcherQuiet switcherCellStyle = iota
	switcherKey
)

func workspaceSwitcherCellStyle(index int, highlight Rect) switcherCellStyle {
	if highlight.Contains(index, 0) {
		return switcherKey
	}
	if index == 0 || (index >= 17 && index < 20) {
		return switcherKey
	}
	return switcherQuiet
}

func renderScratch(model Model) string {
	g := model.Geometry
	presentation := model.Scratch
	document := presentation.Document
	rows := make([]string, 0, g.ScratchRows.Height)
	bar := verticalScrollbar(g.ScratchRows.Height, len(document.Rows), presentation.Top, true)
	cursorRow := document.RowForIndex(presentation.Cursor)
	for visible := 0; visible < g.ScratchRows.Height; visible++ {
		rowIndex := presentation.Top + visible
		line := ""
		if rowIndex < len(document.Rows) {
			line = renderScratchRow(document.Rows[rowIndex], rowIndex == cursorRow, presentation, g.ScratchText.Width)
		}
		line = fit(line, g.ScratchText.Width)
		if g.ScratchBar.Width > 0 {
			lane := " "
			if bar != nil {
				lane = bar[visible]
			}
			line += lane
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Body,
		g.ScratchTitle,
		g.ScratchRows,
		renderTitle("Scratch", true),
		rows,
	)
}

func renderScratchRow(row scratch.Row, cursorRow bool, presentation scratch.Presentation, width int) string {
	var rendered strings.Builder
	for _, cell := range row.Cells {
		value := cell.Display
		selected := presentation.HasSelection && cell.Index >= presentation.SelectionStart && cell.Index < presentation.SelectionEnd
		cursor := cursorRow && cell.Index == presentation.Cursor
		switch {
		case cursor:
			value = headerStyle.Reverse(true).Render(value)
		case selected:
			value = selectionStyle(true).Render(value)
		}
		rendered.WriteString(value)
	}
	if cursorRow && presentation.Cursor == row.End && lipgloss.Width(rendered.String()) < width {
		rendered.WriteString(headerStyle.Reverse(true).Render(" "))
	}
	return rendered.String()
}

// SafeContentLines makes arbitrary worktree bytes inert before terminal output.
func SafeContentLines(content string) []string {
	content = strings.ToValidUTF8(content, "�")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var safe strings.Builder
	for _, char := range content {
		switch char {
		case '\n':
			safe.WriteRune(char)
		case '\t':
			safe.WriteString("    ")
		case '\r':
			safe.WriteRune('␍')
		case 0x7f:
			safe.WriteRune('␡')
		default:
			if char < 0x20 {
				safe.WriteRune(0x2400 + char)
			} else if unicode.IsControl(char) {
				fmt.Fprintf(&safe, "\\u%04X", char)
			} else {
				safe.WriteRune(char)
			}
		}
	}
	return strings.Split(safe.String(), "\n")
}

// SafeSingleLine renders a raw path or error without introducing screen rows.
func SafeSingleLine(value string) string {
	return strings.Join(SafeContentLines(value), "↵")
}

func renderNavigator(model Model) string {
	g := model.Geometry
	rows := make([]string, 0, g.NavigatorRows.Height)
	title := model.NavigatorTitle
	visibleRows := g.NavigatorRows.Height
	scrollbar := verticalScrollbar(visibleRows, len(model.NavigatorRows), model.Top, model.Focus == navigation.FocusNavigator)
	contentWidth := g.NavigatorRows.Width
	if scrollbar != nil {
		contentWidth--
	}
	commitRows := make([]commitrow.Row, 0, len(model.NavigatorRows))
	for _, row := range model.NavigatorRows {
		if row.Commit != nil {
			commitRows = append(commitRows, *row.Commit)
		}
	}
	commitColumns := commitrow.Measure(commitRows, contentWidth)
	now := time.Now()
	for row := 0; row < visibleRows; row++ {
		index := model.Top + row
		if index >= len(model.NavigatorRows) {
			if row == 0 && len(model.NavigatorRows) == 0 {
				rows = append(rows, renderLine(model.NavigatorEmpty))
			} else {
				rows = append(rows, "")
			}
			if scrollbar != nil {
				rows[len(rows)-1] = fit(rows[len(rows)-1], contentWidth) + scrollbar[row]
			}
			continue
		}
		line := renderNavigatorPresentationRow(
			model.NavigatorRows[index],
			contentWidth,
			index == model.Selected,
			model.Focus == navigation.FocusNavigator,
			commitColumns,
			now,
		)
		if scrollbar != nil {
			line += scrollbar[row]
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Navigator,
		g.NavigatorTitle,
		g.NavigatorRows,
		renderTitle(title, model.Focus == navigation.FocusNavigator),
		rows,
	)
}

func renderNavigatorPresentationRow(item NavigatorRow, width int, selected, focused bool, columns commitrow.Columns, now time.Time) string {
	if item.Commit != nil {
		return renderCommitRow(*item.Commit, columns, width, selected, focused, now)
	}
	if len(item.Prefix) != 0 || len(item.Suffix) != 0 {
		return renderCompactNavigatorRow(item, width, selected, focused)
	}
	if !item.Tree {
		return renderNavigatorRow(SafeSingleLine(item.Label), width, selected, focused)
	}
	marker, accent := treeNavigatorStatus(item.Status)
	return renderTreeNavigatorRow(item, width, treeRowStyleLayers{
		statusMarker: marker,
		statusAccent: accent,
		ignored:      item.Dimmed,
		selected:     selected,
		focused:      focused,
	})
}

func renderTreeNavigatorRow(item NavigatorRow, width int, layers treeRowStyleLayers) string {
	layout := LayoutNavigatorRow(item, width)
	depth := max(0, item.Depth)
	marker := " "
	icon := treeFileIcon(item.Label)
	label := SafeSingleLine(item.Label)
	if item.Directory {
		marker = "▸"
		if item.Expanded {
			marker = "▾"
		}
		icon = treeDirectoryIcon(item.Expanded)
		label += "/"
	} else if layers.statusMarker != "" {
		marker = fit(SafeSingleLine(layers.statusMarker), 1)
	}
	styles := resolveTreeRowStyles(item, icon, layers)
	selection := styles.row
	row := selection.Render(" "+strings.Repeat("  ", depth)) +
		styles.marker.Inherit(selection).Render(marker) + selection.Render(" ") +
		styles.icon.Inherit(selection).Render(icon.glyph) + selection.Render(" ") +
		styles.filename.Inherit(selection).Render(label)
	row = lipgloss.NewStyle().MaxWidth(layout.Label.Width).Render(row)
	row += selection.Render(strings.Repeat(" ", max(0, layout.Label.Width-lipgloss.Width(row))))
	if layout.Progress.Width > 0 {
		progress := " " + item.Progress
		row += dimStyle.Inherit(selection).Render(fit(progress, layout.Progress.Width))
	}
	if layout.Changes.Width > 0 {
		additions, deletions := formatLineChanges(*item.Changes)
		row += selection.Render(" ") + addedStyle.Inherit(selection).Render(additions) +
			selection.Render(" ") + errorStyle.Inherit(selection).Render(deletions)
	}
	if layout.Review.Width > 0 {
		badge := " " + item.Review.Badge()
		row += reviewBadgeStyle(*item.Review).Inherit(selection).Render(badge)
	}
	return row
}

func reviewBadgeStyle(state review.State) lipgloss.Style {
	switch state {
	case review.Reviewed:
		return addedStyle
	case review.Updated:
		return headerStyle
	case review.Partial:
		return yellowStyle
	case review.BasisChanged:
		return errorStyle
	default:
		return dimStyle
	}
}

func treeNavigatorStatus(status NavigatorStatus) (string, treeStatusAccent) {
	switch status {
	case StatusModified:
		return "M", treeStatusModified
	case StatusAdded:
		return "A", treeStatusAdded
	case StatusDeleted:
		return "D", treeStatusDeleted
	case StatusRenamed:
		return "R", treeStatusRenamed
	case StatusUntracked:
		return "?", treeStatusUntracked
	case StatusIgnored:
		return "I", treeStatusNone
	default:
		return "", treeStatusNone
	}
}

func renderCompactNavigatorRow(item NavigatorRow, width int, selected, focused bool) string {
	prefix := renderSegments(item.Prefix)
	suffix := renderSegments(item.Suffix)
	label := SafeSingleLine(item.Label)
	row := prefix
	available := max(0, width-lipgloss.Width(prefix))
	labelWidth := lipgloss.Width(label)
	suffixWidth := lipgloss.Width(suffix)
	switch {
	case suffix == "":
		row += clip(label, available)
	case labelWidth+suffixWidth <= available:
		row += label + suffix
	case available < 28 || suffixWidth > available-12:
		row += clip(label, available)
	default:
		row += clip(label, available-suffixWidth) + suffix
	}
	row = fit(row, width)
	if !selected {
		return row
	}
	return selectionStyle(focused).Render(row)
}

func renderReader(model Model) string {
	g := model.Geometry
	title := SafeSingleLine(model.ReaderTitle)
	rows := make([]string, 0, g.ReaderRows.Height)
	content := model.ReaderLines
	commitRows := model.ReaderCommitRows
	if len(content) == 0 && len(commitRows) == 0 && model.ReaderEmpty.Text != "" {
		content = []Line{model.ReaderEmpty}
	}
	total := len(content)
	if len(commitRows) != 0 {
		total = len(commitRows)
	}
	scrollbar := verticalScrollbar(g.ReaderRows.Height, total, model.ReaderOffset, model.Focus == navigation.FocusReader)
	contentWidth := g.ReaderRows.Width
	if scrollbar != nil {
		contentWidth--
	}
	commitColumns := commitrow.Measure(commitRows, contentWidth)
	now := time.Now()
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := model.ReaderOffset + row
		if index < total {
			line := ""
			if len(commitRows) != 0 {
				line = renderCommitRow(commitRows[index], commitColumns, contentWidth, false, false, now)
			} else {
				line = fit(renderLine(content[index]), contentWidth)
			}
			if scrollbar != nil {
				line += scrollbar[row]
			}
			rows = append(rows, line)
		} else {
			line := ""
			if scrollbar != nil {
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

func renderSegments(segments []Segment) string {
	var value strings.Builder
	for _, segment := range segments {
		value.WriteString(renderToneText(SafeSingleLine(segment.Text), segment.Tone))
	}
	return value.String()
}

func renderLine(line Line) string {
	if len(line.Spans) != 0 {
		var rendered strings.Builder
		for _, span := range line.Spans {
			text := SafeSingleLine(span.Text)
			tone := span.Tone
			if tone == ToneDefault {
				tone = line.Tone
			}
			if tone != ToneDefault {
				rendered.WriteString(renderToneText(text, tone))
				continue
			}
			rendered.WriteString(renderTextStyle(text, span.Style))
		}
		return rendered.String()
	}
	text := SafeSingleLine(line.Text)
	return renderToneText(text, line.Tone)
}

func renderTextStyle(text string, value TextStyle) string {
	style := lipgloss.NewStyle().
		Bold(value.Bold).
		Italic(value.Italic).
		Underline(value.Underline)
	if value.Foreground != "" {
		style = style.Foreground(lipgloss.Color(value.Foreground))
	}
	return style.Render(text)
}

func renderToneText(text string, tone Tone) string {
	switch tone {
	case ToneQuiet:
		return dimStyle.Render(text)
	case ToneError:
		return errorStyle.Render(text)
	case ToneAccent:
		return purpleStyle.Render(text)
	case ToneAdded:
		return addedStyle.Render(text)
	case ToneRemoved:
		return errorStyle.Render(text)
	case ToneInfo:
		return headerStyle.Render(text)
	case ToneWarning:
		return yellowStyle.Render(text)
	default:
		return text
	}
}

func renderTitle(title string, focused bool) string {
	if focused {
		return focusedTitleStyle.Render(title)
	}
	return quietTitleStyle.Render(title)
}

func renderNavigatorRow(path string, width int, selected, focused bool) string {
	row := fit("  "+path, width)
	if !selected {
		return row
	}
	return selectionStyle(focused).Render(row)
}

func selectionStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().Reverse(true).Bold(focused)
}

func renderDivider(rect Rect, dragging bool) string {
	if rect.Width <= 0 || rect.Height <= 0 {
		return ""
	}
	style := dimStyle
	if dragging {
		style = headerStyle
	}
	line := fit(style.Render("│"), rect.Width)
	return strings.Repeat(line+"\n", rect.Height-1) + line
}

func renderSurface(surface, titleRect, rowsRect Rect, title string, rows []string) string {
	if surface.Width <= 0 || surface.Height <= 0 {
		return ""
	}
	lines := make([]string, surface.Height)
	for index := range lines {
		lines[index] = strings.Repeat(" ", surface.Width)
	}
	if titleRect.Width > 0 && titleRect.Height > 0 {
		lines[titleRect.Y-surface.Y] = fit(title, titleRect.Width)
	}
	for index := 0; index < rowsRect.Height; index++ {
		row := ""
		if index < len(rows) {
			row = rows[index]
		}
		lines[rowsRect.Y-surface.Y+index] = fit(row, rowsRect.Width)
	}
	return strings.Join(lines, "\n")
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = clip(value, width)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func clip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(value)
}

func blankBlock(width, height int) string {
	line := strings.Repeat(" ", max(0, width))
	return strings.Repeat(line+"\n", max(0, height-1)) + line
}
