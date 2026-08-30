package notes

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

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
