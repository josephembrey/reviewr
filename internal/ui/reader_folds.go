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
		if !foldableContextRow(document.Rows[start]) {
			start++
			continue
		}
		end := contextRunEnd(document.Rows, start)
		if _, _, ok := contextFoldBounds(document.Rows, start, end); ok {
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
	return document.WithContextFoldProgresses(nil, progress, steps)
}

// WithContextFoldProgresses derives a presentation in which each omitted
// context gap owns its own animation progress. A missing identity uses
// defaultProgress, which keeps bulk folding cheap and makes new gaps inherit
// the last bulk choice without coupling existing gaps to one another.
func (document ReaderDocument) WithContextFoldProgresses(progresses map[string]int, defaultProgress, steps int) ReaderDocument {
	if !document.ContextFoldable() {
		return document
	}
	steps = max(1, steps)
	defaultProgress = max(0, min(defaultProgress, steps))
	result := document
	result.Rows = document.contextFoldRows(progresses, defaultProgress, steps)
	return result
}

// ContextFoldIdentities returns stable identities for every independently
// controllable unchanged-context gap in document order.
func (document ReaderDocument) ContextFoldIdentities() []string {
	if document.Kind != ReaderDiffDocument {
		return nil
	}
	identities := make([]string, 0)
	for start := 0; start < len(document.Rows); {
		if !foldableContextRow(document.Rows[start]) {
			start++
			continue
		}
		end := contextRunEnd(document.Rows, start)
		if hiddenStart, hiddenEnd, ok := contextFoldBounds(document.Rows, start, end); ok {
			identities = append(identities, contextFoldRow(document.Rows[hiddenStart:hiddenEnd], false).Identity)
		}
		start = end
	}
	return identities
}

// HunkStarts returns the first semantic row of each change group. Explicit
// unified-diff headers are authoritative; complete endpoint comparisons use
// the independent context folds as their hunk boundaries.
func (document ReaderDocument) HunkStarts() []int {
	if document.Kind != ReaderDiffDocument {
		return nil
	}
	explicit := make([]int, 0)
	for index, row := range document.Rows {
		if row.Kind == ReaderMetadata && len(row.Text) >= 2 && row.Text[:2] == "@@" {
			explicit = append(explicit, index)
		}
	}
	if len(explicit) != 0 {
		return explicit
	}

	starts := make([]int, 0)
	segmentStart := 0
	for index := 0; index <= len(document.Rows); index++ {
		if index < len(document.Rows) && document.Rows[index].Kind != ReaderFold {
			continue
		}
		if start, ok := hunkStartInSegment(document.Rows, segmentStart, index); ok {
			starts = append(starts, start)
		}
		segmentStart = index + 1
	}
	return starts
}

// HunkNavigationTargets returns the row where previous/next hunk navigation
// should place the reader cursor. In complete endpoint comparisons, the fold
// leading into a change group is the most useful landing point because the
// next left/right action can reveal or hide its context. Explicit unified-diff
// headers remain their own navigation targets.
func (document ReaderDocument) HunkNavigationTargets() []int {
	starts := document.HunkStarts()
	for index, start := range starts {
		if start > 0 && document.Rows[start-1].Kind == ReaderFold {
			starts[index] = start - 1
		}
	}
	return starts
}

// HunkOwnership is disposable presentation metadata for one comment header.
// Intersections indexes hunks whose changed source lines overlap the comment;
// Owner is the nearest hunk landmark even when Intersections is empty.
type HunkOwnership struct {
	Intersections []int
	Owner         int
}

// CommentHunkOwnership derives hunk relationships without putting a hunk id
// into canonical comment state. Unchanged-context and File-mode comments are
// therefore valid first-class anchors with zero intersections.
func (document ReaderDocument) CommentHunkOwnership(header int) HunkOwnership {
	relation := HunkOwnership{Owner: -1}
	if header < 0 || header >= len(document.Rows) || document.Rows[header].Kind != ReaderCommentHeader {
		return relation
	}
	targets := document.HunkNavigationTargets()
	if len(targets) == 0 {
		return relation
	}
	bestDistance := len(document.Rows) + 1
	for index, target := range targets {
		distance := target - header
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance = distance
			relation.Owner = index
		}
		end := len(document.Rows)
		if index+1 < len(targets) {
			end = targets[index+1]
		}
		if commentIntersectsChangedRows(document.Rows[header], document.Rows[target:end]) {
			relation.Intersections = append(relation.Intersections, index)
		}
	}
	return relation
}

