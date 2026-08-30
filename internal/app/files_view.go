package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (state filesState) navigatorRows() []ui.NavigatorRow {
	treeRows := state.tree.Rows()
	rows := make([]ui.NavigatorRow, len(treeRows))
	for index, row := range treeRows {
		presentation := ui.NavigatorRow{
			Identity:  row.Identity,
			Label:     row.Name,
			Tree:      true,
			Depth:     row.Depth,
			Directory: row.Kind == filetree.Directory,
			Expanded:  row.Expanded,
		}
		if entry, ok := state.entry(row.Path); ok {
			presentation.Status = navigatorStatus(entry.State)
			presentation.Dimmed = entry.State == repository.FileIgnored
			if entry.Changed() && !entry.Binary {
				changes := ui.LineChanges{Additions: entry.Additions, Deletions: entry.Deletions}
				presentation.Changes = &changes
			}
			if comparison, reviewable := state.reviewSnapshot.Comparisons[row.Path]; reviewable && entry.Changed() {
				reviewState := state.reviewAssessment(row.Path, comparison).State
				presentation.Review = &reviewState
			}
		} else if row.Kind == filetree.Directory {
			reviewed, changed := state.directoryReviewProgress(row.Path)
			if changed > 0 {
				presentation.Progress = fmt.Sprintf("%d/%d", reviewed, changed)
			}
		}
		rows[index] = presentation
	}
	return rows
}

func (state filesState) viewModel(geometry ui.Geometry) ui.Model {
	document := state.readerDocument()
	return state.viewModelWithReader(geometry, document, document.HasContextFold())
}

func (state filesState) viewModelWithReader(geometry ui.Geometry, document ui.ReaderDocument, contextFoldable bool) ui.Model {
	footerWarning := state.reviewWarning
	if footerWarning == "" {
		footerWarning = state.comparisonWarning
	}
	return ui.Model{
		Geometry:              geometry,
		NavigatorTitle:        fmt.Sprintf("%d files", state.tree.FileCount()),
		NavigatorRows:         state.navigatorRows(),
		NavigatorEmpty:        state.navigatorEmpty(),
		Selected:              state.place.Selected,
		Top:                   state.place.Top,
		Focus:                 state.place.Focus,
		ReaderTitle:           state.readerTitle(),
		ReaderDocument:        document,
		ReaderContextFoldable: contextFoldable,
		ReaderEmpty:           state.readerEmpty(),
		ReaderOffset:          state.place.ReaderOffset,
		ReaderColumn:          state.place.ReaderColumn,
		FooterWarning:         footerWarning,
	}
}

func (state filesState) navigatorEmpty() ui.Line {
	empty := ui.Line{Text: "No files", Tone: ui.ToneQuiet}
	if state.listLoading {
		empty.Text = "Loading files…"
	} else if state.listError != nil {
		empty = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}
	return empty
}

func (state filesState) readerTitle() string {
	title := "No selection"
	if state.readerEntry.Path != "" {
		title = state.readerEntry.Path
		if state.readerEntry.State == repository.FileRenamed && state.readerEntry.PreviousPath != "" {
			title = state.readerEntry.PreviousPath + " → " + state.readerEntry.Path
		}
		if state.readerMode == workspace.DiffReader {
			title += "  diff"
			title += state.reviewBoundsTitle()
		}
	}
	if (state.readerLoading || state.listLoading) && (state.reader.Kind != 0 || state.diff.Kind != 0 || state.displayedBounds != nil) {
		title += "  refreshing…"
	}
	return title
}

func (state filesState) reviewBoundsTitle() string {
	if state.displayedBounds == nil || state.displayedComparison == nil {
		return ""
	}
	assessment := state.ledger.Assess(*state.displayedComparison)
	switch {
	case assessment.State == review.Updated && state.displayedBounds.Old != state.displayedComparison.Old:
		return "  since reviewed"
	case assessment.State == review.Updated && state.reviewFull[state.readerEntry.Path]:
		return "  full comparison"
	case assessment.State == review.Partial:
		return "  older review gap; full comparison"
	case assessment.State == review.BasisChanged:
		return "  review basis changed; full comparison"
	default:
		return ""
	}
}

func (state filesState) readerEmpty() ui.Line {
	empty := ui.Line{Text: "Select a file to read its current content.", Tone: ui.ToneQuiet}
	if state.readerMode == workspace.DiffReader {
		empty.Text = "Select a file to read its uncommitted diff."
	}
	if state.readerLoading {
		empty.Text = "Loading file…"
		if state.readerMode == workspace.DiffReader {
			empty.Text = "Loading diff…"
		}
	}
	return empty
}

func navigatorStatus(state repository.FileState) ui.NavigatorStatus {
	switch state {
	case repository.FileModified:
		return ui.StatusModified
	case repository.FileAdded:
		return ui.StatusAdded
	case repository.FileDeleted:
		return ui.StatusDeleted
	case repository.FileRenamed:
		return ui.StatusRenamed
	case repository.FileUntracked:
		return ui.StatusUntracked
	case repository.FileIgnored:
		return ui.StatusIgnored
	default:
		return ui.StatusNone
	}
}
