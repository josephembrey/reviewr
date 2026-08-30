package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type filesState struct {
	place      navigation.State
	tree       filetree.Tree
	reader     repository.File
	readerPath string

	listGeneration    uint64
	contentGeneration uint64
	loaded            bool
	listLoading       bool
	readerLoading     bool
	listError         error
}

func newFilesState() filesState {
	return filesState{
		place:          navigation.State{Focus: navigation.FocusNavigator},
		tree:           filetree.New(nil),
		listGeneration: 1,
		listLoading:    true,
	}
}

func (state *filesState) reload() effect {
	state.listGeneration++
	state.listLoading = true
	state.listError = nil
	return effect{kind: effectLoadFiles, generation: state.listGeneration}
}

func (state filesState) landFiles(msg filesLoadedMsg, visibleRows int) (filesState, effect) {
	if msg.generation != state.listGeneration {
		return state, effect{}
	}
	firstLoad := !state.loaded
	oldFiles := state.tree.Files()
	state.loaded = true
	state.listLoading = false
	if msg.err != nil {
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	state.tree.Rebuild(msg.files)
	state.place.Reconcile(state.tree.Identities())
	if firstLoad {
		state.selectFirstVisibleFile()
	}
	state.place.EnsureSelectionVisible(visibleRows)
	files := state.tree.Files()
	if state.readerPath == "" {
		if row, ok := state.tree.FirstVisibleFile(); ok {
			state.selectIdentity(row.Identity)
			state.place.EnsureSelectionVisible(visibleRows)
			return state, state.requestFile(row.Path)
		}
		state.clearReader()
		return state, effect{}
	}
	path, ok := navigation.ReconcileIdentity(oldFiles, state.readerPath, files)
	if !ok {
		state.clearReader()
		return state, effect{}
	}
	return state, state.requestFile(path)
}

func (state filesState) landContent(msg contentLoadedMsg, visibleRows int) filesState {
	if msg.generation != state.contentGeneration || msg.path != state.readerPath {
		return state
	}
	state.reader = msg.file
	state.readerLoading = false
	state.place.ClampReader(len(fileReaderLines(state.reader)), visibleRows)
	return state
}

func (state *filesState) requestFile(path string) effect {
	state.contentGeneration++
	if state.readerPath != path {
		state.reader = repository.File{}
	}
	state.readerPath = path
	state.readerLoading = true
	return effect{kind: effectLoadFile, generation: state.contentGeneration, identity: path}
}

func (state *filesState) selectDelta(delta, visibleRows int) effect {
	return state.selectIndex(state.place.Selected+delta, visibleRows)
}

func (state *filesState) selectIndex(index, visibleRows int) effect {
	readerOffset := state.place.ReaderOffset
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	identity, _ := state.place.SelectedIdentity()
	row, ok := state.tree.Row(identity)
	if !ok || row.Kind == filetree.Directory {
		state.place.ReaderOffset = readerOffset
		return effect{}
	}
	return state.requestFile(row.Path)
}

func (state *filesState) expandSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Expand(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state *filesState) collapseSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Collapse(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state *filesState) toggleSelected(visibleRows int) bool {
	identity, ok := state.place.SelectedIdentity()
	if !ok || !state.tree.Toggle(identity) {
		return false
	}
	state.reconcileVisibleRows(visibleRows)
	return true
}

func (state *filesState) reconcileVisibleRows(visibleRows int) {
	state.place.Reconcile(state.tree.Identities())
	state.place.EnsureSelectionVisible(visibleRows)
}

func (state *filesState) selectFirstVisibleFile() {
	row, ok := state.tree.FirstVisibleFile()
	if ok {
		state.selectIdentity(row.Identity)
	}
}

func (state *filesState) selectIdentity(identity string) {
	for index, candidate := range state.place.Items {
		if candidate == identity {
			state.place.Selected = index
			return
		}
	}
}

func (state *filesState) clearReader() {
	state.contentGeneration++
	state.reader = repository.File{}
	state.readerPath = ""
	state.readerLoading = false
	state.place.ReaderOffset = 0
}

func (state filesState) viewModel(geometry ui.Geometry) ui.Model {
	treeRows := state.tree.Rows()
	rows := make([]ui.NavigatorRow, len(treeRows))
	for index, row := range treeRows {
		rows[index] = ui.NavigatorRow{
			Identity:  row.Identity,
			Label:     row.Name,
			Tree:      true,
			Depth:     row.Depth,
			Directory: row.Kind == filetree.Directory,
			Expanded:  row.Expanded,
		}
	}

	emptyNavigator := ui.Line{Text: "No files", Tone: ui.ToneQuiet}
	if state.listLoading {
		emptyNavigator.Text = "Loading files…"
	} else if state.listError != nil {
		emptyNavigator = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}

	readerTitle := "No selection"
	if state.readerPath != "" {
		readerTitle = state.readerPath
	}
	if state.readerLoading && state.reader.Kind != 0 {
		readerTitle += "  refreshing…"
	}
	readerEmpty := ui.Line{Text: "Select a file to read its current content.", Tone: ui.ToneQuiet}
	if state.readerLoading {
		readerEmpty = ui.Line{Text: "Loading file…", Tone: ui.ToneQuiet}
	}

	return ui.Model{
		Geometry:       geometry,
		NavigatorTitle: fmt.Sprintf("%d files", state.tree.FileCount()),
		NavigatorRows:  rows,
		NavigatorEmpty: emptyNavigator,
		Selected:       state.place.Selected,
		Top:            state.place.Top,
		Focus:          state.place.Focus,
		ReaderTitle:    readerTitle,
		ReaderLines:    fileReaderLines(state.reader),
		ReaderEmpty:    readerEmpty,
		ReaderOffset:   state.place.ReaderOffset,
	}
}

func fileReaderLines(file repository.File) []ui.Line {
	switch file.Kind {
	case repository.FileReady:
		if file.Symlink {
			return []ui.Line{{Text: "symlink → " + file.Content}}
		}
		rawLines := ui.SafeContentLines(file.Content)
		lines := make([]ui.Line, len(rawLines))
		for index, line := range rawLines {
			lines[index] = ui.Line{Text: line}
		}
		return lines
	case repository.FileMissing:
		return []ui.Line{{Text: "File is missing from the worktree.", Tone: ui.ToneError}}
	case repository.FileUnreadable:
		detail := ""
		if file.Err != nil {
			detail = ": " + file.Err.Error()
		}
		return []ui.Line{{Text: "File is unreadable" + detail, Tone: ui.ToneError}}
	case repository.FileBinary:
		return []ui.Line{{Text: fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", file.Size), Tone: ui.ToneError}}
	case repository.FileTooLarge:
		return []ui.Line{{Text: fmt.Sprintf("File is too large (%d bytes; limit %d bytes).", file.Size, repository.DefaultMaxFileBytes), Tone: ui.ToneError}}
	default:
		return nil
	}
}