func commentIntersectsChangedRows(comment ReaderRow, rows []ReaderRow) bool {
	low, high := comment.CommentStart, comment.CommentEnd
	if low > high {
		low, high = high, low
	}
	for _, row := range rows {
		var line uint64
		if comment.CommentOldSide {
			if row.Kind != ReaderDeletion {
				continue
			}
			line = row.OldLine
		} else {
			if row.Kind != ReaderInsertion {
				continue
			}
			line = row.NewLine
		}
		if line >= low && line <= high {
			return true
		}
	}
	return false
}

func (document ReaderDocument) contextFoldRows(progresses map[string]int, defaultProgress, steps int) []ReaderRow {
	if document.Kind != ReaderDiffDocument || len(document.Rows) == 0 {
		return document.Rows
	}
	rows := make([]ReaderRow, 0, len(document.Rows))
	for start := 0; start < len(document.Rows); {
		if !foldableContextRow(document.Rows[start]) {
			rows = append(rows, document.Rows[start])
			start++
			continue
		}
		end := contextRunEnd(document.Rows, start)
		hiddenStart, hiddenEnd, foldable := contextFoldBounds(document.Rows, start, end)
		if !foldable {
			rows = append(rows, document.Rows[start:end]...)
			start = end
			continue
		}
		rows = append(rows, document.Rows[start:hiddenStart]...)
		hidden := document.Rows[hiddenStart:hiddenEnd]
		fold := contextFoldRow(hidden, false)
		progress := defaultProgress
		if value, ok := progresses[fold.Identity]; ok {
			progress = max(0, min(value, steps))
		}
		visible := contextFoldVisibleRows(len(hidden), progress, steps)
		fold.FoldExpanded = visible > 0
		rows = append(rows, fold)
		rows = append(rows, hidden[:visible]...)
		rows = append(rows, document.Rows[hiddenEnd:end]...)
		if visible > 0 && contextFoldSeparatesChanges(document.Rows, start, end) {
			rows = append(rows, contextFoldEndRow(fold))
		}
		start = end
	}
	return rows
}

func contextFoldSeparatesChanges(rows []ReaderRow, start, end int) bool {
	return start > 0 && end < len(rows) && changedReaderRow(rows[start-1]) && changedReaderRow(rows[end])
}

func contextFoldBounds(rows []ReaderRow, start, end int) (int, int, bool) {
	beforeChange := start > 0 && changedReaderRow(rows[start-1])
	afterChange := end < len(rows) && changedReaderRow(rows[end])
	if !beforeChange && !afterChange {
		return 0, 0, false
	}
	keepBefore, keepAfter := 0, 0
	if beforeChange {
		keepBefore = min(readerContextRadius, end-start)
	}
	if afterChange {
		keepAfter = min(readerContextRadius, end-start-keepBefore)
	}
	hiddenStart, hiddenEnd := start+keepBefore, end-keepAfter
	return hiddenStart, hiddenEnd, hiddenEnd-hiddenStart >= readerMinimumHidden
}

func hunkStartInSegment(rows []ReaderRow, start, end int) (int, bool) {
	changed := false
	firstCode := -1
	for index := start; index < end; index++ {
		switch rows[index].Kind {
		case ReaderContext, ReaderInsertion, ReaderDeletion:
			if firstCode < 0 {
				firstCode = index
			}
			if changedReaderRow(rows[index]) {
				changed = true
			}
		}
	}
	return firstCode, changed && firstCode >= 0
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
	for end < len(rows) && foldableContextRow(rows[end]) {
		end++
	}
	return end
}

func changedReaderRow(row ReaderRow) bool {
	return row.Kind == ReaderInsertion || row.Kind == ReaderDeletion ||
		row.ReviewFresh || row.ReviewRemovedBefore > 0 || row.ReviewRemovedAfter > 0
}

func foldableContextRow(row ReaderRow) bool {
	return row.Kind == ReaderContext && !changedReaderRow(row)
}

func contextFoldRow(hidden []ReaderRow, expanded bool) ReaderRow {
	first, last := hidden[0], hidden[len(hidden)-1]
	identity := fmt.Sprintf("fold:%d:%d:%d:%d", first.OldLine, first.NewLine, last.OldLine, last.NewLine)
	if first.Identity != "" && last.Identity != "" {
		identity = fmt.Sprintf("fold:%d:%s%s", len(first.Identity), first.Identity, last.Identity)
	}
	return ReaderRow{
		Identity:     identity,
		Kind:         ReaderFold,
		Text:         fmt.Sprintf("%d unchanged lines", len(hidden)),
		Tone:         ToneDefault,
		FoldExpanded: expanded,
	}
}

func contextFoldEndRow(fold ReaderRow) ReaderRow {
	return ReaderRow{
		Identity:   fold.Identity + ":end",
		Kind:       ReaderFoldEnd,
		Text:       "change resumes",
		Tone:       ToneDefault,
		FoldTarget: fold.Identity,
	}
}
