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
	if state.readerMode == workspace.DiffReader && state.readerEntry.Path != "" {
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

func (state filesState) landReviewSnapshot(msg reviewSnapshotLoadedMsg, mode workspace.ReaderMode, visibleRows int) (filesState, effect) {
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
	if state.readerEntry.Path == "" || mode != workspace.DiffReader {
		state.readerLoading = false
		return state, effect{}
	}
	pending := state.requestReader(state.readerEntry, mode)
	state.place.ClampReader(len(state.readerLines()), visibleRows)
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

func (state filesState) landReviewDocument(msg reviewDocumentLoadedMsg, visibleRows int) filesState {
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
	state.readerLoading = false
	newIdentities := state.reviewDocument.LineIdentities()
	state.place.ReaderOffset = reconcileLogicalLine(oldIdentities, oldOffset, newIdentities)
	state.reviewCursor = reconcileLogicalLine(oldIdentities, oldCursor, newIdentities)
	state.reviewSelectionAnchor = reconcileLogicalLine(oldIdentities, oldAnchor, newIdentities)
	state.readerPresentation = msg.lines
	if state.readerPresentation == nil {
		state.readerPresentation = state.deriveReaderLines()
	}
	state.place.ClampReader(len(state.readerLines()), visibleRows)
	if !msg.document.Exact && msg.document.Reason != "" {
		state.comparisonWarning = msg.document.Reason
	}
	return state
}

func (state filesState) landReviewFile(msg reviewFileLoadedMsg, visibleRows int) filesState {
	if msg.generation != state.contentGeneration || state.readerMode != workspace.FileReader ||
		msg.entry.Path != state.readerEntry.Path || state.requestedComparison == nil || state.requestedBounds == nil ||
		*state.requestedComparison != msg.comparison {
		return state
	}
	current, ok := state.reviewSnapshot.Comparisons[msg.entry.Path]
	if !ok || current != msg.comparison {
		return state
	}
	oldLines := readerLineIdentities(state.readerLines())
	oldOffset := state.place.ReaderOffset
	state.reviewFile = msg.content
	state.reviewDocument = review.Document{}
	comparison := msg.comparison
	bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
	state.displayedComparison = &comparison
	state.displayedBounds = &bounds
	state.reader = repository.File{}
	state.diff = repository.Diff{}
	state.readerLoading = false
	state.readerPresentation = msg.lines
	if state.readerPresentation == nil {
		state.readerPresentation = state.deriveReaderLines()
	}
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerLineIdentities(state.readerLines()))
	state.place.ClampReader(len(state.readerLines()), visibleRows)
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

func reviewReaderLines(path string, document review.Document) []ui.Line {
	lines := make([]ui.Line, len(document.Lines))
	group := make([]diffCodeRow, 0, len(document.Lines))
	for index, line := range document.Lines {
		tone := ui.ToneDefault
		kind := diffContext
		marker := "  "
		switch line.Kind {
		case review.AddedLine:
			tone = ui.ToneAdded
			kind = diffAdded
			marker = "+ "
		case review.RemovedLine:
			tone = ui.ToneRemoved
			kind = diffRemoved
			marker = "- "
		case review.NoticeLine:
			if !document.Exact {
				tone = ui.ToneError
			} else {
				tone = ui.ToneQuiet
			}
		}
		lines[index] = ui.Line{Text: line.Text, Tone: tone}
		if line.Kind != review.NoticeLine && len(line.Text) >= len(marker) {
			group = append(group, diffCodeRow{index: index, marker: marker, payload: line.Text[len(marker):], kind: kind})
		}
	}
	decorateDiffGroup(path, lines, group)
	return lines
}

func reviewFileReaderLines(content review.Content, entry repository.Entry) []ui.Line {
	if content.Endpoint.Path != entry.Path {
		return []ui.Line{{Text: "File changed; refresh before marking reviewed.", Tone: ui.ToneError}}
	}
	switch content.State {
	case review.ContentText:
		if content.Endpoint.Kind == review.Symlink {
			return []ui.Line{{Text: "symlink → " + content.Text}}
		}
		if content.Endpoint.Kind == review.Submodule {
			return []ui.Line{{Text: "submodule → " + content.Text}}
		}
		return highlightedSourceLines(entry.Path, content.Text)
	case review.ContentAbsent:
		return []ui.Line{{Text: "File was deleted from the worktree.", Tone: ui.ToneError}}
	case review.ContentBinary:
		return []ui.Line{{Text: fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", content.Size), Tone: ui.ToneError}}
	case review.ContentTooLarge:
		return []ui.Line{{Text: fmt.Sprintf("File is too large (%d bytes; bounded review reader).", content.Size), Tone: ui.ToneError}}
	default:
		detail := content.Err
		if detail == "" {
			detail = "exact content unavailable"
		}
		return []ui.Line{{Text: "File is unavailable: " + detail, Tone: ui.ToneError}}
	}
}

func reviewLoadWarning(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("review comparison unavailable: %v", err)
}
