package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
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
	if !focusedTitleStyle.GetBold() || quietTitleStyle.GetBold() {
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
	if !strings.Contains(idle, dimStyle.Render("│")) {
		t.Fatalf("idle divider lacks quiet style: %q", idle)
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

func TestTreeRowsRenderStructureSafelyWithinFixedWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  NavigatorRow
		want string
	}{
		{
			name: "expanded directory",
			row:  NavigatorRow{Tree: true, Label: "src", Directory: true, Expanded: true},
			want: "▾ " + openFolderIcon + " src/",
		},
		{
			name: "collapsed directory",
			row:  NavigatorRow{Tree: true, Label: "src", Directory: true},
			want: "▸ " + closedFolderIcon + " src/",
		},
		{
			name: "nested file",
			row:  NavigatorRow{Tree: true, Label: "app\nunsafe.go", Depth: 2},
			want: treeFileIcon("unsafe.go").glyph + " app↵",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, focused := range []bool{true, false} {
				got := renderNavigatorPresentationRow(test.row, 18, true, focused, commitrow.Columns{}, time.Time{})
				if width := lipgloss.Width(got); width != 18 {
					t.Fatalf("row width = %d, want 18: %q", width, got)
				}
				if plain := ansi.Strip(got); !strings.Contains(plain, test.want) {
					t.Fatalf("row = %q, want structure %q", plain, test.want)
				}
			}
		})
	}

	if got := lipgloss.Width(renderNavigatorPresentationRow(
		NavigatorRow{Tree: true, Label: strings.Repeat("deep/", 20) + "file.go", Depth: 8},
		7,
		false,
		false,
		commitrow.Columns{},
		time.Time{},
	)); got != 7 {
		t.Fatalf("narrow clipped tree row width = %d, want 7", got)
	}
}

func TestBontreeTreeRowsPreserveSelectionAndCompactPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  NavigatorRow
		want string
	}{
		{
			name: "source file",
			row:  NavigatorRow{Tree: true, Label: "render.go", Depth: 1},
			want: treeFileIcon("render.go").glyph + " render.go",
		},
		{
			name: "compact directory chain",
			row:  NavigatorRow{Tree: true, Label: "internal/ui", Directory: true, Expanded: true},
			want: openFolderIcon + " internal/ui/",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			unselected := renderNavigatorPresentationRow(test.row, 24, false, false, commitrow.Columns{}, time.Time{})
			for _, focused := range []bool{false, true} {
				selected := renderNavigatorPresentationRow(test.row, 24, true, focused, commitrow.Columns{}, time.Time{})
				if got := lipgloss.Width(selected); got != 24 {
					t.Fatalf("selected row width = %d, want 24", got)
				}
				if got := ansi.Strip(selected); got != ansi.Strip(unselected) {
					t.Fatalf("selection changed row content: selected=%q unselected=%q", got, ansi.Strip(unselected))
				}
				if !strings.Contains(ansi.Strip(selected), test.want) {
					t.Fatalf("selected row = %q, want %q", ansi.Strip(selected), test.want)
				}
				selection := selectionStyle(focused)
				if !strings.HasSuffix(selected, selection.Render(strings.Repeat(" ", 24-lipgloss.Width(strings.TrimRight(ansi.Strip(selected), " "))))) {
					t.Fatalf("selected row does not carry selection through its trailing fill: %q", selected)
				}
			}
		})
	}
}

func TestBontreeTreeRowsClipAtEveryNarrowWidth(t *testing.T) {
	t.Parallel()
	row := NavigatorRow{Tree: true, Label: "internal/ui/render.go", Depth: 8}
	for width := 1; width <= 12; width++ {
		for _, selected := range []bool{false, true} {
			got := renderNavigatorPresentationRow(row, width, selected, true, commitrow.Columns{}, time.Time{})
			if gotWidth := lipgloss.Width(got); gotWidth != width {
				t.Fatalf("width %d selected=%v rendered width = %d: %q", width, selected, gotWidth, got)
			}
		}
	}
}

