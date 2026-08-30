package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type reviewRollup struct {
	reviewed int
	changed  int
}

func reviewCandidates(entries []repository.Entry) []review.Candidate {
	result := make([]review.Candidate, 0, len(entries))
	for _, entry := range entries {
		if !entry.Changed() {
			continue
		}
		action := review.Modified
		switch entry.State {
		case repository.FileAdded, repository.FileUntracked:
			action = review.Added
		case repository.FileDeleted:
			action = review.Deleted
		case repository.FileRenamed:
			action = review.Renamed
		}
		result = append(result, review.Candidate{Path: entry.Path, PreviousPath: entry.PreviousPath, Action: action})
	}
	return result
}

func (state *filesState) requestComparison(scope string) effect {
	state.reviewGeneration++
	state.reviewScope = scope
	state.reviewSnapshot = review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	state.rederiveReviews()
	state.comparisonWarning = ""
	state.readerContextExpanded = false
	if state.readerEntry.Path != "" {
		state.readerLoading = true
	}
	return effect{
		kind:             effectLoadReviewSnapshot,
		generation:       state.listGeneration,
		reviewGeneration: state.reviewGeneration,
		scope:            scope,
		candidates:       reviewCandidates(state.snapshot.Changed()),
	}
}

func (state filesState) landReviewSnapshot(msg reviewSnapshotLoadedMsg, mode workspace.ReaderMode, _ int) (filesState, effect) {
	if msg.listGeneration != state.listGeneration || msg.reviewGeneration != state.reviewGeneration || msg.scope != state.reviewScope {
		return state, effect{}
	}
	state.comparisonWarning = reviewLoadWarning(msg.err)
	if msg.err != nil {
		state.reviewSnapshot = review.Snapshot{Scope: msg.scope, Comparisons: make(map[string]review.FileComparison)}
	} else {
		state.reviewSnapshot = msg.snapshot
	}
	state.rederiveReviews()
	if state.readerEntry.Path == "" {
		state.readerLoading = false
		return state, effect{}
	}
	pending := state.requestReader(state.readerEntry, mode)
	state.place.ClampReaderSource(len(state.readerRows()))
	return state, pending
}

func (state filesState) landReviewState(msg reviewStateLoadedMsg, mode workspace.ReaderMode) (filesState, effect) {
	state.reviewLoaded = true
	state.store = msg.store
	state.reviewWarning = msg.warning
	state.ledger = msg.ledger.Clone()
	for _, delta := range state.sessionDeltas {
		_ = delta.Apply(&state.ledger)
	}
	state.rederiveReviews()
	state.sessionDeltas = nil
	if msg.err != nil && state.reviewWarning == "" {
		state.reviewWarning = "review state unavailable; marks will not survive restart"
	}
	pending := state.nextReviewPersistence()
	if pending.kind == effectNone && state.readerEntry.Path != "" && mode == workspace.DiffReader {
		pending = state.requestReader(state.readerEntry, mode)
	}
	return state, pending
}

