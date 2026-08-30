package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/navigation"
)

func TestRenderUsesOneContinuousFloatingDivider(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 16)
	frame := ansi.Strip(Render(Model{
		Geometry:       g,
		NavigatorTitle: "Navigator  2 files",
		NavigatorRows:  navigatorRows([]string{"alpha.go", "beta.go"}),
		Selected:       0,
		Focus:          navigation.FocusNavigator,
		ReaderTitle:    "Reader  alpha.go",
		ReaderLines:    []Line{{Text: "package main"}},
	}))
	if strings.ContainsAny(frame, "╭╮╰╯─") {
		t.Fatalf("frameless render contains pane border glyphs:\n%s", frame)
	}
	lines := strings.Split(frame, "\n")
	for y := g.Body.Y; y < g.Body.Y+g.Body.Height; y++ {
		cells := []rune(lines[y])
		if len(cells) != g.Screen.Width {
			t.Fatalf("row %d has %d cells, want %d: %q", y, len(cells), g.Screen.Width, lines[y])
		}
		if cells[g.Divider.X] != '│' {
			t.Fatalf("row %d divider cell = %q, want │", y, cells[g.Divider.X])
		}
	}
	if got := strings.Count(frame, "│"); got != g.Divider.Height {
		t.Fatalf("divider count = %d, want %d", got, g.Divider.Height)
	}
}

func TestRenderFollowsSwappedPaneGeometry(t *testing.T) {
	t.Parallel()
	g := CalculateWithNavigatorWidth(80, 16, 24).SwapPanes()
	frame := ansi.Strip(Render(Model{
		Geometry:       g,
		NavigatorTitle: "24 files",
		NavigatorRows:  navigatorRows([]string{"alpha.go"}),
		ReaderTitle:    "alpha.go",
		ReaderLines:    []Line{{Text: "package main"}},
	}))
	lines := strings.Split(frame, "\n")
	bodyTitle := []rune(lines[g.Body.Y])
	if !strings.HasPrefix(lines[g.Body.Y], "alpha.go") || bodyTitle[g.Divider.X] != '│' {
		t.Fatalf("swapped title row does not follow geometry: %q", lines[g.Body.Y])
	}
	if right := string(bodyTitle[g.Navigator.X:]); !strings.HasPrefix(right, "24 files") {
		t.Fatalf("navigator is not painted on the right: %q", lines[g.Body.Y])
	}
}

func TestFocusStylingDoesNotChangeFrameStructure(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 16)
	model := Model{
		Geometry:       g,
		NavigatorTitle: "Navigator  1 file",
		NavigatorRows:  navigatorRows([]string{"alpha.go"}),
		Selected:       0,
		ReaderTitle:    "Reader  alpha.go",
		ReaderLines:    []Line{{Text: "package main"}},
	}
	model.Focus = navigation.FocusNavigator
	navigatorFocused := Render(model)
	model.Focus = navigation.FocusReader
	readerFocused := Render(model)
	if ansi.Strip(navigatorFocused) != ansi.Strip(readerFocused) {
		t.Fatal("changing focus changed frame text or geometry")
	}
	if !focusedTitleStyle.GetBold() || chromeStyle.GetBold() {
		t.Fatal("focus emphasis must live on the focused title only")
	}
	if got := strings.Count(renderDivider(g.Divider, false), "│"); got != g.Divider.Height {
		t.Fatalf("focus-independent divider count = %d, want %d", got, g.Divider.Height)
	}
}

func TestDividerUsesAccentOnlyWhileDragging(t *testing.T) {
	t.Parallel()
	rect := Rect{Width: 1, Height: 2}
	idle := renderDivider(rect, false)
	dragging := renderDivider(rect, true)
	if !strings.Contains(idle, chromeStyle.Render("│")) || strings.Contains(idle, mutedStyle.Render("│")) {
		t.Fatalf("idle divider does not use readable secondary foreground: %q", idle)
	}
	if !strings.Contains(dragging, headerStyle.Render("│")) {
		t.Fatalf("dragging divider lacks accent style: %q", dragging)
	}
	if ansi.Strip(idle) != ansi.Strip(dragging) {
		t.Fatalf("drag styling changed divider structure: idle=%q dragging=%q", idle, dragging)
	}
}

