package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
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
		if g.Navigator.Width+g.Reader.Width != g.Screen.Width {
			t.Fatalf("pane widths do not partition %d: %+v", size.width, g)
		}
		if g.Header.Height+g.Navigator.Height+g.Footer.Height != g.Screen.Height {
			t.Fatalf("vertical bounds do not partition %d: %+v", size.height, g)
		}
		if g.Navigator.X+g.Navigator.Width != g.Reader.X {
			t.Fatalf("pane boundary disagrees: %+v", g)
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
		{name: "empty row is pane", x: g.NavigatorRows.X, y: g.NavigatorRows.Y + 4, want: Hit{Kind: HitNavigator}},
		{name: "navigator border", x: g.Navigator.X, y: g.Navigator.Y, want: Hit{Kind: HitNavigator}},
		{name: "shared boundary belongs reader", x: g.Reader.X, y: g.Reader.Y, want: Hit{Kind: HitReader}},
		{name: "right boundary outside", x: g.Reader.X + g.Reader.Width, y: g.Reader.Y, want: Hit{Kind: HitNone}},
		{name: "bottom boundary outside", x: g.Reader.X, y: g.Reader.Y + g.Reader.Height, want: Hit{Kind: HitNone}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := g.HitTest(test.x, test.y, 3, 5); got != test.want {
				t.Fatalf("HitTest() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestRenderAndHitTestShareNavigatorRows(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 16)
	files := []string{"zero", "one with space", "日本語", "line\nbreak", "four"}
	model := Model{
		Geometry:   g,
		Root:       "/repo",
		Files:      files,
		Selected:   2,
		Top:        1,
		Focus:      navigation.FocusNavigator,
		Reader:     repository.File{Kind: repository.FileReady, Content: "alpha\nbeta"},
		ReaderPath: files[2],
	}
	rendered := Render(model)
	width, height := lipgloss.Size(rendered)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("render size = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	for row := 0; row < min(g.NavigatorRows.Height, len(files)-model.Top); row++ {
		hit := g.HitTest(g.NavigatorRows.X, g.NavigatorRows.Y+row, model.Top, len(files))
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
			rendered := Render(Model{Geometry: g, Root: "/repo", ListLoading: true})
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
