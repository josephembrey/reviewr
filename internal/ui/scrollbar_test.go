package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/navigation"
)

func TestVerticalScrollbarThumbTracksViewport(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		offset    int
		wantThumb []int
	}{
		{name: "start", offset: 0, wantThumb: []int{0, 1}},
		{name: "middle", offset: 2, wantThumb: []int{1, 2}},
		{name: "end", offset: 5, wantThumb: []int{3, 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bar := verticalScrollbar(5, 10, test.offset, true)
			if len(bar) != 5 {
				t.Fatalf("bar height = %d, want 5", len(bar))
			}
			for row, cell := range bar {
				want := "▕"
				for _, thumbRow := range test.wantThumb {
					if row == thumbRow {
						want = "▐"
					}
				}
				if got := ansi.Strip(cell); got != want {
					t.Fatalf("row %d = %q, want %q", row, got, want)
				}
			}
		})
	}
}

func TestVerticalScrollbarHidesWhenContentFits(t *testing.T) {
	t.Parallel()
	for _, total := range []int{0, 1, 5} {
		if bar := verticalScrollbar(5, total, 0, true); bar != nil {
			t.Fatalf("total %d produced scrollbar %+v", total, bar)
		}
	}
}

func TestScrollbarMapsPointerDragBackToContentOffset(t *testing.T) {
	t.Parallel()
	bar, ok := CalculateScrollbar(Rect{X: 4, Y: 2, Width: 10, Height: 5}, 10, 0)
	if !ok {
		t.Fatal("scrollable content produced no geometry")
	}
	if bar.Track != (Rect{X: 13, Y: 2, Width: 1, Height: 5}) || bar.Thumb != (Rect{X: 13, Y: 2, Width: 1, Height: 2}) {
		t.Fatalf("scrollbar geometry = %+v", bar)
	}
	if got := bar.OffsetAt(bar.Track.Y+bar.Track.Height-1, bar.GrabOffset(bar.Track.Y+bar.Track.Height-1)); got != 5 {
		t.Fatalf("bottom track click offset = %d, want 5", got)
	}
}

func TestRenderAddsIndependentNavigatorAndReaderScrollbars(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 12)
	navigatorRows := make([]NavigatorRow, 20)
	for index := range navigatorRows {
		navigatorRows[index] = NavigatorRow{Identity: "file", Label: "file"}
	}
	readerLines := make([]Line, 30)
	for index := range readerLines {
		readerLines[index] = Line{Text: "line"}
	}
	frame := ansi.Strip(Render(Model{
		Geometry:       g,
		NavigatorRows:  navigatorRows,
		Selected:       5,
		Top:            5,
		Focus:          navigation.FocusNavigator,
		ReaderLines:    readerLines,
		ReaderOffset:   10,
		NavigatorTitle: "20 files",
		ReaderTitle:    "file.go",
	}))
	lines := strings.Split(frame, "\n")
	for row := g.NavigatorRows.Y; row < g.NavigatorRows.Y+g.NavigatorRows.Height; row++ {
		cells := []rune(lines[row])
		if cell := cells[g.Navigator.X+g.Navigator.Width-1]; cell != '▕' && cell != '▐' {
			t.Fatalf("navigator row %d scrollbar cell = %q", row, cell)
		}
		if cell := cells[g.Reader.X+g.Reader.Width-1]; cell != '▕' && cell != '▐' {
			t.Fatalf("reader row %d scrollbar cell = %q", row, cell)
		}
	}
}
