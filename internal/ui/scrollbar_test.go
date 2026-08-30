package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestVerticalScrollbarThumbTracksViewport(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		offset    int
		wantThumb []int
	}{
		{name: "start", offset: 0, wantThumb: []int{0, 1, 2}},
		{name: "middle", offset: 2, wantThumb: []int{1, 2, 3}},
		{name: "end", offset: 5, wantThumb: []int{2, 3, 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			geometry, ok := CalculateScrollbar(Rect{Width: 2, Height: 5}, 10, test.offset)
			if !ok {
				t.Fatal("overflowing content produced no scrollbar")
			}
			cells := verticalScrollbar(geometry, true)
			if len(cells) != 5 {
				t.Fatalf("bar height = %d, want 5", len(cells))
			}
			for row, cell := range cells {
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
		if bar, ok := CalculateScrollbar(Rect{Width: 2, Height: 5}, total, 0); ok {
			t.Fatalf("total %d produced scrollbar %+v", total, bar)
		}
	}
}

func TestCalculateScrollbarMatchesHerdrRoundingMinimumAndEndpoints(t *testing.T) {
	t.Parallel()
	rows := Rect{X: 4, Y: 2, Width: 10, Height: 5}
	bar, ok := CalculateScrollbar(rows, 7, 0)
	if !ok {
		t.Fatal("scrollable content produced no geometry")
	}
	if bar.Content != (Rect{X: 4, Y: 2, Width: 9, Height: 5}) ||
		bar.Track != (Rect{X: 13, Y: 2, Width: 1, Height: 5}) ||
		bar.Thumb != (Rect{X: 13, Y: 2, Width: 1, Height: 4}) {
		t.Fatalf("nearest thumb-length geometry = %+v", bar)
	}

	bar, _ = CalculateScrollbar(rows, 12, 2)
	if bar.Thumb != (Rect{X: 13, Y: 3, Width: 1, Height: 2}) {
		t.Fatalf("nearest thumb-position geometry = %+v", bar)
	}
	bar, _ = CalculateScrollbar(rows, 100, 0)
	if bar.Thumb.Height != 1 || bar.Thumb.Y != bar.Track.Y {
		t.Fatalf("minimum/start thumb geometry = %+v", bar)
	}
	bar, _ = CalculateScrollbar(rows, 100, 95)
	if bar.Thumb.Y+bar.Thumb.Height != bar.Track.Y+bar.Track.Height {
		t.Fatalf("end thumb does not reach track bottom: %+v", bar)
	}
	bar, _ = CalculateScrollbar(rows, 12, -100)
	if bar.Thumb.Y != bar.Track.Y {
		t.Fatalf("negative offset did not clamp to top: %+v", bar)
	}
	bar, _ = CalculateScrollbar(rows, 12, 100)
	if bar.Thumb.Y+bar.Thumb.Height != bar.Track.Y+bar.Track.Height {
		t.Fatalf("large offset did not clamp to bottom: %+v", bar)
	}
}

func TestCalculateScrollbarRejectsTinyAndFittingSurfaces(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		rows  Rect
		total int
	}{
		{name: "zero width", rows: Rect{Height: 5}, total: 10},
		{name: "no content beside lane", rows: Rect{Width: 1, Height: 5}, total: 10},
		{name: "zero height", rows: Rect{Width: 5}, total: 10},
		{name: "empty", rows: Rect{Width: 5, Height: 5}, total: 0},
		{name: "fitting", rows: Rect{Width: 5, Height: 5}, total: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if bar, ok := CalculateScrollbar(test.rows, test.total, 0); ok {
				t.Fatalf("CalculateScrollbar() = %+v, want no lane", bar)
			}
		})
	}
}

func TestScrollbarTrackClicksThumbGrabsAndDragging(t *testing.T) {
	t.Parallel()
	bar, ok := CalculateScrollbar(Rect{X: 4, Y: 2, Width: 10, Height: 10}, 20, 4)
	if !ok {
		t.Fatal("scrollable content produced no geometry")
	}
	if bar.Thumb != (Rect{X: 13, Y: 4, Width: 1, Height: 5}) {
		t.Fatalf("scrollbar geometry = %+v", bar)
	}
	for row := bar.Thumb.Y; row < bar.Thumb.Y+bar.Thumb.Height; row++ {
		grab := bar.GrabOffset(row)
		if grab != row-bar.Thumb.Y {
			t.Fatalf("thumb row %d grab = %d", row, grab)
		}
		if got := bar.OffsetAt(row, grab); got != 4 {
			t.Fatalf("thumb row %d moved offset to %d, want 4", row, got)
		}
	}
	if got := bar.OffsetAt(bar.Track.Y, bar.GrabOffset(bar.Track.Y)); got != 0 {
		t.Fatalf("top track click offset = %d, want 0", got)
	}
	bottom := bar.Track.Y + bar.Track.Height - 1
	if got := bar.OffsetAt(bottom, bar.GrabOffset(bottom)); got != bar.MaxOffset {
		t.Fatalf("bottom track click offset = %d, want %d", got, bar.MaxOffset)
	}
	grab := bar.GrabOffset(bar.Thumb.Y + 3)
	if got := bar.OffsetAt(-100, grab); got != 0 {
		t.Fatalf("drag above track offset = %d, want 0", got)
	}
	if got := bar.OffsetAt(100, grab); got != bar.MaxOffset {
		t.Fatalf("drag below track offset = %d, want %d", got, bar.MaxOffset)
	}
}

