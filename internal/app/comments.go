package app

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/comments"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type visualLineSelection struct {
	File         string
	Context      string
	Side         comments.Side
	Anchor       comments.SourceLine
	Active       comments.SourceLine
	Kind         visualSelectionKind
	AnchorColumn int
	ActiveColumn int
	MouseMoved   bool
}

type visualSelectionKind uint8

const (
	visualSelectionLine visualSelectionKind = iota
	visualSelectionCharacter
)

type commentDraft struct {
	Identity       string
	FileIdentity   string
	File           string
	Context        string
	SourceIdentity string
	Range          comments.Range
	PreferredLine  uint64
	Snippet        string
	Fingerprint    comments.ContextFingerprint
}

type commentHover struct {
	File    string
	Context string
	Side    comments.Side
	Line    comments.SourceLine
}

type readerSourceLine struct {
	index int
	line  comments.SourceLine
}

func (state filesState) commentContext() string {
	if state.readerMode.Label() == "file" {
		return "file"
	}
	comparison := state.snapshot.Comparison()
	// Scope identifies the reader realm; mutable revisions belong in the
	// source identity so a refreshed new side can be reconciled in place.
	return fmt.Sprintf("diff:%s", comparison.Scope)
}

func sourceSide(row ui.ReaderRow) (comments.Side, bool) {
	if row.Kind == ui.ReaderFile && row.NewLine > 0 {
		return comments.CurrentSide, true
	}
	if row.Kind == ui.ReaderDeletion && row.OldLine > 0 {
		return comments.OldSide, true
	}
	if row.NewLine > 0 && row.Commentable() {
		return comments.NewSide, true
	}
	if row.OldLine > 0 && row.Commentable() {
		return comments.OldSide, true
	}
	return comments.NewSide, false
}

func sourceNumber(row ui.ReaderRow, side comments.Side) (uint64, bool) {
	if side == comments.OldSide {
		return row.OldLine, row.OldLine > 0 && row.Commentable()
	}
	return row.NewLine, row.NewLine > 0 && row.Commentable()
}

func encodedSourceRow(row ui.ReaderRow) string {
	marker := " "
	switch row.Kind {
	case ui.ReaderInsertion:
		marker = "+"
	case ui.ReaderDeletion:
		marker = "-"
	}
	return marker + ui.SafeSingleLine(row.Text)
}

func (state filesState) commentSourceSnapshot(side comments.Side) comments.SourceSnapshot {
	document := state.rawReaderDocument()
	source := readerSourceLines(document, side)
	lines := make([]comments.SnapshotLine, 0, len(source))
	var identity strings.Builder
	comparison := state.snapshot.Comparison()
	revision := comparison.Target
	if side == comments.OldSide {
		revision = comparison.Basis
	}
	fmt.Fprintf(&identity, "%s\x00%d\x00%s\x00", state.commentContext(), side, revision)
	for _, line := range source {
		text := encodedSourceRow(document.Rows[line.index])
		lines = append(lines, comments.SnapshotLine{
			Identity: line.line.Identity, Number: line.line.Number, Text: text,
		})
		fmt.Fprintf(&identity, "%d\x00%s\x00", line.line.Number, text)
	}
	if len(lines) == 0 {
		return comments.SourceSnapshot{}
	}
	sum := sha256.Sum256([]byte(identity.String()))
	return comments.SourceSnapshot{Identity: fmt.Sprintf("sha256:%x", sum), Lines: lines}
}

func readerSourceLines(document ui.ReaderDocument, side comments.Side) []readerSourceLine {
	identities := readerRowIdentities(document.Rows)
	lines := make([]readerSourceLine, 0, len(document.Rows))
	for index, row := range document.Rows {
		number, ok := sourceNumber(row, side)
		if !ok {
			continue
		}
		lines = append(lines, readerSourceLine{
			index: index,
			line:  comments.SourceLine{Identity: identities[index], Number: number},
		})
	}
	return lines
}

func readerSourceAt(document ui.ReaderDocument, index int, side *comments.Side) (comments.Side, comments.SourceLine, bool) {
	if index < 0 || index >= len(document.Rows) {
		return comments.NewSide, comments.SourceLine{}, false
	}
	selectedSide := comments.NewSide
	if side == nil {
		var ok bool
		selectedSide, ok = sourceSide(document.Rows[index])
		if !ok {
			return comments.NewSide, comments.SourceLine{}, false
		}
	} else {
		selectedSide = *side
	}
	number, ok := sourceNumber(document.Rows[index], selectedSide)
	if !ok {
		return selectedSide, comments.SourceLine{}, false
	}
	identities := readerRowIdentities(document.Rows)
	return selectedSide, comments.SourceLine{Identity: identities[index], Number: number}, true
}

