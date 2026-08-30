package notes

import "github.com/rivo/uniseg"

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
