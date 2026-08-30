package app

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type filesState struct {
	place            navigation.State
	tree             filetree.Tree
	folds            map[workspace.FileSet]filetree.FoldState
	treeScope        workspace.FileSet
	treeScopeReady   bool
	snapshot         repository.Snapshot
	entries          []repository.Entry
	entriesByPath    map[string]repository.Entry
	directoryIgnored map[string]bool

	readerEntry         repository.Entry
	readerMode          workspace.ReaderMode
	reader              repository.File
	diff                repository.Diff
	readerPresentation  *ui.ReaderDocument
	readerContext       readerContextState
	restoredReaderRows  []string
	reviewSnapshot      review.Snapshot
	ledger              review.Ledger
	store               *review.Store
	reviewDocument      review.Document
	reviewFile          review.Content
	reviewFileDiff      review.Document
	displayedComparison *review.FileComparison
	displayedBounds     *review.Bounds
	requestedComparison *review.FileComparison
	requestedBounds     *review.Bounds
	reviewScope         string
	reviewWarning       string
	comparisonWarning   string
	reviewFull          map[string]bool
	reviewQueue         []review.Delta
	sessionDeltas       []review.Delta
	reviewPersisting    bool
	reviewLoaded        bool
	reviewAssessments   map[string]review.Assessment
	reviewProgress      map[string]reviewRollup
	comparisonCache     map[string]comparisonCacheEntry
	readerCache         map[readerCacheSlot]readerCacheEntry
	readerRequestKey    readerCacheKey
	readerLoadedKey     readerCacheKey

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
		comparisonCache:  make(map[string]comparisonCacheEntry),
		readerCache:      make(map[readerCacheSlot]readerCacheEntry),
	}
}

func (state *filesState) reload(scope string) effect {
	state.invalidateComparison(scope)
	state.listGeneration++
	state.reviewGeneration++
	state.listLoading = true
	state.listError = nil
	if state.reviewScope != scope {
		state.comparisonWarning = ""
	}
	state.reviewScope = scope
	state.reviewSnapshot = review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	state.rederiveReviews()
	return effect{
		kind: effectLoadSnapshot, generation: state.listGeneration,
		reviewGeneration: state.reviewGeneration, scope: scope,
	}
}

func (state filesState) comparisonPending() bool {
	loaded := state.snapshot.Comparison().Scope
	return state.listLoading && loaded != "" && state.reviewScope != "" && loaded != state.reviewScope
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
	state.reviewScope = msg.snapshot.Comparison().Scope
	state.reviewSnapshot = review.Snapshot{Scope: state.reviewScope, Comparisons: make(map[string]review.FileComparison)}
	state.comparisonWarning = ""
	if msg.reviewCapable && msg.reviewGeneration == state.reviewGeneration {
		state.reviewSnapshot = msg.reviewSnapshot
		state.reviewScope = msg.reviewSnapshot.Scope
		state.comparisonWarning = reviewLoadWarning(msg.reviewErr)
	}
	state.rederiveReviews()
	state.rememberComparison(msg)
	pending := state.project(scope, mode, visibleRows, firstLoad, true, msg.background)
	return state, pending
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
	state.entries, state.entriesByPath = orderEntries(entries, state.tree.Files())
	state.directoryIgnored = ignoredDirectories(state.entries)
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

func (state *filesState) selectDelta(delta, visibleRows int, mode workspace.ReaderMode) effect {
	return state.selectIndex(state.place.Selected+delta, visibleRows, mode)
}

func (state *filesState) selectIndex(index, visibleRows int, mode workspace.ReaderMode) effect {
	readerOffset := state.place.ReaderOffset
	readerColumn := state.place.ReaderColumn
	readerCursor := state.place.ReaderCursor
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	identity, _ := state.place.SelectedIdentity()
	row, ok := state.tree.Row(identity)
	if !ok || row.Kind == filetree.Directory {
		state.place.ReaderOffset = readerOffset
		state.place.ReaderColumn = readerColumn
		state.place.ReaderCursor = readerCursor
		return effect{}
	}
	entry, ok := state.entry(row.Path)
	if !ok {
		state.place.ReaderOffset = readerOffset
		state.place.ReaderColumn = readerColumn
		state.place.ReaderCursor = readerCursor
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

func (state filesState) entry(path string) (repository.Entry, bool) {
	if state.entriesByPath != nil {
		entry, ok := state.entriesByPath[path]
		return entry, ok
	}
	for _, entry := range state.entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return repository.Entry{}, false
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

func orderEntries(entries []repository.Entry, paths []string) ([]repository.Entry, map[string]repository.Entry) {
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
	return ordered, byPath
}

func ignoredDirectories(entries []repository.Entry) map[string]bool {
	directories := make(map[string]bool)
	for _, entry := range entries {
		ignored := entry.State == repository.FileIgnored
		parent := entry.Path
		for {
			separator := strings.LastIndexByte(parent, '/')
			if separator < 0 {
				break
			}
			parent = parent[:separator]
			allIgnored, seen := directories[parent]
			directories[parent] = ignored && (!seen || allIgnored)
		}
	}
	return directories
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
