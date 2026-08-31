package review

import (
	"fmt"
	"strconv"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
)

// LineKind identifies one comparison-reader row.
type LineKind uint8

const (
	ContextLine LineKind = iota
	RemovedLine
	AddedLine
	NoticeLine
)

// Line is one stable logical reader row.
type Line struct {
	Identity string
	Text     string
	Kind     LineKind
	OldLine  int
	NewLine  int
}

// Document is one bounded rendering of explicit endpoint bounds.
type Document struct {
	Bounds  Bounds
	Lines   []Line
	Added   int
	Removed int
	Exact   bool
	Reason  string
}

// BuildDocument constructs one complete comparison view from exact endpoints.
// go-udiff's line differ bounds its Myers/LCS search depth, so sparse edits stay
// distinct without exposing large files to unbounded quadratic work.
func BuildDocument(bounds Bounds, oldContent, newContent Content) Document {
	document, complete := initialDocument(bounds, oldContent, newContent)
	if complete {
		return document
	}
	oldText := normalizeLineEndings(contentText(oldContent))
	newText := normalizeLineEndings(contentText(newContent))
	builder := documentBuilder{
		document: &document, oldLine: 1, newLine: 1,
		occurrences: make(map[string]int),
	}
	cursor := 0
	for _, edit := range udiff.Lines(oldText, newText) {
		builder.appendBlock(ContextLine, oldText[cursor:edit.Start])
		builder.appendBlock(RemovedLine, oldText[edit.Start:edit.End])
		builder.appendBlock(AddedLine, edit.New)
		cursor = edit.End
	}
	builder.appendBlock(ContextLine, oldText[cursor:])
	if notice, ok := comparisonNotice(bounds); ok {
		document.Lines = append([]Line{notice}, document.Lines...)
	}
	return document
}

func initialDocument(bounds Bounds, oldContent, newContent Content) (Document, bool) {
	document := Document{Bounds: bounds}
	if oldContent.Endpoint != bounds.Old || newContent.Endpoint != bounds.New || !bounds.Old.Exact() || !bounds.New.Exact() {
		document.Reason = "file changed; refresh before marking reviewed"
		document.Lines = []Line{{Identity: "notice:stale", Text: document.Reason, Kind: NoticeLine}}
		return document, true
	}
	document.Exact = true
	switch {
	case oldContent.State == ContentUnavailable || newContent.State == ContentUnavailable:
		document.Exact = false
		document.Reason = "exact endpoint content unavailable"
		document.Lines = []Line{{Identity: "notice:unavailable", Text: document.Reason, Kind: NoticeLine}}
		return document, true
	case oldContent.State == ContentBinary || newContent.State == ContentBinary:
		document.Lines = []Line{{Identity: "notice:binary", Text: "Binary comparison; exact identity is reviewable.", Kind: NoticeLine}}
		return document, true
	case oldContent.State == ContentTooLarge || newContent.State == ContentTooLarge:
		document.Lines = []Line{{Identity: "notice:oversized", Text: "File is too large for a text comparison; exact identity is reviewable.", Kind: NoticeLine}}
		return document, true
	default:
		return document, false
	}
}

type documentBuilder struct {
	document    *Document
	oldLine     int
	newLine     int
	occurrences map[string]int
}

func (builder *documentBuilder) appendBlock(kind LineKind, text string) {
	for _, line := range splitLines(text) {
		oldLine, newLine := builder.oldLine, builder.newLine
		switch kind {
		case ContextLine:
			builder.oldLine++
			builder.newLine++
		case RemovedLine:
			newLine = 0
			builder.oldLine++
			builder.document.Removed++
		case AddedLine:
			oldLine = 0
			builder.newLine++
			builder.document.Added++
		}
		builder.appendLine(kind, line, oldLine, newLine)
	}
}

func (builder *documentBuilder) appendLine(kind LineKind, text string, oldLine, newLine int) {
	key := strconv.Itoa(int(kind)) + ":" + ContentIdentity([]byte(text))
	builder.occurrences[key]++
	identity := key + ":" + strconv.Itoa(builder.occurrences[key])
	builder.document.Lines = append(builder.document.Lines, Line{
		Identity: identity,
		Text:     linePrefix(kind) + text,
		Kind:     kind,
		OldLine:  oldLine,
		NewLine:  newLine,
	})
}

func comparisonNotice(bounds Bounds) (Line, bool) {
	switch {
	case bounds.Old.Kind == Absent || bounds.New.Kind == Absent:
		return Line{}, false
	case bounds.Old.Kind != bounds.New.Kind:
		return Line{Identity: "notice:kind", Text: fmt.Sprintf("file type %s -> %s", bounds.Old.Kind, bounds.New.Kind), Kind: NoticeLine}, true
	case bounds.Old.Mode != bounds.New.Mode:
		return Line{Identity: "notice:mode", Text: fmt.Sprintf("mode %o -> %o", bounds.Old.Mode, bounds.New.Mode), Kind: NoticeLine}, true
	case bounds.Old.Kind == Submodule && bounds.Old.ContentID != bounds.New.ContentID:
		return Line{Identity: "notice:submodule", Text: "submodule target changed", Kind: NoticeLine}, true
	default:
		return Line{}, false
	}
}

// LineIdentities returns stable row identities for Continuity reconciliation.
func (document Document) LineIdentities() []string {
	identities := make([]string, len(document.Lines))
	for index, line := range document.Lines {
		identities[index] = line.Identity
	}
	return identities
}

func contentText(content Content) string {
	if content.State == ContentAbsent {
		return ""
	}
	return content.Text
}

func normalizeLineEndings(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func splitLines(content string) []string {
	content = normalizeLineEndings(content)
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func linePrefix(kind LineKind) string {
	switch kind {
	case RemovedLine:
		return "- "
	case AddedLine:
		return "+ "
	case ContextLine:
		return "  "
	default:
		return ""
	}
}
