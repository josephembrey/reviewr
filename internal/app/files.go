package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type filesState struct {
	place          navigation.State
	tree           filetree.Tree
	folds          map[workspace.FileSet]filetree.FoldState
	treeScope      workspace.FileSet
	treeScopeReady bool
	snapshot       repository.Snapshot
	entries        []repository.Entry

	readerEntry           repository.Entry
	readerMode            workspace.ReaderMode
	reader                repository.File
	diff                  repository.Diff
	readerPresentation    *ui.ReaderDocument
	readerContextExpanded bool
	restoredReaderRows    []string
	reviewSnapshot        review.Snapshot
	ledger                review.Ledger
	store                 *review.Store
	reviewDocument        review.Document
	reviewFile            review.Content
	reviewFileDiff        review.Document
	displayedComparison   *review.FileComparison
	displayedBounds       *review.Bounds
	requestedComparison   *review.FileComparison
	requestedBounds       *review.Bounds
	reviewScope           string
	reviewWarning         string
	comparisonWarning     string
	reviewFull            map[string]bool
	reviewQueue           []review.Delta
	sessionDeltas         []review.Delta
	reviewPersisting      bool
	reviewLoaded          bool
	reviewCursor          int
	reviewSelectionAnchor int
	reviewAssessments     map[string]review.Assessment
	reviewProgress        map[string]reviewRollup

	listGeneration    uint64
	contentGeneration uint64
	reviewGeneration  uint64
	loaded            bool
	listLoading       bool
	readerLoading     bool
	listError         error
}

func newFilesState() filesState {
	return filesState{
		place:            navigation.State{Focus: navigation.FocusNavigator},
		tree:             filetree.New(nil),
		folds:            make(map[workspace.FileSet]filetree.FoldState),
		listGeneration:   1,
		reviewGeneration: 1,
		listLoading:      true,
		reviewFull:       make(map[string]bool),
	}
}

func (state *filesState) reload() effect {
	state.listGeneration++
	state.reviewGeneration++
	state.listLoading = true
	state.listError = nil
	state.reviewSnapshot = review.Snapshot{Scope: state.reviewScope, Comparisons: make(map[string]review.FileComparison)}
	state.rederiveReviews()
	return effect{kind: effectLoadSnapshot, generation: state.listGeneration, reviewGeneration: state.reviewGeneration}
}

func (state *filesState) poll(scope string) effect {
	state.listGeneration++
	state.reviewGeneration++
	return effect{
		kind: effectLoadSnapshot, generation: state.listGeneration,
		reviewGeneration: state.reviewGeneration, scope: scope, background: true,
	}
}