func (state filesState) landReviewDocument(msg reviewDocumentLoadedMsg, _ int) filesState {
	if msg.generation != state.contentGeneration || state.readerMode != workspace.DiffReader ||
		msg.entry.Path != state.readerEntry.Path || state.requestedComparison == nil || state.requestedBounds == nil ||
		*state.requestedComparison != msg.comparison || *state.requestedBounds != msg.bounds {
		return state
	}
	current, ok := state.reviewSnapshot.Comparisons[msg.entry.Path]
	if !ok || current != msg.comparison || msg.document.Bounds != msg.bounds {
		return state
	}
	oldIdentities := state.reviewDocument.LineIdentities()
	if len(oldIdentities) == 0 && len(state.restoredReaderRows) != 0 {
		oldIdentities = append([]string(nil), state.restoredReaderRows...)
	}
	oldOffset := state.place.ReaderOffset
	oldCursor := state.reviewCursor
	oldAnchor := state.reviewSelectionAnchor
	state.reviewDocument = msg.document
	comparison, bounds := msg.comparison, msg.bounds
	state.displayedComparison = &comparison
	state.displayedBounds = &bounds
	state.diff = repository.Diff{}
	state.reader = repository.File{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.readerLoading = false
	newIdentities := state.reviewDocument.LineIdentities()
	state.place.ReaderOffset = reconcileLogicalLine(oldIdentities, oldOffset, newIdentities)
	state.reviewCursor = reconcileLogicalLine(oldIdentities, oldCursor, newIdentities)
	state.reviewSelectionAnchor = reconcileLogicalLine(oldIdentities, oldAnchor, newIdentities)
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	if !msg.document.Exact && msg.document.Reason != "" {
		state.comparisonWarning = msg.document.Reason
	}
	return state
}

func (state filesState) landReviewFile(msg reviewFileLoadedMsg, _ int) filesState {
	if msg.generation != state.contentGeneration || state.readerMode != workspace.FileReader ||
		msg.entry.Path != state.readerEntry.Path || state.requestedComparison == nil || state.requestedBounds == nil ||
		*state.requestedComparison != msg.comparison {
		return state
	}
	current, ok := state.reviewSnapshot.Comparisons[msg.entry.Path]
	if !ok || current != msg.comparison {
		return state
	}
	oldLines := state.previousReaderRows()
	oldOffset := state.place.ReaderOffset
	state.reviewFile = msg.content
	state.reviewDocument = review.Document{}
	state.reviewFileDiff = msg.document
	comparison := msg.comparison
	bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
	state.displayedComparison = &comparison
	state.displayedBounds = &bounds
	state.reader = repository.File{}
	state.diff = repository.Diff{}
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	if msg.content.Endpoint != comparison.New {
		state.comparisonWarning = "file changed; refresh before marking reviewed"
	}
	return state
}

func reconcileLogicalLine(old []string, oldIndex int, current []string) int {
	if len(current) == 0 {
		return 0
	}
	if len(old) == 0 {
		return clampIndex(oldIndex, len(current))
	}
	oldIndex = clampIndex(oldIndex, len(old))
	indices := make(map[string]int, len(current))
	for index, identity := range current {
		indices[identity] = index
	}
	if index, ok := indices[old[oldIndex]]; ok {
		return index
	}
	for distance := 1; distance < len(old); distance++ {
		if next := oldIndex + distance; next < len(old) {
			if index, ok := indices[old[next]]; ok {
				return index
			}
		}
		if previous := oldIndex - distance; previous >= 0 {
			if index, ok := indices[old[previous]]; ok {
				return index
			}
		}
	}
	return clampIndex(oldIndex, len(current))
}

func clampIndex(index, length int) int {
	if length <= 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func (state *filesState) requestReviewToggle(focus navigation.Focus, rowIndex int) effect {
	path := ""
	if rowIndex >= 0 {
		if rowIndex >= len(state.place.Items) {
			return effect{}
		}
		row, ok := state.tree.Row(state.place.Items[rowIndex])
		if !ok || row.Kind != filetree.File {
			return effect{}
		}
		path = row.Path
	} else if focus == navigation.FocusReader {
		path = state.readerEntry.Path
	} else if identity, ok := state.place.SelectedIdentity(); ok {
		row, exists := state.tree.Row(identity)
		if !exists || row.Kind != filetree.File {
			return effect{}
		}
		path = row.Path
	}
	entry, exists := state.entry(path)
	comparison, reviewable := state.reviewSnapshot.Comparisons[path]
	if !exists || !entry.Changed() || !reviewable || !comparison.Exact() {
		return effect{}
	}
	assessment := state.ledger.Assess(comparison)
	delta := review.Delta{Kind: review.MarkDelta, Comparison: comparison, Bounds: review.Bounds{Old: comparison.Old, New: comparison.New}}
	if focus == navigation.FocusReader {
		if state.readerEntry.Path != path || state.displayedComparison == nil || state.displayedBounds == nil || *state.displayedComparison != comparison {
			return effect{}
		}
		if state.readerMode == workspace.DiffReader && !state.reviewDocument.Exact {
			return effect{}
		}
		if state.readerMode == workspace.FileReader && state.reviewFile.Endpoint != comparison.New {
			return effect{}
		}
		delta.Bounds = *state.displayedBounds
	}
	if assessment.State == review.Reviewed {
		delta.Kind = review.ClearDelta
	}
	return effect{kind: effectVerifyReview, generation: state.listGeneration, entry: entry, comparison: comparison, delta: delta}
}

func (state filesState) landReviewVerified(msg reviewVerifiedMsg) (filesState, effect) {
	current, ok := state.reviewSnapshot.Comparisons[msg.entry.Path]
	if msg.generation != state.listGeneration || !ok || current != msg.comparison ||
		msg.content.Endpoint != current.New || msg.delta.Bounds.New != current.New {
		state.comparisonWarning = "file changed; refresh before marking reviewed"
		return state, effect{}
	}
	delta := msg.delta
	if delta.Kind == review.MarkDelta {
		delta.Retained = msg.content.RetainedText()
	}
	if !delta.Apply(&state.ledger) {
		return state, effect{}
	}
	state.rederiveReviews()
	state.reviewQueue = append(state.reviewQueue, delta)
	if !state.reviewLoaded {
		state.sessionDeltas = append(state.sessionDeltas, delta)
	}
	return state, state.nextReviewPersistence()
}

func (state *filesState) nextReviewPersistence() effect {
	if state.reviewPersisting || !state.reviewLoaded || len(state.reviewQueue) == 0 {
		return effect{}
	}
	state.reviewPersisting = true
	return effect{kind: effectPersistReview, store: state.store, delta: state.reviewQueue[0]}
}

func (state filesState) landReviewPersisted(msg reviewPersistedMsg) (filesState, effect) {
	if !state.reviewPersisting || len(state.reviewQueue) == 0 {
		return state, effect{}
	}
	state.reviewQueue = state.reviewQueue[1:]
	state.reviewPersisting = false
	if msg.err != nil {
		state.reviewWarning = "review mark is in memory but will not survive restart: " + msg.err.Error()
	} else {
		state.ledger = msg.ledger.Clone()
		for _, delta := range state.reviewQueue {
			_ = delta.Apply(&state.ledger)
		}
		state.reviewWarning = ""
	}
	state.rederiveReviews()
	return state, state.nextReviewPersistence()
}

func (state *filesState) toggleReviewBounds(mode workspace.ReaderMode) effect {
	if mode != workspace.DiffReader || state.readerEntry.Path == "" {
		return effect{}
	}
	comparison, ok := state.reviewSnapshot.Comparisons[state.readerEntry.Path]
	if !ok || state.ledger.Assess(comparison).State != review.Updated {
		return effect{}
	}
	state.reviewFull[state.readerEntry.Path] = !state.reviewFull[state.readerEntry.Path]
	state.readerContextExpanded = false
	return state.requestReader(state.readerEntry, mode)
}

func (state *filesState) selectNextReviewGap(visibleRows int, mode workspace.ReaderMode) effect {
	ordered := make([]string, 0)
	for priority := 0; priority < 4; priority++ {
		for _, path := range state.tree.Files() {
			comparison, ok := state.reviewSnapshot.Comparisons[path]
			if !ok {
				continue
			}
			candidatePriority, gap := state.ledger.Assess(comparison).State.GapPriority()
			if gap && candidatePriority == priority {
				ordered = append(ordered, path)
			}
		}
	}
	if len(ordered) == 0 {
		state.comparisonWarning = "All changed files are reviewed."
		return effect{}
	}
	current := state.readerEntry.Path
	next := ordered[0]
	for index, path := range ordered {
		if path == current {
			next = ordered[(index+1)%len(ordered)]
			break
		}
	}
	state.tree.ExpandParents(next)
	state.reconcileVisibleRows(visibleRows)
	state.selectIdentity(filetree.FileIdentity(next))
	state.place.EnsureSelectionVisible(visibleRows)
	entry, ok := state.entry(next)
	if !ok {
		return effect{}
	}
	state.comparisonWarning = ""
	return state.requestReader(entry, mode)
}

func (state filesState) directoryReviewProgress(directory string) (int, int) {
	if progress, ok := state.reviewProgress[directory]; ok {
		return progress.reviewed, progress.changed
	}
	reviewed, changed := 0, 0
	prefix := directory + "/"
	for path, comparison := range state.reviewSnapshot.Comparisons {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		changed++
		if state.ledger.Assess(comparison).State == review.Reviewed {
			reviewed++
		}
	}
	return reviewed, changed
}

func (state filesState) reviewAssessment(path string, comparison review.FileComparison) review.Assessment {
	if assessment, ok := state.reviewAssessments[path]; ok {
		return assessment
	}
	return state.ledger.Assess(comparison)
}

func (state *filesState) rederiveReviews() {
	state.reviewAssessments = state.ledger.AssessAll(state.reviewSnapshot.Comparisons)
	state.reviewProgress = make(map[string]reviewRollup)
	for path := range state.reviewSnapshot.Comparisons {
		assessment := state.reviewAssessments[path]
		segments := strings.Split(path, "/")
		for index := 1; index < len(segments); index++ {
			directory := strings.Join(segments[:index], "/")
			progress := state.reviewProgress[directory]
			progress.changed++
			if assessment.State == review.Reviewed {
				progress.reviewed++
			}
			state.reviewProgress[directory] = progress
		}
	}
}

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

func reviewLoadWarning(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("review comparison unavailable: %v", err)
}
