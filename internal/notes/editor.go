// Package notes implements the Notes editor and its private persistence.
package notes

import (
	"strings"
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

// Place is the user-controlled position within a note. Text and undo history
// are intentionally excluded because they have separate ownership.
type Place struct {
	Cursor       int
	Anchor       int
	PreferredCol int
	Scroll       int
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

// Place returns the editor position for worktree-session persistence.
func (e Editor) Place() Place {
	return Place{
		Cursor: e.cursor, Anchor: e.anchor,
		PreferredCol: e.preferredCol, Scroll: e.scroll,
	}
}

// RestorePlace applies a position to freshly loaded text and clamps stale
// grapheme and viewport offsets without forcing the cursor into view.
func (e *Editor) RestorePlace(place Place) {
	e.cursor = clamp(place.Cursor, 0, len(e.graphemes))
	e.anchor = place.Anchor
	if e.anchor < 0 || e.anchor > len(e.graphemes) || e.anchor == e.cursor {
		e.anchor = -1
	}
	e.preferredCol = place.PreferredCol
	e.scroll = place.Scroll
	e.dragging = false
	e.clampScroll(e.Document())
}

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
