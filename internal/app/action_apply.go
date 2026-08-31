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
	case ToggleSecondary, ToggleTertiary, ToggleComparison, ToggleDiffHighlight, ToggleMarkdownPreview:
		return m.applyViewControl(action)
	case OpenEditor:
		return m.openEditor()
	case ToggleReview, ActivateReviewBadge, ToggleReviewBounds, NextReviewGap:
		return m.applyReviewAction(action)
	case ToggleHelp, ToggleSettings:
		m.toggleModal(action.Kind)
	case SelectNextSetting:
		m.settings.selectDelta(1)
	case SelectPreviousSetting:
		m.settings.selectDelta(-1)
	case ToggleSelectedSetting:
		m.settings.toggleSelected()
	case CommentInsert,
		CommentBackspace,
		CommentDelete,
		CommentMoveLeft,
		CommentMoveRight,
		CommentMoveUp,
		CommentMoveDown,
		CommentMoveWordLeft,
		CommentMoveWordRight,
		CommentMoveHome,
		CommentMoveEnd,
		CommentSubmit,
		CommentCancel:
		m.applyCommentAction(action)
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
	case FocusNavigator, FocusReader, FocusGitRegion, EnterGit, BackGit, ActivateGitRow,
		SelectNext, SelectPrevious, SelectIndex, ActivateNavigatorRow:
		return m.applyNavigationAction(action)
	case SelectNextFile,
		SelectPreviousFile,
		ExpandNavigatorSelection,
		CollapseNavigatorSelection,
		ExpandReaderFold,
		CollapseReaderFold,
		ToggleReaderFold,
		SelectNextLandmark,
		SelectPreviousLandmark,
		MoveReaderSelection,
		MoveReaderPage,
		SelectReaderBoundary,
		SelectReaderViewport,
		SelectReaderLine,
		StartVisualLine,
		CancelVisualLine,
		ComposeComment,
		ComposeCommentAtLine,
		SetCommentHover,
		ClearCommentHover,
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
	case ToggleMarkdownPreview:
		if m.active == workspace.Files {
			m.files.toggleMarkdownPreview(m.geometry.ReaderRows)
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
	m.gitLayout.finish()
	m.controls.Git = m.controls.Git.Toggle()
	m.updateGitGeometry()
	if m.controls.Git == workspace.GitStashes && !m.stashes.loaded && !m.stashes.listLoading {
		return m.stashes.reload()
	}
	if m.controls.Git == workspace.GitHistory && !m.history.sourcesLoaded && !m.history.sourceLoading {
		return m.history.reload()
	}
	return effect{}
}

func (m *Model) toggleTertiaryControl() effect {
	if m.active == workspace.Files {
		m.controls.Reader = m.controls.Reader.Toggle()
		return m.files.requestMode(m.controls.Reader)
	}
	if m.active == workspace.Git && m.controls.Git == workspace.GitHistory {
		m.controls.Traversal = m.controls.Traversal.Toggle()
		return m.history.requestCommits(m.controls.Traversal, false)
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
	if m.active == workspace.Git {
		return m.history.reload()
	}
	return m.files.reload(m.controls.Comparison.Label())
}

func (m *Model) applyNavigationAction(action Action) effect {
	if m.active == workspace.Git {
		return m.applyGitNavigationAction(action)
	}
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
	return effect{}
}

func (m *Model) selectNavigationIndex(index int) effect {
	if m.active == workspace.Files {
		m.files.place.Focus = navigation.FocusNavigator
		return m.files.selectIndex(index, m.geometry.NavigatorRows.Height, m.controls.Reader)
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
	return effect{}
}

func (m *Model) applyReaderAction(action Action) effect {
	if m.active == workspace.Git && action.GitFocus != 0 {
		m.setGitFocus(action.GitFocus)
	}
	switch action.Kind {
	case SelectNextFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(1, m.gitGeometry.FilesRows.Height)
		}
		if m.active == workspace.Git && m.history.inspecting && m.history.inspection.selectDelta(1, m.gitGeometry.FilesRows.Height) {
			return m.history.requestSelectedInspectionFile()
		}
	case SelectPreviousFile:
		if m.gitStashesActive() {
			return m.stashes.selectFileDelta(-1, m.gitGeometry.FilesRows.Height)
		}
		if m.active == workspace.Git && m.history.inspecting && m.history.inspection.selectDelta(-1, m.gitGeometry.FilesRows.Height) {
			return m.history.requestSelectedInspectionFile()
		}
	case ExpandNavigatorSelection:
		return m.applyNavigatorExpansion(true)
	case CollapseNavigatorSelection:
		return m.applyNavigatorExpansion(false)
	case ExpandReaderFold:
		return m.setActiveReaderFold(true)
	case CollapseReaderFold:
		return m.setActiveReaderFold(false)
	case ToggleReaderFold:
		m.activePlace().Focus = navigation.FocusReader
		m.selectActiveReaderLine(action.Index)
		return m.toggleActiveReaderFold(action.Identity)
	case SelectNextLandmark:
		m.selectActiveReaderLandmark(1)
	case SelectPreviousLandmark:
		m.selectActiveReaderLandmark(-1)
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
	case StartVisualLine:
		m.files.startVisualSelection(m.files.place.ReaderCursor)
	case CancelVisualLine:
		m.cancelFilesVisualSelection()
	case ComposeComment:
		m.files.beginComment(m.files.place.ReaderCursor, false, m.geometry)
		m.ensureCommentComposerVisible()
	case ComposeCommentAtLine:
		m.files.place.Focus = navigation.FocusReader
		m.selectActiveReaderLine(action.Index)
		m.files.beginComment(m.files.place.ReaderCursor, true, m.geometry)
		m.ensureCommentComposerVisible()
	case SetCommentHover:
		m.files.setCommentHover(action.Index)
	case ClearCommentHover:
		m.files.clearCommentHover()
	case ScrollReader:
		m.scrollActiveReader(action.Amount)
	}
	return effect{}
}

func (m *Model) applyNavigatorExpansion(expand bool) effect {
	if m.active == workspace.Git && m.controls.Git == workspace.GitHistory && !m.history.inspecting {
		m.history.setSourceGroupExpanded(expand, m.gitGeometry.RailRows.Height)
		return effect{}
	}
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
