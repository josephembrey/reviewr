// Package notes implements the Notes editor and its private persistence.
package notes

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const (
	// UndoLimit bounds whole-buffer snapshots. Notes is deliberately one small note.
	UndoLimit = 100
	TabWidth  = 4
)

// Editor is a grapheme-indexed text buffer. It owns only deterministic editor
// place state; persistence and Bubble Tea messages live outside this package.
type Editor struct {
	graphemes    []string
	cursor       int
	anchor       int
	preferredCol int
	width        int
	height       int
	scroll       int
	dragging     bool
	undo         []snapshot
	redo         []snapshot
}

type snapshot struct {
	graphemes []string
	cursor    int
	anchor    int
	scroll    int
}

// NewEditor returns an empty editor with no selection.
func NewEditor() Editor {
	return Editor{anchor: -1, preferredCol: -1}
}

// Load replaces the buffer without creating undo history.
func (e *Editor) Load(text string) {
	e.graphemes = splitGraphemes(sanitize(text))
	e.cursor = 0
	e.anchor = -1
	e.preferredCol = -1
	e.scroll = 0
	e.dragging = false
	e.undo = nil
	e.redo = nil
	e.clamp()
}

// Reconcile replaces externally refreshed text while preserving cursor,
// selection, preferred column, and the nearest surviving scroll anchor.
func (e *Editor) Reconcile(text string) {
	oldDocument := e.Document()
	topIndex := 0
	if len(oldDocument.Rows) > 0 {
		topIndex = oldDocument.Rows[clamp(e.scroll, 0, len(oldDocument.Rows)-1)].Start
	}
	e.graphemes = splitGraphemes(sanitize(text))
	e.cursor = clamp(e.cursor, 0, len(e.graphemes))
	if e.anchor >= 0 {
		e.anchor = clamp(e.anchor, 0, len(e.graphemes))
		if e.anchor == e.cursor {
			e.anchor = -1
		}
	}
	e.undo = nil
	e.redo = nil
	e.dragging = false
	e.scroll = e.Document().RowForIndex(clamp(topIndex, 0, len(e.graphemes)))
	e.clamp()
}

// Text returns the exact safe note contents.
func (e Editor) Text() string {
	return strings.Join(e.graphemes, "")
}

// Len returns the number of grapheme editing units.
func (e Editor) Len() int { return len(e.graphemes) }

// Cursor returns the grapheme boundary containing the insertion point.
func (e Editor) Cursor() int { return e.cursor }

// Selection returns a normalized half-open grapheme range.
func (e Editor) Selection() (int, int, bool) {
	if e.anchor < 0 || e.anchor == e.cursor {
		return 0, 0, false
	}
	if e.anchor < e.cursor {
		return e.anchor, e.cursor, true
	}
	return e.cursor, e.anchor, true
}

// SelectedText returns the selected safe text.
func (e Editor) SelectedText() string {
	start, end, ok := e.Selection()
	if !ok {
		return ""
	}
	return strings.Join(e.graphemes[start:end], "")
}

// Insert sanitizes and inserts text as one undoable edit.
func (e *Editor) Insert(text string) bool {
	inserted := splitGraphemes(sanitize(text))
	if len(inserted) == 0 {
		return false
	}
	e.remember()
	start, end := e.replacementRange()
	next := make([]string, 0, len(e.graphemes)-(end-start)+len(inserted))
	next = append(next, e.graphemes[:start]...)
	next = append(next, inserted...)
	next = append(next, e.graphemes[end:]...)
	e.graphemes = next
	e.cursor = start + len(inserted)
	e.anchor = -1
	e.preferredCol = -1
	e.ensureCursorVisible()
	return true
}

// Backspace deletes the selection or the grapheme before the cursor.
func (e *Editor) Backspace() bool {
	if start, end, ok := e.Selection(); ok {
		return e.deleteRange(start, end)
	}
	if e.cursor == 0 {
		return false
	}
	return e.deleteRange(e.cursor-1, e.cursor)
}

// Delete deletes the selection or the grapheme after the cursor.
func (e *Editor) Delete() bool {
	if start, end, ok := e.Selection(); ok {
		return e.deleteRange(start, end)
	}
	if e.cursor == len(e.graphemes) {
		return false
	}
	return e.deleteRange(e.cursor, e.cursor+1)
}

func (e *Editor) deleteRange(start, end int) bool {
	start = clamp(start, 0, len(e.graphemes))
	end = clamp(end, start, len(e.graphemes))
	if start == end {
		return false
	}
	e.remember()
	next := make([]string, 0, len(e.graphemes)-(end-start))
	next = append(next, e.graphemes[:start]...)
	next = append(next, e.graphemes[end:]...)
	e.graphemes = next
	e.cursor = start
	e.anchor = -1
	e.preferredCol = -1
	e.ensureCursorVisible()
	return true
}