func (state filesState) landSnapshot(msg snapshotLoadedMsg, scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int) (filesState, effect) {
	if msg.generation != state.listGeneration {
		return state, effect{}
	}
	firstLoad := !state.loaded
	state.loaded = true
	state.listLoading = false
	if msg.err != nil {
		if msg.background {
			return state, effect{}
		}
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	state.snapshot = msg.snapshot
	if msg.reviewCapable && msg.reviewGeneration == state.reviewGeneration && (!msg.background || msg.reviewErr == nil) {
		state.reviewSnapshot = msg.reviewSnapshot
		state.reviewScope = msg.reviewSnapshot.Scope
		if !msg.background {
			state.comparisonWarning = reviewLoadWarning(msg.reviewErr)
		}
		state.rederiveReviews()
	}
	pending := state.project(scope, mode, visibleRows, firstLoad, true, msg.background)
	return state, pending
}

func (state filesState) landFile(msg fileLoadedMsg, _ int) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.FileReader {
		return state
	}
	oldLines := state.previousReaderRows()
	oldOffset := state.place.ReaderOffset
	state.reader = msg.file
	state.diff = repository.Diff{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	return state
}

func (state filesState) landDiff(msg diffLoadedMsg, _ int) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.DiffReader {
		return state
	}
	oldLines := state.previousReaderRows()
	oldOffset := state.place.ReaderOffset
	state.diff = msg.diff
	state.reader = repository.File{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerLoading = false
	presentation := msg.presentation
	if presentation.Kind == ui.ReaderDocumentNone {
		presentation = state.deriveReaderDocument()
	}
	state.readerPresentation = &presentation
	state.place.ReaderOffset = reconcileLogicalLine(oldLines, oldOffset, readerRowIdentities(state.readerRows()))
	state.restoredReaderRows = nil
	state.place.ClampReaderSource(len(state.readerRows()))
	return state
}

// switchScope derives another projection from the loaded snapshot. It never
// starts a snapshot load and the one existing tree is rebuilt in place.
func (state *filesState) switchScope(scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int) effect {
	if !state.loaded {
		state.readerMode = mode
		return effect{}
	}
	return state.project(scope, mode, visibleRows, false, false, false)
}

func (state *filesState) project(scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int, firstLoad, refresh, background bool) effect {
	oldRows := state.tree.Rows()
	oldEntries := append([]repository.Entry(nil), state.entries...)
	oldReader := state.readerEntry
	_, hadSelection := state.place.SelectedIdentity()
	if state.treeScopeReady {
		state.folds[state.treeScope] = state.tree.Folds()
	}

	entries := entriesForScope(state.snapshot, scope)
	state.tree.Rebuild(entryPaths(entries))
	state.tree.RestoreFolds(state.folds[scope], scope == workspace.AllFiles)
	state.treeScope = scope
	state.treeScopeReady = true
	state.folds[scope] = state.tree.Folds()
	state.reconcileCursor(oldRows)
	state.entries = orderEntries(entries, state.tree.Files())
	if firstLoad && !hadSelection {
		state.selectFirstVisibleFile()
	}
	state.place.EnsureSelectionVisible(visibleRows)

	if oldReader.Path == "" {
		if row, ok := state.tree.FirstVisibleFile(); ok {
			state.selectIdentity(row.Identity)
			state.place.EnsureSelectionVisible(visibleRows)
			if entry, exists := state.entry(row.Path); exists {
				return state.requestReader(entry, mode)
			}
		}
		state.clearReader()
		return effect{}
	}

	entry, ok := reconcileReaderEntry(oldEntries, oldReader, state.entries)
	if !ok {
		state.clearReader()
		return effect{}
	}
	if refresh || entry.Path != oldReader.Path || state.readerMode != mode {
		if background {
			return state.requestReaderQuiet(entry, mode)
		}
		return state.requestReader(entry, mode)
	}
	state.readerEntry = entry
	state.readerMode = mode
	return effect{}
}

func (state *filesState) requestReader(entry repository.Entry, mode workspace.ReaderMode) effect {
	return state.requestReaderWithLoading(entry, mode, true)
}

func (state *filesState) requestReaderQuiet(entry repository.Entry, mode workspace.ReaderMode) effect {
	return state.requestReaderWithLoading(entry, mode, false)
}

func (state *filesState) requestReaderWithLoading(entry repository.Entry, mode workspace.ReaderMode, loading bool) effect {
	state.contentGeneration++
	if state.readerEntry.Path != entry.Path || state.readerMode != mode {
		state.reader = repository.File{}
		state.diff = repository.Diff{}
		state.reviewDocument = review.Document{}
		state.reviewFile = review.Content{}
		state.reviewFileDiff = review.Document{}
		state.displayedComparison = nil
		state.displayedBounds = nil
		state.readerPresentation = nil
		state.readerContextExpanded = false
		state.restoredReaderRows = nil
	}
	state.readerEntry = entry
	state.readerMode = mode
	state.readerLoading = loading
	state.requestedComparison = nil
	state.requestedBounds = nil
	kind := effectLoadFile
	if mode == workspace.FileReader {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			comparisonCopy := comparison
			boundsCopy := review.Bounds{Old: comparison.Old, New: comparison.New}
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			return effect{kind: effectLoadReviewFile, generation: state.contentGeneration, entry: entry, comparison: comparison}
		}
	} else {
		if comparison, ok := state.reviewSnapshot.Comparisons[entry.Path]; ok {
			assessment := state.ledger.Assess(comparison)
			bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
			var retained *string
			if assessment.State == review.Updated && !state.reviewFull[entry.Path] && assessment.Frontier != nil && assessment.Retained != nil {
				bounds.Old = *assessment.Frontier
				retained = assessment.Retained
			}
			comparisonCopy, boundsCopy := comparison, bounds
			state.requestedComparison = &comparisonCopy
			state.requestedBounds = &boundsCopy
			return effect{kind: effectLoadReviewDocument, generation: state.contentGeneration, entry: entry, comparison: comparison, bounds: bounds, retained: retained}
		}
		kind = effectLoadDiff
	}
	return effect{kind: kind, generation: state.contentGeneration, entry: entry}
}

