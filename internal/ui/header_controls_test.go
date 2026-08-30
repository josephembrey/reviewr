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
	if !strings.HasPrefix(files, "[files|git|notes] [all] [file] [uncommitted]") {
		t.Fatalf("Files header = %q", files)
	}

	controls := workspace.Controls{Files: workspace.ChangedFiles, Reader: workspace.DiffReader, Comparison: workspace.LastTurn}
	files = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(files, "[files|git|notes] [changed] [diff] [last-turn]") {
		t.Fatalf("cycled Files header = %q", files)
	}

	git := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Changes: changes}))
	if !strings.HasPrefix(git, "[files|git|notes] [log] [graph]") {
		t.Fatalf("Git Log header = %q", git)
	}
	controls.Git = workspace.GitRefs
	git = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(git, "[files|git|notes] [refs]") || strings.Contains(git, "[graph]") || strings.Contains(git, "changes") {
		t.Fatalf("Git Refs header = %q", git)
	}
	controls.Git = workspace.GitStashes
	git = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(git, "[files|git|notes] [stashes]") || strings.Contains(git, "[graph]") || strings.Contains(git, "changes") {
		t.Fatalf("Git Stashes header = %q", git)
	}

	notes := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Notes, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(notes, workspaceSwitcher) || strings.Contains(notes, "[refs]") || strings.Contains(notes, "[changed]") {
		t.Fatalf("Notes header = %q", notes)
	}
}

func TestWideHeaderControlsExposeNumberKeys(t *testing.T) {
	t.Parallel()
	plain := ansi.Strip(Render(Model{Geometry: Calculate(120, 1), Workspace: workspace.Files}))
	if !strings.HasPrefix(plain, "[files|git|notes]  1 [all]  2 [file]  3 [uncommitted]") {
		t.Fatalf("wide Files header = %q", plain)
	}
}

func TestSecondaryControlsNeverUseReverseHighlight(t *testing.T) {
	t.Parallel()
	for _, hit := range []HitKind{HitSecondaryControl, HitTertiaryControl, HitComparisonControl, HitDiffHighlightControl} {
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
	visible := layoutHeaderControls(geometry, workspace.Files, controls)
	if len(visible) != 3 {
		t.Fatalf("Files controls = %+v", visible)
	}
	for _, control := range visible {
		if got := geometry.HitTest(control.rect.X, 0, workspace.Files, controls, 0, 0, 0, 0).Kind; got != control.hit {
			t.Fatalf("Files header x=%d hit %v, want %v", control.rect.X, got, control.hit)
		}
	}
	controls.Git = workspace.GitRefs
	if got := len(layoutHeaderControls(geometry, workspace.Git, controls)); got != 1 {
		t.Fatalf("Git Refs controls = %d, want 1", got)
	}
	if got := geometry.HitTest(32, 0, workspace.Git, controls, 0, 0, 0, 0).Kind; got != HitNone {
		t.Fatalf("Git Refs claimed absent tertiary control: %v", got)
	}
}

func TestDiffHighlightControlAndFooterShareEligibilityAndCompleteShedding(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 12)
	controls := workspace.Controls{Reader: workspace.DiffReader, RichDiff: true}
	frame := ansi.Strip(Render(Model{
		Geometry: geometry, Workspace: workspace.Files, Controls: controls,
	}))
	if !strings.Contains(strings.Split(frame, "\n")[0], "4 [sidebar]") || !strings.Contains(strings.Split(frame, "\n")[geometry.Footer.Y], "4 diff highlight") {
		t.Fatalf("eligible Files controls/footer = %q", frame)
	}
	visible := layoutHeaderControls(geometry, workspace.Files, controls)
	if len(visible) != 4 || visible[3].hit != HitDiffHighlightControl {
		t.Fatalf("eligible controls = %+v", visible)
	}
	if hit := geometry.HitTest(visible[3].rect.X, visible[3].rect.Y, workspace.Files, controls, 0, 0, 0, 0); hit.Kind != HitDiffHighlightControl {
		t.Fatalf("diff highlight hit = %+v", hit)
	}
	controls.DiffHighlight = workspace.DiffHighlightBackground
	if plain := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Controls: controls})); !strings.Contains(strings.Split(plain, "\n")[0], "4 [background]") {
		t.Fatalf("background label missing: %q", plain)
	}

	controls.Git = workspace.GitStashes
	stash := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls}))
	if !strings.Contains(strings.Split(stash, "\n")[0], "1 [stashes]") || !strings.Contains(strings.Split(stash, "\n")[0], "4 [background]") {
		t.Fatalf("eligible Stash header = %q", stash)
	}

	controls.RichDiff = false
	ineligible := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Controls: controls}))
	if strings.Contains(ineligible, "sidebar") || strings.Contains(ineligible, "background") || strings.Contains(ineligible, "diff highlight") {
		t.Fatalf("ineligible reader exposed control: %q", ineligible)
	}

	narrowGeometry := Calculate(56, 12)
	controls.RichDiff = true
	narrow := ansi.Strip(Render(Model{Geometry: narrowGeometry, Workspace: workspace.Files, Controls: controls}))
	if strings.Contains(strings.Split(narrow, "\n")[0], "sidebar") || strings.Contains(strings.Split(narrow, "\n")[0], "background") {
		t.Fatalf("narrow header painted partial optional control: %q", narrow)
	}
	for x := 0; x < narrowGeometry.Header.Width; x++ {
		if hit := narrowGeometry.HitTest(x, narrowGeometry.Header.Y, workspace.Files, controls, 0, 0, 0, 0); hit.Kind == HitDiffHighlightControl {
			t.Fatalf("shed control retained hit at x=%d", x)
		}
	}
}
