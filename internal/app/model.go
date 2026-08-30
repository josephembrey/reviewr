// Package app composes the Go foundation's thin Bubble Tea root.
package app

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/session"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Source is the exact read-only repository contract consumed by the TUI.
type Source interface {
	Snapshot() (repository.Snapshot, error)
	ReadFile(entry repository.Entry) repository.File
	ReadDiff(entry repository.Entry) repository.Diff
	ListCommits(query repository.CommitQuery) ([]repository.Commit, error)
	ReadCommit(oid string) (repository.CommitSummary, error)
	ListRefSources() ([]repository.RefSource, error)
	ListRefCommits(source repository.RefSource) ([]repository.RefCommit, error)
	ListStashes() ([]repository.Stash, error)
	ListStashFiles(source repository.ChangeSource) ([]repository.ChangedFile, error)
	ReadStashFile(source repository.ChangeSource, file repository.ChangedFile) repository.ChangeDocument
}

// SessionStore persists authored worktree UI state outside the repository.
type SessionStore interface {
	Save(generation uint64, state session.State) error
}

// Model is the Bubble Tea root. Input routing and effects are delegated to
// semantic actions and workspace-scoped transitions.
type Model struct {
	source       Source
	host         herdr.Context
	active       workspace.Kind
	controls     workspace.Controls
	lab          labState
	layout       layoutState
	scrollbar    scrollbarDragState
	geometry     ui.Geometry
	files        filesState
	history      historyState
	refs         refsState
	stashes      stashState
	note         scopedNotesState
	poll         repositoryPollState
	sessionStore SessionStore
	sessionSave  uint64

	reviewStateRoot string
}

type effectKind uint8

const (
	effectNone effectKind = iota
	effectLoadSnapshot
	effectLoadFile
	effectLoadDiff
	effectLoadCommits
	effectLoadCommit
	effectLoadReviewSnapshot
	effectLoadReviewState
	effectLoadReviewDocument
	effectLoadReviewFile
	effectVerifyReview
	effectPersistReview
	effectLoadRefSources
	effectLoadRefCommits
	effectLoadStashes
	effectLoadStashFiles
	effectLoadStashFile
	effectLoadNotes
	effectDebounceNotes
	effectSaveNotes
	effectSaveSession
	effectQuit
)

type effect struct {
	kind        effectKind
	generation  uint64
	identity    string
	entry       repository.Entry
	query       repository.CommitQuery
	refSource   repository.RefSource
	stashSource repository.ChangeSource
	changedFile repository.ChangedFile
	text        string
	session     session.State
	background  bool
	activity    uint64
	notesScope  notes.Scope

	scope            string
	reviewGeneration uint64
	comparison       review.FileComparison
	bounds           review.Bounds
	retained         *string
	delta            review.Delta
	store            *review.Store
	candidates       []review.Candidate
}

type snapshotLoadedMsg struct {
	generation       uint64
	snapshot         repository.Snapshot
	err              error
	reviewGeneration uint64
	reviewSnapshot   review.Snapshot
	reviewErr        error
	reviewCapable    bool
	background       bool
	activity         uint64
}

type reviewSnapshotLoadedMsg struct {
	listGeneration   uint64
	reviewGeneration uint64
	scope            string
	snapshot         review.Snapshot
	err              error
}

type reviewStateLoadedMsg struct {
	ledger  review.Ledger
	store   *review.Store
	warning string
	err     error
}

type reviewDocumentLoadedMsg struct {
	generation   uint64
	entry        repository.Entry
	comparison   review.FileComparison
	bounds       review.Bounds
	document     review.Document
	presentation ui.ReaderDocument
	background   bool
	activity     uint64
}

type reviewFileLoadedMsg struct {
	generation   uint64
	entry        repository.Entry
	comparison   review.FileComparison
	content      review.Content
	document     review.Document
	presentation ui.ReaderDocument
	background   bool
	activity     uint64
}

type reviewVerifiedMsg struct {
	generation uint64
	entry      repository.Entry
	comparison review.FileComparison
	delta      review.Delta
	content    review.Content
}

type reviewPersistedMsg struct {
	delta  review.Delta
	ledger review.Ledger
	err    error
}

type fileLoadedMsg struct {
	generation   uint64
	entry        repository.Entry
	file         repository.File
	presentation ui.ReaderDocument
	background   bool
	activity     uint64
}

type diffLoadedMsg struct {
	generation   uint64
	entry        repository.Entry
	diff         repository.Diff
	presentation ui.ReaderDocument
	background   bool
	activity     uint64
}

type commitsLoadedMsg struct {
	generation uint64
	commits    []repository.Commit
	err        error
	query      repository.CommitQuery
	background bool
	activity   uint64
}

type commitLoadedMsg struct {
	generation uint64
	oid        string
	summary    repository.CommitSummary
	err        error
	background bool
	activity   uint64
}

type notesLoadedMsg struct {
	scope      notes.Scope
	generation uint64
	text       string
	readOnly   bool
	err        error
}

type notesSaveDueMsg struct {
	scope      notes.Scope
	generation uint64
}

