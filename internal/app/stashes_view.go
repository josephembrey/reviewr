package app

import (
	"fmt"
	"time"

	"github.com/josephembrey/reviewr/internal/ui"
)

func (state stashState) viewModel(geometry ui.GitGeometry, now time.Time) ui.GitModel {
	document := state.readerDocument()
	return state.viewModelWithReader(geometry, now, document, document.HasContextFold(), nil)
}

func (state stashState) viewModelWithReader(geometry ui.GitGeometry, now time.Time, document ui.ReaderDocument, contextFoldable bool, layout *ui.ReaderLayout) ui.GitModel {
	stashRows := state.stashRows(now)
	fileRows := changedFileNavigatorRows(state.inspection.files)
	return ui.GitModel{
		Geometry:              geometry,
		Focus:                 state.focus,
		RailTitle:             state.stashTitle(len(stashRows)),
		RailRows:              stashRows,
		RailEmpty:             state.stashEmpty(len(stashRows)),
		RailSelected:          state.place.Selected,
		RailTop:               state.place.Top,
		FilesTitle:            fmt.Sprintf("files · %d", len(fileRows)),
		FilesRows:             fileRows,
		FilesEmpty:            state.readerEmpty(),
		FilesSelected:         state.inspection.place.Selected,
		FilesTop:              state.inspection.place.Top,
		ReaderTitle:           state.readerTitle(),
		ReaderDocument:        document,
		ReaderLayout:          layout,
		ReaderContextFoldable: contextFoldable,
		ReaderEmpty:           state.readerEmpty(),
		ReaderOffset:          state.inspection.place.ReaderOffset,
		ReaderColumn:          state.inspection.place.ReaderColumn,
		ReaderCursor:          state.inspection.place.ReaderCursor,
	}
}

func (state stashState) stashRows(now time.Time) []ui.NavigatorRow {
	rows := make([]ui.NavigatorRow, len(state.stashes))
	for index, stash := range state.stashes {
		prose := stash.Message
		if stash.Branch != "" {
			prose = stash.Branch + " · " + prose
		}
		if prose == "" {
			prose = "(no message)"
		}
		additions, deletions := ui.FormatLineChanges(ui.LineChanges{Additions: stash.Additions, Deletions: stash.Deletions})
		suffix := []ui.Segment{{Text: fmt.Sprintf("  %df", stash.Files), Tone: ui.ToneQuiet}}
		if additions != "" {
			suffix = append(suffix, ui.Segment{Text: " " + additions, Tone: ui.ToneAdded})
		}
		if deletions != "" {
			suffix = append(suffix, ui.Segment{Text: " " + deletions, Tone: ui.ToneRemoved})
		}
		suffix = append(suffix, ui.Segment{Text: " " + ageLabel(now, stash.Timestamp), Tone: ui.ToneQuiet})
		rows[index] = ui.NavigatorRow{
			Identity: stash.OID,
			Prefix:   []ui.Segment{{Text: stash.Selector + " ", Tone: ui.ToneSpecial}},
			Label:    prose,
			Suffix:   suffix,
		}
	}
	return rows
}

func (state stashState) stashEmpty(rowCount int) ui.Line {
	empty := ui.Line{Text: "No stashes yet.", Tone: ui.ToneQuiet}
	if state.listLoading && rowCount == 0 {
		empty.Text = "Loading stashes…"
	} else if state.listError != nil && rowCount == 0 {
		empty = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}
	return empty
}

func (state stashState) stashTitle(rowCount int) string {
	title := fmt.Sprintf("stashes · %d", rowCount)
	if state.listError != nil && rowCount > 0 {
		title += " · refresh failed"
	}
	return title
}

func (state stashState) readerTitle() string {
	title := "stash diff"
	if stash, ok := state.selectedStash(); ok {
		title = stash.Selector
		if file, selected := state.inspection.selectedFile(); selected {
			title = fmt.Sprintf("%s › %d/%d · %s", stash.Selector, state.inspection.place.Selected+1, len(state.inspection.files), changedFileLabel(file))
		}
	}
	if state.inspection.readerLoading {
		title += " · loading"
	}
	return title
}

func (state stashState) readerEmpty() ui.Line {
	empty := ui.Line{Text: "Select a stash to inspect its files.", Tone: ui.ToneQuiet}
	switch {
	case len(state.stashes) == 0 && state.listLoading:
		empty.Text = "Loading stashes…"
	case len(state.stashes) == 0 && state.listError != nil:
		empty = ui.Line{Text: "Stashes are unavailable: " + state.listError.Error(), Tone: ui.ToneError}
	case len(state.stashes) == 0:
		empty.Text = "No stashes yet."
	case state.inspection.filesLoading:
		empty.Text = "Loading files stored in this stash…"
	case state.inspection.filesError != nil:
		empty = ui.Line{Text: "Stash is no longer available: " + state.inspection.filesError.Error(), Tone: ui.ToneError}
	case len(state.inspection.files) == 0:
		empty.Text = "No files stored in this stash."
	case state.inspection.readerLoading:
		empty.Text = "Loading stash diff…"
	}
	return empty
}

func ageLabel(now time.Time, timestamp int64) string {
	seconds := max(int64(0), now.Unix()-timestamp)
	switch {
	case seconds < 60:
		return "now"
	case seconds < 60*60:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 24*60*60:
		return fmt.Sprintf("%dh", seconds/(60*60))
	case seconds < 7*24*60*60:
		return fmt.Sprintf("%dd", seconds/(24*60*60))
	case seconds < 30*24*60*60:
		return fmt.Sprintf("%dw", seconds/(7*24*60*60))
	case seconds < 365*24*60*60:
		return fmt.Sprintf("%dmo", seconds/(30*24*60*60))
	default:
		return fmt.Sprintf("%dy", seconds/(365*24*60*60))
	}
}
