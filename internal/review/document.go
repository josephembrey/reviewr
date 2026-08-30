package review

import (
	"fmt"
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
	document := Document{Bounds: bounds}
	if oldContent.Endpoint != bounds.Old || newContent.Endpoint != bounds.New || !bounds.Old.Exact() || !bounds.New.Exact() {
		document.Reason = "file changed; refresh before marking reviewed"
		document.Lines = []Line{{Identity: "notice:stale", Text: document.Reason, Kind: NoticeLine}}
		return document
	}
	document.Exact = true
	if oldContent.State == ContentUnavailable || newContent.State == ContentUnavailable {
		document.Exact = false
		document.Reason = "exact endpoint content unavailable"
		document.Lines = []Line{{Identity: "notice:unavailable", Text: document.Reason, Kind: NoticeLine}}
		return document
	}
	if oldContent.State == ContentBinary || newContent.State == ContentBinary {
		document.Lines = []Line{{Identity: "notice:binary", Text: "Binary comparison; exact identity is reviewable.", Kind: NoticeLine}}
		return document
	}
	if oldContent.State == ContentTooLarge || newContent.State == ContentTooLarge {
		document.Lines = []Line{{Identity: "notice:oversized", Text: "File is too large for a text comparison; exact identity is reviewable.", Kind: NoticeLine}}
		return document
	}
	oldText := normalizeLineEndings(contentText(oldContent))
	newText := normalizeLineEndings(contentText(newContent))
	oldNo, newNo := 1, 1
	occurrences := make(map[string]int)
	appendLine := func(kind LineKind, text string, oldLine, newLine int) {
		key := fmt.Sprintf("%d:%s", kind, ContentIdentity([]byte(text)))
		occurrences[key]++
		identity := fmt.Sprintf("%s:%d", key, occurrences[key])
		document.Lines = append(document.Lines, Line{Identity: identity, Text: linePrefix(kind) + text, Kind: kind, OldLine: oldLine, NewLine: newLine})
	}
	appendBlock := func(kind LineKind, text string) {
		for _, line := range splitLines(text) {
			switch kind {
			case ContextLine:
				appendLine(kind, line, oldNo, newNo)
				oldNo++
				newNo++
			case RemovedLine:
				appendLine(kind, line, oldNo, 0)
				document.Removed++
				oldNo++
			case AddedLine:
				appendLine(kind, line, 0, newNo)
				document.Added++
				newNo++
			}
		}
	}
	cursor := 0
	for _, edit := range udiff.Lines(oldText, newText) {
		appendBlock(ContextLine, oldText[cursor:edit.Start])
		appendBlock(RemovedLine, oldText[edit.Start:edit.End])
		appendBlock(AddedLine, edit.New)
		cursor = edit.End
	}
	appendBlock(ContextLine, oldText[cursor:])
	if bounds.Old.Kind != bounds.New.Kind {
		document.Lines = append([]Line{{Identity: "notice:kind", Text: fmt.Sprintf("file type %s -> %s", bounds.Old.Kind, bounds.New.Kind), Kind: NoticeLine}}, document.Lines...)
	} else if bounds.Old.Mode != bounds.New.Mode {
		document.Lines = append([]Line{{Identity: "notice:mode", Text: fmt.Sprintf("mode %o -> %o", bounds.Old.Mode, bounds.New.Mode), Kind: NoticeLine}}, document.Lines...)
	} else if bounds.Old.Kind == Submodule && bounds.Old.ContentID != bounds.New.ContentID {
		document.Lines = append([]Line{{Identity: "notice:submodule", Text: "submodule target changed", Kind: NoticeLine}}, document.Lines...)
	}
	return document
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