func TestBontreeStatusSeamKeepsMarkerAndFiletypeIndependent(t *testing.T) {
	t.Parallel()
	row := NavigatorRow{Tree: true, Label: "main.rs"}
	got := renderTreeNavigatorRow(row, 20, treeRowStyleLayers{
		statusMarker: "M",
		statusAccent: treeStatusModified,
	})
	plain := ansi.Strip(got)
	want := " M " + treeFileIcon(row.Label).glyph + " main.rs"
	if !strings.Contains(plain, want) {
		t.Fatalf("decorated row = %q, want independent marker and icon %q", plain, want)
	}
	if !strings.Contains(got, lipgloss.NewStyle().Foreground(fileIconOrangeColor).Render(treeFileIcon(row.Label).glyph)) {
		t.Fatalf("status-decorated Rust row lost its filetype icon color: %q", got)
	}
}

func TestTreeStatusMarkersComposeWithIconsWithoutChangingMouseRows(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 16)
	statuses := []NavigatorStatus{
		StatusModified,
		StatusAdded,
		StatusDeleted,
		StatusRenamed,
		StatusUntracked,
		StatusIgnored,
	}
	markers := []string{"M", "A", "D", "R", "?", "I"}
	rows := make([]NavigatorRow, len(statuses))
	for index, status := range statuses {
		rows[index] = NavigatorRow{
			Identity: "file",
			Label:    "hostile\nname.go",
			Tree:     true,
			Status:   status,
			Dimmed:   status == StatusIgnored,
		}
		row := renderNavigatorPresentationRow(rows[index], 18, false, false, commitrow.Columns{}, time.Time{})
		if width := lipgloss.Width(row); width != 18 {
			t.Fatalf("status %v row width = %d, want 18", status, width)
		}
		plain := ansi.Strip(row)
		want := " " + markers[index] + " " + treeFileIcon(rows[index].Label).glyph + " "
		if !strings.Contains(plain, want) {
			t.Fatalf("status %v row = %q, want marker/icon columns %q", status, plain, want)
		}
	}
	frame := Render(Model{
		Geometry:       g,
		NavigatorRows:  rows,
		NavigatorTitle: "status files",
		ReaderTitle:    "reader",
		ReaderLines:    []Line{{Text: "content"}},
		Focus:          navigation.FocusNavigator,
	})
	if width, height := lipgloss.Size(frame); width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("decorated frame = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	for index := range rows {
		hit := g.HitTest(g.NavigatorRows.X+g.NavigatorRows.Width-1, g.NavigatorRows.Y+index, workspace.Files, workspace.Controls{}, 0, len(rows), 0, 0)
		if hit.Kind != HitNavigatorRow || hit.Index != index {
			t.Fatalf("status row %d hit = %+v", index, hit)
		}
	}
}

func TestTreeRowsCoexistWithNavigatorScrollbar(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 12)
	rows := make([]NavigatorRow, 40)
	for index := range rows {
		rows[index] = NavigatorRow{Identity: "file", Label: "file.go", Tree: true, Depth: index % 4}
	}
	frame := Render(Model{
		Geometry:         g,
		Workspace:        workspace.Files,
		PrimaryWorkspace: workspace.Files,
		NavigatorTitle:   "40 files",
		NavigatorRows:    rows,
		Selected:         8,
		Top:              5,
		Focus:            navigation.FocusNavigator,
		ReaderTitle:      "file.go",
		ReaderLines:      []Line{{Text: "content"}},
	})
	width, height := lipgloss.Size(frame)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("render size = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	if !strings.Contains(ansi.Strip(frame), "▐") {
		t.Fatalf("tree frame lacks scrollbar thumb:\n%s", ansi.Strip(frame))
	}
}

func navigatorRows(labels []string) []NavigatorRow {
	rows := make([]NavigatorRow, len(labels))
	for index, label := range labels {
		rows[index] = NavigatorRow{Identity: label, Label: label}
	}
	return rows
}
