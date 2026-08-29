package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type filesState struct {
	place      navigation.State
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
	state.loaded = true
	state.listLoading = false
	if msg.err != nil {
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	state.place.Reconcile(msg.files)
	state.place.EnsureSelectionVisible(visibleRows)
	if _, ok := state.place.SelectedIdentity(); !ok {
		state.contentGeneration++
		state.reader = repository.File{}
		state.readerPath = ""
		state.readerLoading = false
		state.place.ReaderOffset = 0
		return state, effect{}
	}
	return state, state.requestSelectedContent()
}

func (state filesState) landContent(msg contentLoadedMsg, visibleRows int) filesState {
	selectedPath, ok := state.place.SelectedIdentity()
	if msg.generation != state.contentGeneration || !ok || msg.path != selectedPath || msg.path != state.readerPath {
		return state
	}
	state.reader = msg.file
	state.readerLoading = false
	state.place.ClampReader(len(fileReaderLines(state.reader)), visibleRows)
	return state
}

func (state *filesState) requestSelectedContent() effect {
	path, ok := state.place.SelectedIdentity()
	if !ok {
		return effect{}
	}
	state.contentGeneration++
	if state.readerPath != path {
		state.reader = repository.File{}
	}
	state.readerPath = path
	state.readerLoading = true
	return effect{kind: effectLoadFile, generation: state.contentGeneration, identity: path}
}

func (state filesState) viewModel(geometry ui.Geometry) ui.Model {
	rows := make([]ui.NavigatorRow, len(state.place.Items))
	for index, path := range state.place.Items {
		rows[index] = ui.NavigatorRow{Identity: path, Label: path}
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
		NavigatorTitle: fmt.Sprintf("%d files", len(rows)),
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
