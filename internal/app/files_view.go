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
				presentation.Changes = ui.LineChanges{Additions: entry.Additions, Deletions: entry.Deletions}
			}
			if comparison, reviewable := state.reviewSnapshot.Comparisons[row.Path]; reviewable && entry.Changed() {
				presentation.Reviewable = true
				presentation.Review = state.reviewAssessment(row.Path, comparison).State
			}
		}
		if row.Kind == filetree.Directory {
			if state.directoryIgnored[row.Path] {
				presentation.Status = ui.StatusIgnored
				presentation.Dimmed = true
			}
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
	footerWarning := state.editorError
	if footerWarning == "" {
		footerWarning = state.reviewWarning
	}
	if footerWarning == "" {
		footerWarning = state.comparisonWarning
	}
	navigatorRows := state.navigatorRows()
	navigatorTitle := fmt.Sprintf("%d files", state.tree.FileCount())
	navigatorEmpty := state.navigatorEmpty()
	readerEmpty := state.readerEmpty()
	readerOffset := state.place.ReaderOffset
	readerColumn := state.place.ReaderColumn
	if state.comparisonPending() {
		navigatorRows = nil
		navigatorTitle = "0 files"
		document = ui.ReaderDocument{}
		contextFoldable = false
		readerEmpty = ui.Line{Text: "Loading comparison…", Tone: ui.ToneQuiet}
		readerOffset = 0
		readerColumn = 0
	} else if state.readerComparisonPending() {
		document = ui.ReaderDocument{}
		contextFoldable = false
		readerEmpty = ui.Line{Text: "Loading comparison…", Tone: ui.ToneQuiet}
		readerOffset = 0
		readerColumn = 0
	}
	commentHeader, commentExpanded := false, false
	if len(document.Rows) != 0 {
		cursor := max(0, min(state.place.ReaderCursor, len(document.Rows)-1))
		commentHeader = document.Rows[cursor].Kind == ui.ReaderCommentHeader
		commentExpanded = commentHeader && document.Rows[cursor].FoldExpanded
	}
	return ui.Model{
		Geometry:               geometry,
		NavigatorTitle:         navigatorTitle,
		NavigatorRows:          navigatorRows,
		NavigatorEmpty:         navigatorEmpty,
		Selected:               state.place.Selected,
		Top:                    state.place.Top,
		Focus:                  state.place.Focus,
		ReaderTitle:            state.readerTitle(),
		ReaderDocument:         document,
		ReaderContextFoldable:  contextFoldable,
		ReaderEmpty:            readerEmpty,
		ReaderOffset:           readerOffset,
		ReaderColumn:           readerColumn,
		ReaderCursor:           state.place.ReaderCursor,
		FooterWarning:          footerWarning,
		FileActions:            state.fileFooterActions(),
		ReaderVisualSelection:  state.visualSelection != nil,
		ReaderComposingComment: state.composingComment(),
		ReaderCommentHeader:    commentHeader,
		ReaderCommentExpanded:  commentExpanded,
		ReaderCommentable:      len(document.Rows) != 0 && document.Rows[max(0, min(state.place.ReaderCursor, len(document.Rows)-1))].Commentable(),
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
			title += "  [diff]"
			title += state.reviewBoundsTitle()
		} else if state.markdownPreviewActive() {
			title += "  [preview]"
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
	assessment := state.reviewAssessment(state.readerEntry.Path, *state.displayedComparison)
	switch {
	case assessment.State == review.Updated && state.displayedBounds.Old != state.displayedComparison.Old:
		return "  since reviewed"
	case assessment.State == review.Updated && state.reviewFull[state.readerEntry.Path]:
		return "  full comparison"
	case assessment.State == review.Partial || assessment.State == review.BasisChanged:
		return "  re-review required; full comparison"
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