func readerRowIdentities(rows []ui.ReaderRow) []string {
	identities := make([]string, len(rows))
	occurrences := make(map[string]int, len(rows))
	for index, row := range rows {
		identity := row.Identity
		if identity == "" {
			identity = fmt.Sprintf("%d:%d:%d:%s", row.Kind, row.OldLine, row.NewLine, row.Text)
		}
		occurrences[identity]++
		identities[index] = fmt.Sprintf("%s\x00%d", identity, occurrences[identity])
	}
	return identities
}

func (state *filesState) requestMode(mode workspace.ReaderMode) effect {
	if state.readerEntry.Path == "" || state.readerMode == mode {
		return effect{}
	}
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
	return state.requestReader(state.readerEntry, mode)
}

func (state *filesState) selectDelta(delta, visibleRows int, mode workspace.ReaderMode) effect {
	return state.selectIndex(state.place.Selected+delta, visibleRows, mode)
}

func (state *filesState) selectIndex(index, visibleRows int, mode workspace.ReaderMode) effect {
	readerOffset := state.place.ReaderOffset
	readerColumn := state.place.ReaderColumn
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	identity, _ := state.place.SelectedIdentity()
	row, ok := state.tree.Row(identity)
	if !ok || row.Kind == filetree.Directory {
		state.place.ReaderOffset = readerOffset
		state.place.ReaderColumn = readerColumn
		return effect{}
	}
	entry, ok := state.entry(row.Path)
	if !ok {
		state.place.ReaderOffset = readerOffset
		state.place.ReaderColumn = readerColumn
		return effect{}
	}
	return state.requestReader(entry, mode)
}

func (state *filesState) expandSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Expand(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state *filesState) collapseSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Collapse(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state filesState) selectedKind() (filetree.Kind, bool) {
	identity, ok := state.place.SelectedIdentity()
	if !ok {
		return 0, false
	}
	row, ok := state.tree.Row(identity)
	return row.Kind, ok
}

func (state *filesState) toggleSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Toggle(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state *filesState) reconcileVisibleRows(visibleRows int) {
	state.place.Reconcile(state.tree.Identities())
	state.place.EnsureSelectionVisible(visibleRows)
}

func (state *filesState) reconcileCursor(oldRows []filetree.Row) {
	oldIdentity, selected := state.place.SelectedIdentity()
	state.place.Reconcile(state.tree.Identities())
	if !selected {
		return
	}
	if containsIdentity(state.tree.Rows(), oldIdentity) {
		state.selectIdentity(oldIdentity)
		return
	}
	oldKind, ok := rowKind(oldRows, oldIdentity)
	if !ok {
		return
	}
	oldRole := rowIdentities(oldRows, oldKind)
	currentRole := rowIdentities(state.tree.Rows(), oldKind)
	if identity, exists := navigation.ReconcileIdentity(oldRole, oldIdentity, currentRole); exists {
		state.selectIdentity(identity)
	}
}

func (state *filesState) selectFirstVisibleFile() {
	row, ok := state.tree.FirstVisibleFile()
	if ok {
		state.selectIdentity(row.Identity)
	}
}

func (state *filesState) selectIdentity(identity string) {
	for index, candidate := range state.place.Items {
		if candidate == identity {
			state.place.Selected = index
			return
		}
	}
}

func (state *filesState) clearReader() {
	state.contentGeneration++
	state.readerEntry = repository.Entry{}
	state.reader = repository.File{}
	state.diff = repository.Diff{}
	state.reviewDocument = review.Document{}
	state.reviewFile = review.Content{}
	state.reviewFileDiff = review.Document{}
	state.displayedComparison = nil
	state.displayedBounds = nil
	state.readerPresentation = nil
	state.readerContextExpanded = false
	state.restoredReaderRows = nil
	state.requestedComparison = nil
	state.requestedBounds = nil
	state.readerLoading = false
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
}

func (state filesState) entry(path string) (repository.Entry, bool) {
	for _, entry := range state.entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return repository.Entry{}, false
}

