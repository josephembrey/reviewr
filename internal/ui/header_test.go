package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestHeaderRendersPersistentWorkspaceSwitcher(t *testing.T) {
	t.Parallel()
	const switcher = workspaceSwitcher
	for width := 0; width <= 54; width++ {
		for _, active := range []workspace.Kind{workspace.Files, workspace.Git, workspace.Notes} {
			frame := Render(Model{Geometry: Calculate(width, 1), Workspace: active})
			gotWidth, gotHeight := lipgloss.Size(frame)
			if gotWidth != width || gotHeight != 1 {
				t.Fatalf("Render(width=%d, active=%v) size = %dx%d", width, active, gotWidth, gotHeight)
			}
			plain := ansi.Strip(frame)
			visibleSwitcher := switcher[:min(width, len(switcher))]
			if !strings.HasPrefix(plain, visibleSwitcher) {
				t.Fatalf("Render(width=%d) header = %q, want prefix %q", width, plain, visibleSwitcher)
			}
			if strings.Contains(plain, "reviewr") {
				t.Fatalf("Render(width=%d) retained redundant app label: %q", width, plain)
			}
		}
	}

	plain := ansi.Strip(Render(Model{
		Geometry: Calculate(80, 1), Workspace: workspace.Files,
		Changes: ChangeSummary{Files: 12, Additions: 345, Deletions: 67, Ready: true},
	}))
	if !strings.HasPrefix(plain, switcher) || strings.Contains(plain, "changes") || strings.Contains(plain, "+345") || strings.Contains(plain, "-67") {
		t.Fatalf("normal header = %q", plain)
	}
	plain = ansi.Strip(Render(Model{Geometry: Calculate(len(switcher), 1), Workspace: workspace.Files}))
	if plain != switcher {
		t.Fatalf("exact-width header = %q, want switcher only", plain)
	}
	plain = ansi.Strip(Render(Model{Geometry: Calculate(len(switcher)+1, 1), Workspace: workspace.Files}))
	if plain != switcher+" " {
		t.Fatalf("padded header = %q, want switcher only", plain)
	}
}

func TestWorkspaceSwitcherAccentsOnlyTheActiveLabel(t *testing.T) {
	t.Parallel()
	for _, active := range []workspace.Kind{workspace.Files, workspace.Git, workspace.Notes} {
		frame := renderWorkspaceSwitcher(len(workspaceSwitcher), active)
		labels := map[workspace.Kind]string{workspace.Files: "files", workspace.Git: "git", workspace.Notes: "notes"}
		for kind, label := range labels {
			styled := headerStyle.Render(label)
			if got := strings.Contains(frame, styled); got != (kind == active) {
				t.Fatalf("active %v label %q selected=%v frame=%q", active, label, got, frame)
			}
		}
		if strings.Contains(frame, "\x1b[7m") {
			t.Fatalf("active %v uses a reverse-video background: %q", active, frame)
		}
		if plain := ansi.Strip(frame); plain != workspaceSwitcher {
			t.Fatalf("active %v changed stable group: %q", active, plain)
		}
		for _, key := range []string{"g ", "n "} {
			if !strings.Contains(frame, headerStyle.Render(key)) {
				t.Fatalf("active %v lost destination key %q: %q", active, key, frame)
			}
		}
	}
}
