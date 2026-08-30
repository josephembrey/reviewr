package app

import (
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// apply mutates root-owned state for one semantic action and returns at most
// one follow-up effect. Domain state remains responsible for its transitions.
func (m *Model) apply(action Action) effect {
	switch action.Kind {
	case Quit,
		ShowFiles,
		ShowGit,
		ShowNotes,
		ToggleNotesScope,
		SelectProjectNotes,
		SelectWorktreeNotes:
		return m.applyDestinationAction(action)
	case ToggleSecondary, ToggleTertiary, ToggleComparison, ToggleDiffHighlight:
		return m.applyViewControl(action)
	case ToggleReview, ActivateReviewBadge, ToggleReviewBounds, NextReviewGap:
		return m.applyReviewAction(action)
	case ToggleHelp:
		m.helpOpen = !m.helpOpen
	case Reload:
		return m.reloadActiveWorkspace()
	case Resize,
		StartPaneResize,
		ResizePanes,
		FinishPaneResize,
		SwapPanes,
		StartScrollbarDrag,
		DragScrollbar,
		FinishScrollbarDrag:
		m.applyLayoutAction(action)
	case FocusNavigator, FocusReader, SelectNext, SelectPrevious, SelectIndex, ActivateNavigatorRow:
		return m.applyNavigationAction(action)
	case SelectNextFile,
		SelectPreviousFile,
		ExpandNavigatorSelection,
		CollapseNavigatorSelection,
		ExpandReaderContext,
		CollapseReaderContext,
		ToggleReaderFold,
		SelectNextHunk,
		SelectPreviousHunk,
		MoveReaderSelection,
		MoveReaderPage,
		SelectReaderBoundary,
		SelectReaderViewport,
		SelectReaderLine,
		ScrollReader:
		return m.applyReaderAction(action)
	default:
		if m.active == workspace.Notes {
			return m.note.apply(action, m.geometry)
		}
	}
	return effect{}
}

func (m *Model) applyDestinationAction(action Action) effect {
	switch action.Kind {
	case Quit:
		return m.requestNotesExit(notesExitQuit)
	case ShowFiles:
		return m.showDestination(workspace.Files)
	case ShowGit:
		return m.showDestination(workspace.Git)
	case ShowNotes:
		return m.showDestination(workspace.Notes)
	case ToggleNotesScope:
		return m.note.toggleScope()
	case SelectProjectNotes:
		return m.note.selectScope(notes.Project)
	case SelectWorktreeNotes:
		return m.note.selectScope(notes.Worktree)
	default:
		return effect{}
	}
}

func (m *Model) applyViewControl(action Action) effect {
	switch action.Kind {
	case ToggleSecondary:
		return m.toggleSecondaryControl()
	case ToggleTertiary:
		return m.toggleTertiaryControl()
	case ToggleComparison:
		if m.active == workspace.Files {
			m.controls.Comparison = m.controls.Comparison.Next()
			scope := m.controls.Comparison.Label()
			if pending, cached := m.files.activateComparison(
				scope, m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height,
			); cached {
				return pending
			}
			return m.files.reload(scope)
		}
	case ToggleDiffHighlight:
		if m.diffHighlightEligible() {
			m.controls.DiffHighlight = m.controls.DiffHighlight.Toggle()
		}
	}
	return effect{}
}

func (m *Model) toggleSecondaryControl() effect {
	if m.active == workspace.Files {
		m.controls.Files = m.controls.Files.Toggle()
		if m.controls.Files == workspace.AllFiles {
			m.controls.Reader = workspace.FileReader
		}
		return m.files.switchScope(m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height)
	}
	if m.active != workspace.Git {
		return effect{}
	}
	m.scrollbar.finish()
	m.controls.Git = m.controls.Git.Next()
	if m.controls.Git == workspace.GitRefs {
		preferredOID, _ := m.history.place.SelectedIdentity()
		return m.refs.enter(preferredOID)
	}
	if m.controls.Git == workspace.GitStashes && !m.stashes.loaded && !m.stashes.listLoading {
		return m.stashes.reload()
	}
	return effect{}
}

func (m *Model) toggleTertiaryControl() effect {
	if m.active == workspace.Files {
		m.controls.Reader = m.controls.Reader.Toggle()
		return m.files.requestMode(m.controls.Reader)
	}
	if m.active == workspace.Git && m.controls.Git == workspace.GitLog {
		m.controls.Traversal = m.controls.Traversal.Toggle()
		return m.history.reload(m.controls.Traversal, m.selectedHistoryOID())
	}
	return effect{}
}

func (m *Model) applyReviewAction(action Action) effect {
	if m.active != workspace.Files {
		return effect{}
	}
	switch action.Kind {
	case ToggleReview:
		return m.files.requestReviewToggle(m.files.place.Focus, action.Index)
	case ActivateReviewBadge:
		return m.files.requestReviewToggle(navigation.FocusNavigator, action.Index)
	case ToggleReviewBounds:
		return m.files.toggleReviewBounds(m.controls.Reader)
	case NextReviewGap:
		return m.files.selectNextReviewGap(m.geometry.NavigatorRows.Height, m.controls.Reader)
	default:
		return effect{}
	}
}

func (m *Model) reloadActiveWorkspace() effect {
	if m.gitStashesActive() {
		return m.stashes.reload()
	}
	if m.gitRefsActive() {
		return m.refs.reload()
	}
	if m.active == workspace.Git {
		return m.history.reload(m.controls.Traversal, m.selectedHistoryOID())
	}
	return m.files.reload(m.controls.Comparison.Label())
}

func (m *Model) applyNavigationAction(action Action) effect {
	switch action.Kind {
	case FocusNavigator:
		m.activePlace().Focus = navigation.FocusNavigator
	case FocusReader:
		m.activePlace().Focus = navigation.FocusReader
	case SelectNext:
		return m.selectNavigationDelta(1)
	case SelectPrevious:
		return m.selectNavigationDelta(-1)
	case SelectIndex:
		return m.selectNavigationIndex(action.Index)
	case ActivateNavigatorRow:
		return m.activateNavigatorRow(action.Index)
	}
	return effect{}
}

func (m *Model) selectNavigationDelta(delta int) effect {
	if m.active == workspace.Files {
		return m.files.selectDelta(delta, m.geometry.NavigatorRows.Height, m.controls.Reader)
	}
	if m.gitStashesActive() {
		return m.stashes.selectStashDelta(delta, m.geometry.NavigatorRows.Height)
	}
	if m.gitRefsActive() {
		return m.refs.selectDelta(delta, m.geometry.NavigatorRows.Height)
	}
	if m.history.place.SelectDelta(delta, m.geometry.NavigatorRows.Height) {
		return m.history.requestSelectedSummary()
	}
	return effect{}
}

func (m *Model) selectNavigationIndex(index int) effect {
	if m.active == workspace.Files {
		m.files.place.Focus = navigation.FocusNavigator
		return m.files.selectIndex(index, m.geometry.NavigatorRows.Height, m.controls.Reader)
	}
	if m.gitStashesActive() {
		m.stashes.place.Focus = navigation.FocusNavigator
		return m.stashes.selectStashIndex(index, m.geometry.NavigatorRows.Height)
	}
	if m.gitRefsActive() {
		m.refs.place.Focus = navigation.FocusNavigator
		return m.refs.selectIndex(index, m.geometry.NavigatorRows.Height)
	}
	m.history.place.Focus = navigation.FocusNavigator
	if m.history.place.SelectIndex(index, m.geometry.NavigatorRows.Height) {
		return m.history.requestSelectedSummary()
	}
	return effect{}
}

func (m *Model) activateNavigatorRow(index int) effect {
	if m.active == workspace.Files {
		m.files.place.Focus = navigation.FocusNavigator
		pending := m.files.selectIndex(index, m.geometry.NavigatorRows.Height, m.controls.Reader)
		m.files.toggleSelected(m.geometry.NavigatorRows.Height)
		return pending
	}
	if m.gitStashesActive() {
		m.stashes.place.Focus = navigation.FocusNavigator
		return m.stashes.selectStashIndex(index, m.geometry.NavigatorRows.Height)
	}
	if m.gitRefsActive() {
		m.refs.place.Focus = navigation.FocusNavigator
		return m.refs.selectIndex(index, m.geometry.NavigatorRows.Height)
	}
	m.history.place.Focus = navigation.FocusNavigator
	if m.history.place.SelectIndex(index, m.geometry.NavigatorRows.Height) {
		return m.history.requestSelectedSummary()
	}
	return effect{}
}

func (m *Model) applyReaderAction(action Action) effect {
	switch action.Kind {
	case SelectNextFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(1, m.geometry.ReaderRows.Height)
		}
	case SelectPreviousFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(-1, m.geometry.ReaderRows.Height)
		}
	case ExpandNavigatorSelection:
		return m.applyNavigatorExpansion(true)
	case CollapseNavigatorSelection:
		return m.applyNavigatorExpansion(false)
	case ExpandReaderContext:
		return m.setActiveReaderContextFold(true)
	case CollapseReaderContext:
		return m.setActiveReaderContextFold(false)
	case ToggleReaderFold:
		m.activePlace().Focus = navigation.FocusReader
		m.selectActiveReaderLine(action.Index)
		return m.toggleActiveReaderContextFold(action.Identity)
	case SelectNextHunk:
		m.selectActiveReaderHunk(1)
	case SelectPreviousHunk:
		m.selectActiveReaderHunk(-1)
	case MoveReaderSelection:
		m.moveActiveReaderSelection(action.Amount)
	case MoveReaderPage:
		m.moveActiveReaderPage(action.Amount)
	case SelectReaderBoundary:
		m.selectActiveReaderBoundary(action.Amount > 0)
	case SelectReaderViewport:
		m.selectActiveReaderViewport(action.Amount)
	case SelectReaderLine:
		m.activePlace().Focus = navigation.FocusReader
		m.selectActiveReaderLine(action.Index)
	case ScrollReader:
		m.scrollActiveReader(action.Amount)
	}
	return effect{}
}

func (m *Model) applyNavigatorExpansion(expand bool) effect {
	if m.active != workspace.Files {
		return effect{}
	}
	kind, ok := m.files.selectedKind()
	if !ok {
		return effect{}
	}
	if kind == filetree.Directory {
		if expand {
			m.files.expandSelected(m.geometry.NavigatorRows.Height)
		} else {
			m.files.collapseSelected(m.geometry.NavigatorRows.Height)
		}
		return effect{}
	}
	if m.controls.Reader == workspace.DiffReader {
		return m.setFilesReaderContext(expand)
	}
	return effect{}
}