func (state filesState) viewModel(geometry ui.Geometry) ui.Model {
	treeRows := state.tree.Rows()
	rows := make([]ui.NavigatorRow, len(treeRows))
	for index, row := range treeRows {
		presentation := ui.NavigatorRow{
			Identity:  row.Identity,
			Label:     row.Name,
			Tree:      true,
			Depth:     row.Depth,
			Directory: row.Kind == filetree.Directory,
			Expanded:  row.Expanded,
		}
		if entry, ok := state.entry(row.Path); ok {
			presentation.Status = navigatorStatus(entry.State)
			presentation.Dimmed = entry.State == repository.FileIgnored
			if entry.Changed() && !entry.Binary {
				changes := ui.LineChanges{Additions: entry.Additions, Deletions: entry.Deletions}
				presentation.Changes = &changes
			}
			if comparison, reviewable := state.reviewSnapshot.Comparisons[row.Path]; reviewable && entry.Changed() {
				reviewState := state.reviewAssessment(row.Path, comparison).State
				presentation.Review = &reviewState
			}
		} else if row.Kind == filetree.Directory {
			reviewed, changed := state.directoryReviewProgress(row.Path)
			if changed > 0 {
				presentation.Progress = fmt.Sprintf("%d/%d", reviewed, changed)
			}
		}
		rows[index] = presentation
	}

	emptyNavigator := ui.Line{Text: "No files", Tone: ui.ToneQuiet}
	if state.listLoading {
		emptyNavigator.Text = "Loading files…"
	} else if state.listError != nil {
		emptyNavigator = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}

	readerTitle := "No selection"
	if state.readerEntry.Path != "" {
		readerTitle = state.readerEntry.Path
		if state.readerEntry.State == repository.FileRenamed && state.readerEntry.PreviousPath != "" {
			readerTitle = state.readerEntry.PreviousPath + " → " + state.readerEntry.Path
		}
		if state.readerMode == workspace.DiffReader {
			readerTitle += "  diff"
			if state.displayedBounds != nil && state.displayedComparison != nil {
				assessment := state.ledger.Assess(*state.displayedComparison)
				if assessment.State == review.Updated && state.displayedBounds.Old != state.displayedComparison.Old {
					readerTitle += "  since reviewed"
				} else if state.reviewFull[state.readerEntry.Path] && assessment.State == review.Updated {
					readerTitle += "  full comparison"
				} else if assessment.State == review.Partial {
					readerTitle += "  older review gap; full comparison"
				} else if assessment.State == review.BasisChanged {
					readerTitle += "  review basis changed; full comparison"
				}
			}
		}
	}
	if (state.readerLoading || state.listLoading) && (state.reader.Kind != 0 || state.diff.Kind != 0 || state.displayedBounds != nil) {
		readerTitle += "  refreshing…"
	}
	readerEmpty := ui.Line{Text: "Select a file to read its current content.", Tone: ui.ToneQuiet}
	if state.readerMode == workspace.DiffReader {
		readerEmpty.Text = "Select a file to read its uncommitted diff."
	}
	if state.readerLoading {
		readerEmpty = ui.Line{Text: "Loading file…", Tone: ui.ToneQuiet}
		if state.readerMode == workspace.DiffReader {
			readerEmpty.Text = "Loading diff…"
		}
	}

	return ui.Model{
		Geometry:              geometry,
		NavigatorTitle:        fmt.Sprintf("%d files", state.tree.FileCount()),
		NavigatorRows:         rows,
		NavigatorEmpty:        emptyNavigator,
		Selected:              state.place.Selected,
		Top:                   state.place.Top,
		Focus:                 state.place.Focus,
		ReaderTitle:           readerTitle,
		ReaderDocument:        state.readerDocument(),
		ReaderContextFoldable: state.rawReaderDocument().ContextFoldable(),
		ReaderEmpty:           readerEmpty,
		ReaderOffset:          state.place.ReaderOffset,
		ReaderColumn:          state.place.ReaderColumn,
		FooterWarning:         firstWarning(state.reviewWarning, state.comparisonWarning),
	}
}

func (state filesState) readerDocument() ui.ReaderDocument {
	return state.rawReaderDocument().WithContextFolds(state.readerContextExpanded)
}

func (state filesState) rawReaderDocument() ui.ReaderDocument {
	if state.readerPresentation != nil {
		return *state.readerPresentation
	}
	if state.readerEntry.Path == "" || state.readerLoading {
		return ui.ReaderDocument{}
	}
	return state.deriveReaderDocument()
}

