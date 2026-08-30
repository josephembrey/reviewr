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
	payload string
	kind    diffCodeKind
}

func highlightedSourceRows(path, content string) []ui.ReaderRow {
	return highlightedSafeRows(path, ui.SafeContentLines(content))
}

func highlightedSafeRows(path string, safeLines []string) []ui.ReaderRow {
	highlighted := sourceHighlighter.Lines(path, safeLines)
	rows := make([]ui.ReaderRow, len(safeLines))
	for index, text := range safeLines {
		rows[index] = ui.ReaderRow{
			Identity: "file:" + text,
			Kind:     ui.ReaderFile, Text: text, NewLine: uint64(index + 1),
		}
		if highlighted != nil {
			rows[index].Spans = presentationSpans(highlighted[index])
		}
	}
	return rows
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

func decorateDiffGroup(path string, rows []ui.ReaderRow, group []diffCodeRow) {
	if len(group) == 0 {
		return
	}
	oldText, newText := make([]string, 0, len(group)), make([]string, 0, len(group))
	oldRows, newRows := make([]diffCodeRow, 0, len(group)), make([]diffCodeRow, 0, len(group))
	for _, row := range group {
		if row.kind != diffAdded {
			oldText = append(oldText, row.payload)
			oldRows = append(oldRows, row)
		}
		if row.kind != diffRemoved {
			newText = append(newText, row.payload)
			newRows = append(newRows, row)
		}
	}
	oldHighlighted := highlightedSafeRows(path, oldText)
	newHighlighted := highlightedSafeRows(path, newText)
	for index, row := range oldRows {
		if row.kind == diffRemoved {
			decorateDiffRow(&rows[row.index], oldHighlighted[index])
		}
	}
	for index, row := range newRows {
		decorateDiffRow(&rows[row.index], newHighlighted[index])
	}
}

func decorateDiffRow(row *ui.ReaderRow, payload ui.ReaderRow) {
	row.Spans = payload.Spans
}
