//go:build dev

package lab

import (
	"strings"

	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func (model Model) viewReviewIndicators(width, height int) string {
	lines := []string{
		title.Render("lab / review indicators"),
		quiet.Render("tab next page  •  exact production rendering  •  updated files use an ordinary incremental diff  •  ctrl+l or esc close"),
	}
	previewHeight := max(0, height-len(lines))
	preview := ui.Render(reviewIndicatorPreview(width, previewHeight))
	if preview != "" {
		lines = append(lines, strings.Split(preview, "\n")...)
	}
	return fitPage(lines, max(0, width), max(0, height))
}

func reviewIndicatorPreview(width, height int) ui.Model {
	geometry := ui.Calculate(width, height)
	return ui.Model{
		Geometry:  geometry,
		Workspace: workspace.Files,
		Controls: workspace.Controls{
			Files: workspace.ChangedFiles, Reader: workspace.DiffReader,
			Comparison: workspace.Uncommitted, DiffHighlight: workspace.DiffHighlightSidebar,
			RichDiff: true,
		},
		Changes:        ui.ChangeSummary{Files: 4, Additions: 11, Deletions: 6, Ready: true},
		NavigatorTitle: "4 changes",
		NavigatorRows: []ui.NavigatorRow{
			{Identity: "src", Label: "src", Tree: true, Directory: true, Expanded: true, Progress: "1/4"},
			reviewIndicatorFile("unreviewed.go", ui.StatusModified, 2, 1, review.Unreviewed),
			reviewIndicatorFile("reviewed.go", ui.StatusModified, 1, 1, review.Reviewed),
			reviewIndicatorFile("updated.go", ui.StatusModified, 5, 2, review.Updated),
			reviewIndicatorFile("re-review.go", ui.StatusModified, 3, 2, review.BasisChanged),
		},
		Selected:    0,
		Focus:       navigation.FocusReader,
		ReaderTitle: "src/updated.go",
		ReaderDocument: ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
			{Identity: "context:18", Kind: ui.ReaderContext, Text: "func total(items []Item) int {", OldLine: 18, NewLine: 18},
			{Identity: "base-add:19", Kind: ui.ReaderInsertion, Text: "    subtotal := sum(items)", NewLine: 19},
			{Identity: "context:20", Kind: ui.ReaderContext, Text: "    tax := calculateTax(items)", OldLine: 19, NewLine: 20},
			{Identity: "fresh-add:21", Kind: ui.ReaderInsertion, Text: "    discount := activeDiscount(items)", NewLine: 21},
			{Identity: "fresh-context:22", Kind: ui.ReaderContext, Text: "    return subtotal + tax - discount", OldLine: 20, NewLine: 22},
			{Identity: "context:23", Kind: ui.ReaderContext, Text: "}", OldLine: 21, NewLine: 23},
		}},
		ReaderCursor: 3,
	}
}

func reviewIndicatorFile(label string, status ui.NavigatorStatus, additions, deletions uint64, state review.State) ui.NavigatorRow {
	return ui.NavigatorRow{
		Identity: label, Label: label, Tree: true, Depth: 1, Status: status,
		Changes:    ui.LineChanges{Additions: additions, Deletions: deletions},
		Reviewable: true, Review: state,
	}
}
