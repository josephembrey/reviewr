package app

import "github.com/josephembrey/reviewr/internal/workspace"

func (m *Model) applyGitNavigationAction(action Action) effect {
	if action.GitFocus != 0 {
		m.setGitFocus(action.GitFocus)
	}
	switch action.Kind {
	case FocusGitRegion:
		return effect{}
	case EnterGit:
		return m.enterGitRegion()
	case BackGit:
		if m.controls.Git == workspace.GitHistory && m.history.inspecting {
			m.history.leaveInspection()
			m.updateGitGeometry()
			m.resizeWorkspaceState()
		}
		return effect{}
	case ActivateGitRow:
		return m.activateGitRow(action.GitFocus, action.Index)
	case SelectNext:
		return m.moveGitSelection(1)
	case SelectPrevious:
		return m.moveGitSelection(-1)
	}
	return effect{}
}

func (m *Model) setGitFocus(focus workspace.GitFocus) {
	if m.controls.Git == workspace.GitStashes {
		if focus == workspace.GitStash || focus == workspace.GitFiles || focus == workspace.GitDiff {
			m.stashes.focus = focus
		}
		return
	}
	if m.history.inspecting {
		if focus == workspace.GitFiles || focus == workspace.GitDiff {
			m.history.focus = focus
		}
		return
	}
	if focus == workspace.GitSource || focus == workspace.GitTimeline {
		m.history.focus = focus
	}
}

func (m *Model) moveGitSelection(delta int) effect {
	if m.controls.Git == workspace.GitStashes {
		switch m.stashes.focus {
		case workspace.GitStash:
			return m.stashes.selectStashDelta(delta, m.gitGeometry.RailRows.Height)
		case workspace.GitFiles:
			return m.stashes.selectFileDelta(delta, m.gitGeometry.FilesRows.Height)
		case workspace.GitDiff:
			m.moveActiveReaderSelection(delta)
		}
		return effect{}
	}
	if m.history.inspecting {
		if m.history.focus == workspace.GitFiles {
			if m.history.inspection.selectDelta(delta, m.gitGeometry.FilesRows.Height) {
				return m.history.requestSelectedInspectionFile()
			}
		} else {
			m.moveActiveReaderSelection(delta)
		}
		return effect{}
	}
	if m.history.focus == workspace.GitSource {
		return m.history.moveSource(delta, m.gitGeometry.RailRows.Height)
	}
	m.history.moveTimeline(delta, m.gitGeometry.ContentRows.Height)
	return effect{}
}

func (m *Model) activateGitRow(focus workspace.GitFocus, index int) effect {
	m.setGitFocus(focus)
	if m.controls.Git == workspace.GitStashes {
		switch focus {
		case workspace.GitStash:
			return m.stashes.selectStashIndex(index, m.gitGeometry.RailRows.Height)
		case workspace.GitFiles:
			if index != m.stashes.inspection.place.Selected {
				return m.stashes.selectFileIndex(index, m.gitGeometry.FilesRows.Height)
			}
		}
		return effect{}
	}
	if m.history.inspecting {
		if focus == workspace.GitFiles && m.history.inspection.selectIndex(index, m.gitGeometry.FilesRows.Height) {
			return m.history.requestSelectedInspectionFile()
		}
		return effect{}
	}
	if focus == workspace.GitSource {
		return m.history.selectSourceIndex(index, m.gitGeometry.RailRows.Height)
	}
	if focus == workspace.GitTimeline {
		m.history.selectTimelineIndex(index, m.gitGeometry.ContentRows.Height)
	}
	return effect{}
}

func (m *Model) enterGitRegion() effect {
	if m.controls.Git == workspace.GitStashes {
		switch m.stashes.focus {
		case workspace.GitStash:
			m.stashes.focus = workspace.GitFiles
		case workspace.GitFiles:
			m.stashes.focus = workspace.GitDiff
		}
		return effect{}
	}
	if m.history.inspecting {
		if m.history.focus == workspace.GitFiles {
			m.history.focus = workspace.GitDiff
		}
		return effect{}
	}
	if m.history.focus == workspace.GitSource {
		row, ok := m.history.selectedSourceRow()
		if ok && row.source == nil {
			expanded := m.history.sourceFolds[row.group]
			m.history.setSourceGroupExpanded(expanded, m.gitGeometry.RailRows.Height)
		}
		return effect{}
	}
	pending := m.history.enterInspection()
	m.updateGitGeometry()
	m.resizeWorkspaceState()
	return pending
}
