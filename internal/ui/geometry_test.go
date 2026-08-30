package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestRectContainsHalfOpen(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 2, Y: 3, Width: 4, Height: 2}
	tests := []struct {
		name string
		x    int
		y    int
		want bool
	}{
		{name: "top left", x: 2, y: 3, want: true},
		{name: "last cell", x: 5, y: 4, want: true},
		{name: "left outside", x: 1, y: 3},
		{name: "top outside", x: 2, y: 2},
		{name: "right boundary", x: 6, y: 3},
		{name: "bottom boundary", x: 2, y: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := rect.Contains(test.x, test.y); got != test.want {
				t.Fatalf("Contains(%d, %d) = %v, want %v", test.x, test.y, got, test.want)
			}
		})
	}
}

func TestCalculatePartitionsScreen(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ width, height int }{{0, 0}, {1, 1}, {10, 2}, {39, 8}, {80, 24}, {200, 50}} {
		g := Calculate(size.width, size.height)
		if g.Screen.Width != max(0, size.width) || g.Screen.Height != max(0, size.height) {
			t.Fatalf("Calculate(%d, %d) screen = %+v", size.width, size.height, g.Screen)
		}
		if g.Navigator.Width+g.Divider.Width+g.Reader.Width != g.Screen.Width {
			t.Fatalf("pane widths do not partition %d: %+v", size.width, g)
		}
		if g.Header.Height+g.Body.Height+g.Footer.Height != g.Screen.Height {
			t.Fatalf("vertical bounds do not partition %d: %+v", size.height, g)
		}
		for name, rect := range map[string]Rect{
			"switcher":      g.HeaderSwitcher,
			"files label":   g.HeaderFiles,
			"git label":     g.HeaderGit,
			"scratch label": g.HeaderScratch,
		} {
			if rect.Y != g.Header.Y || rect.X < g.Header.X || rect.X+rect.Width > g.Header.X+g.Header.Width || rect.Height > g.Header.Height {
				t.Fatalf("%s is outside header: rect=%+v header=%+v", name, rect, g.Header)
			}
		}
		if g.Navigator.X+g.Navigator.Width != g.Divider.X || g.Divider.X+g.Divider.Width != g.Reader.X {
			t.Fatalf("pane boundary disagrees: %+v", g)
		}
		assertSurfaceGeometry(t, g.Navigator, g.NavigatorTitle, g.NavigatorRows)
		assertSurfaceGeometry(t, g.Reader, g.ReaderTitle, g.ReaderRows)
		assertSurfaceGeometry(t, g.Body, g.ScratchTitle, g.ScratchRows)
		if g.ScratchText.Width+g.ScratchBar.Width != g.ScratchRows.Width || g.ScratchText.Y != g.ScratchRows.Y {
			t.Fatalf("Scratch text and bar do not partition rows: %+v", g)
		}
	}
}

func TestCalculateTinyWidthsRemainBounded(t *testing.T) {
	t.Parallel()
	for width := 0; width <= 42; width++ {
		for height := 0; height <= 5; height++ {
			g := Calculate(width, height)
			for name, rect := range map[string]Rect{
				"screen": g.Screen, "header": g.Header, "header switcher": g.HeaderSwitcher,
				"header files": g.HeaderFiles, "header git": g.HeaderGit, "header scratch": g.HeaderScratch, "body": g.Body,
				"navigator": g.Navigator, "navigator title": g.NavigatorTitle,
				"navigator rows": g.NavigatorRows, "divider": g.Divider,
				"reader": g.Reader, "reader title": g.ReaderTitle,
				"reader rows": g.ReaderRows, "scratch title": g.ScratchTitle,
				"scratch rows": g.ScratchRows, "scratch text": g.ScratchText,
				"scratch bar": g.ScratchBar, "footer": g.Footer,
			} {
				if rect.X < 0 || rect.Y < 0 || rect.Width < 0 || rect.Height < 0 ||
					rect.X+rect.Width > width || rect.Y+rect.Height > height {
					t.Fatalf("Calculate(%d, %d) %s out of bounds: %+v in %+v", width, height, name, rect, g.Screen)
				}
			}
			wantDivider := width >= 3 && g.Body.Height > 0
			if (g.Divider.Width == 1) != wantDivider {
				t.Fatalf("Calculate(%d, %d) divider = %+v, want present %v", width, height, g.Divider, wantDivider)
			}
			if g.Divider.Width > 0 && (g.Navigator.Width == 0 || g.Reader.Width == 0) {
				t.Fatalf("Calculate(%d, %d) divider separates empty surface: %+v", width, height, g)
			}
		}
	}
}

