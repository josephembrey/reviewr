// Package app composes the Go foundation's thin Bubble Tea root.
package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

// Source is the exact read-only repository contract consumed by the TUI.
type Source interface {
	Root() string
	ListFiles() ([]string, error)
	ReadFile(path string) repository.File
}

// Model is the Bubble Tea root. Input routing and effects are delegated to
// semantic actions and tagged commands.
type Model struct {
	source Source

	navigation navigation.State
	geometry   ui.Geometry
	reader     repository.File
	readerPath string

	listGeneration    uint64
	contentGeneration uint64
	listLoading       bool
	readerLoading     bool
	listError         error
}

type effectKind uint8

const (
	effectNone effectKind = iota
	effectLoadFiles
	effectLoadContent
	effectQuit
)

type effect struct {
	kind       effectKind
	generation uint64
	path       string
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

// New creates a model whose initial command loads repository files.
func New(source Source) Model {
	return Model{
		source:         source,
		navigation:     navigation.State{Focus: navigation.FocusNavigator},
		listGeneration: 1,
		listLoading:    true,
	}
}

// Init starts the first tagged repository load.
func (m Model) Init() tea.Cmd {
	return m.command(effect{kind: effectLoadFiles, generation: m.listGeneration})
}

// Update routes external input to one semantic action and lands tagged results.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case filesLoadedMsg:
		next, pending := m.landFiles(msg)
		return next, next.command(pending)
	case contentLoadedMsg:
		return m.landContent(msg), nil
	}

	action, ok := routeMessage(msg, m.navigation.Focus, m.geometry, m.navigation.Top, len(m.navigation.Files))
	if !ok {
		return m, nil
	}
	pending := m.apply(action)
	return m, m.command(pending)
}

// View renders from the same stored Geometry used by mouse routing.
func (m Model) View() tea.View {
	listError := ""
	if m.listError != nil {
		listError = m.listError.Error()
	}
	content := ui.Render(ui.Model{
		Geometry:      m.geometry,
		Root:          m.source.Root(),
		Files:         m.navigation.Files,
		Selected:      m.navigation.Selected,
		Top:           m.navigation.Top,
		Focus:         m.navigation.Focus,
		Reader:        m.reader,
		ReaderPath:    m.readerPath,
		ReaderOffset:  m.navigation.ReaderOffset,
		ReaderLoading: m.readerLoading,
		ListLoading:   m.listLoading,
		ListError:     listError,
	})
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
	case Reload:
		m.listGeneration++
		m.listLoading = true
		m.listError = nil
		return effect{kind: effectLoadFiles, generation: m.listGeneration}
	case Resize:
		m.geometry = ui.Calculate(action.Width, action.Height)
		m.navigation.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
		if m.reader.Kind != 0 {
			m.navigation.ClampReader(len(ui.ReaderLines(m.reader)), m.geometry.ReaderRows.Height)
		}
	case ToggleFocus:
		m.navigation.ToggleFocus()
	case FocusNavigator:
		m.navigation.Focus = navigation.FocusNavigator
	case FocusReader:
		m.navigation.Focus = navigation.FocusReader
	case SelectNext:
		if m.navigation.SelectDelta(1, m.geometry.NavigatorRows.Height) {
			return m.requestSelectedContent()
		}
	case SelectPrevious:
		if m.navigation.SelectDelta(-1, m.geometry.NavigatorRows.Height) {
			return m.requestSelectedContent()
		}
	case SelectIndex:
		m.navigation.Focus = navigation.FocusNavigator
		if m.navigation.SelectIndex(action.Index, m.geometry.NavigatorRows.Height) {
			return m.requestSelectedContent()
		}
	case ScrollReader:
		m.navigation.ScrollReader(action.Amount, len(ui.ReaderLines(m.reader)), m.geometry.ReaderRows.Height)
	}
	return effect{}
}

func (m Model) landFiles(msg filesLoadedMsg) (Model, effect) {
	if msg.generation != m.listGeneration {
		return m, effect{}
	}
	m.listLoading = false
	if msg.err != nil {
		m.listError = msg.err
		return m, effect{}
	}
	m.listError = nil
	m.navigation.Reconcile(msg.files)
	m.navigation.EnsureSelectionVisible(m.geometry.NavigatorRows.Height)
	if _, ok := m.navigation.SelectedPath(); !ok {
		m.contentGeneration++
		m.reader = repository.File{}
		m.readerPath = ""
		m.readerLoading = false
		m.navigation.ReaderOffset = 0
		return m, effect{}
	}
	return m, m.requestSelectedContent()
}

func (m Model) landContent(msg contentLoadedMsg) Model {
	selectedPath, ok := m.navigation.SelectedPath()
	if msg.generation != m.contentGeneration || !ok || msg.path != selectedPath || msg.path != m.readerPath {
		return m
	}
	m.reader = msg.file
	m.readerLoading = false
	m.navigation.ClampReader(len(ui.ReaderLines(m.reader)), m.geometry.ReaderRows.Height)
	return m
}

func (m *Model) requestSelectedContent() effect {
	path, ok := m.navigation.SelectedPath()
	if !ok {
		return effect{}
	}
	m.contentGeneration++
	if m.readerPath != path {
		m.reader = repository.File{}
	}
	m.readerPath = path
	m.readerLoading = true
	return effect{kind: effectLoadContent, generation: m.contentGeneration, path: path}
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
	case effectLoadContent:
		source := m.source
		generation := pending.generation
		path := pending.path
		return func() tea.Msg {
			return contentLoadedMsg{generation: generation, path: path, file: source.ReadFile(path)}
		}
	case effectQuit:
		return tea.Quit
	default:
		return nil
	}
}