func findReaderSource(document ui.ReaderDocument, side comments.Side, target comments.SourceLine) (int, bool) {
	lines := readerSourceLines(document, side)
	if len(lines) == 0 {
		return 0, false
	}
	for _, line := range lines {
		if line.line.Identity == target.Identity {
			return line.index, true
		}
	}
	best := 0
	for index := 1; index < len(lines); index++ {
		if lineDistance(lines[index].line.Number, target.Number) < lineDistance(lines[best].line.Number, target.Number) {
			best = index
		}
	}
	return lines[best].index, true
}

func lineDistance(left, right uint64) uint64 {
	if left > right {
		return left - right
	}
	return right - left
}

func reconcileSourceLine(oldDocument, currentDocument ui.ReaderDocument, side comments.Side, target comments.SourceLine) (comments.SourceLine, bool) {
	oldLines := readerSourceLines(oldDocument, side)
	currentLines := readerSourceLines(currentDocument, side)
	if len(currentLines) == 0 {
		return comments.SourceLine{}, false
	}
	currentByIdentity := make(map[string]comments.SourceLine, len(currentLines))
	for _, line := range currentLines {
		currentByIdentity[line.line.Identity] = line.line
	}
	if line, ok := currentByIdentity[target.Identity]; ok {
		return line, true
	}
	oldIndex := 0
	found := false
	for index, line := range oldLines {
		if line.line.Identity == target.Identity {
			oldIndex, found = index, true
			break
		}
	}
	if !found {
		for index := 1; index < len(oldLines); index++ {
			if lineDistance(oldLines[index].line.Number, target.Number) < lineDistance(oldLines[oldIndex].line.Number, target.Number) {
				oldIndex = index
			}
		}
	}
	oldIdentities := make([]string, len(oldLines))
	currentIdentities := make([]string, len(currentLines))
	for index, line := range oldLines {
		oldIdentities[index] = line.line.Identity
	}
	for index, line := range currentLines {
		currentIdentities[index] = line.line.Identity
	}
	return currentLines[reconcileLogicalLine(oldIdentities, oldIndex, currentIdentities)].line, true
}

func (state *filesState) reconcileCommentInteraction(oldDocument ui.ReaderDocument) {
	current := state.rawReaderDocument()
	context := state.commentContext()
	if selection := state.visualSelection; selection != nil {
		if selection.File != state.readerEntry.Path || selection.Context != context {
			state.visualSelection = nil
			state.readerSelectionDragging = false
		} else {
			anchor, anchorOK := reconcileSourceLine(oldDocument, current, selection.Side, selection.Anchor)
			active, activeOK := reconcileSourceLine(oldDocument, current, selection.Side, selection.Active)
			if !anchorOK || !activeOK {
				state.visualSelection = nil
				state.readerSelectionDragging = false
			} else {
				selection.Anchor, selection.Active = anchor, active
				clampVisualSelectionColumns(current, selection)
			}
		}
	}
	if hover := state.commentHover; hover != nil {
		if hover.File != state.readerEntry.Path || hover.Context != context {
			state.commentHover = nil
		} else if line, ok := reconcileSourceLine(oldDocument, current, hover.Side, hover.Line); ok {
			hover.Line = line
		} else {
			state.commentHover = nil
		}
	}
	fileIdentity := filetree.FileIdentity(state.readerEntry.Path)
	for _, side := range []comments.Side{comments.CurrentSide, comments.NewSide} {
		snapshot := state.commentSourceSnapshot(side)
		if state.comments.Reconcile(fileIdentity, context, side, snapshot) {
			state.readerRevision++
		}
	}
	state.readerRevision++
}

func clampVisualSelectionColumns(document ui.ReaderDocument, selection *visualLineSelection) {
	if selection == nil || selection.Kind != visualSelectionCharacter {
		return
	}
	if index, ok := findReaderSource(document, selection.Side, selection.Anchor); ok {
		selection.AnchorColumn = min(max(0, selection.AnchorColumn), ansi.StringWidth(ui.SafeSingleLine(document.Rows[index].Text)))
	}
	if index, ok := findReaderSource(document, selection.Side, selection.Active); ok {
		selection.ActiveColumn = min(max(0, selection.ActiveColumn), ansi.StringWidth(ui.SafeSingleLine(document.Rows[index].Text)))
	}
}

