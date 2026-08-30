package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (m *Model) showDestination(next workspace.Kind) effect {
	if m.active != workspace.Notes || next == workspace.Notes {
		return m.activate(next)
	}
	if next == workspace.Git {
		return m.requestNotesExit(notesExitGit)
	}
	return m.requestNotesExit(notesExitFiles)
}

func (m *Model) cycleDestination(previous bool) effect {
	next := workspace.Files
	switch m.active {
	case workspace.Files:
		if previous {
			next = workspace.Notes
		} else {
			next = workspace.Git
		}
	case workspace.Git:
		if previous {
			next = workspace.Files
		} else {
			next = workspace.Notes
		}
	case workspace.Notes:
		if previous {
			next = workspace.Git
		}
	}
	return m.showDestination(next)
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
		return m.files.reload(m.controls.Comparison.Label())
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

func (m Model) selectedHistoryOID() string {
	oid, _ := m.history.place.SelectedIdentity()
	return oid
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
