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
	switchers := map[workspace.Kind]string{
		workspace.Files:   "1 [files] git  | esc  scratch ",
		workspace.Git:     "1  files [git] | esc  scratch ",
		workspace.Scratch: "1 [files] git  | esc [scratch]",
	}
	for width := 0; width <= 54; width++ {
		for _, active := range []workspace.Kind{workspace.Files, workspace.Git, workspace.Scratch} {
			frame := Render(Model{Geometry: Calculate(width, 1), Workspace: active})
			gotWidth, gotHeight := lipgloss.Size(frame)
			if gotWidth != width || gotHeight != 1 {
				t.Fatalf("Render(width=%d, active=%v) size = %dx%d", width, active, gotWidth, gotHeight)
			}
			plain := ansi.Strip(frame)
			switcher := switchers[active]
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
	if !strings.HasPrefix(plain, switchers[workspace.Files]) || !strings.HasSuffix(plain, "0 changes") || strings.Contains(plain, "+0") || strings.Contains(plain, "-0") {
		t.Fatalf("normal header = %q", plain)
	}
	plain = ansi.Strip(Render(Model{Geometry: Calculate(30, 1), Workspace: workspace.Files}))
	if plain != switchers[workspace.Files] {
		t.Fatalf("30-column header = %q, want switcher only", plain)
	}
	plain = ansi.Strip(Render(Model{Geometry: Calculate(31, 1), Workspace: workspace.Files}))
	if plain != switchers[workspace.Files]+" " {
		t.Fatalf("31-column header = %q, want switcher only", plain)
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

func TestWorkspaceSwitcherUsesDrawerBracketsWithoutReverseHighlight(t *testing.T) {
	t.Parallel()
	for _, active := range []workspace.Kind{workspace.Files, workspace.Git, workspace.Scratch} {
		frame := Render(Model{Geometry: Calculate(80, 1), Workspace: active})
		if strings.Contains(frame, "\x1b[7m") {
			t.Fatalf("workspace %v uses reverse-video highlight: %q", active, frame)
		}
	}
}

func TestScratchKeepsInactivePrimaryWorkspaceBracketed(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		primary workspace.Kind
		plain   string
	}{
		{name: "Files", primary: workspace.Files, plain: "1 [files] git  | esc [scratch]"},
		{name: "Git", primary: workspace.Git, plain: "1  files [git] | esc [scratch]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame := Render(Model{
				Geometry:         Calculate(80, 1),
				Workspace:        workspace.Scratch,
				PrimaryWorkspace: test.primary,
			})
			if plain := ansi.Strip(frame); !strings.HasPrefix(plain, test.plain) {
				t.Fatalf("Scratch header = %q, want prefix %q", plain, test.plain)
			}
			primary := workspaceSwitcherRect(test.primary)
			primaryLabel := ansi.Strip(renderWorkspaceSwitcher(primary.X+primary.Width, workspace.Scratch, test.primary))[primary.X : primary.X+primary.Width]
			if strings.Contains(frame, headerStyle.Render(primaryLabel)) {
				t.Fatalf("hidden primary workspace remains highlighted: %q", frame)
			}
			if !strings.Contains(frame, headerStyle.Render("[scratch]")) {
				t.Fatalf("Scratch lacks active highlight: %q", frame)
			}
		})
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
	if strings.Contains(frame, mutedStyle.Render("12 changes ")) {
		t.Fatalf("header change count is incorrectly muted: %q", frame)
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
	if !strings.HasPrefix(plain, "1 [files] git  | esc  scratch") || strings.Contains(plain, "12 changes") {
		t.Fatalf("narrow header = %q", plain)
	}
}

func TestHeaderKeepsChangeTotalsWhenFullSummaryCannotFit(t *testing.T) {
	t.Parallel()
	model := Model{
		Geometry:  Calculate(72, 1),
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
