package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestFooterShowsOnlyAvailableWorkflowActions(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 20)
	frame := Render(Model{
		Geometry:  geometry,
		Workspace: workspace.Files,
		Controls: workspace.Controls{
			MarkdownPreviewEligible: true,
		},
		FileActions: FileFooterActions{Review: true, ReviewBounds: true, NextGap: true},
	})
	footer := strings.Split(frame, "\n")[geometry.Footer.Y]
	plain := ansi.Strip(footer)
	for _, expected := range []string{"m preview", "x review", "R bounds", "X next gap"} {
		if !strings.Contains(plain, expected) {
			t.Errorf("footer is missing available action %q: %q", expected, footer)
		}
	}
	for _, routine := range []string{"tab", "j/k", "h/l", "e edit", "z swap", "[/] hunks"} {
		if strings.Contains(plain, routine) {
			t.Errorf("footer retained routine help action %q: %q", routine, footer)
		}
	}
	if !strings.Contains(footer, headerStyle.Render("x")) ||
		!strings.Contains(footer, chromeStyle.Render(" review")) ||
		!strings.Contains(footer, mutedStyle.Render(" • ")) {
		t.Fatalf("footer lost its key, label, or separator treatment: %q", footer)
	}

	empty := strings.Split(Render(Model{Geometry: geometry, Workspace: workspace.Files}), "\n")[geometry.Footer.Y]
	if got := strings.TrimSpace(ansi.Strip(empty)); got != "?" {
		t.Fatalf("Files footer without specialized actions = %q, want only help", got)
	}
}

func TestNotesFooterKeepsOnlyStatusAndAvailableScope(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 14)
	frame := Render(Model{
		Geometry:         geometry,
		Workspace:        workspace.Notes,
		NotesStatus:      "Ln 1, Col 1  •  saved",
		NotesHasWorktree: true,
	})
	footer := strings.Split(frame, "\n")[geometry.Footer.Y]
	if !strings.Contains(footer, headerStyle.Render("ctrl+t")) || !strings.Contains(footer, chromeStyle.Render(" scope")) ||
		!strings.Contains(footer, "Ln 1, Col 1") || strings.Contains(footer, "Esc Files") || strings.Contains(footer, "tab") {
		t.Fatalf("Notes footer is not truthful: %q", footer)
	}
}
