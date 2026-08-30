package app

import (
	"github.com/josephembrey/reviewr/internal/highlight"
	"github.com/josephembrey/reviewr/internal/ui"
)

const syntaxCacheDocuments = 24

var sourceHighlighter = highlight.New(syntaxCacheDocuments)

type diffCodeKind uint8

const (
	diffContext diffCodeKind = iota
	diffRemoved
	diffAdded
)

type diffCodeRow struct {
	index   int
	marker  string
	payload string
	kind    diffCodeKind
}

func highlightedSourceLines(path, content string) []ui.Line {
	return highlightedSafeLines(path, ui.SafeContentLines(content))
}

func highlightedSafeLines(path string, safeLines []string) []ui.Line {
	highlighted := sourceHighlighter.Lines(path, safeLines)
	lines := make([]ui.Line, len(safeLines))
	for index, text := range safeLines {
		lines[index] = ui.Line{Text: text}
		if highlighted != nil {
			lines[index].Spans = presentationSpans(highlighted[index])
		}
	}
	return lines
}

func presentationSpans(spans []highlight.Span) []ui.TextSpan {
	result := make([]ui.TextSpan, len(spans))
	for index, span := range spans {
		result[index] = ui.TextSpan{
			Text: span.Text,
			Style: ui.TextStyle{
				Foreground: span.Style.Foreground,
				Bold:       span.Style.Bold,
				Italic:     span.Style.Italic,
				Underline:  span.Style.Underline,
			},
		}
	}
	return result
}

func decorateDiffGroup(path string, lines []ui.Line, rows []diffCodeRow) {
	if len(rows) == 0 {
		return
	}
	oldText, newText := make([]string, 0, len(rows)), make([]string, 0, len(rows))
	oldRows, newRows := make([]diffCodeRow, 0, len(rows)), make([]diffCodeRow, 0, len(rows))
	for _, row := range rows {
		if row.kind != diffAdded {
			oldText = append(oldText, row.payload)
			oldRows = append(oldRows, row)
		}
		if row.kind != diffRemoved {
			newText = append(newText, row.payload)
			newRows = append(newRows, row)
		}
	}
	oldLines := highlightedSafeLines(path, oldText)
	newLines := highlightedSafeLines(path, newText)
	for index, row := range oldRows {
		if row.kind == diffRemoved {
			decorateDiffLine(&lines[row.index], row.marker, oldLines[index])
		}
	}
	for index, row := range newRows {
		decorateDiffLine(&lines[row.index], row.marker, newLines[index])
	}
}

func decorateDiffLine(line *ui.Line, marker string, payload ui.Line) {
	spans := []ui.TextSpan{{Text: marker, Tone: line.Tone}}
	if len(payload.Spans) != 0 {
		spans = append(spans, payload.Spans...)
	} else if payload.Text != "" {
		spans = append(spans, ui.TextSpan{Text: payload.Text})
	}
	line.Spans = spans
	line.Tone = ui.ToneDefault
}
