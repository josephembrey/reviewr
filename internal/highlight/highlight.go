// Package highlight turns source text into presentation-neutral syntax spans.
package highlight

import (
	"crypto/sha256"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Style is the subset of a Chroma token style that composes with terminal
// selection and diff backgrounds.
type Style struct {
	Foreground string
	Bold       bool
	Italic     bool
	Underline  bool
}

// Span is one contiguous source fragment with a shared token style.
type Span struct {
	Text  string
	Style Style
}

type cacheKey struct {
	path string
	sum  [sha256.Size]byte
}

// Highlighter detects lexers by path, falling back to content analysis. Its
// bounded cache keeps tokenization out of the frame rendering hot path.
type Highlighter struct {
	mu       sync.Mutex
	capacity int
	entries  map[cacheKey][][]Span
	order    []cacheKey
}

// New constructs a highlighter retaining at most capacity source documents.
func New(capacity int) *Highlighter {
	return &Highlighter{capacity: max(0, capacity), entries: make(map[cacheKey][][]Span)}
}

// Lines highlights safe, newline-free source lines. A nil result means no
// suitable lexer was found or tokenization failed, so callers should use plain
// text. The returned document always has the same lines and text as the input.
func (h *Highlighter) Lines(path string, lines []string) [][]Span {
	content := strings.Join(lines, "\n")
	key := cacheKey{path: path, sum: sha256.Sum256([]byte(content))}
	if cached, ok := h.cached(key); ok {
		return cloneLines(cached)
	}

	highlighted := tokenize(path, content, lines)
	h.store(key, highlighted)
	return cloneLines(highlighted)
}

func (h *Highlighter) cached(key cacheKey) ([][]Span, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	lines, ok := h.entries[key]
	return lines, ok
}

func (h *Highlighter) store(key cacheKey, lines [][]Span) {
	if h.capacity == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.entries[key]; exists {
		return
	}
	if len(h.order) == h.capacity {
		delete(h.entries, h.order[0])
		h.order = h.order[1:]
	}
	h.entries[key] = cloneLines(lines)
	h.order = append(h.order, key)
}

func tokenize(path, content string, sourceLines []string) (result [][]Span) {
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		return nil
	}

	defer func() {
		if recover() != nil {
			result = nil
		}
	}()
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, content)
	if err != nil {
		return nil
	}
	result = make([][]Span, len(sourceLines))
	line := 0
	for token := iterator(); token != chroma.EOF; token = iterator() {
		value := token.Value
		for {
			newline := strings.IndexByte(value, '\n')
			fragment := value
			if newline >= 0 {
				fragment = value[:newline]
			}
			if fragment != "" && line < len(result) {
				appendSpan(&result[line], Span{Text: fragment, Style: tokenStyle(token.Type)})
			}
			if newline < 0 {
				break
			}
			line++
			value = value[newline+1:]
		}
	}
	if !sameText(result, sourceLines) {
		return nil
	}
	return result
}

func appendSpan(line *[]Span, span Span) {
	if len(*line) > 0 && (*line)[len(*line)-1].Style == span.Style {
		(*line)[len(*line)-1].Text += span.Text
		return
	}
	*line = append(*line, span)
}

func tokenStyle(tokenType chroma.TokenType) Style {
	entry := reviewrStyle.Get(tokenType)
	style := Style{
		Bold:      entry.Bold == chroma.Yes,
		Italic:    entry.Italic == chroma.Yes,
		Underline: entry.Underline == chroma.Yes,
	}
	if entry.Colour.IsSet() {
		style.Foreground = entry.Colour.String()
	}
	return style
}

func sameText(lines [][]Span, source []string) bool {
	if len(lines) != len(source) {
		return false
	}
	for index, spans := range lines {
		var text strings.Builder
		for _, span := range spans {
			text.WriteString(span.Text)
		}
		if text.String() != source[index] {
			return false
		}
	}
	return true
}

func cloneLines(lines [][]Span) [][]Span {
	if lines == nil {
		return nil
	}
	cloned := make([][]Span, len(lines))
	for index := range lines {
		cloned[index] = append([]Span(nil), lines[index]...)
	}
	return cloned
}

var reviewrStyle = chroma.MustNewStyle("reviewr", chroma.StyleEntries{
	chroma.Error:                 "#F7768E",
	chroma.Keyword:               "#BB9AF7",
	chroma.KeywordNamespace:      "#7DCFFF",
	chroma.KeywordType:           "#2AC3DE",
	chroma.NameAttribute:         "#7DCFFF",
	chroma.NameBuiltin:           "#7DCFFF",
	chroma.NameClass:             "#E0AF68",
	chroma.NameConstant:          "#FF9E64",
	chroma.NameDecorator:         "#7AA2F7",
	chroma.NameException:         "#FF9E64",
	chroma.NameFunction:          "#7AA2F7",
	chroma.NameTag:               "#F7768E",
	chroma.LiteralString:         "#9ECE6A",
	chroma.LiteralStringEscape:   "#7DCFFF",
	chroma.LiteralStringInterpol: "#E0AF68",
	chroma.LiteralNumber:         "#FF9E64",
	chroma.Operator:              "#89DDFF",
	chroma.Comment:               "italic #737AA2",
	chroma.GenericDeleted:        "#F7768E",
	chroma.GenericInserted:       "#9ECE6A",
})
