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
				selection := treeSelectionStyle(focused)
				if !strings.HasSuffix(selected, selection.Render(strings.Repeat(" ", 24-lipgloss.Width(strings.TrimRight(ansi.Strip(selected), " "))))) {
					t.Fatalf("selected row does not carry selection through its trailing fill: %q", selected)
				}
			}
		})
	}
}

func TestTreeSelectionUsesOneWhiteBarInsteadOfReversingFileColors(t *testing.T) {
	t.Parallel()
	icon := treeFileIcon("main.go")
	styles := resolveTreeRowStyles(
		NavigatorRow{Tree: true, Label: "main.go", Status: StatusModified},
		icon,
		treeRowStyleLayers{statusAccent: treeStatusModified, selected: true, focused: true},
	)
	if styles.row.GetReverse() {
		t.Fatal("tree selection still reverses token colors")
	}
	if styles.row.GetForeground() != lipgloss.Black || styles.row.GetBackground() != lipgloss.White {
		t.Fatalf("selection colors = foreground %v background %v, want black on white", styles.row.GetForeground(), styles.row.GetBackground())
	}
	assertSameColor(t, styles.marker.GetForeground(), lipgloss.BrightBlue)
	assertSameColor(t, styles.icon.GetForeground(), fileTreeIconColor(icon.tone))
	assertSameColor(t, styles.filename.GetForeground(), lipgloss.BrightBlue)
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
	if !strings.Contains(got, lipgloss.NewStyle().Foreground(vividOrangeColor).Render(treeFileIcon(row.Label).glyph)) {
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
		Geometry:       g,
		Workspace:      workspace.Files,
		NavigatorTitle: "40 files",
		NavigatorRows:  rows,
		Selected:       8,
		Top:            5,
		Focus:          navigation.FocusNavigator,
		ReaderTitle:    "file.go",
		ReaderLines:    []Line{{Text: "content"}},
	})
	width, height := lipgloss.Size(frame)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("render size = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	if !strings.Contains(ansi.Strip(frame), "▐") {
		t.Fatalf("tree frame lacks scrollbar thumb:\n%s", ansi.Strip(frame))
	}
}