func TestScratchHitTestUsesFullWidthRowsAndScrollbar(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 12)
	if got := g.ScratchHitTest(g.ScratchText.X+10, g.ScratchText.Y+2, 30, 3); got.Kind != HitScratchText {
		t.Fatalf("Scratch text hit = %+v", got)
	}
	bar, ok := CalculateScrollbar(g.ScratchRows, 30, 3)
	if !ok {
		t.Fatal("missing Scratch scrollbar")
	}
	if got := g.ScratchHitTest(bar.Thumb.X, bar.Thumb.Y, 30, 3); got.Kind != HitScratchScrollbar || got.GrabOffset != 0 {
		t.Fatalf("Scratch scrollbar hit = %+v", got)
	}
	if got := g.ScratchHitTest(g.ScratchTitle.X, g.ScratchTitle.Y, 30, 3); got.Kind != HitNone {
		t.Fatalf("Scratch title hit = %+v", got)
	}
	if got := g.ScratchHitTest(g.HeaderFiles.X, g.HeaderFiles.Y, 30, 3); got.Kind != HitFilesWorkspace {
		t.Fatalf("Scratch header hit = %+v", got)
	}
}

func TestCalculateWithNavigatorWidthClampsBothPanes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		requested int
		want      int
	}{
		{name: "left bound", requested: 0, want: MinimumPaneWidth},
		{name: "requested", requested: 34, want: 34},
		{name: "right bound", requested: 80, want: 80 - 1 - MinimumPaneWidth},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := CalculateWithNavigatorWidth(80, 24, test.requested)
			if g.Navigator.Width != test.want || g.Reader.Width < MinimumPaneWidth {
				t.Fatalf("requested %d => navigator %d reader %d, want navigator %d", test.requested, g.Navigator.Width, g.Reader.Width, test.want)
			}
			if g.Divider.X != g.Navigator.Width {
				t.Fatalf("divider x=%d disagrees with navigator width=%d", g.Divider.X, g.Navigator.Width)
			}
		})
	}
}

func TestHeaderSwitcherGeometryAndHits(t *testing.T) {
	t.Parallel()
	for width := 0; width <= 34; width++ {
		g := Calculate(width, 6)
		wantSwitcherWidth := min(width, 30)
		if g.HeaderSwitcher != (Rect{Width: wantSwitcherWidth, Height: 1}) {
			t.Fatalf("Calculate(%d) switcher = %+v, want width %d", width, g.HeaderSwitcher, wantSwitcherWidth)
		}
		for x := 0; x < width; x++ {
			want := HitNone
			if x >= 2 && x < 9 {
				want = HitFilesWorkspace
			} else if x >= 9 && x < 14 {
				want = HitGitWorkspace
			} else if x >= 21 && x < 30 {
				want = HitScratchWorkspace
			}
			if got := g.HitTest(x, 0, workspace.Scratch, workspace.Controls{}, 0, 0, 0, 0).Kind; got != want {
				t.Fatalf("Calculate(%d) header hit x=%d = %v, want %v", width, x, got, want)
			}
		}
	}
}

func TestHitTestPrecedenceAndBoundaries(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 20)
	rowY := g.NavigatorRows.Y + 1
	tests := []struct {
		name string
		x    int
		y    int
		want Hit
	}{
		{name: "visible row precedes pane", x: g.NavigatorRows.X, y: rowY, want: Hit{Kind: HitNavigatorRow, Index: 4}},
		{name: "files workspace", x: g.HeaderFiles.X, y: g.HeaderFiles.Y, want: Hit{Kind: HitFilesWorkspace}},
		{name: "git workspace", x: g.HeaderGit.X, y: g.HeaderGit.Y, want: Hit{Kind: HitGitWorkspace}},
		{name: "scratch workspace", x: g.HeaderScratch.X, y: g.HeaderScratch.Y, want: Hit{Kind: HitScratchWorkspace}},
		{name: "header punctuation", x: 15, y: g.Header.Y, want: Hit{Kind: HitNone}},
		{name: "header gap", x: 30, y: g.Header.Y, want: Hit{Kind: HitNone}},
		{name: "empty row is pane", x: g.NavigatorRows.X, y: g.NavigatorRows.Y + 4, want: Hit{Kind: HitNavigator}},
		{name: "navigator title", x: g.NavigatorTitle.X, y: g.NavigatorTitle.Y, want: Hit{Kind: HitNavigator}},
		{name: "divider is neutral", x: g.Divider.X, y: g.Divider.Y, want: Hit{Kind: HitNone}},
		{name: "reader title", x: g.ReaderTitle.X, y: g.ReaderTitle.Y, want: Hit{Kind: HitReader}},
		{name: "right boundary outside", x: g.Reader.X + g.Reader.Width, y: g.Reader.Y, want: Hit{Kind: HitNone}},
		{name: "bottom boundary outside", x: g.Reader.X, y: g.Reader.Y + g.Reader.Height, want: Hit{Kind: HitNone}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := g.HitTest(test.x, test.y, workspace.Scratch, workspace.Controls{}, 3, 5, 0, 0); got != test.want {
				t.Fatalf("HitTest() = %+v, want %+v", got, test.want)
			}
		})
	}
	if got := g.HitTest(g.Divider.X, g.Divider.Y, workspace.Files, workspace.Controls{}, 0, 0, 0, 0); got != (Hit{Kind: HitDivider}) {
		t.Fatalf("Files divider hit = %+v, want draggable divider", got)
	}
	navigatorBar, _ := CalculateScrollbar(g.NavigatorRows, 40, 3)
	if got := g.HitTest(navigatorBar.Thumb.X, navigatorBar.Thumb.Y, workspace.Files, workspace.Controls{}, 3, 40, 0, 0); got != (Hit{Kind: HitNavigatorScrollbar}) {
		t.Fatalf("navigator scrollbar hit = %+v", got)
	}
	readerBar, _ := CalculateScrollbar(g.ReaderRows, 60, 4)
	if got := g.HitTest(readerBar.Thumb.X, readerBar.Thumb.Y, workspace.Files, workspace.Controls{}, 0, 0, 4, 60); got != (Hit{Kind: HitReaderScrollbar}) {
		t.Fatalf("reader scrollbar hit = %+v", got)
	}
}

