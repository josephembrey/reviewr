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
		copy(h.order, h.order[1:])
		h.order[len(h.order)-1] = key
	} else {
		h.order = append(h.order, key)
	}
	// tokenize owns lines and callers only receive clones, so the cache can
	// retain the original document without cloning it a second time.
	h.entries[key] = lines
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
	// Chroma owns tokenization; presentation uses terminal ANSI roles so code
	// follows the active palette instead of imposing an unrelated RGB theme.
	style := Style{Foreground: tokenForeground(tokenType)}
	switch {
	case tokenType.InCategory(chroma.Comment):
		style.Italic = true
	case tokenType == chroma.GenericHeading, tokenType == chroma.GenericSubheading:
		style.Bold = true
	case tokenType == chroma.GenericStrong:
		style.Bold = true
	case tokenType == chroma.GenericEmph:
		style.Italic = true
	case tokenType == chroma.GenericUnderline:
		style.Underline = true
	}
	return style
}

func tokenForeground(tokenType chroma.TokenType) string {
	switch {
	case tokenType == chroma.Error, tokenType == chroma.NameException,
		tokenType == chroma.NameTag, tokenType == chroma.GenericError,
		tokenType == chroma.GenericTraceback, tokenType == chroma.GenericDeleted:
		return "1"
	case tokenType == chroma.GenericInserted:
		return "2"
	case tokenType == chroma.NameConstant, tokenType == chroma.LiteralDate,
		tokenType.InSubCategory(chroma.LiteralNumber), tokenType == chroma.LiteralStringInterpol:
		return "3"
	case tokenType == chroma.KeywordNamespace, tokenType == chroma.NameAttribute,
		tokenType.InSubCategory(chroma.NameBuiltin), tokenType.InSubCategory(chroma.NameFunction),
		tokenType == chroma.NameNamespace, tokenType == chroma.LiteralStringEscape:
		return "6"
	case tokenType.InSubCategory(chroma.LiteralString),
		tokenType == chroma.KeywordType, tokenType == chroma.NameClass:
		return "2"
	case tokenType.InCategory(chroma.Keyword), tokenType == chroma.NameDecorator,
		tokenType == chroma.GenericHeading, tokenType == chroma.GenericSubheading:
		return "4"
	case tokenType.InCategory(chroma.Comment):
		return "5"
	case tokenType.InCategory(chroma.Operator):
		return "8"
	default:
		return ""
	}
}

func sameText(lines [][]Span, source []string) bool {
	if len(lines) != len(source) {
		return false
	}
	for index, spans := range lines {
		offset := 0
		for _, span := range spans {
			end := offset + len(span.Text)
			if end > len(source[index]) || source[index][offset:end] != span.Text {
				return false
			}
			offset = end
		}
		if offset != len(source[index]) {
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
