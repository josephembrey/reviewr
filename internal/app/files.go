package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type filesState struct {
	place    navigation.State
	tree     filetree.Tree
	snapshot repository.Snapshot
	entries  []repository.Entry

	readerEntry repository.Entry
	readerMode  workspace.ReaderMode
	reader      repository.File
	diff        repository.Diff

	listGeneration    uint64
	contentGeneration uint64
	loaded            bool
	listLoading       bool
	readerLoading     bool
	listError         error
}

func newFilesState() filesState {
	return filesState{
		place:          navigation.State{Focus: navigation.FocusNavigator},
		tree:           filetree.New(nil),
		listGeneration: 1,
		listLoading:    true,
	}
}

func (state *filesState) reload() effect {
	state.listGeneration++
	state.listLoading = true
	state.listError = nil
	return effect{kind: effectLoadSnapshot, generation: state.listGeneration}
}

func (state filesState) landSnapshot(msg snapshotLoadedMsg, scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int) (filesState, effect) {
	if msg.generation != state.listGeneration {
		return state, effect{}
	}
	firstLoad := !state.loaded
	state.loaded = true
	state.listLoading = false
	if msg.err != nil {
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	state.snapshot = msg.snapshot
	pending := state.project(scope, mode, visibleRows, firstLoad, true)
	return state, pending
}

func (state filesState) landFile(msg fileLoadedMsg, visibleRows int) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.FileReader {
		return state
	}
	state.reader = msg.file
	state.diff = repository.Diff{}
	state.readerLoading = false
	state.place.ClampReader(len(state.readerLines()), visibleRows)
	return state
}

func (state filesState) landDiff(msg diffLoadedMsg, visibleRows int) filesState {
	if msg.generation != state.contentGeneration || msg.entry.Path != state.readerEntry.Path || state.readerMode != workspace.DiffReader {
		return state
	}
	state.diff = msg.diff
	state.reader = repository.File{}
	state.readerLoading = false
	state.place.ClampReader(len(state.readerLines()), visibleRows)
	return state
}

// switchScope derives another projection from the loaded snapshot. It never
// starts a snapshot load and the one existing tree is rebuilt in place.
func (state *filesState) switchScope(scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int) effect {
	if !state.loaded {
		state.readerMode = mode
		return effect{}
	}
	return state.project(scope, mode, visibleRows, false, false)
}

func (state *filesState) project(scope workspace.FileSet, mode workspace.ReaderMode, visibleRows int, firstLoad, refresh bool) effect {
	oldRows := state.tree.Rows()
	oldEntries := append([]repository.Entry(nil), state.entries...)
	oldReader := state.readerEntry

	entries := entriesForScope(state.snapshot, scope)
	state.tree.Rebuild(entryPaths(entries))
	state.reconcileCursor(oldRows)
	state.entries = orderEntries(entries, state.tree.Files())
	if firstLoad {
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
		return state.requestReader(entry, mode)
	}
	state.readerEntry = entry
	state.readerMode = mode
	return effect{}
}

func (state *filesState) requestReader(entry repository.Entry, mode workspace.ReaderMode) effect {
	state.contentGeneration++
	if state.readerEntry.Path != entry.Path || state.readerMode != mode {
		state.reader = repository.File{}
		state.diff = repository.Diff{}
	}
	state.readerEntry = entry
	state.readerMode = mode
	state.readerLoading = true
	kind := effectLoadFile
	if mode == workspace.DiffReader {
		kind = effectLoadDiff
	}
	return effect{kind: kind, generation: state.contentGeneration, entry: entry}
}

func (state *filesState) requestMode(mode workspace.ReaderMode) effect {
	if state.readerEntry.Path == "" || state.readerMode == mode {
		return effect{}
	}
	state.place.ReaderOffset = 0
	return state.requestReader(state.readerEntry, mode)
}

func (state *filesState) selectDelta(delta, visibleRows int, mode workspace.ReaderMode) effect {
	return state.selectIndex(state.place.Selected+delta, visibleRows, mode)
}

func (state *filesState) selectIndex(index, visibleRows int, mode workspace.ReaderMode) effect {
	readerOffset := state.place.ReaderOffset
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	identity, _ := state.place.SelectedIdentity()
	row, ok := state.tree.Row(identity)
	if !ok || row.Kind == filetree.Directory {
		state.place.ReaderOffset = readerOffset
		return effect{}
	}
	entry, ok := state.entry(row.Path)
	if !ok {
		state.place.ReaderOffset = readerOffset
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
	state.readerLoading = false
	state.place.ReaderOffset = 0
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
		}
	}
	if state.readerLoading && (state.reader.Kind != 0 || state.diff.Kind != 0) {
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
		Geometry:       geometry,
		NavigatorTitle: fmt.Sprintf("%d files", state.tree.FileCount()),
		NavigatorRows:  rows,
		NavigatorEmpty: emptyNavigator,
		Selected:       state.place.Selected,
		Top:            state.place.Top,
		Focus:          state.place.Focus,
		ReaderTitle:    readerTitle,
		ReaderLines:    state.readerLines(),
		ReaderEmpty:    readerEmpty,
		ReaderOffset:   state.place.ReaderOffset,
	}
}

func (state filesState) readerLines() []ui.Line {
	return (readerDocument{
		File: state.reader, Entry: state.readerEntry, Diff: state.diff, Mode: state.readerMode,
	}).lines()
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