func (state *filesState) resetReaderInteraction() {
	state.visualSelection = nil
	state.readerSelectionDragging = false
	state.commentDraft = nil
	state.commentHover = nil
	state.readerRevision++
}

func (state *filesState) startVisualSelection(index int) bool {
	document := state.readerDocument()
	side, line, ok := readerSourceAt(document, index, nil)
	if !ok || state.readerEntry.Path == "" || state.markdownPreviewActive() {
		return false
	}
	state.visualSelection = &visualLineSelection{
		File: state.readerEntry.Path, Context: state.commentContext(), Side: side,
		Anchor: line, Active: line,
	}
	state.commentHover = nil
	state.readerRevision++
	return true
}

func (state *filesState) startCharacterSelection(point ui.ReaderPoint) bool {
	document := state.readerDocument()
	side, line, ok := readerSourceAt(document, point.Source, nil)
	if !ok || state.readerEntry.Path == "" || state.markdownPreviewActive() {
		return false
	}
	column := max(0, point.Column)
	state.visualSelection = &visualLineSelection{
		File: state.readerEntry.Path, Context: state.commentContext(), Side: side,
		Anchor: line, Active: line, Kind: visualSelectionCharacter,
		AnchorColumn: column, ActiveColumn: column,
	}
	state.readerSelectionDragging = true
	state.commentHover = nil
	state.readerRevision++
	return true
}

func (state *filesState) finishCharacterSelection() bool {
	if !state.readerSelectionDragging {
		return false
	}
	state.readerSelectionDragging = false
	if state.visualSelection != nil && state.visualSelection.Kind == visualSelectionCharacter && !state.visualSelection.MouseMoved {
		state.visualSelection = nil
	}
	state.readerRevision++
	return true
}

func (m *Model) beginFilesCharacterSelection(point ui.ReaderPoint) {
	document := m.files.readerDocument()
	if point.Source < 0 || point.Source >= len(document.Rows) || !document.Rows[point.Source].Commentable() {
		return
	}
	m.files.place.Focus = navigation.FocusReader
	m.files.place.ReaderCursor = point.Source
	if m.files.startCharacterSelection(point) {
		m.ensureActiveReaderSelectionVisible()
	}
}

func (m *Model) dragFilesCharacterSelection(point ui.ReaderPoint) {
	selection := m.files.visualSelection
	if !m.files.readerSelectionDragging || selection == nil || selection.Kind != visualSelectionCharacter {
		return
	}
	document := m.files.readerDocument()
	previousLine, previousColumn := selection.Active, selection.ActiveColumn
	m.selectFilesVisualLine(document, point.Source)
	activeIndex, ok := findReaderSource(document, selection.Side, selection.Active)
	if ok {
		width := ansi.StringWidth(ui.SafeSingleLine(document.Rows[activeIndex].Text))
		if activeIndex == point.Source {
			selection.ActiveColumn = max(0, min(point.Column, width))
		} else if point.Source < activeIndex {
			selection.ActiveColumn = 0
		} else {
			selection.ActiveColumn = width
		}
	}
	selection.MouseMoved = selection.MouseMoved || selection.Active != selection.Anchor || selection.ActiveColumn != selection.AnchorColumn
	if selection.Active != previousLine || selection.ActiveColumn != previousColumn {
		m.files.readerRevision++
	}
}

