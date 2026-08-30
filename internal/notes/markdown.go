package notes

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/highlight"
	"github.com/rivo/uniseg"
)

const markdownCacheDocuments = 8

var markdownHighlighter = highlight.New(markdownCacheDocuments)

// MarkdownStyles returns one terminal-theme-aware Chroma style per editor
// grapheme. Markdown ink is deliberately separate from the editor document so
// syntax never changes wrapping, cursor, selection, or pointer geometry.
func MarkdownStyles(text string) []highlight.Style {
	lines := strings.Split(text, "\n")
	highlighted := markdownHighlighter.Lines("note.md", lines)
	if highlighted == nil {
		return nil
	}

	styles := make([]highlight.Style, 0, len(splitGraphemes(text)))
	for lineIndex, line := range highlighted {
		for _, span := range line {
			graphemes := uniseg.NewGraphemes(span.Text)
			for graphemes.Next() {
				styles = append(styles, span.Style)
			}
		}
		if lineIndex+1 < len(highlighted) {
			styles = append(styles, highlight.Style{})
		}
	}
	if len(styles) != len(splitGraphemes(text)) {
		return nil
	}
	return styles
}
