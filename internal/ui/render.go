package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/workspace"
)

var (
	// Structural and semantic roles use the terminal's basic palette. This
	// keeps reviewr coherent with both generated palettes and conventional
	// ANSI themes; file-type icons are the deliberate truecolor exception.
	accentColor    = lipgloss.Cyan
	secondaryColor = lipgloss.White
	mutedColor     = lipgloss.BrightBlack
	errorColor     = lipgloss.Red
	addedColor     = lipgloss.Green
	purpleColor    = lipgloss.Magenta
	yellowColor    = lipgloss.Yellow

	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	focusedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	chromeStyle       = lipgloss.NewStyle().Foreground(secondaryColor)
	mutedStyle        = lipgloss.NewStyle().Foreground(mutedColor)
	readerFoldStyle   = lipgloss.NewStyle().Foreground(accentColor)
	// Scrollbars follow Herdr's restrained three-level hierarchy while staying
	// within reviewr's terminal ANSI roles.
	scrollbarTrackStyle          = mutedStyle.Faint(true)
	scrollbarUnfocusedThumbStyle = mutedStyle
	scrollbarFocusedThumbStyle   = lipgloss.NewStyle().Foreground(accentColor)
	errorStyle                   = lipgloss.NewStyle().Foreground(errorColor)
	addedStyle                   = lipgloss.NewStyle().Foreground(addedColor)
	purpleStyle                  = lipgloss.NewStyle().Foreground(purpleColor)
	yellowStyle                  = lipgloss.NewStyle().Foreground(yellowColor)
)

