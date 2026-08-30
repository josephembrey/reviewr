// Package app composes the Go foundation's thin Bubble Tea root.
package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Source is the exact read-only repository contract consumed by the TUI.
type Source interface {
	Snapshot() (repository.Snapshot, error)
	ReadFile(entry repository.Entry) repository.File
	ReadDiff(entry repository.Entry) repository.Diff
	WorktreeSummary() (repository.ChangeSummary, error)
	ListCommits(query repository.CommitQuery) ([]repository.Commit, error)
	ReadCommit(oid string) (repository.CommitSummary, error)
	ListRefSources() ([]repository.RefSource, error)
	ListRefCommits(source repository.RefSource) ([]repository.RefCommit, error)
	ListStashes() ([]repository.Stash, error)
	ListStashFiles(source repository.ChangeSource) ([]repository.ChangedFile, error)
	ReadStashFile(source repository.ChangeSource, file repository.ChangedFile) repository.ChangeDocument
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
	refs      refsState
	stashes   stashState
	summary   summaryState
}

type effectKind uint8

const (
	effectNone effectKind = iota
	effectLoadSnapshot
	effectLoadFile
	effectLoadDiff
	effectLoadSummary
	effectLoadCommits
	effectLoadCommit
	effectLoadRefSources
	effectLoadRefCommits
	effectLoadStashes
	effectLoadStashFiles
	effectLoadStashFile
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
}

type snapshotLoadedMsg struct {
	generation uint64
	snapshot   repository.Snapshot
	err        error
}

type fileLoadedMsg struct {
	generation uint64
	entry      repository.Entry
	file       repository.File
}

type diffLoadedMsg struct {
	generation uint64
	entry      repository.Entry
	diff       repository.Diff
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
	query      repository.CommitQuery
}

type commitLoadedMsg struct {
	generation uint64
	oid        string
	summary    repository.CommitSummary
	err        error
}

type refSourcesLoadedMsg struct {
	generation uint64
	sources    []repository.RefSource
	err        error
}

type refCommitsLoadedMsg struct {
	generation uint64
	sourceID   repository.RefSourceID
	commits    []repository.RefCommit
	err        error
}

type stashesLoadedMsg struct {
	generation uint64
	stashes    []repository.Stash
	err        error
}

type stashFilesLoadedMsg struct {
	generation uint64
	oid        string
	files      []repository.ChangedFile
	err        error
}

type stashFileLoadedMsg struct {
	generation   uint64
	oid          string
	fileIdentity string
	document     repository.ChangeDocument
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
		refs:    newRefsState(),
		stashes: newStashState(),
		summary: newSummaryState(),
	}
}

