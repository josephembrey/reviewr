package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// handleRootMessage owns completions whose lifetime is the application rather
// than one visible destination.
func (m *Model) handleRootMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case readerContextFrameMsg:
		return true, m.command(m.landReaderContextFrame(msg))
	case sessionSaveDueMsg:
		if m.sessionStore == nil {
			m.sessionPending = false
			return true, nil
		}
		if msg.generation != m.sessionSave {
			return true, scheduleSessionSave(m.sessionSave)
		}
		m.sessionPending = false
		return true, m.command(effect{kind: effectSaveSession, generation: msg.generation, session: m.sessionState()})
	case sessionSavedMsg:
		return true, nil
	case repositoryPollTickMsg:
		return true, m.beginRepositoryPoll()
	case repositoryPolledMsg:
		return true, m.landRepositoryPoll(msg)
	case editorFinishedMsg:
		m.noteRepositoryActivity()
		m.files.editorError = ""
		if msg.err != nil {
			m.files.editorError = "Editor: " + msg.err.Error()
		}
		m.files.invalidateComparisons()
		return true, m.command(m.files.reload(m.controls.Comparison.Label()))
	default:
		return false, nil
	}
}

func (m *Model) handleMinimumSizeInput(msg tea.Msg) (bool, tea.Cmd) {
	if ui.MeetsMinimumSize(m.geometry.Screen.Width, m.geometry.Screen.Height) {
		return false, nil
	}
	switch msg.(type) {
	case tea.KeyPressMsg,
		tea.MouseClickMsg,
		tea.MouseReleaseMsg,
		tea.MouseWheelMsg,
		tea.MouseMotionMsg,
		tea.WindowSizeMsg:
		action, ok := m.route(msg)
		if !ok || !minimumSizeActionAllowed(action.Kind) {
			return true, nil
		}
		m.noteRepositoryActivity()
		return true, m.commandAfterAction(m.apply(action))
	default:
		return false, nil
	}
}

func minimumSizeActionAllowed(kind ActionKind) bool {
	switch kind {
	case Resize, Quit, FinishPaneResize, FinishScrollbarDrag:
		return true
	default:
		return false
	}
}

func (m *Model) landFilesResult(msg tea.Msg) (bool, effect) {
	switch msg := msg.(type) {
	case snapshotLoadedMsg:
		var pending effect
		m.files, pending = m.files.landSnapshot(msg, m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return true, pending
	case reviewStateLoadedMsg:
		var pending effect
		m.files, pending = m.files.landReviewState(msg, m.controls.Reader)
		m.readerViewport = readerViewport{}
		return true, pending
	case reviewDocumentLoadedMsg:
		m.files = m.files.landReviewDocument(msg)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return true, effect{}
	case reviewFileLoadedMsg:
		m.files = m.files.landReviewFile(msg)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return true, effect{}
	case reviewVerifiedMsg:
		var pending effect
		m.files, pending = m.files.landReviewVerified(msg)
		m.readerViewport = readerViewport{}
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return true, pending
	case reviewPersistedMsg:
		var pending effect
		m.files, pending = m.files.landReviewPersisted(msg)
		return true, pending
	case fileLoadedMsg:
		m.files = m.files.landFile(msg)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return true, effect{}
	case diffLoadedMsg:
		m.files = m.files.landDiff(msg)
		m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		return true, effect{}
	default:
		return false, effect{}
	}
}

func (m *Model) landGitResult(msg tea.Msg) (bool, effect) {
	switch msg := msg.(type) {
	case commitsLoadedMsg:
		visibleRows := 0
		if m.active == workspace.Git && m.controls.Git == workspace.GitHistory && !m.history.inspecting {
			visibleRows = m.gitGeometry.ContentRows.Height
		}
		m.history = m.history.landCommits(msg, visibleRows)
		return true, effect{}
	case historySourcesLoadedMsg:
		var pending effect
		visibleRows := 0
		if m.active == workspace.Git && m.controls.Git == workspace.GitHistory && !m.history.inspecting {
			visibleRows = m.gitGeometry.RailRows.Height
		}
		m.history, pending = m.history.landSources(msg, visibleRows)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return true, pending
	case commitFilesLoadedMsg:
		return true, m.history.landInspectionFiles(msg)
	case commitFileLoadedMsg:
		if m.history.inspection.landReader(msg.generation, msg.oid, msg.fileIdentity, msg.document, msg.presentation) {
			if m.active == workspace.Git && m.controls.Git == workspace.GitHistory && m.history.inspecting {
				m.clampDocumentReader(&m.history.inspection.place, m.history.inspection.readerDocument())
			}
		}
		return true, effect{}
	case stashesLoadedMsg:
		var pending effect
		visibleRows := 0
		if m.gitStashesActive() {
			visibleRows = m.gitGeometry.RailRows.Height
		}
		m.stashes, pending = m.stashes.landStashes(msg, visibleRows)
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return true, pending
	case stashFilesLoadedMsg:
		var pending effect
		pending = m.stashes.landFiles(msg)
		if m.gitStashesActive() {
			m.stashes.ensureFileVisible(m.gitGeometry.FilesRows.Height)
		}
		if msg.background {
			pending = tagRepositoryPoll(pending, msg.activity)
		}
		return true, pending
	case stashFileLoadedMsg:
		m.stashes.landReader(msg)
		if m.gitStashesActive() {
			m.clampDocumentReader(&m.stashes.inspection.place, m.stashes.readerDocument())
		}
		return true, effect{}
	default:
		return false, effect{}
	}
}

func (m *Model) landNotesResult(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case notesLoadedMsg:
		m.note.landLoad(msg.scope, msg, m.geometry)
		return true, nil
	case notesSaveDueMsg:
		return true, m.command(m.note.due(msg.scope, msg))
	case notesSavedMsg:
		exit, next := m.note.landSave(msg.scope, msg)
		if exit != notesExitNone {
			next = m.finishNotesExit(exit)
		} else if msg.err != nil && m.active != workspace.Notes && m.note.forScope(msg.scope).modified() {
			// A failed quit-time flush must remain visible and recoverable.
			m.active = workspace.Notes
			m.note.scope = m.note.normalize(msg.scope)
		}
		return true, m.commandAfterAction(next)
	default:
		return false, nil
	}
}

func (m *Model) acceptsBackgroundResult(msg tea.Msg) bool {
	result, ok := msg.(backgroundRepositoryResult)
	if !ok {
		return true
	}
	background, activity := result.repositoryPollContext()
	return m.acceptsRepositoryPoll(background, activity)
}
