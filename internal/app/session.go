package app

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/session"
	"github.com/josephembrey/reviewr/internal/workspace"
)

const sessionSaveDebounce = 200 * time.Millisecond

func (m *Model) commandAfterAction(pending effect) tea.Cmd {
	primary := m.command(pending)
	if m.sessionStore == nil || pending.kind == effectQuit {
		return primary
	}
	m.sessionSave++
	if m.sessionPending {
		return primary
	}
	m.sessionPending = true
	return batchCommands(primary, scheduleSessionSave(m.sessionSave))
}

func scheduleSessionSave(generation uint64) tea.Cmd {
	return tea.Tick(sessionSaveDebounce, func(time.Time) tea.Msg {
		return sessionSaveDueMsg{generation: generation}
	})
}

func (m Model) sessionState() session.State {
	files := m.files
	fileFolds := make(map[string]session.Folds, len(files.folds))
	for scope, folds := range files.folds {
		known, collapsed := folds.Paths()
		fileFolds[scope.Label()] = session.Folds{Known: known, Collapsed: collapsed}
	}
	if files.treeScopeReady {
		known, collapsed := files.tree.Folds().Paths()
		fileFolds[files.treeScope.Label()] = session.Folds{Known: known, Collapsed: collapsed}
	}
	readerRows := files.restoredReaderRows
	if rows := readerRowIdentities(files.readerRows()); len(rows) != 0 {
		readerRows = rows
	}

	stashes := m.stashes
	stashes.inspection.readerPlaces = cloneChangePlaces(m.stashes.inspection.readerPlaces)
	stashes.inspection.saveReaderPlace()
	stashPlaces := make(map[string]session.ChangeReaderPlace, len(stashes.inspection.readerPlaces))
	for oid, place := range stashes.inspection.readerPlaces {
		stashPlaces[oid] = session.ChangeReaderPlace{
			FileIdentity: place.fileIdentity,
			FileTop:      place.fileTop,
			ReaderOffset: place.readerOffset,
			ReaderColumn: place.readerColumn,
			ReaderCursor: place.readerCursor,
		}
	}
	stashRows := stashes.inspection.restoredReaderRows
	if rows := readerRowIdentities(stashes.readerRows()); len(rows) != 0 {
		stashRows = rows
	}

	historyInspection := m.history.inspection
	historyInspection.readerPlaces = cloneChangePlaces(m.history.inspection.readerPlaces)
	historyInspection.saveReaderPlace()
	historyPlaces := make(map[string]session.ChangeReaderPlace, len(historyInspection.readerPlaces))
	for oid, place := range historyInspection.readerPlaces {
		historyPlaces[oid] = session.ChangeReaderPlace{
			FileIdentity: place.fileIdentity, FileTop: place.fileTop,
			ReaderOffset: place.readerOffset, ReaderColumn: place.readerColumn, ReaderCursor: place.readerCursor,
		}
	}
	historyRows := historyInspection.restoredReaderRows
	if rows := readerRowIdentities(historyInspection.readerRows()); len(rows) != 0 {
		historyRows = rows
	}
	sourceFolds := make(map[string]bool)
	for group, collapsed := range m.history.sourceFolds {
		if collapsed {
			sourceFolds[group.identity()] = true
		}
	}

	return session.State{
		Active: workspaceLabel(m.active),
		Controls: session.Controls{
			Files: m.controls.Files.Label(), Reader: m.controls.Reader.Label(),
			Comparison: m.controls.Comparison.Label(), Git: m.controls.Git.Label(),
			Traversal: m.controls.Traversal.Label(), DiffHighlight: m.controls.DiffHighlight.Label(),
		},
		Settings: session.Settings{
			ExcludeCommentsFromHunkNavigation: !m.settings.includeCommentsInHunkNavigation,
			DiffsStartUnfolded:                !m.settings.diffsStartFolded,
		},
		Layout: session.Layout{
			NavigatorWidth: m.layout.navigatorWidth,
			Customized:     m.layout.customized,
			Swapped:        m.layout.swapped,
			GitSourceWidth: m.gitLayout.sourceWidth, GitSourceCustom: m.gitLayout.sourceCustom,
			GitStashWidth: m.gitLayout.stashWidth, GitStashCustom: m.gitLayout.stashCustom,
			GitFilesSize: m.gitLayout.filesSize, GitFilesCustom: m.gitLayout.filesCustom,
		},
		Files: session.Files{
			Place: placeSession(files.place), ReaderPath: files.readerEntry.Path,
			ReaderRows:           readerRows,
			ContextExpanded:      files.readerContext.defaultExpanded,
			ContextFoldOverrides: files.readerContext.overrides(),
			Folds:                fileFolds, ReviewFull: cloneBools(files.reviewFull),
			MarkdownPreviews: sortedTrueKeys(files.markdownPreviewPaths),
		},
		History: session.History{
			SourcePlace: placeSession(m.history.sourcePlace), TimelinePlace: placeSession(m.history.timelinePlace),
			SelectedSource: m.history.selectedSource, SourceFolds: sourceFolds, Focus: m.history.focus.Label(),
			Inspecting: m.history.inspecting, InspectionOID: m.history.inspectionOID,
			Inspection: session.ChangeInspection{
				Place: placeSession(historyInspection.place), ReaderRows: historyRows,
				ContextExpanded:      historyInspection.readerContext.defaultExpanded,
				ContextFoldOverrides: historyInspection.readerContext.overrides(), ReaderPlaces: historyPlaces,
			},
		},
		Stashes: session.Stashes{
			Place: placeSession(stashes.place), Focus: stashes.focus.Label(),
			Inspection: session.ChangeInspection{
				Place: placeSession(stashes.inspection.place), ReaderRows: stashRows,
				ContextExpanded:      stashes.inspection.readerContext.defaultExpanded,
				ContextFoldOverrides: stashes.inspection.readerContext.overrides(), ReaderPlaces: stashPlaces,
			},
		},
		Notes: session.Notes{
			Scope:    noteScopeLabel(m.note.scope),
			Project:  notePlaceSession(m.note.notesState),
			Worktree: notePlaceSession(m.note.worktree),
		},
	}
}