// Render paints one fixed-size frame from the shared Geometry.
func Render(model Model) string {
	g := model.Geometry
	blocks := make([]string, 0, 3)
	if g.Header.Height > 0 {
		blocks = append(blocks, renderHeader(model))
	}
	if g.Body.Height > 0 {
		if model.Workspace == workspace.Notes {
			blocks = append(blocks, renderNotes(model))
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
		blocks = append(blocks, renderFooter(model))
	}
	if len(blocks) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

type footerEntry struct {
	key   string
	label string
}

func renderFooter(model Model) string {
	width := model.Geometry.Footer.Width
	if model.Workspace == workspace.Files && model.FooterWarning != "" {
		return fit(errorStyle.Render(SafeSingleLine(model.FooterWarning)), width)
	}
	if model.Workspace == workspace.Notes {
		style := chromeStyle
		if model.NotesError {
			style = errorStyle
		}
		footer := renderFooterEntry(footerEntry{key: "Esc", label: "Files"}) +
			renderFooterSeparator() + style.Render(SafeSingleLine(model.NotesStatus)) +
			renderFooterSeparator() + renderFooterEntry(footerEntry{key: "Tab", label: "next"})
		if model.NotesHasWorktree {
			footer += renderFooterSeparator() + renderFooterEntry(footerEntry{key: "ctrl+t", label: "scope"})
		}
		return fit(footer, width)
	}

	destinations := []footerEntry{
		{key: "1", label: "files"},
		{key: "2", label: "git"},
		{key: "3", label: "notes"},
		{key: "tab", label: "next"},
	}
	entries := append(destinations, []footerEntry{
		{key: "j/k or ↑/↓", label: "navigate"},
		{key: "z", label: "swap"},
		{key: "r", label: "refresh"},
		{key: "q", label: "quit"},
	}...)
	if model.Workspace == workspace.Files {
		local := []footerEntry{
			{key: workspace.SecondaryControlKey, label: model.Controls.Files.Label()},
			{key: workspace.TertiaryControlKey, label: model.Controls.Reader.Label()},
			{key: workspace.ComparisonControlKey, label: model.Controls.Comparison.Label()},
		}
		if model.Controls.RichDiff {
			local = append(local, footerEntry{key: workspace.DiffHighlightKey, label: "diff highlight"})
		}
		entries = append(destinations, append(local,
			footerEntry{key: "j/k", label: "move"},
			footerEntry{key: "h/l", label: "fold"},
			footerEntry{key: "z", label: "swap"},
			footerEntry{key: "x", label: "review"},
			footerEntry{key: "R", label: "bounds"},
			footerEntry{key: "X", label: "next gap"},
			footerEntry{key: "r", label: "refresh"},
			footerEntry{key: "q", label: "quit"},
		)...)
	}
	if model.Workspace == workspace.Git && model.Controls.Git == workspace.GitStashes {
		local := []footerEntry{
			{key: workspace.SecondaryControlKey, label: model.Controls.Git.Label()},
		}
		if model.Controls.RichDiff {
			local = append(local, footerEntry{key: workspace.DiffHighlightKey, label: "diff highlight"})
		}
		stashEntries := []footerEntry{
			footerEntry{key: "j/k", label: "move stashes"},
			footerEntry{key: "f/F", label: "move files"},
		}
		if model.ReaderContextFoldable {
			stashEntries = append(stashEntries, footerEntry{key: "h/l", label: "context"})
		}
		stashEntries = append(stashEntries,
			footerEntry{key: "z", label: "swap"},
			footerEntry{key: "r", label: "refresh"},
			footerEntry{key: "q", label: "quit"},
		)
		entries = append(destinations, append(local, stashEntries...)...)
	} else if model.Workspace == workspace.Git {
		local := []footerEntry{{key: workspace.SecondaryControlKey, label: model.Controls.Git.Label()}}
		if model.Controls.Git == workspace.GitLog {
			local = append(local, footerEntry{key: workspace.TertiaryControlKey, label: model.Controls.Traversal.Label()})
		}
		entries = append(destinations, append(local,
			footerEntry{key: "j/k or ↑/↓", label: "navigate"},
			footerEntry{key: "z", label: "swap"},
			footerEntry{key: "r", label: "refresh"},
			footerEntry{key: "q", label: "quit"},
		)...)
	}
	return fit(renderFooterEntries(entries), width)
}

func renderFooterEntries(entries []footerEntry) string {
	var rendered strings.Builder
	for index, entry := range entries {
		if index > 0 {
			rendered.WriteString(renderFooterSeparator())
		}
		rendered.WriteString(renderFooterEntry(entry))
	}
	return rendered.String()
}

func renderFooterEntry(entry footerEntry) string {
	key := headerStyle.Render(SafeSingleLine(entry.key))
	if entry.label == "" {
		return key
	}
	return key + chromeStyle.Render(" "+SafeSingleLine(entry.label))
}

func renderFooterSeparator() string {
	return mutedStyle.Render(" • ")
}

func renderHeader(model Model) string {
	g := model.Geometry
	switcher := renderWorkspaceSwitcher(g.HeaderSwitcher.Width, model.Workspace)
	left := switcher
	for _, control := range layoutHeaderControls(g, model.Workspace, model.Controls) {
		padding := strings.Repeat(" ", max(0, control.rect.X-lipgloss.Width(left)))
		left += padding + renderHeaderControl(control, g.Header.Width >= wideHeaderControls)
	}
	if model.Workspace != workspace.Files || !model.Changes.Ready {
		return fit(left, g.Header.Width)
	}
	for _, summary := range []string{renderChangeSummary(model.Changes), renderChangeTotals(model.Changes)} {
		if summary == "" {
			continue
		}
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
	case HitDiffHighlightControl:
		style = headerStyle
	}
	key := ""
	if wide {
		key = style.Bold(true).Render(control.key) + " "
	}
	return key + chromeStyle.Render("[") + style.Bold(true).Render(control.value) + chromeStyle.Render("]")
}

func renderChangeSummary(summary ChangeSummary) string {
	result := chromeStyle.Render(fmt.Sprintf("%d changes", summary.Files))
	if totals := renderChangeTotals(summary); totals != "" {
		result += " " + totals
	}
	return result
}

func renderChangeTotals(summary ChangeSummary) string {
	additions, deletions := FormatLineChanges(LineChanges{
		Additions: summary.Additions,
		Deletions: summary.Deletions,
	})
	parts := make([]string, 0, 2)
	if additions != "" {
		parts = append(parts, addedStyle.Render(additions))
	}
	if deletions != "" {
		parts = append(parts, errorStyle.Render(deletions))
	}
	return strings.Join(parts, " ")
}

func renderWorkspaceSwitcher(width int, activeWorkspace workspace.Kind) string {
	labels := []struct {
		kind  workspace.Kind
		label string
	}{
		{workspace.Files, "files"},
		{workspace.Git, "git"},
		{workspace.Notes, "notes"},
	}
	var rendered strings.Builder
	for index, item := range labels {
		if index > 0 {
			rendered.WriteString(mutedStyle.Render(" | "))
		}
		if item.kind == activeWorkspace {
			rendered.WriteString(headerStyle.Render(item.label))
		} else {
			rendered.WriteString(chromeStyle.Render(item.label))
		}
	}
	return fit(rendered.String(), min(max(0, width), len(workspaceSwitcher)))
}

func renderNotes(model Model) string {
	g := model.Geometry
	presentation := model.Notes
	document := presentation.Document
	rows := make([]string, 0, g.NotesRows.Height)
	bar, overflow := CalculateScrollbar(g.NotesRows, len(document.Rows), presentation.Top)
	textRows := g.NotesRows
	var scrollbar []string
	if overflow {
		textRows = bar.Content
		scrollbar = verticalScrollbar(bar, true)
	}
	cursorRow := document.RowForIndex(presentation.Cursor)
	for visible := 0; visible < g.NotesRows.Height; visible++ {
		rowIndex := presentation.Top + visible
		line := ""
		if rowIndex < len(document.Rows) {
			line = renderNotesRow(document.Rows[rowIndex], rowIndex == cursorRow, presentation, textRows.Width)
		}
		line = fit(line, textRows.Width)
		if overflow {
			line += scrollbar[visible]
		}
		rows = append(rows, line)
	}
	return renderSurface(
		g.Body,
		g.NotesTitle,
		g.NotesRows,
		renderNotesTitle(g, model.NotesScope, model.NotesHasWorktree),
		rows,
	)
}

func renderNotesTitle(g Geometry, scope notes.Scope, hasWorktree bool) string {
	if !hasWorktree {
		return renderTitle("Notes", true)
	}
	width := min(g.NotesTitle.Width, g.NotesWorktreeScope.X+g.NotesWorktreeScope.Width-g.NotesTitle.X)
	value := []byte(strings.Repeat(" ", max(0, width)))
	paint := func(x int, label string) {
		for index := 0; index < len(label); index++ {
			position := x - g.NotesTitle.X + index
			if position >= 0 && position < len(value) {
				value[position] = label[index]
			}
		}
	}
	paint(g.NotesTitle.X, "Notes")
	paint(g.NotesProjectScope.X+1, "project")
	paint(g.NotesWorktreeScope.X+1, "worktree")
	selected := g.NotesProjectScope
	if scope == notes.Worktree {
		selected = g.NotesWorktreeScope
	}
	if selected.Width > 0 {
		value[selected.X-g.NotesTitle.X] = '['
		if selected.Width > 1 {
			value[selected.X-g.NotesTitle.X+selected.Width-1] = ']'
		}
	}
	var rendered strings.Builder
	isFocused := func(index int) bool {
		x := g.NotesTitle.X + index
		return x < g.NotesTitle.X+len("Notes") || selected.Contains(x, g.NotesTitle.Y)
	}
	for index := 0; index < len(value); {
		focused := isFocused(index)
		style := chromeStyle
		if focused {
			style = focusedTitleStyle
		}
		end := index + 1
		for end < len(value) && isFocused(end) == focused {
			end++
		}
		rendered.WriteString(style.Render(string(value[index:end])))
		index = end
	}
	return rendered.String()
}

func renderNotesRow(row notes.Row, cursorRow bool, presentation notes.Presentation, width int) string {
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
		case cell.Index >= 0 && cell.Index < len(presentation.Styles):
			style := presentation.Styles[cell.Index]
			value = renderTextStyle(value, TextStyle{
				Foreground: style.Foreground,
				Bold:       style.Bold,
				Italic:     style.Italic,
				Underline:  style.Underline,
			})
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
	bar, overflow := CalculateScrollbar(g.NavigatorRows, len(model.NavigatorRows), model.Top)
	contentWidth := g.NavigatorRows.Width
	var scrollbar []string
	if overflow {
		contentWidth = bar.Content.Width
		scrollbar = verticalScrollbar(bar, model.Focus == navigation.FocusNavigator)
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
			if overflow {
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
		if overflow {
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
		row += chromeStyle.Inherit(selection).Render(fit(progress, layout.Progress.Width))
	}
	if layout.Changes.Width > 0 {
		additions, deletions := FormatLineChanges(*item.Changes)
		if additions != "" {
			row += selection.Render(" ") + addedStyle.Inherit(selection).Render(additions)
		}
		if deletions != "" {
			row += selection.Render(" ") + errorStyle.Inherit(selection).Render(deletions)
		}
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
		return chromeStyle
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
	document := model.ReaderDocument
	commitRows := model.ReaderCommitRows
	if document.Kind == ReaderDocumentNone && len(content) == 0 && len(commitRows) == 0 && model.ReaderEmpty.Text != "" {
		content = []Line{model.ReaderEmpty}
	}
	total := len(content)
	readerOffset := model.ReaderOffset
	readerLayout := ReaderLayout{}
	if document.Kind != ReaderDocumentNone {
		readerLayout = CalculateReaderLayout(g.ReaderRows, document)
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
	readerGeometry := CalculateReaderGeometry(g.ReaderRows, document, scrollbar != nil)
	if document.Kind != ReaderDocumentNone {
		readerGeometry = readerLayout.Geometry
	}
	commitColumns := commitrow.Measure(commitRows, contentWidth)
	now := time.Now()
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := readerOffset + row
		if index < total {
			line := ""
			if len(commitRows) != 0 {
				line = renderCommitRow(commitRows[index], commitColumns, contentWidth, false, false, now)
			} else if document.Kind != ReaderDocumentNone {
				wrapped, continuation := readerLayout.Row(index)
				line = renderReaderRowPart(wrapped, readerGeometry, model.Controls.DiffHighlight, continuation)
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

func renderReaderRow(row ReaderRow, geometry ReaderGeometry, highlight workspace.DiffHighlight) string {
	return renderReaderRowPart(row, geometry, highlight, false)
}

func renderReaderRowPart(row ReaderRow, geometry ReaderGeometry, highlight workspace.DiffHighlight, continuation bool) string {
	width := geometry.Content.Width
	if width <= 0 {
		return ""
	}
	changed := row.Kind == ReaderInsertion || row.Kind == ReaderDeletion
	background := changed && highlight == workspace.DiffHighlightBackground

	bar := " "
	barStyle := lipgloss.NewStyle()
	switch row.Kind {
	case ReaderInsertion:
		bar = "▌"
		barStyle = barStyle.Foreground(addedColor).Bold(true)
	case ReaderDeletion:
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

	payload := renderReaderPayload(row, nil)
	if row.Kind == ReaderFold {
		payload = renderReaderFoldPayload(row.Text, geometry.Code.Width)
	}
	line := barStyle.Render(bar) + mutedStyle.Render(number) + payload
	return fit(line, width)
}

func renderReaderFoldPayload(text string, width int) string {
	if width <= 0 {
		return ""
	}
	label := "── ▸ folded · " + SafeSingleLine(text) + " "
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
		return mutedStyle.Render(text)
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
	return chromeStyle.Render(title)
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
	style := chromeStyle
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