func (state *filesState) setReaderContextExpanded(expanded bool) bool {
	if state.readerContextExpanded == expanded || !state.rawReaderDocument().ContextFoldable() {
		return false
	}
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset := state.place.ReaderOffset
	state.readerContextExpanded = expanded
	state.place.ReaderOffset = reconcileLogicalLine(oldRows, oldOffset, readerRowIdentities(state.readerRows()))
	if state.place.ReaderOffset != oldOffset {
		state.place.ReaderColumn = 0
	}
	state.place.ClampReaderSource(len(state.readerRows()))
	return true
}

func (state filesState) readerRows() []ui.ReaderRow {
	return state.readerDocument().Rows
}

func (state filesState) previousReaderRows() []string {
	current := readerRowIdentities(state.readerRows())
	if len(current) == 0 && len(state.restoredReaderRows) != 0 {
		return append([]string(nil), state.restoredReaderRows...)
	}
	return current
}

func (state filesState) deriveReaderDocument() ui.ReaderDocument {
	if state.readerMode == workspace.DiffReader {
		if state.displayedBounds != nil {
			return reviewReaderDocument(state.readerEntry.Path, state.reviewDocument)
		}
		return (readerDocument{Diff: state.diff, Mode: state.readerMode}).build()
	}
	if state.displayedComparison != nil {
		if state.reviewFile.Endpoint != state.displayedComparison.New {
			return ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: noticeRows("File changed; refresh before marking reviewed.", ui.ToneError)}
		}
		return annotatedReviewFileReaderDocument(
			state.reviewFile,
			state.readerEntry,
			*state.displayedComparison,
			state.reviewFileDiff,
		)
	}
	return (readerDocument{
		File: state.reader, Entry: state.readerEntry, Diff: state.diff, Mode: state.readerMode,
	}).build()
}

func entriesForScope(snapshot repository.Snapshot, scope workspace.FileSet) []repository.Entry {
	if scope == workspace.ChangedFiles {
		return snapshot.Changed()
	}
	return snapshot.All()
}

func entryPaths(entries []repository.Entry) []string {
	paths := make([]string, len(entries))
	for index, entry := range entries {
		paths[index] = entry.Path
	}
	return paths
}

func orderEntries(entries []repository.Entry, paths []string) []repository.Entry {
	byPath := make(map[string]repository.Entry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	ordered := make([]repository.Entry, 0, len(paths))
	for _, path := range paths {
		if entry, ok := byPath[path]; ok {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

func reconcileReaderEntry(old []repository.Entry, entry repository.Entry, current []repository.Entry) (repository.Entry, bool) {
	for _, candidate := range current {
		if candidate.Path == entry.Path {
			return candidate, true
		}
	}
	for _, candidate := range current {
		if candidate.PreviousPath == entry.Path {
			return candidate, true
		}
	}
	currentPaths := entryPaths(current)
	path, ok := navigation.ReconcileIdentity(entryPaths(old), entry.Path, currentPaths)
	if !ok {
		return repository.Entry{}, false
	}
	for _, candidate := range current {
		if candidate.Path == path {
			return candidate, true
		}
	}
	return repository.Entry{}, false
}

func rowKind(rows []filetree.Row, identity string) (filetree.Kind, bool) {
	for _, row := range rows {
		if row.Identity == identity {
			return row.Kind, true
		}
	}
	return filetree.File, false
}

func rowIdentities(rows []filetree.Row, kind filetree.Kind) []string {
	identities := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Kind == kind {
			identities = append(identities, row.Identity)
		}
	}
	return identities
}

func containsIdentity(rows []filetree.Row, identity string) bool {
	for _, row := range rows {
		if row.Identity == identity {
			return true
		}
	}
	return false
}

func navigatorStatus(state repository.FileState) ui.NavigatorStatus {
	switch state {
	case repository.FileModified:
		return ui.StatusModified
	case repository.FileAdded:
		return ui.StatusAdded
	case repository.FileDeleted:
		return ui.StatusDeleted
	case repository.FileRenamed:
		return ui.StatusRenamed
	case repository.FileUntracked:
		return ui.StatusUntracked
	case repository.FileIgnored:
		return ui.StatusIgnored
	default:
		return ui.StatusNone
	}
}

func firstWarning(warnings ...string) string {
	for _, warning := range warnings {
		if warning != "" {
			return warning
		}
	}
	return ""
}