func (m *Model) restoreSession(state session.State) {
	m.settings.includeCommentsInHunkNavigation = !state.Settings.ExcludeCommentsFromHunkNavigation
	m.settings.diffsStartFolded = !state.Settings.DiffsStartUnfolded
	m.configureDiffContextDefaults()
	m.active = parseWorkspace(state.Active)
	m.controls.Files = parseFileSet(state.Controls.Files)
	m.controls.Reader = parseReaderMode(state.Controls.Reader)
	m.controls.Comparison = parseComparison(state.Controls.Comparison)
	m.controls.Git = parseGitMode(state.Controls.Git)
	m.controls.Traversal = parseTraversal(state.Controls.Traversal)
	m.history.traversal = m.controls.Traversal
	m.controls.DiffHighlight = parseDiffHighlight(state.Controls.DiffHighlight)
	m.layout = layoutState{
		navigatorWidth: max(0, state.Layout.NavigatorWidth),
		customized:     state.Layout.Customized && state.Layout.NavigatorWidth > 0,
		swapped:        state.Layout.Swapped,
	}
	m.gitLayout = gitLayoutState{
		sourceWidth: max(0, state.Layout.GitSourceWidth), sourceCustom: state.Layout.GitSourceCustom && state.Layout.GitSourceWidth > 0,
		stashWidth: max(0, state.Layout.GitStashWidth), stashCustom: state.Layout.GitStashCustom && state.Layout.GitStashWidth > 0,
		filesSize: max(0, state.Layout.GitFilesSize), filesCustom: state.Layout.GitFilesCustom && state.Layout.GitFilesSize > 0,
	}

	m.files.place = placeFromSession(state.Files.Place)
	m.files.readerEntry = repository.Entry{Path: state.Files.ReaderPath}
	m.files.readerMode = m.controls.Reader
	m.files.readerContext.restore(state.Files.ContextExpanded, state.Files.ContextFoldOverrides)
	m.files.restoredReaderRows = append([]string(nil), state.Files.ReaderRows...)
	m.files.reviewFull = cloneBools(state.Files.ReviewFull)
	m.files.markdownPreviewPaths = make(map[string]bool, len(state.Files.MarkdownPreviews))
	for _, path := range state.Files.MarkdownPreviews {
		if path != "" {
			m.files.markdownPreviewPaths[path] = true
		}
	}
	for label, folds := range state.Files.Folds {
		m.files.folds[parseFileSet(label)] = filetree.NewFoldState(folds.Known, folds.Collapsed)
	}

	m.history.sourcePlace = placeFromSession(state.History.SourcePlace)
	m.history.timelinePlace = placeFromSession(state.History.TimelinePlace)
	m.history.selectedSource = state.History.SelectedSource
	if oid, ok := m.history.timelinePlace.SelectedIdentity(); ok {
		m.history.preferredOID = oid
	}
	m.history.sourceFolds = make(map[historySourceGroup]bool)
	for _, group := range historySourceGroups {
		m.history.sourceFolds[group] = state.History.SourceFolds[group.identity()]
	}
	m.history.focus = parseGitFocus(state.History.Focus, workspace.GitSource)
	m.history.inspecting = state.History.Inspecting && state.History.InspectionOID != ""
	m.history.inspectionOID = state.History.InspectionOID
	m.history.inspection.place = placeFromSession(state.History.Inspection.Place)
	m.history.inspection.readerContext.restore(state.History.Inspection.ContextExpanded, state.History.Inspection.ContextFoldOverrides)
	m.history.inspection.restoredReaderRows = append([]string(nil), state.History.Inspection.ReaderRows...)
	m.history.inspection.readerPlaces = restoreChangePlaces(state.History.Inspection.ReaderPlaces)
	if m.history.inspecting {
		m.history.focus = parseGitFocus(state.History.Focus, workspace.GitFiles)
		m.history.inspection.ownerID = m.history.inspectionOID
		m.history.inspection.filesGeneration++
		m.history.inspection.filesLoading = true
	}
	m.stashes.place = placeFromSession(state.Stashes.Place)
	m.stashes.focus = parseGitFocus(state.Stashes.Focus, workspace.GitStash)
	m.stashes.inspection.place = placeFromSession(state.Stashes.Inspection.Place)
	m.stashes.inspection.readerContext.restore(state.Stashes.Inspection.ContextExpanded, state.Stashes.Inspection.ContextFoldOverrides)
	m.stashes.inspection.restoredReaderRows = append([]string(nil), state.Stashes.Inspection.ReaderRows...)
	m.stashes.inspection.readerPlaces = restoreChangePlaces(state.Stashes.Inspection.ReaderPlaces)
	if oid, ok := m.stashes.place.SelectedIdentity(); ok {
		m.stashes.inspection.ownerID = oid
	}

	m.note.scope = m.note.normalize(parseNoteScope(state.Notes.Scope))
	restoreNotePlace(&m.note.notesState, state.Notes.Project)
	if m.note.worktreeEnabled {
		restoreNotePlace(&m.note.worktree, state.Notes.Worktree)
	}
	if m.active == workspace.Notes {
		note := m.note.current()
		note.loadGeneration++
		note.loading = true
	} else if m.gitStashesActive() {
		m.stashes.listGeneration++
		m.stashes.listLoading = true
	}
}

