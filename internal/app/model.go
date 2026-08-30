// Package app composes the Go foundation's thin Bubble Tea root.
package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Source is the exact read-only repository contract consumed by the TUI.
type Source interface {
	ListFiles() ([]string, error)
	ReadFile(path string) repository.File
	WorktreeSummary() (repository.ChangeSummary, error)
	ListCommits() ([]repository.Commit, error)
	ReadCommit(oid string) (repository.CommitSummary, error)
}

// Model is the Bubble Tea root. Input routing and effects are delegated to
// semantic actions and workspace-scoped transitions.
type Model struct {
	source    Source
	host      herdr.Context
	active    workspace.Kind
	scratch   bool
	controls  workspace.Controls
	lab       labState
	layout    layoutState
	scrollbar scrollbarDragState
	geometry  ui.Geometry
	files     filesState
	history   historyState
	summary   summaryState
}

type effectKind uint8

const (
	effectNone effectKind = iota
	effectLoadFiles
	effectLoadFile
	effectLoadSummary
	effectLoadCommits
	effectLoadCommit
	effectQuit
)

type effect struct {
	kind       effectKind
	generation uint64
	identity   string
}

type filesLoadedMsg struct {
	generation uint64
	files      []string
	err        error
}

type contentLoadedMsg struct {
	generation uint64
	path       string
	file       repository.File
}

type summaryLoadedMsg struct {
	generation uint64
	summary    repository.ChangeSummary
	err        error
}

type commitsLoadedMsg struct {
	generation uint64
	commits    []repository.Commit
	err        error
}

type commitLoadedMsg struct {
	generation uint64
	oid        string
	summary    repository.CommitSummary
	err        error
}

// New creates a model with both primary workspaces ready for their tagged
// startup loads. History is warmed while Files remains the visible workspace.
func New(source Source, host herdr.Context) Model {
	return Model{
		source:  source,
		host:    host,
		active:  workspace.Files,
		lab:     newLabState(),
		files:   newFilesState(),
		history: newHistoryState(),
		summary: newSummaryState(),
	}
}

// Init starts both workspace loads concurrently so the first switch to Git is
// a pure place-state change rather than a visible first-visit load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.command(effect{kind: effectLoadFiles, generation: m.files.listGeneration}),
		m.command(effect{kind: effectLoadCommits, generation: m.history.listGeneration}),
		m.command(effect{kind: effectLoadSummary, generation: m.summary.generation}),
	)
}