type notesSavedMsg struct {
	scope      notes.Scope
	generation uint64
	err        error
}

type sessionSaveDueMsg struct {
	generation uint64
}

type sessionSavedMsg struct {
	generation uint64
	err        error
}

type refSourcesLoadedMsg struct {
	generation uint64
	sources    []repository.RefSource
	err        error
	background bool
	activity   uint64
}

type refCommitsLoadedMsg struct {
	generation uint64
	sourceID   repository.RefSourceID
	commits    []repository.RefCommit
	err        error
	background bool
	activity   uint64
}

type stashesLoadedMsg struct {
	generation uint64
	stashes    []repository.Stash
	err        error
	background bool
	activity   uint64
}

type stashFilesLoadedMsg struct {
	generation uint64
	oid        string
	files      []repository.ChangedFile
	err        error
	background bool
	activity   uint64
}

type stashFileLoadedMsg struct {
	generation   uint64
	oid          string
	fileIdentity string
	document     repository.ChangeDocument
	presentation ui.ReaderDocument
	background   bool
	activity     uint64
}

// New creates a model with both repository browsers ready for their tagged
// startup loads. History is warmed while Files remains the visible workspace.
func New(source Source, host herdr.Context) Model {
	return NewWithNotes(source, host, notes.NewMemoryStore())
}

// NewWithNotes creates a model with an explicit project-scoped Notes store.
func NewWithNotes(source Source, host herdr.Context, store notes.Store) Model {
	return NewWithSession(source, host, store, nil, session.State{})
}

// NewWithNotesScopes creates a model with project and optional linked-
// worktree Notes sessions.
func NewWithNotesScopes(source Source, host herdr.Context, stores notes.Stores) Model {
	return NewWithSessionAndNotesScopes(source, host, stores, nil, session.State{})
}

// NewWithSession creates a model with a restored worktree session and its
// external persistence boundary.
func NewWithSession(source Source, host herdr.Context, notesStore notes.Store, store SessionStore, restored session.State) Model {
	return NewWithSessionAndNotesScopes(source, host, notes.Stores{Project: notesStore}, store, restored)
}

// NewWithSessionAndNotesScopes creates a model with independently scoped Notes
// sessions and restores authored worktree place before the first frame.
func NewWithSessionAndNotesScopes(source Source, host herdr.Context, stores notes.Stores, store SessionStore, restored session.State) Model {
	model := Model{
		source:       source,
		host:         host,
		active:       workspace.Files,
		lab:          newLabState(),
		files:        newFilesState(),
		history:      newHistoryState(),
		refs:         newRefsState(),
		stashes:      newStashState(),
		note:         newScopedNotesState(stores),
		sessionStore: store,
	}
	model.restoreSession(restored)
	return model
}

// NewWithReviewStateRoot injects an application-private state root for tests
// and embeddings. An empty root selects the platform default.
func NewWithReviewStateRoot(source Source, host herdr.Context, root string) Model {
	model := New(source, host)
	model.reviewStateRoot = root
	return model
}

// Init starts both workspace loads concurrently so the first switch to Git is
// a pure place-state change rather than a visible first-visit load.
func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{
		m.command(effect{kind: effectLoadSnapshot, generation: m.files.listGeneration, reviewGeneration: m.files.reviewGeneration, scope: m.controls.Comparison.Label()}),
		m.command(effect{
			kind:       effectLoadCommits,
			generation: m.history.listGeneration,
			query:      commitQuery(m.controls.Traversal, ""),
		}),
	}
	if m.active == workspace.Notes {
		commands = append(commands, m.command(m.note.initialLoad()))
	} else if m.gitRefsActive() {
		commands = append(commands, m.command(effect{kind: effectLoadRefSources, generation: m.refs.sourceGeneration}))
	} else if m.gitStashesActive() {
		commands = append(commands, m.command(effect{kind: effectLoadStashes, generation: m.stashes.listGeneration}))
	}
	if _, ok := m.source.(review.Provider); ok {
		commands = append(commands, m.command(effect{kind: effectLoadReviewState}))
	}
	if _, ok := m.source.(repositoryPoller); ok {
		commands = append(commands, scheduleRepositoryPoll())
	}
	return tea.Batch(commands...)
}