func placeSession(place navigation.State) session.Place {
	focus := "navigator"
	if place.Focus == navigation.FocusReader {
		focus = "reader"
	}
	return session.Place{
		Items: append([]string(nil), place.Items...), Selected: place.Selected,
		Top: place.Top, Focus: focus, ReaderOffset: place.ReaderOffset,
		ReaderColumn: place.ReaderColumn, ReaderCursor: place.ReaderCursor,
	}
}

func placeFromSession(place session.Place) navigation.State {
	focus := navigation.FocusNavigator
	if place.Focus == "reader" {
		focus = navigation.FocusReader
	}
	return navigation.State{
		Items: append([]string(nil), place.Items...), Selected: max(0, place.Selected),
		Top: max(0, place.Top), Focus: focus, ReaderOffset: max(0, place.ReaderOffset),
		ReaderColumn: max(0, place.ReaderColumn), ReaderCursor: max(0, place.ReaderCursor),
	}
}

func notePlaceSession(state notesState) session.NotePlace {
	if state.restoredPlace != nil {
		return notePlaceToSession(*state.restoredPlace)
	}
	if !state.loaded {
		return session.NotePlace{}
	}
	return notePlaceToSession(state.editor.Place())
}

func notePlaceToSession(place notes.Place) session.NotePlace {
	return session.NotePlace{
		Valid: true, Cursor: place.Cursor, Anchor: place.Anchor,
		PreferredCol: place.PreferredCol, Scroll: place.Scroll,
	}
}

