package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestHeaderControlsFollowActiveWorkspace(t *testing.T) {
	t.Parallel()
	geometry := Calculate(80, 1)
	changes := ChangeSummary{Ready: true}

	files := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Changes: changes}))
	if !strings.HasPrefix(files, "1 [files] git  | esc  scratch  [all] [file] [uncommitted]") {
		t.Fatalf("Files header = %q", files)
	}

	controls := workspace.Controls{Files: workspace.ChangedFiles, Reader: workspace.DiffReader, Comparison: workspace.LastTurn}
	files = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(files, "1 [files] git  | esc  scratch  [changed] [diff] [last-turn]") {
		t.Fatalf("cycled Files header = %q", files)
	}

	git := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Changes: changes}))
	if !strings.HasPrefix(git, "1  files [git] | esc  scratch  [log] [graph]") {
		t.Fatalf("Git Log header = %q", git)
	}
	controls.Git = workspace.GitRefs
	git = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(git, "1  files [git] | esc  scratch  [refs]") || strings.Contains(git, "[graph]") {
		t.Fatalf("Git Refs header = %q", git)
	}

	scratch := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Scratch, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(scratch, "1 [files] git  | esc [scratch]") || strings.Contains(scratch, "[refs]") || strings.Contains(scratch, "[changed]") {
		t.Fatalf("Scratch header = %q", scratch)
	}
}

func TestWideHeaderControlsExposeNumberKeys(t *testing.T) {
	t.Parallel()
	plain := ansi.Strip(Render(Model{Geometry: Calculate(120, 1), Workspace: workspace.Files}))
	if !strings.HasPrefix(plain, "1 [files] git  | esc  scratch   2 [all]  3 [file]  4 [uncommitted]") {
		t.Fatalf("wide Files header = %q", plain)
	}
}

func TestSecondaryControlsNeverUseReverseHighlight(t *testing.T) {
	t.Parallel()
	for _, hit := range []HitKind{HitSecondaryControl, HitTertiaryControl, HitComparisonControl} {
		control := renderHeaderControl(headerControl{hit: hit, key: "2", value: "active"}, true)
		if strings.Contains(control, "\x1b[7m") {
			t.Fatalf("control %v uses reverse-video highlight: %q", hit, control)
		}
	}
}

func TestHeaderControlHitsUsePaintedLayout(t *testing.T) {
	t.Parallel()
	geometry := Calculate(80, 1)
	controls := workspace.Controls{}
	tests := []struct {
		x    int
		want HitKind
	}{
		{x: 31, want: HitSecondaryControl},
		{x: 37, want: HitTertiaryControl},
		{x: 44, want: HitComparisonControl},
		{x: 30, want: HitNone},
	}
	for _, test := range tests {
		if got := geometry.HitTest(test.x, 0, workspace.Files, controls, 0, 0, 0, 0).Kind; got != test.want {
			t.Fatalf("Files header x=%d hit %v, want %v", test.x, got, test.want)
		}
	}
	controls.Git = workspace.GitRefs
	if got := geometry.HitTest(38, 0, workspace.Git, controls, 0, 0, 0, 0).Kind; got != HitNone {
		t.Fatalf("Git Refs claimed absent tertiary control: %v", got)
	}
}
