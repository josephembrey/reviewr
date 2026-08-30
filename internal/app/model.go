// Package app composes the Go foundation's thin Bubble Tea root.
package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/session"
	"github.com/josephembrey/reviewr/internal/turn"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Source is the exact read-only repository contract consumed by the TUI.
type Source interface {
	Root() string
	Snapshot(scope string) (repository.Snapshot, error)
	ReadFile(entry repository.Entry) repository.File
	ReadDiff(comparison repository.Comparison, entry repository.Entry) repository.Diff
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
	source         Source
	host           herdr.Context
	active         workspace.Kind
	controls       workspace.Controls
	lab            labState
	layout         layoutState
	scrollbar      scrollbarDragState
	modal          modalKind
	settings       settingsState
	geometry       ui.Geometry
	files          filesState
	history        historyState
	refs           refsState
	stashes        stashState
	note           scopedNotesState
	poll           repositoryPollState
	turn           *turn.Host
	readerViewport readerViewport
	sessionStore   SessionStore
	sessionSave    uint64
	sessionPending bool

	reviewStateRoot string
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
		settings:     newSettingsState(),
		files:        newFilesState(),
		history:      newHistoryState(),
		refs:         newRefsState(),
		stashes:      newStashState(),
		note:         newScopedNotesState(stores),
		sessionStore: store,
	}
	model.restoreSession(restored)
	agents := herdr.NewAgentSampler(host)
	if repo, ok := source.(turn.Repository); ok && agents.Available() {
		model.turn = turn.NewHost(repo, agents)
	}
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
	if handled, command := m.handleRootMessage(msg); handled {
		return m, command
	}
	if handled, command := m.handleMinimumSizeInput(msg); handled {
		return m, command
	}
	if handled, command := m.updateLab(msg); handled {
		return m, command
	}
	if !m.acceptsBackgroundResult(msg) {
		return m, nil
	}
	if handled, pending := m.landFilesResult(msg); handled {
		return m, m.command(pending)
	}
	if handled, pending := m.landGitResult(msg); handled {
		return m, m.command(pending)
	}
	if handled, command := m.landNotesResult(msg); handled {
		return m, command
	}

	action, ok := m.route(msg)
	if !ok {
		return m, nil
	}
	if action.Kind == ActionNone {
		return m, nil
	}
	if action.Kind != SetCommentHover && action.Kind != ClearCommentHover {
		m.noteRepositoryActivity()
	}
	return m, m.commandAfterAction(m.apply(action))
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
			Geometry:            m.geometry,
			Notes:               note.presentation(),
			NotesStatus:         status,
			NotesError:          statusError,
			NotesStatusPriority: statusError || note.readOnly,
			NotesScope:          m.note.scope,
			NotesHasWorktree:    m.note.hasWorktree(),
		}
	} else if m.gitStashesActive() {
		if reader, ok := m.cachedActiveReaderViewport(); ok {
			presentation = m.stashes.viewModelWithReader(m.geometry, time.Now(), reader.document, reader.foldable)
			presentation.ReaderLayout = &reader.layout
		} else {
			presentation = m.stashes.viewModel(m.geometry, time.Now())
		}
	} else if m.gitRefsActive() {
		presentation = m.refs.viewModel(m.geometry)
	} else if m.active == workspace.Git {
		presentation = m.history.viewModel(m.geometry)
	} else {
		if reader, ok := m.cachedActiveReaderViewport(); ok {
			presentation = m.files.viewModelWithReader(m.geometry, reader.document, reader.foldable)
			presentation.ReaderLayout = &reader.layout
		} else {
			presentation = m.files.viewModel(m.geometry)
		}
	}
	presentation.Workspace = m.active
	presentation.DividerDragging = m.layout.dragging
	presentation.HelpOpen = m.modal == modalHelp
	presentation.Settings = m.settings.presentation(m.modal == modalSettings)
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