func TestVerticalScrollbarMatchesHerdrGlyphAndANSIHierarchy(t *testing.T) {
	t.Parallel()
	bar, _ := CalculateScrollbar(Rect{X: 7, Y: 3, Width: 2, Height: 5}, 10, 0)
	focused := verticalScrollbar(bar, true)
	unfocused := verticalScrollbar(bar, false)
	if got := ansi.Strip(focused[0]); got != "▐" {
		t.Fatalf("focused thumb glyph = %q, want ▐", got)
	}
	if got := ansi.Strip(unfocused[0]); got != "▕" {
		t.Fatalf("unfocused thumb glyph = %q, want ▕", got)
	}
	if got := ansi.Strip(focused[len(focused)-1]); got != "▕" {
		t.Fatalf("track glyph = %q, want ▕", got)
	}
	if focused[0] != scrollbarFocusedThumbStyle.Render("▐") ||
		unfocused[0] != scrollbarUnfocusedThumbStyle.Render("▕") ||
		focused[len(focused)-1] != scrollbarTrackStyle.Render("▕") {
		t.Fatalf("scrollbar cells do not use semantic styles: focused=%q unfocused=%q track=%q",
			focused[0], unfocused[0], focused[len(focused)-1])
	}
	assertSameColor(t, scrollbarTrackStyle.GetForeground(), mutedColor)
	assertSameColor(t, scrollbarUnfocusedThumbStyle.GetForeground(), mutedColor)
	assertSameColor(t, scrollbarFocusedThumbStyle.GetForeground(), secondaryColor)
	if !scrollbarTrackStyle.GetFaint() || scrollbarUnfocusedThumbStyle.GetFaint() || scrollbarFocusedThumbStyle.GetFaint() {
		t.Fatalf("faint hierarchy = track %v unfocused %v focused %v",
			scrollbarTrackStyle.GetFaint(), scrollbarUnfocusedThumbStyle.GetFaint(), scrollbarFocusedThumbStyle.GetFaint())
	}
	if scrollbarFocusedThumbStyle.GetBold() || scrollbarFocusedThumbStyle.GetForeground() == headerStyle.GetForeground() {
		t.Fatal("focused scrollbar thumb reintroduced bold accent styling")
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
	navigatorBar, _ := CalculateScrollbar(g.NavigatorRows, len(navigatorRows), 5)
	readerBar, _ := CalculateScrollbar(g.ReaderRows, len(readerLines), 10)
	for _, focus := range []navigation.Focus{navigation.FocusNavigator, navigation.FocusReader} {
		frame := ansi.Strip(Render(Model{
			Geometry:       g,
			Workspace:      workspace.Files,
			NavigatorRows:  navigatorRows,
			Selected:       5,
			Top:            5,
			Focus:          focus,
			ReaderLines:    readerLines,
			ReaderOffset:   10,
			NavigatorTitle: "20 files",
			ReaderTitle:    "file.go",
		}))
		lines := strings.Split(frame, "\n")
		for row := g.NavigatorRows.Y; row < g.NavigatorRows.Y+g.NavigatorRows.Height; row++ {
			cells := []rune(lines[row])
			wantNavigator := '▕'
			if focus == navigation.FocusNavigator && navigatorBar.Thumb.Contains(navigatorBar.Thumb.X, row) {
				wantNavigator = '▐'
			}
			if got := cells[navigatorBar.Track.X]; got != wantNavigator {
				t.Fatalf("focus %v navigator row %d scrollbar = %q, want %q", focus, row, got, wantNavigator)
			}
			wantReader := '▕'
			if focus == navigation.FocusReader && readerBar.Thumb.Contains(readerBar.Thumb.X, row) {
				wantReader = '▐'
			}
			if got := cells[readerBar.Track.X]; got != wantReader {
				t.Fatalf("focus %v reader row %d scrollbar = %q, want %q", focus, row, got, wantReader)
			}
		}
		if hit := g.HitTest(navigatorBar.Track.X, navigatorBar.Thumb.Y, workspace.Files, workspace.Controls{}, 5, len(navigatorRows), 10, len(readerLines)); hit.Kind != HitNavigatorScrollbar || hit.GrabOffset != 0 {
			t.Fatalf("navigator painted thumb hit = %+v", hit)
		}
		if hit := g.HitTest(readerBar.Track.X, readerBar.Thumb.Y, workspace.Files, workspace.Controls{}, 5, len(navigatorRows), 10, len(readerLines)); hit.Kind != HitReaderScrollbar || hit.GrabOffset != 0 {
			t.Fatalf("reader painted thumb hit = %+v", hit)
		}
	}
}

func TestFilesReaderModesReserveScrollbarWithoutPaintingOverContent(t *testing.T) {
	t.Parallel()
	g := Calculate(70, 12)
	reader := make([]Line, 30)
	for index := range reader {
		reader[index] = Line{Text: strings.Repeat("x", g.ReaderRows.Width)}
	}
	bar, ok := CalculateScrollbar(g.ReaderRows, len(reader), 0)
	if !ok {
		t.Fatal("overflowing reader produced no scrollbar")
	}
	for _, mode := range []workspace.ReaderMode{workspace.FileReader, workspace.DiffReader} {
		frame := ansi.Strip(Render(Model{
			Geometry:     g,
			Workspace:    workspace.Files,
			Controls:     workspace.Controls{Reader: mode},
			Focus:        navigation.FocusReader,
			ReaderLines:  reader,
			ReaderOffset: 0,
		}))
		line := []rune(strings.Split(frame, "\n")[g.ReaderRows.Y])
		if got := string(line[bar.Content.X : bar.Content.X+bar.Content.Width]); got != strings.Repeat("x", bar.Content.Width) {
			t.Fatalf("%s content before lane = %q", mode.Label(), got)
		}
		if got := line[bar.Track.X]; got != '▐' {
			t.Fatalf("%s thumb cell = %q, want ▐", mode.Label(), got)
		}
	}
}

func TestFittingAndEmptySurfacesKeepTheirFinalContentColumn(t *testing.T) {
	t.Parallel()
	g := Calculate(70, 12)
	navigatorLabel := strings.Repeat("n", max(0, g.NavigatorRows.Width-2))
	readerLine := strings.Repeat("r", g.ReaderRows.Width)
	frame := ansi.Strip(Render(Model{
		Geometry:      g,
		Workspace:     workspace.Files,
		NavigatorRows: []NavigatorRow{{Identity: "only", Label: navigatorLabel}},
		ReaderLines:   []Line{{Text: readerLine}},
		Focus:         navigation.FocusReader,
	}))
	lines := strings.Split(frame, "\n")
	navigator := []rune(lines[g.NavigatorRows.Y])
	if got := string(navigator[g.NavigatorRows.X : g.NavigatorRows.X+g.NavigatorRows.Width]); got != "  "+navigatorLabel {
		t.Fatalf("fitting navigator row = %q", got)
	}
	reader := []rune(lines[g.ReaderRows.Y])
	if got := string(reader[g.ReaderRows.X : g.ReaderRows.X+g.ReaderRows.Width]); got != readerLine {
		t.Fatalf("fitting reader row = %q", got)
	}

	empty := ansi.Strip(Render(Model{Geometry: g, Workspace: workspace.Files}))
	if strings.ContainsAny(empty, "▕▐") {
		t.Fatalf("empty surfaces painted scrollbar chrome:\n%s", empty)
	}
}

func TestGitModesUseSharedIndependentPaneScrollbars(t *testing.T) {
	t.Parallel()
	g := Calculate(72, 13)
	navigatorRows := make([]NavigatorRow, 30)
	readerLines := make([]Line, 40)
	for index := range navigatorRows {
		navigatorRows[index] = NavigatorRow{Identity: "item", Label: "item"}
	}
	for index := range readerLines {
		readerLines[index] = Line{Text: "detail"}
	}
	for _, mode := range []workspace.GitView{workspace.GitLog, workspace.GitRefs, workspace.GitStashes} {
		mode := mode
		t.Run(mode.Label(), func(t *testing.T) {
			t.Parallel()
			plain := ansi.Strip(Render(Model{
				Geometry:       g,
				Workspace:      workspace.Git,
				Controls:       workspace.Controls{Git: mode},
				NavigatorRows:  navigatorRows,
				Top:            8,
				Focus:          navigation.FocusReader,
				ReaderLines:    readerLines,
				ReaderOffset:   12,
				NavigatorTitle: "items",
				ReaderTitle:    "details",
			}))
			lines := strings.Split(plain, "\n")
			for row := g.NavigatorRows.Y; row < g.NavigatorRows.Y+g.NavigatorRows.Height; row++ {
				cells := []rune(lines[row])
				if got := cells[g.NavigatorRows.X+g.NavigatorRows.Width-1]; got != '▕' {
					t.Fatalf("unfocused navigator row %d glyph = %q", row, got)
				}
				got := cells[g.ReaderRows.X+g.ReaderRows.Width-1]
				if got != '▕' && got != '▐' {
					t.Fatalf("focused reader row %d glyph = %q", row, got)
				}
			}
		})
	}
}
