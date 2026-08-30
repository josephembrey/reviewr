package ui

import (
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestFooterSeparatesKeyLabelAndSeparatorStyles(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 20)
	frame := Render(Model{Geometry: geometry, Workspace: workspace.Files})
	footer := strings.Split(frame, "\n")[geometry.Footer.Y]

	if !strings.Contains(footer, headerStyle.Render("j/k")) {
		t.Fatalf("footer key does not use accent treatment: %q", footer)
	}
	if !strings.Contains(footer, chromeStyle.Render(" move")) {
		t.Fatalf("footer label does not use readable text treatment: %q", footer)
	}
	if !strings.Contains(footer, mutedStyle.Render(" • ")) {
		t.Fatalf("footer separator does not use subdued treatment: %q", footer)
	}
	if !strings.Contains(footer, headerStyle.Render("Tab/Shift+Tab")) || !strings.Contains(footer, chromeStyle.Render(" cycle")) {
		t.Fatalf("footer does not describe bidirectional tab cycling: %q", footer)
	}
}

func TestNotesFooterUsesTruthfulHomeAndScopeKeys(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 14)
	frame := Render(Model{
		Geometry:         geometry,
		Workspace:        workspace.Notes,
		NotesStatus:      "Ln 1, Col 1  •  saved",
		NotesHasWorktree: true,
	})
	footer := strings.Split(frame, "\n")[geometry.Footer.Y]
	if !strings.Contains(footer, headerStyle.Render("Esc")) || !strings.Contains(footer, chromeStyle.Render(" Files")) ||
		!strings.Contains(footer, headerStyle.Render("Tab/Shift+Tab")) || !strings.Contains(footer, chromeStyle.Render(" cycle")) ||
		!strings.Contains(footer, headerStyle.Render("ctrl+t")) || !strings.Contains(footer, chromeStyle.Render(" scope")) ||
		strings.Contains(footer, "1 files") {
		t.Fatalf("Notes footer is not truthful: %q", footer)
	}
}
