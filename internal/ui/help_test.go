package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestFooterReservesRightmostHelpAndKeepsOnlyLocalHints(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 20)
	models := []Model{
		{Geometry: geometry, Workspace: workspace.Files},
		{Geometry: geometry, Workspace: workspace.Git},
		{Geometry: geometry, Workspace: workspace.Git, Controls: workspace.Controls{Git: workspace.GitStashes}},
		{Geometry: geometry, Workspace: workspace.Notes, NotesStatus: "Ln 1, Col 1"},
	}
	for _, model := range models {
		footer := renderFooter(model)
		plain := ansi.Strip(footer)
		if lipgloss.Width(footer) != geometry.Footer.Width || !strings.HasSuffix(plain, "?") {
			t.Fatalf("workspace %v footer = %q", model.Workspace, plain)
		}
		for _, global := range []string{"q quit", "r refresh"} {
			if strings.Contains(plain, global) {
				t.Fatalf("workspace %v footer retained global hint %q: %q", model.Workspace, global, plain)
			}
		}
	}
	if !geometry.FooterHelp.Contains(geometry.Footer.Width-1, geometry.Footer.Y) ||
		geometry.FooterHelp.Contains(geometry.Footer.Width-2, geometry.Footer.Y) {
		t.Fatalf("footer help target = %+v", geometry.FooterHelp)
	}
}

func TestHelpPopupShowsEveryHotkeyGroupWithoutResizingFrame(t *testing.T) {
	t.Parallel()
	geometry := Calculate(80, 20)
	frame := Render(Model{Geometry: geometry, Workspace: workspace.Files, HelpOpen: true})
	width, height := lipgloss.Size(frame)
	if width != geometry.Screen.Width || height != geometry.Screen.Height {
		t.Fatalf("help frame = %dx%d, want %dx%d", width, height, geometry.Screen.Width, geometry.Screen.Height)
	}
	plain := ansi.Strip(frame)
	for _, expected := range []string{
		"hotkeys · ?/esc close",
		"Browser", "q/ctrl+c quit", "r refresh",
		"Files", "x review", "R bounds", "X gap",
		"Git", "f/F files", "h/l/←→ context",
		"Notes", "ctrl+z/y undo/redo", "backspace/delete edit",
	} {
		if !strings.Contains(plain, expected) {
			t.Errorf("help popup is missing %q", expected)
		}
	}
}

func TestHelpPopupFitsMinimumApplicationSurface(t *testing.T) {
	t.Parallel()
	popup := renderHelpPopup(min(helpPopupWidth, MinimumWidth))
	width, height := lipgloss.Size(popup)
	if width != helpPopupWidth || height > MinimumHeight {
		t.Fatalf("minimum help popup = %dx%d, want %dx<=%d", width, height, helpPopupWidth, MinimumHeight)
	}
	for _, row := range helpRows {
		if rowWidth := lipgloss.Width(renderHelpRow(row)); rowWidth > helpPopupWidth-2 {
			t.Errorf("help row %q is %d cells wide and would be clipped", row.section, rowWidth)
		}
	}
}
