package app

import (
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
	if len(files.reviewDocument.Lines) != 0 {
		readerRows = files.reviewDocument.LineIdentities()
	} else if rows := readerRowIdentities(files.readerRows()); len(rows) != 0 {
		readerRows = rows
	}

	stashes := m.stashes
	stashes.readerPlaces = cloneStashPlaces(m.stashes.readerPlaces)
	stashes.saveReaderPlace()
	stashPlaces := make(map[string]session.StashReaderPlace, len(stashes.readerPlaces))
	for oid, place := range stashes.readerPlaces {
		stashPlaces[oid] = session.StashReaderPlace{
			FileIdentity: place.fileIdentity,
			ReaderOffset: place.readerOffset,
			ReaderColumn: place.readerColumn,
		}
	}
	stashRows := stashes.restoredReaderRows
	if rows := readerRowIdentities(stashes.readerRows()); len(rows) != 0 {
		stashRows = rows
	}

	refRows := m.refs.restoredPreviewRows
	if len(m.refs.commits) != 0 {
		refRows = make([]string, len(m.refs.commits))
		for index, commit := range m.refs.commits {
			refRows[index] = commit.OID
		}
	}

	return session.State{
		Active: workspaceLabel(m.active),
		Controls: session.Controls{
			Files: m.controls.Files.Label(), Reader: m.controls.Reader.Label(),
			Comparison: m.controls.Comparison.Label(), Git: m.controls.Git.Label(),
			Traversal: m.controls.Traversal.Label(), DiffHighlight: m.controls.DiffHighlight.Label(),
		},
		Layout: session.Layout{
			NavigatorWidth: m.layout.navigatorWidth,
			Customized:     m.layout.customized,
			Swapped:        m.layout.swapped,
		},
		Files: session.Files{
			Place: placeSession(files.place), ReaderPath: files.readerEntry.Path,
			ReaderRows: readerRows, ContextExpanded: files.readerContextExpanded,
			Folds: fileFolds, ReviewFull: cloneBools(files.reviewFull),
			ReviewCursor: files.reviewCursor, ReviewAnchor: files.reviewSelectionAnchor,
		},
		History: placeSession(m.history.place),
		Refs:    session.Refs{Place: placeSession(m.refs.place), PreviewRows: refRows},
		Stashes: session.Stashes{
			Place: placeSession(stashes.place), ReaderRows: stashRows,
			ContextExpanded: stashes.readerContextExpanded, ReaderPlaces: stashPlaces,
		},
		Notes: session.Notes{
			Scope:    noteScopeLabel(m.note.scope),
			Project:  notePlaceSession(m.note.notesState),
			Worktree: notePlaceSession(m.note.worktree),
		},
	}
}

func (m *Model) restoreSession(state session.State) {
	m.active = parseWorkspace(state.Active)
	m.controls.Files = parseFileSet(state.Controls.Files)
	m.controls.Reader = parseReaderMode(state.Controls.Reader)
	m.controls.Comparison = parseComparison(state.Controls.Comparison)
	m.controls.Git = parseGitView(state.Controls.Git)
	m.controls.Traversal = parseTraversal(state.Controls.Traversal)
	m.controls.DiffHighlight = parseDiffHighlight(state.Controls.DiffHighlight)
	m.layout = layoutState{
		navigatorWidth: max(0, state.Layout.NavigatorWidth),
		customized:     state.Layout.Customized && state.Layout.NavigatorWidth > 0,
		swapped:        state.Layout.Swapped,
	}

	m.files.place = placeFromSession(state.Files.Place)
	m.files.readerEntry = repository.Entry{Path: state.Files.ReaderPath}
	m.files.readerMode = m.controls.Reader
	m.files.readerContextExpanded = state.Files.ContextExpanded
	m.files.readerContextProgress = readerContextTarget(state.Files.ContextExpanded)
	m.files.restoredReaderRows = append([]string(nil), state.Files.ReaderRows...)
	m.files.reviewFull = cloneBools(state.Files.ReviewFull)
	m.files.reviewCursor = max(0, state.Files.ReviewCursor)
	m.files.reviewSelectionAnchor = max(0, state.Files.ReviewAnchor)
	for label, folds := range state.Files.Folds {
		m.files.folds[parseFileSet(label)] = filetree.NewFoldState(folds.Known, folds.Collapsed)
	}

	m.history.place = placeFromSession(state.History)
	m.refs.place = placeFromSession(state.Refs.Place)
	m.refs.restoredPreviewRows = append([]string(nil), state.Refs.PreviewRows...)
	m.stashes.place = placeFromSession(state.Stashes.Place)
	m.stashes.readerContextExpanded = state.Stashes.ContextExpanded
	m.stashes.readerContextProgress = readerContextTarget(state.Stashes.ContextExpanded)
	m.stashes.restoredReaderRows = append([]string(nil), state.Stashes.ReaderRows...)
	m.stashes.readerPlaces = make(map[string]stashReaderPlace, len(state.Stashes.ReaderPlaces))
	for oid, place := range state.Stashes.ReaderPlaces {
		m.stashes.readerPlaces[oid] = stashReaderPlace{
			fileIdentity: place.FileIdentity,
			readerOffset: max(0, place.ReaderOffset),
			readerColumn: max(0, place.ReaderColumn),
		}
	}
	if oid, ok := m.stashes.place.SelectedIdentity(); ok {
		m.stashes.filesOID = oid
		if place := m.stashes.readerPlaces[oid]; place.fileIdentity != "" {
			m.stashes.readerOID = oid
			m.stashes.readerFileID = place.fileIdentity
		}
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
	} else if m.gitRefsActive() {
		m.refs.entered = true
		m.refs.sourceGeneration++
		m.refs.sourceLoading = true
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
		ReaderColumn: place.ReaderColumn,
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
		ReaderColumn: max(0, place.ReaderColumn),
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

func cloneStashPlaces(source map[string]stashReaderPlace) map[string]stashReaderPlace {
	result := make(map[string]stashReaderPlace, len(source))
	for oid, place := range source {
		result[oid] = place
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

func parseGitView(label string) workspace.GitView {
	switch label {
	case "refs":
		return workspace.GitRefs
	case "stashes":
		return workspace.GitStashes
	default:
		return workspace.GitLog
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
