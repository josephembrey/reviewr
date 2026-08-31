package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

func abbreviateOID(oid string) string {
	oid = strings.TrimSpace(oid)
	if len(oid) <= 7 {
		return oid
	}
	return oid[:7]
}

func (state historyState) viewModel(geometry ui.GitGeometry, now time.Time) ui.GitModel {
	if state.inspecting {
		return state.inspectionViewModel(geometry)
	}
	sourceRows := state.sourceNavigatorRows()
	commits := append([]repository.Commit(nil), state.commits...)
	title := "history"
	if source, ok := state.selectedSourceValue(); ok {
		title += " · " + source.Label
	}
	title += fmt.Sprintf(" · %d", len(commits))
	if state.listLoading {
		title += " · loading"
	} else if state.listError != nil {
		title += " · error"
	}
	status := ui.Line{Text: "Select a commit to inspect.", Tone: ui.ToneQuiet}
	if commit, ok := state.selectedCommit(); ok {
		status.Text = fmt.Sprintf("%s  %s · %s  %s", commit.ShortOID, commit.Author, ageLabel(now, commit.AuthoredUnix), commit.Subject)
	}
	return ui.GitModel{
		Geometry:         geometry,
		Focus:            state.focus,
		RailTitle:        fmt.Sprintf("sources · %d", len(state.sources)),
		RailRows:         sourceRows,
		RailEmpty:        state.sourceEmpty(),
		RailSelected:     state.sourcePlace.Selected,
		RailTop:          state.sourcePlace.Top,
		TimelineTitle:    title,
		TimelineRows:     append([]commitrow.Row(nil), state.rows...),
		TimelineEmpty:    state.timelineEmpty(),
		TimelineSelected: state.timelinePlace.Selected,
		TimelineTop:      state.timelinePlace.Top,
		Status:           status,
	}
}

func (state historyState) sourceEmpty() ui.Line {
	if state.sourceLoading {
		return ui.Line{Text: "Loading sources…", Tone: ui.ToneQuiet}
	}
	if state.sourceError != nil {
		return ui.Line{Text: "Git error: " + state.sourceError.Error(), Tone: ui.ToneError}
	}
	return ui.Line{Text: "No refs or worktrees", Tone: ui.ToneQuiet}
}

func (state historyState) timelineEmpty() ui.Line {
	if state.listLoading {
		return ui.Line{Text: "Loading history…", Tone: ui.ToneQuiet}
	}
	if state.listError != nil {
		return ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}
	return ui.Line{Text: "No commits reachable from this source.", Tone: ui.ToneQuiet}
}

func (state historyState) inspectionViewModel(geometry ui.GitGeometry) ui.GitModel {
	inspection := state.inspection
	commit, _ := state.inspectionCommit()
	fileRows := changedFileNavigatorRows(inspection.files)
	path := "No file selected"
	if file, ok := inspection.selectedFile(); ok {
		path = changedFileLabel(file)
	}
	readerTitle := fmt.Sprintf("%s · %s", commit.ShortOID, commit.Subject)
	if len(inspection.files) != 0 {
		readerTitle += fmt.Sprintf(" › %d/%d · %s", inspection.place.Selected+1, len(inspection.files), path)
	}
	if inspection.readerLoading {
		readerTitle += " · loading"
	}
	return ui.GitModel{
		Geometry:              geometry,
		Focus:                 state.focus,
		FilesTitle:            fmt.Sprintf("files · %d", len(fileRows)),
		FilesRows:             fileRows,
		FilesEmpty:            inspectionEmpty(inspection),
		FilesSelected:         inspection.place.Selected,
		FilesTop:              inspection.place.Top,
		ReaderTitle:           readerTitle,
		ReaderDocument:        inspection.readerDocument(),
		ReaderContextFoldable: inspection.readerDocument().HasContextFold(),
		ReaderEmpty:           inspectionEmpty(inspection),
		ReaderOffset:          inspection.place.ReaderOffset,
		ReaderColumn:          inspection.place.ReaderColumn,
		ReaderCursor:          inspection.place.ReaderCursor,
	}
}

func inspectionEmpty(state changeInspectionState) ui.Line {
	switch {
	case state.filesLoading:
		return ui.Line{Text: "Loading changed files…", Tone: ui.ToneQuiet}
	case state.filesError != nil:
		return ui.Line{Text: "Git error: " + state.filesError.Error(), Tone: ui.ToneError}
	case len(state.files) == 0:
		return ui.Line{Text: "This commit has no first-parent file changes.", Tone: ui.ToneQuiet}
	case state.readerLoading:
		return ui.Line{Text: "Loading commit diff…", Tone: ui.ToneQuiet}
	default:
		return ui.Line{Text: "Select a file to inspect its diff.", Tone: ui.ToneQuiet}
	}
}

func changedFileNavigatorRows(files []repository.ChangedFile) []ui.NavigatorRow {
	rows := make([]ui.NavigatorRow, len(files))
	for index, file := range files {
		marker, tone := changedFileMarker(file.Kind)
		additions, deletions := ui.FormatLineChanges(ui.LineChanges{Additions: file.Additions, Deletions: file.Deletions})
		suffix := make([]ui.Segment, 0, 2)
		if additions != "" {
			suffix = append(suffix, ui.Segment{Text: " " + additions, Tone: ui.ToneAdded})
		}
		if deletions != "" {
			suffix = append(suffix, ui.Segment{Text: " " + deletions, Tone: ui.ToneRemoved})
		}
		rows[index] = ui.NavigatorRow{
			Identity: file.Identity(), Label: changedFileLabel(file),
			Prefix: []ui.Segment{{Text: " " + marker + " ", Tone: tone}}, Suffix: suffix,
		}
	}
	return rows
}

func changedFileLabel(file repository.ChangedFile) string {
	if file.PreviousPath != "" && file.PreviousPath != file.Path {
		return file.PreviousPath + " → " + file.Path
	}
	return file.Path
}

func changedFileMarker(kind repository.ChangeKind) (string, ui.Tone) {
	switch kind {
	case repository.ChangeAdded, repository.ChangeUntracked:
		return "A", ui.ToneAdded
	case repository.ChangeDeleted:
		return "D", ui.ToneRemoved
	case repository.ChangeRenamed:
		return "R", ui.ToneInfo
	case repository.ChangeCopied:
		return "C", ui.ToneInfo
	default:
		return "M", ui.ToneWarning
	}
}