// Undo restores the preceding bounded edit snapshot.
func (e *Editor) Undo() bool {
	if len(e.undo) == 0 {
		return false
	}
	e.redo = appendBounded(e.redo, e.snapshot())
	state := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.restore(state)
	return true
}

// Redo reapplies the most recently undone edit.
func (e *Editor) Redo() bool {
	if len(e.redo) == 0 {
		return false
	}
	e.undo = appendBounded(e.undo, e.snapshot())
	state := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.restore(state)
	return true
}

// SelectAll selects the complete note with the insertion point at its end.
func (e *Editor) SelectAll() {
	e.anchor = 0
	e.cursor = len(e.graphemes)
	e.preferredCol = -1
	e.ensureCursorVisible()
}

// MoveHorizontal moves by grapheme boundaries. An unshifted movement first
// collapses an existing selection toward the requested direction.
func (e *Editor) MoveHorizontal(delta int, selecting bool) {
	if delta == 0 {
		return
	}
	if !selecting {
		if start, end, ok := e.Selection(); ok {
			if delta < 0 {
				e.cursor = start
			} else {
				e.cursor = end
			}
			e.anchor = -1
			e.preferredCol = -1
			e.ensureCursorVisible()
			return
		}
	}
	e.moveTo(e.cursor+delta, selecting, false)
}

// MoveWord moves across a run of whitespace, word characters, or punctuation.
func (e *Editor) MoveWord(delta int, selecting bool) {
	if delta == 0 {
		return
	}
	target := e.cursor
	if delta < 0 {
		if target == 0 {
			e.moveTo(0, selecting, false)
			return
		}
		target--
		class := graphemeClass(e.graphemes[target])
		for target > 0 && graphemeClass(e.graphemes[target-1]) == class {
			target--
		}
	} else {
		if target == len(e.graphemes) {
			e.moveTo(target, selecting, false)
			return
		}
		class := graphemeClass(e.graphemes[target])
		for target < len(e.graphemes) && graphemeClass(e.graphemes[target]) == class {
			target++
		}
	}
	e.moveTo(target, selecting, false)
}

// MoveVertical moves by wrapped visual rows while preserving a preferred cell column.
func (e *Editor) MoveVertical(delta int, selecting bool) {
	doc := e.Document()
	row, col := doc.Position(e.cursor)
	if e.preferredCol < 0 {
		e.preferredCol = col
	}
	targetRow := clamp(row+delta, 0, len(doc.Rows)-1)
	e.moveTo(doc.Rows[targetRow].BoundaryAt(e.preferredCol), selecting, true)
}

// MoveHome moves to the beginning of the current visual row.
func (e *Editor) MoveHome(selecting bool) {
	doc := e.Document()
	row, _ := doc.Position(e.cursor)
	e.moveTo(doc.Rows[row].Start, selecting, false)
}

// MoveEnd moves to the end of the current visual row.
func (e *Editor) MoveEnd(selecting bool) {
	doc := e.Document()
	row, _ := doc.Position(e.cursor)
	e.moveTo(doc.Rows[row].End, selecting, false)
}

// MovePage moves by one viewport, retaining the preferred visual column.
func (e *Editor) MovePage(delta int, selecting bool) {
	rows := max(1, e.height)
	e.MoveVertical(delta*rows, selecting)
}

func (e *Editor) moveTo(target int, selecting, keepPreferred bool) {
	target = clamp(target, 0, len(e.graphemes))
	if selecting {
		if e.anchor < 0 {
			e.anchor = e.cursor
		}
	} else {
		e.anchor = -1
	}
	e.cursor = target
	if e.anchor == e.cursor {
		e.anchor = -1
	}
	if !keepPreferred {
		e.preferredCol = -1
	}
	e.ensureCursorVisible()
}

// Resize changes the wrap viewport while preserving the old top grapheme as
// the scroll anchor. If the cursor was visible, it remains visible.
func (e *Editor) Resize(width, height int) {
	old := e.Document()
	topIndex := 0
	if len(old.Rows) > 0 {
		topIndex = old.Rows[clamp(e.scroll, 0, len(old.Rows)-1)].Start
	}
	cursorRow, _ := old.Position(e.cursor)
	cursorVisible := e.height > 0 && cursorRow >= e.scroll && cursorRow < e.scroll+e.height
	e.width = max(0, width)
	e.height = max(0, height)
	next := e.Document()
	e.scroll = next.RowForIndex(topIndex)
	e.clampScroll(next)
	if cursorVisible {
		e.ensureCursorVisibleWith(next)
	}
}

// Scroll moves only the viewport, never the cursor or selection.
func (e *Editor) Scroll(delta int) {
	e.scroll += delta
	e.clampScroll(e.Document())
}

// SetScroll sets a wrapped-row offset, clamped to the current document.
func (e *Editor) SetScroll(offset int) {
	e.scroll = offset
	e.clampScroll(e.Document())
}

func (e Editor) ScrollOffset() int { return e.scroll }