// Update routes external input to one semantic action and lands tagged results.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionSaveDueMsg:
		if msg.generation != m.sessionSave || m.sessionStore == nil {
			return m, nil
		}
		return m, m.command(effect{kind: effectSaveSession, generation: msg.generation, session: m.sessionState()})
	case sessionSavedMsg:
		return m, nil
	case repositoryPollTickMsg:
		return m, m.beginRepositoryPoll()
	case repositoryPolledMsg:
		return m, m.landRepositoryPoll(msg)
	}
	if !ui.MeetsMinimumSize(m.geometry.Screen.Width, m.geometry.Screen.Height) {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseWheelMsg, tea.MouseMotionMsg, tea.WindowSizeMsg:
			action, ok := m.route(msg)
			if !ok || (action.Kind != Resize && action.Kind != Quit && action.Kind != FinishPaneResize && action.Kind != FinishScrollbarDrag) {
				return m, nil
			}
			m.noteRepositoryActivity()
			pending := m.apply(action)
			return m, m.commandAfterAction(pending)
		}
	}
	if handled, command := m.updateLab(msg); handled {
		return m, command
	}
	var pending effect
	switch msg := msg.(type) {
	case snapshotLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.files, pending = m.files.landSnapshot(msg, m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return m, m.command(pending)
	case reviewSnapshotLoadedMsg:
		m.files, pending = m.files.landReviewSnapshot(msg, m.controls.Reader, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case reviewStateLoadedMsg:
		m.files, pending = m.files.landReviewState(msg, m.controls.Reader)
		return m, m.command(pending)
	case reviewDocumentLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.files = m.files.landReviewDocument(msg, m.geometry.ReaderRows.Height)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return m, nil
	case reviewFileLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.files = m.files.landReviewFile(msg, m.geometry.ReaderRows.Height)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return m, nil
	case reviewVerifiedMsg:
		m.files, pending = m.files.landReviewVerified(msg)
		return m, m.command(pending)
	case reviewPersistedMsg:
		m.files, pending = m.files.landReviewPersisted(msg)
		return m, m.command(pending)
	case fileLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.files = m.files.landFile(msg, m.geometry.ReaderRows.Height)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return m, nil
	case diffLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.files = m.files.landDiff(msg, m.geometry.ReaderRows.Height)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return m, nil
	case commitsLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.history, pending = m.history.landCommits(msg, m.geometry.NavigatorRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return m, m.command(pending)
	case commitLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.history = m.history.landSummary(msg, m.geometry.ReaderRows.Height)
		return m, nil
	case notesLoadedMsg:
		m.note.landLoad(msg.scope, msg, m.geometry)
		return m, nil
	case notesSaveDueMsg:
		return m, m.command(m.note.due(msg.scope, msg))
	case notesSavedMsg:
		exit, next := m.note.landSave(msg.scope, msg)
		if exit != notesExitNone {
			next = m.finishNotesExit(exit)
		} else if msg.err != nil && m.active != workspace.Notes && m.note.forScope(msg.scope).modified() {
			// A failed quit-time flush must remain visible and recoverable.
			m.active = workspace.Notes
			m.note.scope = m.note.normalize(msg.scope)
		}
		return m, m.commandAfterAction(next)
	case refSourcesLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.refs, pending = m.refs.landSources(msg, m.geometry.NavigatorRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return m, m.command(pending)
	case refCommitsLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.refs = m.refs.landPreview(msg, m.geometry.ReaderRows.Height)
		return m, nil
	case stashesLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.stashes, pending = m.stashes.landStashes(msg, m.geometry.NavigatorRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return m, m.command(pending)
	case stashFilesLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.stashes, pending = m.stashes.landFiles(msg, m.geometry.ReaderRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return m, m.command(pending)
	case stashFileLoadedMsg:
		if !m.acceptsRepositoryPoll(msg.background, msg.activity) {
			return m, nil
		}
		m.stashes = m.stashes.landReader(msg, m.geometry.ReaderRows.Height)
		m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
		return m, nil
	}

	action, ok := m.route(msg)
	if !ok {
		return m, nil
	}
	m.noteRepositoryActivity()
	pending = m.apply(action)
	return m, m.commandAfterAction(pending)
}

// View renders from the same stored Geometry used by mouse routing.
func (m Model) View() tea.View {
	if !ui.MeetsMinimumSize(m.geometry.Screen.Width, m.geometry.Screen.Height) {
		view := tea.NewView(ui.RenderMinimumSize(m.geometry.Screen.Width, m.geometry.Screen.Height))
		view.AltScreen = true
		view.MouseMode = tea.MouseModeCellMotion
		view.WindowTitle = "reviewr"
		return view
	}
	if view, active := m.labView(); active {
		return view
	}
	var presentation ui.Model
	if m.active == workspace.Notes {
		note := m.note.current()
		status, statusError := note.status()
		presentation = ui.Model{
			Geometry:         m.geometry,
			Notes:            note.presentation(),
			NotesStatus:      status,
			NotesError:       statusError,
			NotesScope:       m.note.scope,
			NotesHasWorktree: m.note.hasWorktree(),
		}
	} else if m.gitStashesActive() {
		presentation = m.stashes.viewModel(m.geometry, time.Now())
	} else if m.gitRefsActive() {
		presentation = m.refs.viewModel(m.geometry)
	} else if m.active == workspace.Git {
		presentation = m.history.viewModel(m.geometry)
	} else {
		presentation = m.files.viewModel(m.geometry)
	}
	presentation.Workspace = m.active
	presentation.DividerDragging = m.layout.dragging
	presentation.Controls = m.presentationControls()
	if presentation.Workspace == workspace.Files {
		summary := m.files.snapshot.Summary()
		presentation.Changes = ui.ChangeSummary{
			Files:     summary.Files,
			Additions: summary.Additions,
			Deletions: summary.Deletions,
			Ready:     m.files.loaded,
		}
	}
	content := ui.Render(presentation)
	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "reviewr"
	return view
}

func (m *Model) apply(action Action) effect {
	switch action.Kind {
	case Quit:
		pending := m.note.requestExit(notesExitQuit)
		if pending.kind != effectNone || m.note.current().saving {
			return pending
		}
		m.note.finishExit()
		return effect{kind: effectQuit}
	case ShowFiles:
		if m.active == workspace.Notes {
			return m.requestNotesExit(notesExitFiles)
		}
		return m.activate(workspace.Files)
	case ShowGit:
		if m.active == workspace.Notes {
			return m.requestNotesExit(notesExitGit)
		}
		return m.activate(workspace.Git)
	case ShowNotes:
		return m.activate(workspace.Notes)
	case CycleDestination:
		switch m.active {
		case workspace.Files:
			return m.activate(workspace.Git)
		case workspace.Git:
			return m.activate(workspace.Notes)
		default:
			return m.requestNotesExit(notesExitFiles)
		}
	case ToggleNotesScope:
		return m.note.toggleScope()
	case SelectProjectNotes:
		return m.note.selectScope(notes.Project)
	case SelectWorktreeNotes:
		return m.note.selectScope(notes.Worktree)
	case ToggleSecondary:
		if m.active == workspace.Git {
			m.scrollbar.finish()
			m.controls.Git = m.controls.Git.Next()
			if m.controls.Git == workspace.GitRefs {
				preferredOID, _ := m.history.place.SelectedIdentity()
				return m.refs.enter(preferredOID)
			}
			if m.controls.Git == workspace.GitStashes && !m.stashes.loaded && !m.stashes.listLoading {
				return m.stashes.reload()
			}
		} else if m.active == workspace.Files {
			m.controls.Files = m.controls.Files.Toggle()
			if m.controls.Files == workspace.AllFiles {
				m.controls.Reader = workspace.FileReader
			}
			return m.files.switchScope(m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height)
		}
	case ToggleTertiary:
		if m.active == workspace.Git {
			if m.controls.Git == workspace.GitLog {
				m.controls.Traversal = m.controls.Traversal.Toggle()
				return m.history.reload(m.controls.Traversal, m.selectedHistoryOID())
			}
		} else if m.active == workspace.Files {
			m.controls.Reader = m.controls.Reader.Toggle()
			return m.files.requestMode(m.controls.Reader)
		}
	case ToggleComparison:
		if m.active == workspace.Files {
			m.controls.Comparison = m.controls.Comparison.Next()
			return m.files.requestComparison(m.controls.Comparison.Label())
		}
	case ToggleDiffHighlight:
		if m.diffHighlightEligible() {
			m.controls.DiffHighlight = m.controls.DiffHighlight.Toggle()
		}
	case ToggleReview:
		if m.active == workspace.Files {
			return m.files.requestReviewToggle(m.files.place.Focus, action.Index)
		}
	case ActivateReviewBadge:
		if m.active == workspace.Files {
			return m.files.requestReviewToggle(navigation.FocusNavigator, action.Index)
		}
	case ToggleReviewBounds:
		if m.active == workspace.Files {
			return m.files.toggleReviewBounds(m.controls.Reader)
		}
	case NextReviewGap:
		if m.active == workspace.Files {
			return m.files.selectNextReviewGap(m.geometry.NavigatorRows.Height, m.controls.Reader)
		}
	case Reload:
		if m.gitStashesActive() {
			return m.stashes.reload()
		}
		if m.gitRefsActive() {
			return m.refs.reload()
		}
		if m.active == workspace.Git {
			return m.history.reload(m.controls.Traversal, m.selectedHistoryOID())
		}
		pending := m.files.reload()
		pending.scope = m.controls.Comparison.Label()
		pending.reviewGeneration = m.files.reviewGeneration
		return pending
	case Resize:
		m.geometry = m.layout.resize(action.Width, action.Height)
		if !ui.MeetsMinimumSize(action.Width, action.Height) {
			m.layout.finishDrag()
			m.scrollbar.finish()
			m.note.finishPointers()
		}
		m.resizeWorkspaceState()
		m.note.resize(m.geometry)
	case StartPaneResize:
		m.scrollbar.finish()
		m.layout.startDrag()
	case ResizePanes:
		if m.layout.dragging {
			m.geometry = m.layout.dragTo(action.Position, m.geometry.Screen.Width, m.geometry.Screen.Height)
			m.resizeWorkspaceState()
		}
	case FinishPaneResize:
		m.layout.finishDrag()
	case SwapPanes:
		m.scrollbar.finish()
		m.geometry = m.layout.swap(m.geometry.Screen.Width, m.geometry.Screen.Height)
		m.resizeWorkspaceState()
	case StartScrollbarDrag:
		m.layout.finishDrag()
		m.scrollbar.start(action.Pane, action.Grab)
		m.dragScrollbarTo(action.Position)
	case DragScrollbar:
		m.dragScrollbarTo(action.Position)
	case FinishScrollbarDrag:
		m.scrollbar.finish()
	case FocusNavigator:
		m.activePlace().Focus = navigation.FocusNavigator
	case FocusReader:
		m.activePlace().Focus = navigation.FocusReader
	case SelectNext:
		if m.active == workspace.Files {
			return m.files.selectDelta(1, m.geometry.NavigatorRows.Height, m.controls.Reader)
		}
		if m.gitStashesActive() {
			return m.stashes.selectStashDelta(1, m.geometry.NavigatorRows.Height)
		}
		if m.gitRefsActive() {
			return m.refs.selectDelta(1, m.geometry.NavigatorRows.Height)
		}
		if m.history.place.SelectDelta(1, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case SelectPrevious:
		if m.active == workspace.Files {
			return m.files.selectDelta(-1, m.geometry.NavigatorRows.Height, m.controls.Reader)
		}
		if m.gitStashesActive() {
			return m.stashes.selectStashDelta(-1, m.geometry.NavigatorRows.Height)
		}
		if m.gitRefsActive() {
			return m.refs.selectDelta(-1, m.geometry.NavigatorRows.Height)
		}
		if m.history.place.SelectDelta(-1, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case SelectIndex:
		if m.active == workspace.Files {
			m.files.place.Focus = navigation.FocusNavigator
			return m.files.selectIndex(action.Index, m.geometry.NavigatorRows.Height, m.controls.Reader)
		}
		if m.gitStashesActive() {
			m.stashes.place.Focus = navigation.FocusNavigator
			return m.stashes.selectStashIndex(action.Index, m.geometry.NavigatorRows.Height)
		}
		if m.gitRefsActive() {
			m.refs.place.Focus = navigation.FocusNavigator
			return m.refs.selectIndex(action.Index, m.geometry.NavigatorRows.Height)
		}
		m.history.place.Focus = navigation.FocusNavigator
		if m.history.place.SelectIndex(action.Index, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case ActivateNavigatorRow:
		if m.active == workspace.Files {
			m.files.place.Focus = navigation.FocusNavigator
			pending := m.files.selectIndex(action.Index, m.geometry.NavigatorRows.Height, m.controls.Reader)
			m.files.toggleSelected(m.geometry.NavigatorRows.Height)
			return pending
		}
		if m.gitStashesActive() {
			m.stashes.place.Focus = navigation.FocusNavigator
			return m.stashes.selectStashIndex(action.Index, m.geometry.NavigatorRows.Height)
		}
		if m.gitRefsActive() {
			m.refs.place.Focus = navigation.FocusNavigator
			return m.refs.selectIndex(action.Index, m.geometry.NavigatorRows.Height)
		}
		m.history.place.Focus = navigation.FocusNavigator
		if m.history.place.SelectIndex(action.Index, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case SelectNextFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(1, m.geometry.ReaderRows.Height)
		}
	case SelectPreviousFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(-1, m.geometry.ReaderRows.Height)
		}
	case ExpandNavigatorSelection:
		if m.active == workspace.Files {
			if kind, ok := m.files.selectedKind(); ok {
				if kind == filetree.Directory {
					m.files.expandSelected(m.geometry.NavigatorRows.Height)
				} else if m.controls.Reader == workspace.DiffReader && m.files.setReaderContextExpanded(true) {
					m.clampDocumentReader(&m.files.place, m.files.readerDocument())
				}
			}
		}
	case CollapseNavigatorSelection:
		if m.active == workspace.Files {
			if kind, ok := m.files.selectedKind(); ok {
				if kind == filetree.Directory {
					m.files.collapseSelected(m.geometry.NavigatorRows.Height)
				} else if m.controls.Reader == workspace.DiffReader && m.files.setReaderContextExpanded(false) {
					m.clampDocumentReader(&m.files.place, m.files.readerDocument())
				}
			}
		}
	case ExpandReaderContext:
		if m.gitStashesActive() {
			if m.stashes.setReaderContextExpanded(true) {
				m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
			}
		} else if m.active == workspace.Files {
			if m.files.setReaderContextExpanded(true) {
				m.clampDocumentReader(&m.files.place, m.files.readerDocument())
			}
		}
	case CollapseReaderContext:
		if m.gitStashesActive() {
			if m.stashes.setReaderContextExpanded(false) {
				m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
			}
		} else if m.active == workspace.Files {
			if m.files.setReaderContextExpanded(false) {
				m.clampDocumentReader(&m.files.place, m.files.readerDocument())
			}
		}
	case ToggleReaderContext:
		if m.gitStashesActive() {
			m.stashes.place.Focus = navigation.FocusReader
			if m.stashes.setReaderContextExpanded(!m.stashes.readerContextExpanded) {
				m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
			}
		} else if m.active == workspace.Files {
			m.files.place.Focus = navigation.FocusReader
			if m.files.setReaderContextExpanded(!m.files.readerContextExpanded) {
				m.clampDocumentReader(&m.files.place, m.files.readerDocument())
			}
		}
	case ScrollReader:
		m.scrollActiveReader(action.Amount)
	default:
		if m.active == workspace.Notes {
			return m.note.apply(action, m.geometry)
		}
	}
	return effect{}
}

func (m *Model) route(msg tea.Msg) (Action, bool) {
	if m.active == workspace.Notes {
		note := m.note.current()
		return routeNotesMessage(msg, m.geometry, note.presentation(), note.editor.Dragging(), note.scrollbarDragging, m.note.hasWorktree())
	}
	place := m.activePlace()
	if layout, ok := m.activeReaderLayout(); ok {
		if action, handled := routeReaderFoldMessage(msg, layout, m.activeReaderVisualOffset()); handled {
			return action, true
		}
	}
	return routeMessageWithRows(msg, place.Focus, m.geometry, m.active, m.presentationControls(), m.layout.dragging, m.scrollbar.active, place.Top, len(place.Items), m.activeReaderVisualOffset(), m.activeReaderLineCount(), m.activeNavigatorRows())
}

func (m *Model) activate(next workspace.Kind) effect {
	if next == m.active {
		return effect{}
	}
	m.layout.finishDrag()
	m.scrollbar.finish()
	m.active = next
	if next == workspace.Notes {
		return m.note.open()
	}
	if next == workspace.Git {
		if m.controls.Git == workspace.GitStashes {
			if !m.stashes.loaded && !m.stashes.listLoading {
				return m.stashes.reload()
			}
			return effect{}
		}
		if m.controls.Git == workspace.GitRefs {
			preferredOID, _ := m.history.place.SelectedIdentity()
			return m.refs.enter(preferredOID)
		}
		if !m.history.loaded && !m.history.listLoading {
			return m.history.reload(m.controls.Traversal, m.selectedHistoryOID())
		}
		return effect{}
	}
	if !m.files.loaded && !m.files.listLoading {
		pending := m.files.reload()
		pending.scope = m.controls.Comparison.Label()
		pending.reviewGeneration = m.files.reviewGeneration
		return pending
	}
	return effect{}
}

func (m *Model) activePlace() *navigation.State {
	if m.gitStashesActive() {
		return &m.stashes.place
	}
	if m.gitRefsActive() {
		return &m.refs.place
	}
	if m.active == workspace.Git {
		return &m.history.place
	}
	return &m.files.place
}

func (m *Model) requestNotesExit(exit notesExit) effect {
	pending := m.note.requestExit(exit)
	if pending.kind != effectNone || m.note.current().saving {
		return pending
	}
	return m.finishNotesExit(exit)
}

func (m *Model) finishNotesExit(exit notesExit) effect {
	m.note.finishExit()
	switch exit {
	case notesExitFiles:
		return m.activate(workspace.Files)
	case notesExitGit:
		return m.activate(workspace.Git)
	case notesExitQuit:
		return effect{kind: effectQuit}
	default:
		return effect{}
	}
}

func (m Model) activeReaderLineCount() int {
	if layout, ok := m.activeReaderLayout(); ok {
		return layout.Total
	}
	if m.gitStashesActive() {
		return len(m.stashes.readerRows())
	}
	if m.gitRefsActive() {
		return len(m.refs.commits)
	}
	if m.active == workspace.Git {
		return len(commitSummaryLines(m.history.summary))
	}
	return len(m.files.readerRows())
}

func (m Model) activeReaderLayout() (ui.ReaderLayout, bool) {
	var document ui.ReaderDocument
	switch {
	case m.gitStashesActive():
		document = m.stashes.readerDocument()
	case m.active == workspace.Files:
		document = m.files.readerDocument()
	default:
		return ui.ReaderLayout{}, false
	}
	if document.Kind == ui.ReaderDocumentNone {
		return ui.ReaderLayout{}, false
	}
	return ui.CalculateReaderLayout(m.geometry.ReaderRows, document), true
}

func (m Model) activeReaderVisualOffset() int {
	place := m.activePlace()
	if layout, ok := m.activeReaderLayout(); ok {
		return layout.VisualOffset(place.ReaderOffset, place.ReaderColumn)
	}
	return place.ReaderOffset
}

func (m *Model) setActiveReaderVisualOffset(offset int) {
	place := m.activePlace()
	if layout, ok := m.activeReaderLayout(); ok {
		maximum := max(0, layout.Total-m.geometry.ReaderRows.Height)
		source, column := layout.SourceOffset(min(max(offset, 0), maximum))
		place.ReaderOffset = source
		place.ReaderColumn = column
		return
	}
	place.ReaderOffset = min(max(offset, 0), max(0, m.activeReaderLineCount()-m.geometry.ReaderRows.Height))
	place.ReaderColumn = 0
}

func (m *Model) scrollActiveReader(delta int) {
	m.setActiveReaderVisualOffset(m.activeReaderVisualOffset() + delta)
}

func (m Model) clampDocumentReader(place *navigation.State, document ui.ReaderDocument) {
	if document.Kind == ui.ReaderDocumentNone {
		place.ClampReader(0, m.geometry.ReaderRows.Height)
		return
	}
	layout := ui.CalculateReaderLayout(m.geometry.ReaderRows, document)
	maximum := max(0, layout.Total-m.geometry.ReaderRows.Height)
	source, column := layout.SourceOffset(min(layout.VisualOffset(place.ReaderOffset, place.ReaderColumn), maximum))
	place.ReaderOffset = source
	place.ReaderColumn = column
}

func (m Model) activeNavigatorRows() []ui.NavigatorRow {
	if m.active != workspace.Files {
		return nil
	}
	return m.files.viewModel(m.geometry).NavigatorRows
}

func (m Model) selectedHistoryOID() string {
	oid, _ := m.history.place.SelectedIdentity()
	return oid
}

func (m *Model) resizeWorkspaceState() {
	m.files.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.history.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.refs.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.stashes.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	if len(m.files.restoredReaderRows) == 0 &&
		(m.files.reader.Kind != 0 || m.files.diff.Kind != 0 || m.files.displayedBounds != nil || m.files.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
	}
	if m.history.summary.OID != "" {
		m.history.place.ClampReader(len(commitSummaryLines(m.history.summary)), m.geometry.ReaderRows.Height)
	}
	m.refs.place.ClampReader(len(m.refs.commits), m.geometry.ReaderRows.Height)
	if len(m.stashes.restoredReaderRows) == 0 &&
		(m.stashes.readerFileID != "" || m.stashes.readerDocument().Kind != ui.ReaderDocumentNone) {
		m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
	}
}

func (m Model) gitRefsActive() bool {
	return m.active == workspace.Git && m.controls.Git == workspace.GitRefs
}

func (m Model) gitStashesActive() bool {
	return m.active == workspace.Git && m.controls.Git == workspace.GitStashes
}

// diffHighlightEligible is the one visibility predicate used to derive input,
// header, mouse, and footer behavior from the rich document actually visible.
func (m Model) diffHighlightEligible() bool {
	if m.active == workspace.Notes {
		return false
	}
	if m.gitStashesActive() {
		return m.stashes.readerDocument().DiffEligible()
	}
	if m.active == workspace.Files && m.controls.Reader == workspace.DiffReader {
		return m.files.readerDocument().DiffEligible()
	}
	return false
}

func (m Model) presentationControls() workspace.Controls {
	controls := m.controls
	controls.RichDiff = m.diffHighlightEligible()
	return controls
}

func (m Model) command(pending effect) tea.Cmd {
	switch pending.kind {
	case effectLoadSnapshot:
		source := m.source
		generation := pending.generation
		reviewGeneration := pending.reviewGeneration
		scope := pending.scope
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			snapshot, err := source.Snapshot()
			message := snapshotLoadedMsg{
				generation: generation, snapshot: snapshot, err: err,
				reviewGeneration: reviewGeneration, background: background, activity: activity,
			}
			provider, ok := source.(review.Provider)
			if err == nil && ok {
				message.reviewCapable = true
				message.reviewSnapshot, message.reviewErr = provider.ReviewComparisons(scope, reviewCandidates(snapshot.Changed()))
			}
			return message
		}
	case effectLoadReviewSnapshot:
		provider, ok := m.source.(review.Provider)
		if !ok {
			return nil
		}
		listGeneration := pending.generation
		reviewGeneration := pending.reviewGeneration
		scope := pending.scope
		candidates := append([]review.Candidate(nil), pending.candidates...)
		return func() tea.Msg {
			snapshot, err := provider.ReviewComparisons(scope, candidates)
			return reviewSnapshotLoadedMsg{listGeneration: listGeneration, reviewGeneration: reviewGeneration, scope: scope, snapshot: snapshot, err: err}
		}
	case effectLoadReviewState:
		provider, ok := m.source.(review.Provider)
		if !ok {
			return nil
		}
		root := m.reviewStateRoot
		return func() tea.Msg {
			identity, err := provider.ReviewRepositoryID()
			if err != nil {
				return reviewStateLoadedMsg{err: err, warning: "review state unavailable; marks will not survive restart"}
			}
			ledger, store, warning := review.OpenStore(identity, root)
			return reviewStateLoadedMsg{ledger: ledger, store: store, warning: warning}
		}
	case effectLoadReviewDocument:
		provider, ok := m.source.(review.Provider)
		if !ok {
			return nil
		}
		generation, entry := pending.generation, pending.entry
		comparison, bounds, retained := pending.comparison, pending.bounds, pending.retained
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			var oldContent review.Content
			if retained != nil && bounds.Old != comparison.Old {
				oldContent = review.Content{Endpoint: bounds.Old, State: review.ContentText, Text: *retained, Size: int64(len(*retained))}
			} else {
				oldContent = provider.ReadReviewContent(comparison.OldSource, bounds.Old)
			}
			newContent := provider.ReadReviewContent(comparison.NewSource, bounds.New)
			document := review.BuildDocument(bounds, oldContent, newContent)
			return reviewDocumentLoadedMsg{
				generation: generation, entry: entry, comparison: comparison, bounds: bounds,
				document: document, presentation: reviewReaderDocument(entry.Path, document), background: background, activity: activity,
			}
		}
	case effectLoadReviewFile:
		provider, ok := m.source.(review.Provider)
		if !ok {
			return nil
		}
		generation, entry, comparison := pending.generation, pending.entry, pending.comparison
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
			oldContent := provider.ReadReviewContent(comparison.OldSource, comparison.Old)
			content := provider.ReadReviewContent(comparison.NewSource, comparison.New)
			document := review.BuildDocument(bounds, oldContent, content)
			return reviewFileLoadedMsg{
				generation: generation, entry: entry, comparison: comparison, content: content, document: document,
				presentation: annotatedReviewFileReaderDocument(content, entry, comparison, document), background: background, activity: activity,
			}
		}
	case effectVerifyReview:
		provider, ok := m.source.(review.Provider)
		if !ok {
			return nil
		}
		generation, entry := pending.generation, pending.entry
		comparison, delta := pending.comparison, pending.delta
		return func() tea.Msg {
			content := provider.ReadReviewContent(comparison.NewSource, comparison.New)
			return reviewVerifiedMsg{generation: generation, entry: entry, comparison: comparison, delta: delta, content: content}
		}
	case effectPersistReview:
		store, delta := pending.store, pending.delta
		return func() tea.Msg {
			if store == nil {
				return reviewPersistedMsg{delta: delta, err: errors.New("review state store unavailable")}
			}
			ledger, err := store.Replay(delta)
			return reviewPersistedMsg{delta: delta, ledger: ledger, err: err}
		}
	case effectLoadFile:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			file := source.ReadFile(entry)
			return fileLoadedMsg{
				generation: generation, entry: entry, file: file, presentation: fileReaderDocument(file, entry),
				background: background, activity: activity,
			}
		}
	case effectLoadDiff:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			diff := source.ReadDiff(entry)
			return diffLoadedMsg{
				generation: generation, entry: entry, diff: diff, presentation: diffReaderDocument(diff),
				background: background, activity: activity,
			}
		}
	case effectLoadCommits:
		source := m.source
		generation := pending.generation
		query := pending.query
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			commits, err := source.ListCommits(query)
			return commitsLoadedMsg{
				generation: generation, commits: commits, err: err, query: query,
				background: background, activity: activity,
			}
		}
	case effectLoadCommit:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			summary, err := source.ReadCommit(oid)
			return commitLoadedMsg{
				generation: generation, oid: oid, summary: summary, err: err,
				background: background, activity: activity,
			}
		}
	case effectLoadNotes:
		scope := pending.notesScope
		store := m.note.forScope(scope).store
		generation := pending.generation
		return func() tea.Msg {
			text, readOnly, err := store.Load()
			return notesLoadedMsg{scope: scope, generation: generation, text: text, readOnly: readOnly, err: err}
		}
	case effectDebounceNotes:
		scope := pending.notesScope
		generation := pending.generation
		return tea.Tick(notesSaveDebounce, func(time.Time) tea.Msg {
			return notesSaveDueMsg{scope: scope, generation: generation}
		})
	case effectSaveNotes:
		scope := pending.notesScope
		store := m.note.forScope(scope).store
		generation := pending.generation
		text := pending.text
		return func() tea.Msg {
			return notesSavedMsg{scope: scope, generation: generation, err: store.Save(text)}
		}
	case effectSaveSession:
		store := m.sessionStore
		generation, state := pending.generation, pending.session
		if store == nil {
			return nil
		}
		return func() tea.Msg {
			return sessionSavedMsg{generation: generation, err: store.Save(generation, state)}
		}
	case effectLoadRefSources:
		source := m.source
		generation := pending.generation
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			sources, err := source.ListRefSources()
			return refSourcesLoadedMsg{
				generation: generation, sources: sources, err: err,
				background: background, activity: activity,
			}
		}
	case effectLoadRefCommits:
		source := m.source
		generation := pending.generation
		refSource := pending.refSource
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			commits, err := source.ListRefCommits(refSource)
			return refCommitsLoadedMsg{
				generation: generation, sourceID: refSource.ID, commits: commits, err: err,
				background: background, activity: activity,
			}
		}
	case effectLoadStashes:
		source := m.source
		generation := pending.generation
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			stashes, err := source.ListStashes()
			return stashesLoadedMsg{
				generation: generation, stashes: stashes, err: err,
				background: background, activity: activity,
			}
		}
	case effectLoadStashFiles:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		stashSource := pending.stashSource
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			files, err := source.ListStashFiles(stashSource)
			return stashFilesLoadedMsg{
				generation: generation, oid: oid, files: files, err: err,
				background: background, activity: activity,
			}
		}
	case effectLoadStashFile:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		stashSource := pending.stashSource
		file := pending.changedFile
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			document := source.ReadStashFile(stashSource, file)
			return stashFileLoadedMsg{
				generation: generation, oid: oid, fileIdentity: file.Identity(),
				document: document, presentation: changeDiffDocument(document),
				background: background, activity: activity,
			}
		}
	case effectQuit:
		return tea.Quit
	default:
		return nil
	}
}

// Shutdown synchronously saves final place, flushes edited notes, and releases
// their OS-backed locks. The final generation supersedes delayed save commands.
func (m *Model) Shutdown() error {
	var sessionErr error
	if m.sessionStore != nil {
		m.sessionSave++
		sessionErr = m.sessionStore.Save(m.sessionSave, m.sessionState())
	}
	return errors.Join(sessionErr, m.note.shutdown())
}

func batchCommands(commands ...tea.Cmd) tea.Cmd {
	filtered := commands[:0]
	for _, command := range commands {
		if command != nil {
			filtered = append(filtered, command)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}
