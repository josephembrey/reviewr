package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func renderGit(model Model) string {
	git := model.Git
	if git == nil {
		return ""
	}
	switch git.Geometry.Kind {
	case GitHistoryLayout:
		return renderGitHistory(model, *git)
	case GitCommitLayout:
		return renderGitCommit(model, *git)
	default:
		return renderGitStashes(model, *git)
	}
}

func renderGitHistory(model Model, git GitModel) string {
	rail := renderGitNavigator(model, git.Geometry.Rail, git.Geometry.RailTitle, git.Geometry.RailRows,
		git.RailTitle, git.RailRows, git.RailEmpty, git.RailSelected, git.RailTop, git.Focus == workspace.GitSource)
	timeline := renderGitTimeline(model, git)
	divider := renderDivider(git.Geometry.PrimaryDivider, git.DividerDragging == GitPrimaryDivider)
	return lipgloss.JoinHorizontal(lipgloss.Top, rail, divider, timeline)
}

func renderGitCommit(model Model, git GitModel) string {
	files := renderGitNavigator(model, git.Geometry.Files, git.Geometry.FilesTitle, git.Geometry.FilesRows,
		git.FilesTitle, git.FilesRows, git.FilesEmpty, git.FilesSelected, git.FilesTop, git.Focus == workspace.GitFiles)
	diff := renderGitReader(model, git)
	if git.Geometry.FilesStacked {
		divider := renderHorizontalDivider(git.Geometry.SecondaryDivider, git.DividerDragging == GitSecondaryDivider)
		return lipgloss.JoinVertical(lipgloss.Left, files, divider, diff)
	}
	divider := renderDivider(git.Geometry.SecondaryDivider, git.DividerDragging == GitSecondaryDivider)
	return lipgloss.JoinHorizontal(lipgloss.Top, files, divider, diff)
}

func renderGitStashes(model Model, git GitModel) string {
	rail := renderGitNavigator(model, git.Geometry.Rail, git.Geometry.RailTitle, git.Geometry.RailRows,
		git.RailTitle, git.RailRows, git.RailEmpty, git.RailSelected, git.RailTop, git.Focus == workspace.GitStash)
	files := renderGitNavigator(model, git.Geometry.Files, git.Geometry.FilesTitle, git.Geometry.FilesRows,
		git.FilesTitle, git.FilesRows, git.FilesEmpty, git.FilesSelected, git.FilesTop, git.Focus == workspace.GitFiles)
	diff := renderGitReader(model, git)
	secondary := ""
	if git.Geometry.FilesStacked {
		divider := renderHorizontalDivider(git.Geometry.SecondaryDivider, git.DividerDragging == GitSecondaryDivider)
		secondary = lipgloss.JoinVertical(lipgloss.Left, files, divider, diff)
	} else {
		divider := renderDivider(git.Geometry.SecondaryDivider, git.DividerDragging == GitSecondaryDivider)
		secondary = lipgloss.JoinHorizontal(lipgloss.Top, files, divider, diff)
	}
	primary := renderDivider(git.Geometry.PrimaryDivider, git.DividerDragging == GitPrimaryDivider)
	return lipgloss.JoinHorizontal(lipgloss.Top, rail, primary, secondary)
}

func renderGitNavigator(base Model, surface, titleRect, rowsRect Rect, title string, rows []NavigatorRow, empty Line, selected, top int, focused bool) string {
	model := base
	model.Geometry.Navigator = surface
	model.Geometry.NavigatorTitle = titleRect
	model.Geometry.NavigatorRows = rowsRect
	model.NavigatorTitle = title
	model.NavigatorRows = rows
	model.NavigatorEmpty = empty
	model.Selected = selected
	model.Top = top
	model.Focus = navigation.FocusReader
	if focused {
		model.Focus = navigation.FocusNavigator
	}
	return renderNavigator(model)
}

func renderGitTimeline(base Model, git GitModel) string {
	model := base
	model.Geometry.Reader = git.Geometry.Content
	model.Geometry.ReaderTitle = git.Geometry.ContentTitle
	model.Geometry.ReaderRows = git.Geometry.ContentRows
	model.ReaderTitle = git.TimelineTitle
	model.ReaderCommitRows = git.TimelineRows
	model.ReaderEmpty = git.TimelineEmpty
	model.ReaderOffset = git.TimelineTop
	model.ReaderCursor = git.TimelineSelected
	model.Focus = navigation.FocusNavigator
	if git.Focus == workspace.GitTimeline {
		model.Focus = navigation.FocusReader
	}
	timeline := renderReader(model)
	if git.Geometry.Status.Height == 0 {
		return timeline
	}
	status := fit(renderLine(git.Status), git.Geometry.Status.Width)
	return lipgloss.JoinVertical(lipgloss.Left, timeline, status)
}

func renderGitReader(base Model, git GitModel) string {
	model := base
	model.Geometry.Reader = git.Geometry.Content
	model.Geometry.ReaderTitle = git.Geometry.ContentTitle
	model.Geometry.ReaderRows = git.Geometry.ContentRows
	model.ReaderTitle = git.ReaderTitle
	model.ReaderDocument = git.ReaderDocument
	model.ReaderLayout = git.ReaderLayout
	model.ReaderContextFoldable = git.ReaderContextFoldable
	model.ReaderEmpty = git.ReaderEmpty
	model.ReaderOffset = git.ReaderOffset
	model.ReaderColumn = git.ReaderColumn
	model.ReaderCursor = git.ReaderCursor
	model.ReaderCommitRows = nil
	model.ReaderLines = nil
	model.Focus = navigation.FocusNavigator
	if git.Focus == workspace.GitDiff {
		model.Focus = navigation.FocusReader
	}
	return renderReader(model)
}

func renderGitStatus(text string, tone Tone, width int) string {
	return fit(renderLine(Line{Text: strings.TrimSpace(text), Tone: tone}), width)
}
