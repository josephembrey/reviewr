package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/review"
)

func TestReviewBadgesUseIndependentAlignedRightSideField(t *testing.T) {
	states := []review.State{review.Unreviewed, review.Reviewed, review.Updated, review.Partial, review.BasisChanged}
	for _, state := range states {
		state := state
		t.Run(state.Label(), func(t *testing.T) {
			row := NavigatorRow{Label: "main.go", Tree: true, Status: StatusModified, Review: &state}
			rendered := ansi.Strip(renderNavigatorPresentationRow(row, 24, false, false, commitrow.Columns{}, time.Time{}))
			if len([]rune(rendered)) != 24 || !strings.HasPrefix(rendered, " M ") || !strings.HasSuffix(rendered, " "+state.Badge()) {
				t.Fatalf("rendered row = %q", rendered)
			}
		})
	}
}

func TestUnreviewedBadgeAndDirectoryProgressUseReadableSecondaryForeground(t *testing.T) {
	t.Parallel()
	assertSameColor(t, reviewBadgeStyle(review.Unreviewed).GetForeground(), secondaryColor)

	directory := NavigatorRow{Label: "src", Tree: true, Directory: true, Expanded: true, Progress: "2/3"}
	rendered := renderNavigatorPresentationRow(directory, 20, false, false, commitrow.Columns{}, time.Time{})
	if strings.Contains(rendered, mutedStyle.Render(" 2/3")) {
		t.Fatalf("directory review progress is incorrectly muted: %q", rendered)
	}
}

func TestNavigatorRowLayoutReservesProgressAndCompleteBadge(t *testing.T) {
	state := review.Partial
	changes := LineChanges{Additions: 12, Deletions: 3}
	layout := LayoutNavigatorRow(NavigatorRow{Review: &state, Changes: &changes, Progress: "12/30"}, 30)
	if layout.Label.Width != 13 || layout.Progress != (Rect{X: 13, Width: 6, Height: 1}) ||
		layout.Changes != (Rect{X: 19, Width: 7, Height: 1}) || layout.Review != (Rect{X: 26, Width: 4, Height: 1}) {
		t.Fatalf("layout = %+v", layout)
	}
	if narrow := LayoutNavigatorRow(NavigatorRow{Review: &state}, 4); narrow.Label.Width != 0 || narrow.Review.Width != 4 {
		t.Fatalf("narrow layout = %+v", narrow)
	}
}

func TestChangedTreeRowRightAlignsLineStatsBeforeReviewBadge(t *testing.T) {
	state := review.Reviewed
	changes := LineChanges{Additions: 12, Deletions: 3}
	row := NavigatorRow{Label: "main.go", Tree: true, Status: StatusModified, Changes: &changes, Review: &state}
	rendered := renderNavigatorPresentationRow(row, 30, false, false, commitrow.Columns{}, time.Time{})
	if plain := ansi.Strip(rendered); !strings.HasSuffix(plain, " +12 -3 [x]") {
		t.Fatalf("changed row = %q", plain)
	}
	if !strings.Contains(rendered, addedStyle.Render("+12")) || !strings.Contains(rendered, errorStyle.Render("-3")) {
		t.Fatalf("changed row stats lack semantic colors: %q", rendered)
	}
}

func TestLineChangeFormattingOmitsZeroSides(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                 string
		changes              LineChanges
		additions, deletions string
	}{
		{name: "Neither", changes: LineChanges{}},
		{name: "Additions only", changes: LineChanges{Additions: 12}, additions: "+12"},
		{name: "Deletions only", changes: LineChanges{Deletions: 3}, deletions: "-3"},
		{name: "Both", changes: LineChanges{Additions: 12, Deletions: 3}, additions: "+12", deletions: "-3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			additions, deletions := FormatLineChanges(test.changes)
			if additions != test.additions || deletions != test.deletions {
				t.Fatalf("FormatLineChanges(%+v) = %q, %q", test.changes, additions, deletions)
			}
		})
	}
}

func TestChangedTreeRowOmitsZeroLineStats(t *testing.T) {
	t.Parallel()
	state := review.Reviewed
	for _, test := range []struct {
		name    string
		changes LineChanges
		want    string
	}{
		{name: "Additions only", changes: LineChanges{Additions: 12}, want: " +12 [x]"},
		{name: "Deletions only", changes: LineChanges{Deletions: 3}, want: " -3 [x]"},
		{name: "Neither", changes: LineChanges{}, want: " [x]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			row := NavigatorRow{Label: "main.go", Tree: true, Status: StatusModified, Changes: &test.changes, Review: &state}
			plain := ansi.Strip(renderNavigatorPresentationRow(row, 30, false, false, commitrow.Columns{}, time.Time{}))
			if !strings.HasSuffix(plain, test.want) || strings.Contains(plain, "+0") || strings.Contains(plain, "-0") {
				t.Fatalf("changed row = %q, want suffix %q without zero stats", plain, test.want)
			}
			if test.changes == (LineChanges{}) && LayoutNavigatorRow(row, 30).Changes.Width != 0 {
				t.Fatalf("zero stats reserved layout space: %+v", LayoutNavigatorRow(row, 30))
			}
		})
	}
}

func TestEveryBadgeCellHitsReviewWithoutActivatingTheRow(t *testing.T) {
	g := Calculate(80, 20)
	state := review.Unreviewed
	rows := make([]NavigatorRow, 30)
	for index := range rows {
		rows[index] = NavigatorRow{Label: "file.go", Tree: true}
	}
	rows[3].Review = &state
	top := 2
	contentWidth := g.NavigatorRows.Width - 1 // scrollbar is present
	layout := LayoutNavigatorRow(rows[3], contentWidth)
	y := g.NavigatorRows.Y + 1
	for x := g.NavigatorRows.X + layout.Review.X; x < g.NavigatorRows.X+layout.Review.X+layout.Review.Width; x++ {
		index, ok := g.HitNavigatorReview(x, y, top, rows)
		if !ok || index != 3 {
			t.Fatalf("badge cell (%d,%d) = (%d,%v)", x, y, index, ok)
		}
	}
	if _, ok := g.HitNavigatorReview(g.NavigatorRows.X+layout.Review.X-1, y, top, rows); ok {
		t.Fatal("label cell hit review badge")
	}
	if _, ok := g.HitNavigatorReview(g.NavigatorRows.X+layout.Review.X, g.NavigatorRows.Y, top, rows); ok {
		t.Fatal("different row hit review badge")
	}
}

func TestDirectoryProgressAndUnchangedRowsHaveIndependentPresentation(t *testing.T) {
	directory := NavigatorRow{Label: "src", Tree: true, Directory: true, Expanded: true, Progress: "2/3"}
	got := ansi.Strip(renderNavigatorPresentationRow(directory, 20, true, true, commitrow.Columns{}, time.Time{}))
	if !strings.HasSuffix(got, " 2/3") || strings.Contains(got, "[") {
		t.Fatalf("directory row = %q", got)
	}
	unchanged := NavigatorRow{Label: "plain.go", Tree: true}
	got = ansi.Strip(renderNavigatorPresentationRow(unchanged, 20, false, false, commitrow.Columns{}, time.Time{}))
	if strings.Contains(got, "[") || LayoutNavigatorRow(unchanged, 20).Review.Width != 0 {
		t.Fatalf("unchanged row = %q", got)
	}
}
