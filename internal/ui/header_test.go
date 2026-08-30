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
		Changes: ChangeSummary{Ready: true},
	}))
	if !strings.HasPrefix(plain, switcher) || !strings.HasSuffix(plain, "0 changes") || strings.Contains(plain, "+0") || strings.Contains(plain, "-0") {
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

func TestHeaderOmitsEachZeroChangeTotal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		changes ChangeSummary
		want    string
		absent  string
	}{
		{name: "Additions only", changes: ChangeSummary{Files: 2, Additions: 7, Ready: true}, want: "2 changes +7", absent: "-0"},
		{name: "Deletions only", changes: ChangeSummary{Files: 2, Deletions: 9, Ready: true}, want: "2 changes -9", absent: "+0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plain := ansi.Strip(Render(Model{Geometry: Calculate(80, 1), Workspace: workspace.Files, Changes: test.changes}))
			if !strings.HasSuffix(plain, test.want) || strings.Contains(plain, test.absent) {
				t.Fatalf("header = %q, want suffix %q without %q", plain, test.want, test.absent)
			}
		})
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

func TestHeaderRightAlignsChangeSummary(t *testing.T) {
	t.Parallel()
	model := Model{
		Geometry:  Calculate(80, 1),
		Workspace: workspace.Files,
		Changes:   ChangeSummary{Files: 12, Additions: 345, Deletions: 67, Ready: true},
	}
	frame := Render(model)
	plain := ansi.Strip(frame)
	if !strings.HasSuffix(plain, "12 changes +345 -67") {
		t.Fatalf("header = %q, want right-aligned summary", plain)
	}
	if width := lipgloss.Width(frame); width != 80 {
		t.Fatalf("header width = %d, want 80", width)
	}
	if !strings.Contains(frame, addedStyle.Render("+345")) || !strings.Contains(frame, errorStyle.Render("-67")) {
		t.Fatalf("header stats lack semantic colors: %q", frame)
	}
	if !strings.Contains(frame, mutedStyle.Render("12 changes")) {
		t.Fatalf("header change count lacks muted treatment: %q", frame)
	}
}

func TestHeaderKeepsSwitcherWhenSummaryCannotFit(t *testing.T) {
	t.Parallel()
	model := Model{
		Geometry:  Calculate(40, 1),
		Workspace: workspace.Files,
		Changes:   ChangeSummary{Files: 12, Additions: 345, Deletions: 67, Ready: true},
	}
	plain := ansi.Strip(Render(model))
	if !strings.HasPrefix(plain, workspaceSwitcher) || strings.Contains(plain, "12 changes") {
		t.Fatalf("narrow header = %q", plain)
	}
}

func TestHeaderKeepsChangeTotalsWhenFullSummaryCannotFit(t *testing.T) {
	t.Parallel()
	model := Model{
		Geometry:  Calculate(52, 1),
		Workspace: workspace.Files,
		Controls: workspace.Controls{
			Files:      workspace.ChangedFiles,
			Reader:     workspace.DiffReader,
			Comparison: workspace.LastTurn,
		},
		Changes: ChangeSummary{Files: 12, Additions: 345, Deletions: 67, Ready: true},
	}
	frame := Render(model)
	plain := ansi.Strip(frame)
	if !strings.HasSuffix(plain, "+345 -67") || strings.Contains(plain, "12 changes") {
		t.Fatalf("compact header = %q, want right-aligned change totals", plain)
	}
	if !strings.Contains(frame, addedStyle.Render("+345")) || !strings.Contains(frame, errorStyle.Render("-67")) {
		t.Fatalf("compact header stats lack semantic colors: %q", frame)
	}
}