func TestSelectionStyleUsesTerminalReverseWithoutAccentColor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		focused bool
		bold    bool
	}{
		{name: "focused", focused: true, bold: true},
		{name: "unfocused", focused: false, bold: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			style := selectionStyle(test.focused)
			if !style.GetReverse() || style.GetBold() != test.bold {
				t.Fatalf("selection style reverse=%v bold=%v, want reverse=true bold=%v", style.GetReverse(), style.GetBold(), test.bold)
			}
			if _, ok := style.GetForeground().(lipgloss.NoColor); !ok {
				t.Fatalf("selection foreground = %T, want terminal default", style.GetForeground())
			}
		})
	}
}

func TestStructuralPaletteUsesTerminalANSIRoles(t *testing.T) {
	t.Parallel()
	got := []ansi.BasicColor{accentColor, secondaryColor, mutedColor, errorColor, addedColor, specialColor, warningColor}
	want := []ansi.BasicColor{
		lipgloss.Cyan,
		lipgloss.White,
		lipgloss.BrightBlack,
		lipgloss.Red,
		lipgloss.Green,
		lipgloss.Magenta,
		lipgloss.Yellow,
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("structural palette[%d] = %v, want terminal ANSI %v", index, got[index], want[index])
		}
	}
}

func TestStructuralChromeUsesReadableSecondaryForeground(t *testing.T) {
	t.Parallel()
	assertSameColor(t, chromeStyle.GetForeground(), secondaryColor)
	if _, ok := mutedStyle.GetForeground().(lipgloss.NoColor); ok {
		t.Fatal("muted style unexpectedly uses terminal default foreground")
	}
}

func TestNavigatorRowsKeepFullWidthAcrossContentAndFocus(t *testing.T) {
	t.Parallel()
	const rowWidth = 18
	paths := []string{
		"short.go",
		"日本語.nix",
		strings.Repeat("long-path/", 5) + "file.go",
		"",
	}
	for _, focused := range []bool{true, false} {
		for _, path := range paths {
			row := renderNavigatorRow(path, rowWidth, true, focused)
			if got := lipgloss.Width(row); got != rowWidth {
				t.Fatalf("renderNavigatorRow(%q, focused=%v) width = %d, want %d", path, focused, got, rowWidth)
			}
			plain := ansi.Strip(row)
			if got := lipgloss.Width(plain); got != rowWidth {
				t.Fatalf("plain navigator row %q width = %d, want %d", plain, got, rowWidth)
			}
			if lipgloss.Width("  "+path) < rowWidth && !strings.HasSuffix(plain, " ") {
				t.Fatalf("short navigator row lacks trailing selection padding: %q", plain)
			}
		}
	}

	g := Calculate(52, 12)
	files := paths[:3]
	for _, test := range []struct {
		name     string
		files    []string
		selected int
		focus    navigation.Focus
	}{
		{name: "short focused", files: files, selected: 0, focus: navigation.FocusNavigator},
		{name: "unicode focused", files: files, selected: 1, focus: navigation.FocusNavigator},
		{name: "long unfocused", files: files, selected: 2, focus: navigation.FocusReader},
		{name: "empty", focus: navigation.FocusNavigator},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame := Render(Model{
				Geometry:       g,
				NavigatorTitle: "Navigator",
				NavigatorRows:  navigatorRows(test.files),
				NavigatorEmpty: Line{Text: "No files", Tone: ToneQuiet},
				Selected:       test.selected,
				Focus:          test.focus,
			})
			width, height := lipgloss.Size(frame)
			if width != g.Screen.Width || height != g.Screen.Height {
				t.Fatalf("render size = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
			}
		})
	}
}

func navigatorRows(labels []string) []NavigatorRow {
	rows := make([]NavigatorRow, len(labels))
	for index, label := range labels {
		rows[index] = NavigatorRow{Identity: label, Label: label}
	}
	return rows
}
