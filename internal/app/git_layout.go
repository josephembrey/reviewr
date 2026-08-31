package app

import (
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// gitLayoutState owns only authored Git split sizes. Files retains its own
// independent layoutState.
type gitLayoutState struct {
	sourceWidth  int
	stashWidth   int
	filesSize    int
	sourceCustom bool
	stashCustom  bool
	filesCustom  bool
	dragging     ui.GitDividerKind
}

func (m *Model) gitLayoutKind() ui.GitLayoutKind {
	if m.controls.Git == workspace.GitStashes {
		return ui.GitStashesLayout
	}
	if m.history.inspecting {
		return ui.GitCommitLayout
	}
	return ui.GitHistoryLayout
}

func (m *Model) updateGitGeometry() {
	kind := m.gitLayoutKind()
	widths := ui.GitWidths{}
	switch kind {
	case ui.GitHistoryLayout:
		if m.gitLayout.sourceCustom {
			widths.Rail = m.gitLayout.sourceWidth
		}
	case ui.GitCommitLayout:
		if m.gitLayout.filesCustom {
			widths.Files = m.gitLayout.filesSize
		}
	case ui.GitStashesLayout:
		if m.gitLayout.stashCustom {
			widths.Rail = m.gitLayout.stashWidth
		}
		if m.gitLayout.filesCustom {
			widths.Files = m.gitLayout.filesSize
		}
	}
	m.gitGeometry = ui.CalculateGitGeometry(m.geometry, kind, widths)
}

func (state *gitLayoutState) start(divider ui.GitDividerKind) {
	state.dragging = divider
}

func (state *gitLayoutState) finish() {
	state.dragging = ui.GitDividerNone
}

func (m *Model) dragGitDivider(x, y int) {
	switch m.gitLayout.dragging {
	case ui.GitPrimaryDivider:
		width := max(1, x-m.gitGeometry.Base.Body.X)
		if m.gitGeometry.Kind == ui.GitHistoryLayout {
			m.gitLayout.sourceWidth = width
			m.gitLayout.sourceCustom = true
		} else if m.gitGeometry.Kind == ui.GitStashesLayout {
			m.gitLayout.stashWidth = width
			m.gitLayout.stashCustom = true
		}
	case ui.GitSecondaryDivider:
		if m.gitGeometry.FilesStacked {
			m.gitLayout.filesSize = max(1, y-m.gitGeometry.Files.Y)
		} else {
			m.gitLayout.filesSize = max(1, x-m.gitGeometry.Files.X)
		}
		m.gitLayout.filesCustom = true
	default:
		return
	}
	m.updateGitGeometry()
	m.resizeWorkspaceState()
}