func restoreNotePlace(state *notesState, place session.NotePlace) {
	if !place.Valid {
		return
	}
	restored := notes.Place{
		Cursor: max(0, place.Cursor), Anchor: place.Anchor,
		PreferredCol: place.PreferredCol, Scroll: max(0, place.Scroll),
	}
	state.restoredPlace = &restored
}

func cloneBools(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		if value {
			result[key] = true
		}
	}
	return result
}

func sortedTrueKeys(source map[string]bool) []string {
	result := make([]string, 0, len(source))
	for key, value := range source {
		if value {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}

func cloneChangePlaces(source map[string]changeReaderPlace) map[string]changeReaderPlace {
	result := make(map[string]changeReaderPlace, len(source))
	for oid, place := range source {
		result[oid] = place
	}
	return result
}

func restoreChangePlaces(source map[string]session.ChangeReaderPlace) map[string]changeReaderPlace {
	result := make(map[string]changeReaderPlace, len(source))
	for oid, place := range source {
		result[oid] = changeReaderPlace{
			fileIdentity: place.FileIdentity, fileTop: max(0, place.FileTop),
			readerOffset: max(0, place.ReaderOffset), readerColumn: max(0, place.ReaderColumn), readerCursor: max(0, place.ReaderCursor),
		}
	}
	return result
}

func workspaceLabel(kind workspace.Kind) string {
	switch kind {
	case workspace.Git:
		return "git"
	case workspace.Notes:
		return "notes"
	default:
		return "files"
	}
}

func parseWorkspace(label string) workspace.Kind {
	switch label {
	case "git":
		return workspace.Git
	case "notes":
		return workspace.Notes
	default:
		return workspace.Files
	}
}

func parseFileSet(label string) workspace.FileSet {
	if label == "changed" {
		return workspace.ChangedFiles
	}
	return workspace.AllFiles
}

func parseReaderMode(label string) workspace.ReaderMode {
	if label == "diff" {
		return workspace.DiffReader
	}
	return workspace.FileReader
}

func parseComparison(label string) workspace.Comparison {
	switch label {
	case "branch":
		return workspace.Branch
	case "last-turn":
		return workspace.LastTurn
	default:
		return workspace.Uncommitted
	}
}

func parseGitMode(label string) workspace.GitMode {
	if label == "stashes" {
		return workspace.GitStashes
	}
	return workspace.GitHistory
}

func parseGitFocus(label string, fallback workspace.GitFocus) workspace.GitFocus {
	switch label {
	case "timeline":
		return workspace.GitTimeline
	case "stashes":
		return workspace.GitStash
	case "files":
		return workspace.GitFiles
	case "diff":
		return workspace.GitDiff
	case "source":
		return workspace.GitSource
	default:
		return fallback
	}
}

func parseTraversal(label string) workspace.GitTraversal {
	if label == "first-parent" {
		return workspace.GitFirstParent
	}
	return workspace.GitGraph
}

func parseDiffHighlight(label string) workspace.DiffHighlight {
	if label == "background" {
		return workspace.DiffHighlightBackground
	}
	return workspace.DiffHighlightSidebar
}

func noteScopeLabel(scope notes.Scope) string {
	if scope == notes.Worktree {
		return "worktree"
	}
	return "project"
}

func parseNoteScope(label string) notes.Scope {
	if label == "worktree" {
		return notes.Worktree
	}
	return notes.Project
}