// BeginDrag places the cursor and begins a pointer selection.
func (e *Editor) BeginDrag(x, y int) {
	position := e.Document().Point(x, y, e.scroll, e.height)
	e.cursor = position
	e.anchor = position
	e.preferredCol = -1
	e.dragging = true
}

// DragTo extends the pointer selection. Pointer rows outside the viewport
// scroll one row per motion event without any polling loop.
func (e *Editor) DragTo(x, y int) {
	if !e.dragging {
		return
	}
	if y < 0 {
		e.Scroll(-1)
		y = 0
	} else if e.height > 0 && y >= e.height {
		e.Scroll(1)
		y = e.height - 1
	}
	e.cursor = e.Document().Point(x, y, e.scroll, e.height)
	e.preferredCol = -1
}

func (e *Editor) EndDrag() {
	e.dragging = false
	if e.anchor == e.cursor {
		e.anchor = -1
	}
}

func (e Editor) Dragging() bool { return e.dragging }

// CursorLineColumn returns one-based logical line and display-cell column.
func (e Editor) CursorLineColumn() (int, int) {
	line := 1
	column := 0
	for index := 0; index < e.cursor; index++ {
		value := e.graphemes[index]
		if value == "\n" {
			line++
			column = 0
			continue
		}
		if value == "\t" {
			column += TabWidth - column%TabWidth
			continue
		}
		column += max(1, uniseg.StringWidth(value))
	}
	return line, column + 1
}

func (e *Editor) remember() {
	e.undo = appendBounded(e.undo, e.snapshot())
	e.redo = nil
}

func (e Editor) snapshot() snapshot {
	return snapshot{graphemes: clone(e.graphemes), cursor: e.cursor, anchor: e.anchor, scroll: e.scroll}
}

func (e *Editor) restore(state snapshot) {
	e.graphemes = clone(state.graphemes)
	e.cursor = state.cursor
	e.anchor = state.anchor
	e.scroll = state.scroll
	e.preferredCol = -1
	e.dragging = false
	e.clamp()
	e.ensureCursorVisible()
}

func (e Editor) replacementRange() (int, int) {
	if start, end, ok := e.Selection(); ok {
		return start, end
	}
	return e.cursor, e.cursor
}

func (e *Editor) clamp() {
	e.cursor = clamp(e.cursor, 0, len(e.graphemes))
	if e.anchor < -1 || e.anchor > len(e.graphemes) {
		e.anchor = -1
	}
	e.clampScroll(e.Document())
}

func (e *Editor) ensureCursorVisible() {
	e.ensureCursorVisibleWith(e.Document())
}

func (e *Editor) ensureCursorVisibleWith(doc Document) {
	if e.height <= 0 {
		e.scroll = 0
		return
	}
	row, _ := doc.Position(e.cursor)
	if row < e.scroll {
		e.scroll = row
	} else if row >= e.scroll+e.height {
		e.scroll = row - e.height + 1
	}
	e.clampScroll(doc)
}

func (e *Editor) clampScroll(doc Document) {
	maximum := max(0, len(doc.Rows)-max(1, e.height))
	e.scroll = clamp(e.scroll, 0, maximum)
}

func splitGraphemes(text string) []string {
	if text == "" {
		return nil
	}
	segments := make([]string, 0, len(text))
	iterator := uniseg.NewGraphemes(text)
	for iterator.Next() {
		segments = append(segments, iterator.Str())
	}
	return segments
}

// sanitize preserves tabs and line feeds, normalizes CR line endings, and
// replaces every other terminal control with an inert visible character.
func sanitize(text string) string {
	text = strings.ToValidUTF8(text, "�")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var safe strings.Builder
	for _, value := range text {
		switch value {
		case '\n', '\t':
			safe.WriteRune(value)
		case '\r':
			safe.WriteRune('\n')
		case 0x7f:
			safe.WriteRune('␡')
		default:
			if value < 0x20 {
				safe.WriteRune(0x2400 + value)
			} else if unicode.IsControl(value) {
				safe.WriteRune('�')
			} else {
				safe.WriteRune(value)
			}
		}
	}
	return safe.String()
}

type characterClass uint8

const (
	classSpace characterClass = iota
	classWord
	classPunctuation
)

func graphemeClass(value string) characterClass {
	if value == "" {
		return classSpace
	}
	runes := []rune(value)
	if len(runes) == 0 || unicode.IsSpace(runes[0]) {
		return classSpace
	}
	if unicode.IsLetter(runes[0]) || unicode.IsDigit(runes[0]) || runes[0] == '_' {
		return classWord
	}
	return classPunctuation
}

func appendBounded(history []snapshot, state snapshot) []snapshot {
	if len(history) == UndoLimit {
		copy(history, history[1:])
		history[len(history)-1] = state
		return history
	}
	return append(history, state)
}

func clone(values []string) []string { return append([]string(nil), values...) }

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