func assertSurfaceGeometry(t *testing.T, surface, title, rows Rect) {
	t.Helper()
	if title.X != surface.X || rows.X != surface.X || title.Width != surface.Width || rows.Width != surface.Width {
		t.Fatalf("surface columns disagree: surface=%+v title=%+v rows=%+v", surface, title, rows)
	}
	if title.Y != surface.Y || title.Height+rows.Height != surface.Height || title.Y+title.Height != rows.Y {
		t.Fatalf("surface rows disagree: surface=%+v title=%+v rows=%+v", surface, title, rows)
	}
}

func TestRenderAndHitTestShareNavigatorRows(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 16)
	files := []string{"zero", "one with space", "日本語", "line\nbreak", "four"}
	model := Model{
		Geometry:       g,
		NavigatorTitle: "Navigator  5 files",
		NavigatorRows:  navigatorRows(files),
		Selected:       2,
		Top:            1,
		Focus:          navigation.FocusNavigator,
		ReaderTitle:    "Reader  " + files[2],
		ReaderLines:    []Line{{Text: "alpha"}, {Text: "beta"}},
	}
	rendered := Render(model)
	width, height := lipgloss.Size(rendered)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("render size = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	for row := 0; row < min(g.NavigatorRows.Height, len(files)-model.Top); row++ {
		hit := g.HitTest(g.NavigatorRows.X, g.NavigatorRows.Y+row, workspace.Files, workspace.Controls{}, model.Top, len(files), 0, 0)
		if hit.Kind != HitNavigatorRow || hit.Index != model.Top+row {
			t.Fatalf("visible row %d hit = %+v", row, hit)
		}
		if !strings.Contains(rendered, SafeSingleLine(files[model.Top+row])) {
			t.Fatalf("render omitted visible path %q", files[model.Top+row])
		}
	}
}

func TestRenderKeepsRequestedSize(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ width, height int }{{1, 1}, {10, 2}, {20, 5}, {39, 8}, {80, 24}, {160, 40}} {
		size := size
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			t.Parallel()
			g := Calculate(size.width, size.height)
			rendered := Render(Model{
				Geometry:       g,
				NavigatorTitle: "Navigator  0 files",
				NavigatorEmpty: Line{Text: "Loading files…", Tone: ToneQuiet},
			})
			width, height := lipgloss.Size(rendered)
			if width != size.width || height != size.height {
				t.Fatalf("render size = %dx%d, want %dx%d", width, height, size.width, size.height)
			}
		})
	}
}

func TestSafeContentLines(t *testing.T) {
	t.Parallel()
	lines := SafeContentLines("ok\t\x1b[31m\r\ninvalid:\xff\x00")
	joined := strings.Join(lines, "\n")
	if strings.ContainsRune(joined, '\x1b') || strings.ContainsRune(joined, '\x00') || strings.ContainsRune(joined, '\r') {
		t.Fatalf("unsafe control byte survived: %q", joined)
	}
	for _, want := range []string{"    ", "␛", "�", "␀"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("safe output %q lacks %q", joined, want)
		}
	}
}

func TestNavigatorRowHitCoversTreePrefixAndLabelCells(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 20)
	y := g.NavigatorRows.Y + 2
	for _, x := range []int{g.NavigatorRows.X, g.NavigatorRows.X + g.NavigatorRows.Width - 2} {
		hit := g.HitTest(x, y, workspace.Files, workspace.Controls{}, 4, 20, 0, 0)
		if hit.Kind != HitNavigatorRow || hit.Index != 6 {
			t.Fatalf("HitTest(%d, %d) = %+v, want row index 6", x, y, hit)
		}
	}
}
