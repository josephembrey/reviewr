package commitrow

import (
	"testing"
	"time"

	"github.com/josephembrey/reviewr/internal/commitgraph"
)

func TestMeasureCapsGraphAndGrowsAuthorSlowly(t *testing.T) {
	t.Parallel()
	wideGraph := commitgraph.Row{Cells: make([]commitgraph.Cell, 8)}
	rows := []Row{{Graph: wideGraph, Author: "A very long author name"}}
	for _, test := range []struct {
		width       int
		graph       int
		maxAuthor   int
		ageExpected bool
	}{
		{width: 20, graph: 5},
		{width: 40, graph: 10, maxAuthor: 1, ageExpected: true},
		{width: 80, graph: 16, maxAuthor: AuthorCap, ageExpected: true},
	} {
		columns := Measure(rows, test.width)
		if columns.Graph != test.graph || columns.Author > test.maxAuthor || (columns.Age > 0) != test.ageExpected {
			t.Fatalf("Measure(%d) = %+v", test.width, columns)
		}
	}
}

func TestTrailReserveProtectsCommitMessage(t *testing.T) {
	t.Parallel()
	row := Row{Refs: []Ref{{Kind: Branch, Name: "feature/long-name"}, {Kind: Tag, Name: "v1"}}, Merge: true}
	if got := TrailReserve(row, preferredText); got != 0 {
		t.Fatalf("tight trail reserve = %d", got)
	}
	if got := TrailReserve(row, 80); got <= 0 || got > RefTrailCap {
		t.Fatalf("wide trail reserve = %d", got)
	}
}

func TestAgeLabelStaysCompact(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_000, 0)
	for _, test := range []struct {
		seconds int64
		want    string
	}{
		{seconds: 5, want: "now"},
		{seconds: 5 * 60, want: "5m"},
		{seconds: 2 * 60 * 60, want: "2h"},
		{seconds: 3 * 24 * 60 * 60, want: "3d"},
		{seconds: 14 * 24 * 60 * 60, want: "2w"},
		{seconds: 2 * 365 * 24 * 60 * 60, want: "2y"},
	} {
		if got := AgeLabel(now, now.Unix()-test.seconds); got != test.want {
			t.Fatalf("AgeLabel(%d) = %q, want %q", test.seconds, got, test.want)
		}
	}
}
