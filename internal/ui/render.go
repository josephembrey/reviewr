package ui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

var (
	accentColor = lipgloss.Color("#7AA2F7")
	dimColor    = lipgloss.Color("#777777")
	errorColor  = lipgloss.Color("#F7768E")
	addedColor  = lipgloss.Color("#9ECE6A")
	purpleColor = lipgloss.Color("#BB9AF7")
	yellowColor = lipgloss.Color("#E0AF68")

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
			blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, navigator, divider, reader))
		}
	}
	if g.Footer.Height > 0 {
		footer := "j/k or ↑/↓ navigate  •  tab focus  •  r refresh  •  q quit"
		if model.Workspace == workspace.Files {
			footer = "j/k move • h/l fold • tab focus • r refresh • q quit"
		}
		if model.Workspace == workspace.Scratch {
			footer = "esc close scratch  •  1 files/git  •  q quit"
		}
		blocks = append(blocks, fit(dimStyle.Render(footer), g.Footer.Width))
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
	if !model.Changes.Ready {
		return fit(left, g.Header.Width)
	}
	summary := renderChangeSummary(model.Changes)
	summaryWidth := lipgloss.Width(summary)
	summaryX := g.Header.Width - summaryWidth
	minimumSummaryX := lipgloss.Width(left) + 2
	if summaryX >= minimumSummaryX {
		padding := strings.Repeat(" ", max(0, summaryX-lipgloss.Width(left)))
		return fit(left+padding+summary, g.Header.Width)
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
	return quietTitleStyle.Render(fmt.Sprintf("%d changes ", summary.Files)) +
		addedStyle.Render(fmt.Sprintf("+%d", summary.Additions)) + " " +
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
	title, rows := surfaceRows(model.Geometry.Body)
	return renderSurface(
		model.Geometry.Body,
		title,
		rows,
		renderTitle("Scratch", true),
		[]string{dimStyle.Render("Scratch editor coming next.")},
	)
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

func renderNavigatorPresentationRow(item NavigatorRow, width int, selected, focused bool) string {
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
	row = lipgloss.NewStyle().MaxWidth(width).Render(row)
	return row + selection.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(row))))
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

func renderReader(model Model) string {
	g := model.Geometry
	title := SafeSingleLine(model.ReaderTitle)
	rows := make([]string, 0, g.ReaderRows.Height)
	content := model.ReaderLines
	if len(content) == 0 && model.ReaderEmpty.Text != "" {
		content = []Line{model.ReaderEmpty}
	}
	scrollbar := verticalScrollbar(g.ReaderRows.Height, len(content), model.ReaderOffset, model.Focus == navigation.FocusReader)
	contentWidth := g.ReaderRows.Width
	if scrollbar != nil {
		contentWidth--
	}
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := model.ReaderOffset + row
		if index < len(content) {
			line := fit(renderLine(content[index]), contentWidth)
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

func renderLine(line Line) string {
	text := SafeSingleLine(line.Text)
	switch line.Tone {
	case ToneQuiet:
		return dimStyle.Render(text)
	case ToneError:
		return errorStyle.Render(text)
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
	value = lipgloss.NewStyle().MaxWidth(width).Render(value)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func blankBlock(width, height int) string {
	line := strings.Repeat(" ", max(0, width))
	return strings.Repeat(line+"\n", max(0, height-1)) + line
}
