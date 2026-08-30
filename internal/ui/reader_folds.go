package ui

import "fmt"

const (
	readerContextRadius = 3
	readerMinimumHidden = 3
)

// ContextFoldable reports whether compact diff presentation would hide at
// least one meaningful run of unchanged lines.
func (document ReaderDocument) ContextFoldable() bool {
	if document.Kind != ReaderDiffDocument {
		return false
	}
	for start := 0; start < len(document.Rows); {
		if document.Rows[start].Kind != ReaderContext {
			start++
			continue
		}
		end := contextRunEnd(document.Rows, start)
		keep := retainedContextLines(document.Rows, start, end)
		if keep >= 0 && end-start-keep >= readerMinimumHidden {
			return true
		}
		start = end
	}
	return false
}

// HasContextFold reports whether a presented document contains the inline
// context control produced by WithContextFolds. Both compact and expanded
// presentations retain that control, so callers need not rescan the raw diff.
func (document ReaderDocument) HasContextFold() bool {
	for _, row := range document.Rows {
		if row.Kind == ReaderFold {
			return true
		}
	}
	return false
}

// WithContextFolds derives compact diff presentation without changing the
// semantic source document. Expanded documents preserve their original rows.
func (document ReaderDocument) WithContextFolds(expanded bool) ReaderDocument {
	if expanded {
		return document.WithContextFoldProgress(1, 1)
	}
	return document.WithContextFoldProgress(0, 1)
}

// WithContextFoldProgress derives an intermediate context-fold presentation.
// Progress is shared by every fold in the document and expressed as a fraction
// so callers can bound transition duration independently of hidden-run size.
func (document ReaderDocument) WithContextFoldProgress(progress, steps int) ReaderDocument {
	if !document.ContextFoldable() {
		return document
	}
	steps = max(1, steps)
	progress = max(0, min(progress, steps))
	result := document
	result.Rows = document.contextFoldRows(progress, steps)
	return result
}

func (document ReaderDocument) contextFoldRows(progress, steps int) []ReaderRow {
	if document.Kind != ReaderDiffDocument || len(document.Rows) == 0 {
		return document.Rows
	}
	rows := make([]ReaderRow, 0, len(document.Rows))
	for start := 0; start < len(document.Rows); {
		if document.Rows[start].Kind != ReaderContext {
			rows = append(rows, document.Rows[start])
			start++
			continue
		}
		end := contextRunEnd(document.Rows, start)
		beforeChange := start > 0 && changedReaderRow(document.Rows[start-1])
		afterChange := end < len(document.Rows) && changedReaderRow(document.Rows[end])
		if !beforeChange && !afterChange {
			rows = append(rows, document.Rows[start:end]...)
			start = end
			continue
		}
		keepBefore, keepAfter := 0, 0
		if beforeChange {
			keepBefore = min(readerContextRadius, end-start)
		}
		if afterChange {
			keepAfter = min(readerContextRadius, end-start-keepBefore)
		}
		hiddenStart, hiddenEnd := start+keepBefore, end-keepAfter
		if hiddenEnd-hiddenStart < readerMinimumHidden {
			rows = append(rows, document.Rows[start:end]...)
			start = end
			continue
		}
		rows = append(rows, document.Rows[start:hiddenStart]...)
		hidden := document.Rows[hiddenStart:hiddenEnd]
		visible := contextFoldVisibleRows(len(hidden), progress, steps)
		rows = append(rows, contextFoldRow(hidden, visible > 0))
		rows = append(rows, hidden[:visible]...)
		rows = append(rows, document.Rows[hiddenEnd:end]...)
		start = end
	}
	return rows
}

func contextFoldVisibleRows(hidden, progress, steps int) int {
	if progress <= 0 {
		return 0
	}
	if progress >= steps {
		return hidden
	}
	// Round up so the first animation frame always changes the document.
	return min(hidden, (hidden*progress+steps-1)/steps)
}

func contextRunEnd(rows []ReaderRow, start int) int {
	end := start + 1
	for end < len(rows) && rows[end].Kind == ReaderContext {
		end++
	}
	return end
}

// retainedContextLines returns -1 when a context run is unrelated to a
// change, otherwise the number of boundary lines that compact mode retains.
func retainedContextLines(rows []ReaderRow, start, end int) int {
	retained := 0
	if start > 0 && changedReaderRow(rows[start-1]) {
		retained += min(readerContextRadius, end-start)
	}
	if end < len(rows) && changedReaderRow(rows[end]) {
		retained += min(readerContextRadius, end-start-retained)
	}
	if retained == 0 {
		return -1
	}
	return retained
}

func changedReaderRow(row ReaderRow) bool {
	return row.Kind == ReaderInsertion || row.Kind == ReaderDeletion
}

func contextFoldRow(hidden []ReaderRow, expanded bool) ReaderRow {
	first, last := hidden[0], hidden[len(hidden)-1]
	return ReaderRow{
		Identity:     fmt.Sprintf("fold:%d:%d:%d:%d", first.OldLine, first.NewLine, last.OldLine, last.NewLine),
		Kind:         ReaderFold,
		Text:         fmt.Sprintf("%d unchanged lines", len(hidden)),
		Tone:         ToneDefault,
		FoldExpanded: expanded,
	}
}
