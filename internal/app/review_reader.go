package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
)

func reviewReaderDocument(path string, document review.Document) ui.ReaderDocument {
	result := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: make([]ui.ReaderRow, len(document.Lines))}
	group := make([]diffCodeRow, 0, len(document.Lines))
	for index, line := range document.Lines {
		tone := ui.ToneDefault
		kind := diffContext
		rowKind := ui.ReaderContext
		prefix := "  "
		switch line.Kind {
		case review.AddedLine:
			kind = diffAdded
			rowKind = ui.ReaderInsertion
			prefix = "+ "
		case review.RemovedLine:
			kind = diffRemoved
			rowKind = ui.ReaderDeletion
			prefix = "- "
		case review.NoticeLine:
			rowKind = ui.ReaderNotice
			prefix = ""
			if !document.Exact {
				tone = ui.ToneError
			} else {
				tone = ui.ToneQuiet
			}
		}
		payload := line.Text
		if line.Kind != review.NoticeLine && strings.HasPrefix(payload, prefix) {
			payload = payload[len(prefix):]
		}
		payload = ui.SafeSingleLine(payload)
		result.Rows[index] = ui.ReaderRow{
			Identity: line.Identity, Kind: rowKind, Text: payload, Tone: tone,
			OldLine: uint64(max(0, line.OldLine)), NewLine: uint64(max(0, line.NewLine)),
		}
		if line.Kind != review.NoticeLine {
			group = append(group, diffCodeRow{index: index, payload: payload, kind: kind})
		}
	}
	decorateDiffGroup(path, result.Rows, group)
	return result
}

func reviewFileReaderDocument(content review.Content, entry repository.Entry) ui.ReaderDocument {
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	if content.Endpoint.Path != entry.Path {
		document.Rows = noticeRows("File changed; refresh before marking reviewed.", ui.ToneError)
		return document
	}
	switch content.State {
	case review.ContentText:
		if content.Endpoint.Kind == review.Symlink {
			document.Rows = noticeRows("symlink → "+content.Text, ui.ToneDefault)
			return document
		}
		if content.Endpoint.Kind == review.Submodule {
			document.Rows = noticeRows("submodule → "+content.Text, ui.ToneDefault)
			return document
		}
		document.Rows = highlightedSourceRows(entry.Path, content.Text)
	case review.ContentAbsent:
		document.Rows = noticeRows("File was deleted from the worktree.", ui.ToneError)
	case review.ContentBinary:
		document.Rows = noticeRows(fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", content.Size), ui.ToneError)
	case review.ContentTooLarge:
		document.Rows = noticeRows(fmt.Sprintf("File is too large (%d bytes; bounded review reader).", content.Size), ui.ToneError)
	default:
		detail := content.Err
		if detail == "" {
			detail = "exact content unavailable"
		}
		document.Rows = noticeRows("File is unavailable: "+detail, ui.ToneError)
	}
	return document
}

// annotatedReviewFileReaderDocument keeps File mode a complete rendering of the
// current endpoint while projecting exact comparison metadata into its gutter.
// It never inserts a synthetic source row: additions decorate their current
// line and removed runs attach to the next surviving line, or the final line at
// EOF.
func annotatedReviewFileReaderDocument(content review.Content, entry repository.Entry, comparison review.FileComparison, diff review.Document) ui.ReaderDocument {
	if content.Endpoint != comparison.New {
		return ui.ReaderDocument{
			Kind: ui.ReaderFileDocument,
			Rows: noticeRows("File changed; refresh before marking reviewed.", ui.ToneError),
		}
	}
	document := reviewFileReaderDocument(content, entry)
	bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
	if !diff.Exact || diff.Bounds != bounds {
		return document
	}
	annotateReviewFileChanges(document.Rows, diff)
	return document
}

func annotateReviewFileChanges(rows []ui.ReaderRow, diff review.Document) {
	removed := uint64(0)
	lastCurrent := -1
	for _, line := range diff.Lines {
		switch line.Kind {
		case review.RemovedLine:
			removed++
		case review.AddedLine, review.ContextLine:
			if line.NewLine <= 0 || line.NewLine > len(rows) {
				continue
			}
			lastCurrent = line.NewLine - 1
			row := &rows[lastCurrent]
			if removed > 0 {
				row.RemovedBefore += removed
				removed = 0
			}
			if line.Kind == review.AddedLine {
				row.Kind = ui.ReaderInsertion
			}
		}
	}
	if removed == 0 || len(rows) == 0 {
		return
	}
	if lastCurrent < 0 {
		rows[0].RemovedBefore += removed
		return
	}
	rows[lastCurrent].RemovedAfter += removed
}

// annotateReviewFreshness projects the exact reviewed-frontier delta onto a
// full comparison or current-file presentation. Current line identities are
// authoritative; removed frontier lines attach to the next surviving line (or
// the final row) because they have no current line number to paint.
func annotateReviewFreshness(document ui.ReaderDocument, freshness review.Document) ui.ReaderDocument {
	if !freshness.Exact || len(document.Rows) == 0 {
		return document
	}
	result := document
	result.Rows = append([]ui.ReaderRow(nil), document.Rows...)
	byCurrentLine := make(map[uint64]int, len(result.Rows))
	lastCurrent := -1
	for index, row := range result.Rows {
		if row.NewLine == 0 || row.Kind == ui.ReaderDeletion {
			continue
		}
		byCurrentLine[row.NewLine] = index
		lastCurrent = index
	}

	removed := uint64(0)
	marked := false
	for _, line := range freshness.Lines {
		switch line.Kind {
		case review.RemovedLine:
			removed++
		case review.AddedLine, review.ContextLine:
			if line.NewLine <= 0 {
				continue
			}
			index, ok := byCurrentLine[uint64(line.NewLine)]
			if !ok {
				continue
			}
			lastCurrent = index
			if removed > 0 {
				result.Rows[index].ReviewRemovedBefore += removed
				removed = 0
				marked = true
			}
			if line.Kind == review.AddedLine {
				result.Rows[index].ReviewFresh = true
				marked = true
			}
		}
	}
	if removed > 0 {
		if lastCurrent < 0 {
			lastCurrent = len(result.Rows) - 1
		}
		result.Rows[lastCurrent].ReviewRemovedAfter += removed
		marked = true
	}
	if marked {
		return result
	}
	for _, line := range freshness.Lines {
		if line.Kind != review.NoticeLine {
			continue
		}
		index := 0
		for candidate, row := range result.Rows {
			if row.Kind == ui.ReaderNotice || row.Kind == ui.ReaderMetadata {
				index = candidate
				break
			}
		}
		result.Rows[index].ReviewFresh = true
		return result
	}
	return document
}