func (state filesState) visualSelectionText() (string, bool) {
	selection := state.visualSelection
	if selection == nil || selection.File != state.readerEntry.Path || selection.Context != state.commentContext() {
		return "", false
	}
	document := state.readerDocument()
	anchor, anchorOK := findReaderSource(document, selection.Side, selection.Anchor)
	active, activeOK := findReaderSource(document, selection.Side, selection.Active)
	if !anchorOK || !activeOK {
		return "", false
	}
	first, last := anchor, active
	firstColumn, lastColumn := selection.AnchorColumn, selection.ActiveColumn
	if first > last {
		first, last = last, first
		firstColumn, lastColumn = lastColumn, firstColumn
	}
	lines := make([]string, 0, last-first+1)
	for index := first; index <= last; index++ {
		if _, ok := sourceNumber(document.Rows[index], selection.Side); !ok {
			continue
		}
		text := ui.SafeSingleLine(document.Rows[index].Text)
		if selection.Kind == visualSelectionCharacter {
			width := ansi.StringWidth(text)
			start, end := 0, width
			switch {
			case first == last:
				start = min(firstColumn, lastColumn)
				end = max(firstColumn, lastColumn) + 1
			case index == first:
				start = firstColumn
			case index == last:
				end = lastColumn + 1
			}
			start = max(0, min(start, width))
			end = max(start, min(end, width))
			text = ansi.Cut(text, start, end)
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n"), len(lines) != 0
}

func (state filesState) visualActive() bool {
	return state.visualSelection != nil && state.commentDraft == nil
}

func (state filesState) characterSelectionActive() bool {
	return state.visualActive() && state.visualSelection.Kind == visualSelectionCharacter
}

func (state filesState) composingComment() bool { return state.commentDraft != nil }

func (state *filesState) cancelComment() bool {
	if state.commentDraft == nil {
		return false
	}
	state.commentDraft = nil
	state.commentEditor.Load("")
	state.readerRevision++
	return true
}

func (state *filesState) setCommentHover(index int) bool {
	document := state.readerDocument()
	side, line, ok := readerSourceAt(document, index, nil)
	if !ok {
		return state.clearCommentHover()
	}
	next := commentHover{File: state.readerEntry.Path, Context: state.commentContext(), Side: side, Line: line}
	if state.commentHover != nil && *state.commentHover == next {
		return false
	}
	state.commentHover = &next
	state.readerRevision++
	return true
}

func (state *filesState) clearCommentHover() bool {
	if state.commentHover == nil {
		return false
	}
	state.commentHover = nil
	state.readerRevision++
	return true
}

func (state filesState) hoveredCommentLine(index int) bool {
	if state.commentHover == nil {
		return false
	}
	document := state.readerDocument()
	side, line, ok := readerSourceAt(document, index, nil)
	return ok && side == state.commentHover.Side && line == state.commentHover.Line
}

func (state *filesState) beginComment(index int, single bool, geometry ui.Geometry) bool {
	document := state.readerDocument()
	var side comments.Side
	var anchor, active comments.SourceLine
	if !single && state.visualSelection != nil {
		selection := state.visualSelection
		if selection.File != state.readerEntry.Path || selection.Context != state.commentContext() {
			return false
		}
		side, anchor, active = selection.Side, selection.Anchor, selection.Active
	} else {
		var ok bool
		side, anchor, ok = readerSourceAt(document, index, nil)
		if !ok {
			return false
		}
		active = anchor
		if single {
			state.visualSelection = nil
			state.readerSelectionDragging = false
		}
	}
	rangeValue := comments.Range{Side: side, Start: anchor, End: active}.Normalize()
	snapshot := state.commentSourceSnapshot(side)
	fingerprint := commentFingerprint(snapshot, anchor, active)
	state.commentDraftSequence++
	state.commentDraft = &commentDraft{
		Identity:     fmt.Sprintf("comment-draft:%d", state.commentDraftSequence),
		FileIdentity: filetree.FileIdentity(state.readerEntry.Path),
		File:         state.readerEntry.Path, Context: state.commentContext(), SourceIdentity: snapshot.Identity,
		Range: rangeValue, PreferredLine: active.Number,
		Snippet:     state.commentSnippet(side, anchor, active),
		Fingerprint: fingerprint,
	}
	state.commentEditor = notes.NewEditor()
	state.resizeCommentComposer(geometry)
	state.commentHover = nil
	state.readerRevision++
	return true
}

func commentFingerprint(snapshot comments.SourceSnapshot, anchor, active comments.SourceLine) comments.ContextFingerprint {
	first, last := -1, -1
	for index, line := range snapshot.Lines {
		if line.Identity == anchor.Identity {
			first = index
		}
		if line.Identity == active.Identity {
			last = index
		}
	}
	if first < 0 || last < 0 {
		return comments.ContextFingerprint{}
	}
	if first > last {
		first, last = last, first
	}
	beforeStart := max(0, first-comments.ContextRadius)
	afterEnd := min(len(snapshot.Lines), last+1+comments.ContextRadius)
	fingerprint := comments.ContextFingerprint{}
	for _, line := range snapshot.Lines[beforeStart:first] {
		fingerprint.Before = append(fingerprint.Before, line.Text)
	}
	for _, line := range snapshot.Lines[last+1 : afterEnd] {
		fingerprint.After = append(fingerprint.After, line.Text)
	}
	return fingerprint
}

func (state filesState) commentSnippet(side comments.Side, anchor, active comments.SourceLine) string {
	document := state.rawReaderDocument()
	lines := readerSourceLines(document, side)
	first, last := -1, -1
	for index, line := range lines {
		if line.line.Identity == anchor.Identity {
			first = index
		}
		if line.line.Identity == active.Identity {
			last = index
		}
	}
	if first < 0 || last < 0 {
		return ""
	}
	if first > last {
		first, last = last, first
	}
	result := make([]string, 0, last-first+1)
	for _, line := range lines[first : last+1] {
		row := document.Rows[line.index]
		result = append(result, encodedSourceRow(row))
	}
	return strings.Join(result, "\n")
}

func (state *filesState) submitComment() (comments.Comment, bool) {
	if state.commentDraft == nil {
		return comments.Comment{}, false
	}
	text := strings.TrimSpace(state.commentEditor.Text())
	if text == "" {
		state.cancelComment()
		return comments.Comment{}, false
	}
	draft := state.commentDraft
	comment := state.comments.Add(comments.Draft{
		FileIdentity: draft.FileIdentity, File: draft.File, Context: draft.Context,
		SourceIdentity: draft.SourceIdentity, Range: draft.Range, PreferredLine: draft.PreferredLine,
		Snippet: draft.Snippet, Fingerprint: draft.Fingerprint, Text: text,
	})
	state.commentDraft = nil
	state.visualSelection = nil
	state.readerSelectionDragging = false
	state.commentEditor.Load("")
	state.readerRevision++
	return comment, true
}

func (state *filesState) setCommentFold(identity string, expanded bool) bool {
	if identity == "" {
		return false
	}
	collapsed := state.commentFolds[identity]
	if expanded == !collapsed {
		return false
	}
	if expanded {
		delete(state.commentFolds, identity)
	} else {
		state.commentFolds[identity] = true
	}
	state.readerRevision++
	return true
}

func (state *filesState) toggleCommentFold(identity string) bool {
	return state.setCommentFold(identity, state.commentFolds[identity])
}

func (state filesState) commentExpanded(identity string) bool { return !state.commentFolds[identity] }

func (state *filesState) resizeCommentComposer(geometry ui.Geometry) {
	if state.commentDraft == nil {
		return
	}
	width := max(1, geometry.ReaderRows.Width-7)
	height := max(1, min(6, geometry.ReaderRows.Height-3))
	state.commentEditor.Resize(width, height)
}

func (state filesState) decorateReaderDocument(source ui.ReaderDocument) ui.ReaderDocument {
	if source.Kind == ui.ReaderDocumentNone {
		return source
	}
	rows := append([]ui.ReaderRow(nil), source.Rows...)
	if selection := state.visualSelection; selection != nil && selection.File == state.readerEntry.Path && selection.Context == state.commentContext() {
		decorateVisualSelection(rows, selection)
	}
	if hover := state.commentHover; hover != nil && hover.File == state.readerEntry.Path && hover.Context == state.commentContext() {
		if index, ok := findReaderSource(source, hover.Side, hover.Line); ok {
			rows[index].CommentHover = true
		}
	}

	insertions := make(map[int][]ui.ReaderRow)
	for _, comment := range state.comments.In(state.readerEntry.Path, state.commentContext()) {
		if anchor, ok := state.commentPresentationAnchor(source, comment); ok {
			insertions[anchor] = append(insertions[anchor], state.commentRows(comment)...)
		}
	}
	if draft := state.commentDraft; draft != nil && draft.File == state.readerEntry.Path && draft.Context == state.commentContext() {
		if anchor, ok := findReaderSource(source, draft.Range.Side, draft.Range.End); ok {
			insertions[anchor] = append(insertions[anchor], state.commentComposerRows()...)
		}
	}
	if len(insertions) == 0 {
		source.Rows = rows
		return source
	}
	decorated := make([]ui.ReaderRow, 0, len(rows)+len(insertions)*3)
	for index, row := range rows {
		decorated = append(decorated, row)
		decorated = append(decorated, insertions[index]...)
	}
	source.Rows = decorated
	return source
}

func decorateVisualSelection(rows []ui.ReaderRow, selection *visualLineSelection) {
	low, high := selection.Anchor.Number, selection.Active.Number
	lowColumn, highColumn := selection.AnchorColumn, selection.ActiveColumn
	if low > high {
		low, high = high, low
		lowColumn, highColumn = highColumn, lowColumn
	}
	for index := range rows {
		number, ok := sourceNumber(rows[index], selection.Side)
		if !ok || number < low || number > high {
			continue
		}
		if selection.Kind == visualSelectionLine {
			rows[index].VisualSelected = true
			continue
		}
		width := ansi.StringWidth(ui.SafeSingleLine(rows[index].Text))
		start, end := 0, width
		switch {
		case low == high:
			start = min(lowColumn, highColumn)
			end = max(lowColumn, highColumn) + 1
		case number == low:
			start = lowColumn
		case number == high:
			end = highColumn + 1
		}
		start = max(0, min(start, width))
		end = max(start, min(end, width))
		if start < end {
			rows[index].VisualCharacter = true
			rows[index].VisualStart = start
			rows[index].VisualEnd = end
		}
	}
}

func (state filesState) commentRows(comment comments.Comment) []ui.ReaderRow {
	expanded := state.commentExpanded(comment.ID)
	rows := []ui.ReaderRow{{
		Identity: comment.ID + ":header", Kind: ui.ReaderCommentHeader,
		Text: comment.Location(), CommentID: comment.ID, FoldExpanded: expanded,
		CommentOldSide: comment.Range.Side == comments.OldSide,
		CommentStart:   comment.Range.Start.Number, CommentEnd: comment.Range.End.Number,
		CommentStale: comment.Stale,
	}}
	if expanded {
		for index, line := range strings.Split(comment.Text, "\n") {
			rows = append(rows, ui.ReaderRow{
				Identity: fmt.Sprintf("%s:body:%d", comment.ID, index),
				Kind:     ui.ReaderCommentBody, Text: line, CommentID: comment.ID,
			})
		}
	}
	return append(rows, ui.ReaderRow{
		Identity: comment.ID + ":end", Kind: ui.ReaderCommentEnd, CommentID: comment.ID,
	})
}

func (state filesState) commentComposerRows() []ui.ReaderRow {
	draft := state.commentDraft
	if draft == nil {
		return nil
	}
	location := comments.Comment{File: draft.File, Range: draft.Range}.Location()
	rows := []ui.ReaderRow{{
		Identity: draft.Identity + ":header", Kind: ui.ReaderCommentComposerHeader,
		Text: location, CommentID: draft.Identity,
	}}
	presentation := state.commentEditor.Presentation()
	cursorRow := presentation.Document.RowForIndex(presentation.Cursor)
	end := min(len(presentation.Document.Rows), presentation.Top+max(1, presentation.Height))
	for rowIndex := presentation.Top; rowIndex < end; rowIndex++ {
		row := presentation.Document.Rows[rowIndex]
		var text strings.Builder
		for _, cell := range row.Cells {
			text.WriteString(cell.Display)
		}
		cursor := rowIndex == cursorRow
		column := 0
		if cursor {
			column = row.ColumnAt(presentation.Cursor)
		}
		rows = append(rows, ui.ReaderRow{
			Identity: fmt.Sprintf("%s:body:%d", draft.Identity, row.Start),
			Kind:     ui.ReaderCommentComposerBody, Text: text.String(), CommentID: draft.Identity,
			ComposerCursor: cursor, ComposerCursorColumn: column,
		})
	}
	return append(rows, ui.ReaderRow{
		Identity: draft.Identity + ":end", Kind: ui.ReaderCommentComposerEnd, CommentID: draft.Identity,
	})
}

func (state filesState) commentPresentationAnchor(document ui.ReaderDocument, comment comments.Comment) (int, bool) {
	snapshot := state.commentSourceSnapshot(comment.Range.Side)
	if comment.Range.Side == comments.OldSide && comment.SourceIdentity != snapshot.Identity {
		return 0, false
	}
	if !comment.Stale && comment.SourceIdentity == snapshot.Identity {
		lines := readerSourceLines(document, comment.Range.Side)
		for _, line := range lines {
			if line.line.Identity == comment.Range.End.Identity {
				return line.index, true
			}
		}
	}
	// A stale comment may remain visibly unresolved at a disposable nearest
	// presentation owner. This never mutates its canonical authored range.
	return findReaderSource(document, comment.Range.Side, comments.SourceLine{Number: comment.PreferredLine})
}

func (m *Model) applyCommentAction(action Action) {
	if !m.files.composingComment() {
		return
	}
	changed := false
	switch action.Kind {
	case CommentInsert:
		changed = m.files.commentEditor.Insert(action.Text)
	case CommentBackspace:
		changed = m.files.commentEditor.Backspace()
	case CommentDelete:
		changed = m.files.commentEditor.Delete()
	case CommentMoveLeft:
		m.files.commentEditor.MoveHorizontal(-1, action.Selecting)
	case CommentMoveRight:
		m.files.commentEditor.MoveHorizontal(1, action.Selecting)
	case CommentMoveUp:
		m.files.commentEditor.MoveVertical(-1, action.Selecting)
	case CommentMoveDown:
		m.files.commentEditor.MoveVertical(1, action.Selecting)
	case CommentMoveWordLeft:
		m.files.commentEditor.MoveWord(-1, action.Selecting)
	case CommentMoveWordRight:
		m.files.commentEditor.MoveWord(1, action.Selecting)
	case CommentMoveHome:
		m.files.commentEditor.MoveHome(action.Selecting)
	case CommentMoveEnd:
		m.files.commentEditor.MoveEnd(action.Selecting)
	case CommentSubmit:
		_, changed = m.files.submitComment()
	case CommentCancel:
		changed = m.files.cancelComment()
	}
	if action.Kind != CommentSubmit && action.Kind != CommentCancel {
		m.files.resizeCommentComposer(m.geometry)
		m.files.readerRevision++
	}
	if changed || m.files.composingComment() {
		m.ensureCommentComposerVisible()
	}
}

func (m *Model) cancelFilesVisualSelection() {
	selection := m.files.visualSelection
	if selection == nil {
		return
	}
	document := m.files.readerDocument()
	anchor, ok := findReaderSource(document, selection.Side, selection.Anchor)
	m.files.visualSelection = nil
	m.files.readerSelectionDragging = false
	m.files.readerRevision++
	if ok {
		m.files.place.ReaderCursor = document.SelectionTarget(anchor)
		m.ensureActiveReaderSelectionVisible()
	}
}

func (m *Model) ensureCommentComposerVisible() {
	if !m.files.composingComment() {
		return
	}
	document := m.files.readerDocument()
	last := -1
	for index, row := range document.Rows {
		if row.Kind == ui.ReaderCommentComposerEnd {
			last = index
		}
	}
	if last < 0 {
		return
	}
	layout := ui.CalculateReaderLayout(m.geometry.ReaderRows, document)
	visual := layout.VisualOffset(last, 0)
	top := m.activeReaderVisualOffset()
	if visual >= top+m.geometry.ReaderRows.Height {
		m.setActiveReaderVisualOffset(visual - m.geometry.ReaderRows.Height + 1)
	}
}

func (m *Model) setActiveReaderFold(expanded bool) effect {
	document, ok := m.activeReaderDocument()
	if !ok || len(document.Rows) == 0 {
		return effect{}
	}
	index := max(0, min(m.activePlace().ReaderCursor, len(document.Rows)-1))
	if identity, comment := document.Rows[index].CommentHeaderIdentity(); comment && m.active == workspace.Files {
		oldRows := readerRowIdentities(document.Rows)
		oldOffset, oldCursor := m.files.place.ReaderOffset, m.files.place.ReaderCursor
		if m.files.setCommentFold(identity, expanded) {
			m.files.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
		}
		return effect{}
	}
	return m.setActiveReaderContextFold(expanded)
}

func (m *Model) toggleActiveReaderFold(identity string) effect {
	if m.active == workspace.Files {
		for _, row := range m.files.readerDocument().Rows {
			if row.Kind == ui.ReaderCommentHeader && row.CommentID == identity {
				oldRows := readerRowIdentities(m.files.readerDocument().Rows)
				oldOffset, oldCursor := m.files.place.ReaderOffset, m.files.place.ReaderCursor
				if m.files.toggleCommentFold(identity) {
					m.files.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
				}
				return effect{}
			}
		}
	}
	return m.toggleActiveReaderContextFold(identity)
}