// Init starts both workspace loads concurrently so the first switch to Git is
// a pure place-state change rather than a visible first-visit load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.command(effect{kind: effectLoadSnapshot, generation: m.files.listGeneration}),
		m.command(effect{
			kind:       effectLoadCommits,
			generation: m.history.listGeneration,
			query:      commitQuery(workspace.GitGraph, ""),
		}),
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
	case snapshotLoadedMsg:
		m.files, pending = m.files.landSnapshot(msg, m.controls.Files, m.controls.Reader, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case fileLoadedMsg:
		m.files = m.files.landFile(msg, m.geometry.ReaderRows.Height)
		return m, nil
	case diffLoadedMsg:
		m.files = m.files.landDiff(msg, m.geometry.ReaderRows.Height)
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
	case refSourcesLoadedMsg:
		m.refs, pending = m.refs.landSources(msg, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case refCommitsLoadedMsg:
		m.refs = m.refs.landPreview(msg, m.geometry.ReaderRows.Height)
		return m, nil
	case stashesLoadedMsg:
		m.stashes, pending = m.stashes.landStashes(msg, m.geometry.NavigatorRows.Height)
		return m, m.command(pending)
	case stashFilesLoadedMsg:
		m.stashes, pending = m.stashes.landFiles(msg, m.geometry.ReaderRows.Height)
		return m, m.command(pending)
	case stashFileLoadedMsg:
		m.stashes = m.stashes.landReader(msg, m.geometry.ReaderRows.Height)
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
	} else if m.gitStashesActive() {
		presentation = m.stashes.viewModel(m.geometry, time.Now())
	} else if m.gitRefsActive() {
		presentation = m.refs.viewModel(m.geometry)
	} else if m.active == workspace.Git {
		presentation = m.history.viewModel(m.geometry)
	} else {
		presentation = m.files.viewModel(m.geometry)
	}
	presentation.Workspace = m.destination()
	presentation.PrimaryWorkspace = m.active
	presentation.DividerDragging = m.layout.dragging
	presentation.Controls = m.controls
	if presentation.Workspace == workspace.Files {
		presentation.Changes = ui.ChangeSummary{
			Files:     m.summary.value.Files,
			Additions: m.summary.value.Additions,
			Deletions: m.summary.value.Deletions,
			Ready:     m.summary.loaded,
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
			m.scrollbar.finish()
			m.controls.Git = m.controls.Git.Next()
			if m.controls.Git == workspace.GitRefs {
				preferredOID, _ := m.history.place.SelectedIdentity()
				return m.refs.enter(preferredOID)
			}
			if m.controls.Git == workspace.GitStashes && !m.stashes.loaded && !m.stashes.listLoading {
				return m.stashes.reload()
			}
		} else {
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
		} else {
			m.controls.Reader = m.controls.Reader.Toggle()
			return m.files.requestMode(m.controls.Reader)
		}
	case ToggleComparison:
		if m.active == workspace.Files {
			m.controls.Comparison = m.controls.Comparison.Next()
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

func (m Model) activeReaderLineCount() int {
	if m.gitStashesActive() {
		return len(m.stashes.readerLines())
	}
	if m.gitRefsActive() {
		return len(m.refs.commits)
	}
	if m.active == workspace.Git {
		return len(commitSummaryLines(m.history.summary))
	}
	return len(m.files.readerLines())
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
	if m.files.reader.Kind != 0 || m.files.diff.Kind != 0 {
		m.files.place.ClampReader(len(m.files.readerLines()), m.geometry.ReaderRows.Height)
	}
	if m.history.summary.OID != "" {
		m.history.place.ClampReader(len(commitSummaryLines(m.history.summary)), m.geometry.ReaderRows.Height)
	}
	m.refs.place.ClampReader(len(m.refs.commits), m.geometry.ReaderRows.Height)
	if m.stashes.readerFileID != "" {
		m.stashes.place.ClampReader(len(m.stashes.readerLines()), m.geometry.ReaderRows.Height)
	}
}

func (m Model) gitRefsActive() bool {
	return m.active == workspace.Git && m.controls.Git == workspace.GitRefs
}

func (m Model) gitStashesActive() bool {
	return m.active == workspace.Git && m.controls.Git == workspace.GitStashes
}

func (m Model) command(pending effect) tea.Cmd {
	switch pending.kind {
	case effectLoadSnapshot:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			snapshot, err := source.Snapshot()
			return snapshotLoadedMsg{generation: generation, snapshot: snapshot, err: err}
		}
	case effectLoadFile:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		return func() tea.Msg {
			return fileLoadedMsg{generation: generation, entry: entry, file: source.ReadFile(entry)}
		}
	case effectLoadDiff:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		return func() tea.Msg {
			return diffLoadedMsg{generation: generation, entry: entry, diff: source.ReadDiff(entry)}
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
		query := pending.query
		return func() tea.Msg {
			commits, err := source.ListCommits(query)
			return commitsLoadedMsg{generation: generation, commits: commits, err: err, query: query}
		}
	case effectLoadCommit:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		return func() tea.Msg {
			summary, err := source.ReadCommit(oid)
			return commitLoadedMsg{generation: generation, oid: oid, summary: summary, err: err}
		}
	case effectLoadRefSources:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			sources, err := source.ListRefSources()
			return refSourcesLoadedMsg{generation: generation, sources: sources, err: err}
		}
	case effectLoadRefCommits:
		source := m.source
		generation := pending.generation
		refSource := pending.refSource
		return func() tea.Msg {
			commits, err := source.ListRefCommits(refSource)
			return refCommitsLoadedMsg{generation: generation, sourceID: refSource.ID, commits: commits, err: err}
		}
	case effectLoadStashes:
		source := m.source
		generation := pending.generation
		return func() tea.Msg {
			stashes, err := source.ListStashes()
			return stashesLoadedMsg{generation: generation, stashes: stashes, err: err}
		}
	case effectLoadStashFiles:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		stashSource := pending.stashSource
		return func() tea.Msg {
			files, err := source.ListStashFiles(stashSource)
			return stashFilesLoadedMsg{generation: generation, oid: oid, files: files, err: err}
		}
	case effectLoadStashFile:
		source := m.source
		generation := pending.generation
		oid := pending.identity
		stashSource := pending.stashSource
		file := pending.changedFile
		return func() tea.Msg {
			return stashFileLoadedMsg{
				generation: generation, oid: oid, fileIdentity: file.Identity(),
				document: source.ReadStashFile(stashSource, file),
			}
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