// Update routes external input to one semantic action and lands tagged results.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !ui.MeetsMinimumSize(m.geometry.Screen.Width, m.geometry.Screen.Height) {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseWheelMsg, tea.MouseMotionMsg, tea.WindowSizeMsg:
			place := m.activePlace()
			action, ok := routeMessage(msg, place.Focus, m.geometry, m.destination(), m.controls, m.layout.dragging, m.scrollbar.active, place.Top, len(place.Items), place.ReaderOffset, m.activeReaderLineCount())
			if !ok || (action.Kind != Resize && action.Kind != Quit && action.Kind != FinishPaneResize && action.Kind != FinishScrollbarDrag) {
				return m, nil
			}
			pending := m.apply(action)
			return m, m.command(pending)
		}
	}
	if handled, command := m.updateLab(msg); handled {
		return m, command
	}
	var pending effect
	switch msg := msg.(type) {
	case filesLoadedMsg:
		m.files, pending = m.files.landFiles(msg, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case contentLoadedMsg:
		m.files = m.files.landContent(msg, m.geometry.ReaderRows.Height)
		return m, nil
	case summaryLoadedMsg:
		m.summary = m.summary.land(msg)
		return m, nil
	case commitsLoadedMsg:
		m.history, pending = m.history.landCommits(msg, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case commitLoadedMsg:
		m.history = m.history.landSummary(msg, m.geometry.ReaderRows.Height)
		return m, nil
	}

	place := m.activePlace()
	action, ok := routeMessage(msg, place.Focus, m.geometry, m.destination(), m.controls, m.layout.dragging, m.scrollbar.active, place.Top, len(place.Items), place.ReaderOffset, m.activeReaderLineCount())
	if !ok {
		return m, nil
	}
	if m.scratch && !allowedInScratch(action.Kind) {
		return m, nil
	}
	pending = m.apply(action)
	command := m.command(pending)
	if action.Kind == Reload {
		command = batchCommands(command, m.command(m.summary.reload()))
	}
	return m, command
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
	if m.scratch {
		presentation = ui.Model{Geometry: m.geometry}
	} else if m.active == workspace.Git {
		presentation = m.history.viewModel(m.geometry)
	} else {
		presentation = m.files.viewModel(m.geometry)
	}
	presentation.Workspace = m.destination()
	presentation.PrimaryWorkspace = m.active
	presentation.DividerDragging = m.layout.dragging
	presentation.Controls = m.controls
	presentation.Changes = ui.ChangeSummary{
		Files:     m.summary.value.Files,
		Additions: m.summary.value.Additions,
		Deletions: m.summary.value.Deletions,
		Ready:     m.summary.loaded,
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
		return effect{kind: effectQuit}
	case ToggleWorkspace:
		m.scrollbar.finish()
		if m.scratch {
			m.scratch = false
			return effect{}
		}
		m.scratch = false
		return m.activate(m.active.Toggle())
	case ToggleScratch:
		m.scrollbar.finish()
		m.scratch = !m.scratch
		if m.scratch {
			m.layout.finishDrag()
		}
	case ShowFiles:
		m.scrollbar.finish()
		return m.activate(workspace.Files)
	case ShowGit:
		m.scrollbar.finish()
		return m.activate(workspace.Git)
	case ShowScratch:
		m.scrollbar.finish()
		m.scratch = true
		m.layout.finishDrag()
	case ToggleSecondary:
		if m.active == workspace.Git {
			m.controls.Git = m.controls.Git.Next()
		} else {
			m.controls.Files = m.controls.Files.Toggle()
			if m.controls.Files == workspace.AllFiles {
				m.controls.Reader = workspace.FileReader
			}
		}
	case ToggleTertiary:
		if m.active == workspace.Git {
			if m.controls.Git == workspace.GitLog {
				m.controls.Traversal = m.controls.Traversal.Toggle()
			}
		} else {
			m.controls.Reader = m.controls.Reader.Toggle()
		}
	case ToggleComparison:
		if m.active == workspace.Files {
			m.controls.Comparison = m.controls.Comparison.Next()
		}
	case Reload:
		if m.active == workspace.Git {
			return m.history.reload()
		}
		return m.files.reload()
	case Resize:
		m.geometry = m.layout.resize(action.Width, action.Height)
		if !ui.MeetsMinimumSize(action.Width, action.Height) {
			m.layout.finishDrag()
			m.scrollbar.finish()
		}
		m.resizeWorkspaceState()
	case StartPaneResize:
		m.scrollbar.finish()
		m.layout.startDrag()
	case ResizePanes:
		if m.layout.dragging {
			m.geometry = m.layout.dragTo(action.Position, m.geometry.Screen.Width, m.geometry.Screen.Height)
		}
	case FinishPaneResize:
		m.layout.finishDrag()
	case StartScrollbarDrag:
		m.layout.finishDrag()
		m.scrollbar.start(action.Pane, action.Grab)
		m.dragScrollbarTo(action.Position)
	case DragScrollbar:
		m.dragScrollbarTo(action.Position)
	case FinishScrollbarDrag:
		m.scrollbar.finish()
	case ToggleFocus:
		m.activePlace().ToggleFocus()
	case FocusNavigator:
		m.activePlace().Focus = navigation.FocusNavigator
	case FocusReader:
		m.activePlace().Focus = navigation.FocusReader
	case SelectNext:
		if m.active == workspace.Files {
			return m.files.selectDelta(1, m.geometry.NavigatorRows.Height)
		}
		if m.history.place.SelectDelta(1, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case SelectPrevious:
		if m.active == workspace.Files {
			return m.files.selectDelta(-1, m.geometry.NavigatorRows.Height)
		}
		if m.history.place.SelectDelta(-1, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case SelectIndex:
		if m.active == workspace.Files {
			m.files.place.Focus = navigation.FocusNavigator
			return m.files.selectIndex(action.Index, m.geometry.NavigatorRows.Height)
		}
		m.history.place.Focus = navigation.FocusNavigator
		if m.history.place.SelectIndex(action.Index, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case ActivateNavigatorRow:
		if m.active == workspace.Files {
			m.files.place.Focus = navigation.FocusNavigator
			pending := m.files.selectIndex(action.Index, m.geometry.NavigatorRows.Height)
			m.files.toggleSelected(m.geometry.NavigatorRows.Height)
			return pending
		}
		m.history.place.Focus = navigation.FocusNavigator
		if m.history.place.SelectIndex(action.Index, m.geometry.NavigatorRows.Height) {
			return m.history.requestSelectedSummary()
		}
	case ExpandDirectory:
		if m.active == workspace.Files {
			m.files.expandSelected(m.geometry.NavigatorRows.Height)
		}
	case CollapseDirectory:
		if m.active == workspace.Files {
			m.files.collapseSelected(m.geometry.NavigatorRows.Height)
		}
	case ScrollReader:
		m.activePlace().ScrollReader(action.Amount, m.activeReaderLineCount(), m.geometry.ReaderRows.Height)
	}
	return effect{}
}

func (m *Model) activate(next workspace.Kind) effect {
	m.scratch = false
	if next == m.active {
		return effect{}
	}
	m.active = next
	if next == workspace.Git {
		if !m.history.loaded && !m.history.listLoading {
			return m.history.reload()
		}
		return effect{}
	}
	if !m.files.loaded && !m.files.listLoading {
		return m.files.reload()
	}
	return effect{}
}

func (m Model) destination() workspace.Kind {
	if m.scratch {
		return workspace.Scratch
	}
	return m.active
}

func allowedInScratch(kind ActionKind) bool {
	switch kind {
	case Quit, ToggleWorkspace, ToggleScratch, ShowFiles, ShowGit, ShowScratch, Resize:
		return true
	default:
		return false
	}
}

func (m *Model) activePlace() *navigation.State {
	if m.active == workspace.Git {
		return &m.history.place
	}
	return &m.files.place
}

func (m Model) activeReaderLineCount() int {
	if m.active == workspace.Git {
		return len(commitSummaryLines(m.history.summary))
	}
	return len(fileReaderLines(m.files.reader))
}

func (m *Model) resizeWorkspaceState() {
	m.files.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	m.history.place.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	if m.files.reader.Kind != 0 {
		m.files.place.ClampReader(len(fileReaderLines(m.files.reader)), m.geometry.ReaderRows.Height)
	}
	if m.history.summary.OID != "" {
		m.history.place.ClampReader(len(commitSummaryLines(m.history.summary)), m.geometry.ReaderRows.Height)
	}
}

func (m Model) command(pending effect) tea.Cmd {
	switch pending.kind {
	case effectLoadFiles:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			files, err := source.ListFiles()
			return filesLoadedMsg{generation: generation, files: files, err: err}
		}
	case effectLoadFile:
		source := m.source
		generation := pending.generation
		path := pending.identity
		return func() tea.Msg {
			return contentLoadedMsg{generation: generation, path: path, file: source.ReadFile(path)}
		}
	case effectLoadSummary:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			summary, err := source.WorktreeSummary()
			return summaryLoadedMsg{generation: generation, summary: summary, err: err}
		}
	case effectLoadCommits:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			commits, err := source.ListCommits()
			return commitsLoadedMsg{generation: generation, commits: commits, err: err}
		}
	case effectLoadCommit:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		return func() tea.Msg {
			summary, err := source.ReadCommit(oid)
			return commitLoadedMsg{generation: generation, oid: oid, summary: summary, err: err}
		}
	case effectQuit:
		return tea.Quit
	default:
		return nil
	}
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
