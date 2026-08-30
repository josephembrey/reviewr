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
	if !strings.HasPrefix(files, "[files | g git | n notes] [uncommitted]") || strings.Contains(files, "[all]") || strings.Contains(files, "[file]") {
		t.Fatalf("Files header = %q", files)
	}

	controls := workspace.Controls{Files: workspace.ChangedFiles, Reader: workspace.DiffReader, Comparison: workspace.LastTurn}
	files = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Files, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(files, "[files | g git | n notes] [last-turn]") || strings.Contains(files, "[changed]") || strings.Contains(files, "[diff]") {
		t.Fatalf("cycled Files header = %q", files)
	}

	git := ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Changes: changes}))
	if !strings.HasPrefix(git, "[files | g git | n notes] [log] [graph]") {
		t.Fatalf("Git Log header = %q", git)
	}
	controls.Git = workspace.GitRefs
	git = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(git, "[files | g git | n notes] [refs]") || strings.Contains(git, "[graph]") || strings.Contains(git, "changes") {
		t.Fatalf("Git Refs header = %q", git)
	}
	controls.Git = workspace.GitStashes
	git = ansi.Strip(Render(Model{Geometry: geometry, Workspace: workspace.Git, Controls: controls, Changes: changes}))
	if !strings.HasPrefix(git, "[files | g git | n notes] [stashes]") || strings.Contains(git, "[graph]") || strings.Contains(git, "changes") {
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
	if !strings.HasPrefix(plain, "[files | g git | n notes]  3 [uncommitted]") || strings.Contains(plain, "1 [all]") || strings.Contains(plain, "2 [file]") {
		t.Fatalf("wide Files header = %q", plain)
	}
}

func TestFileScopeAndReaderModeLiveOnTheirPaneHeaders(t *testing.T) {
	t.Parallel()
	geometry := Calculate(100, 14)
	controls := workspace.Controls{Files: workspace.ChangedFiles, Reader: workspace.DiffReader}
	model := Model{
		Geometry: geometry, Workspace: workspace.Files, Controls: controls,
		NavigatorTitle: "12 files", ReaderTitle: "src/main.go  [diff]",
		Changes: ChangeSummary{Files: 12, Additions: 345, Deletions: 67, Ready: true},
	}
	rendered := Render(model)
	frame := ansi.Strip(rendered)
	line := strings.Split(frame, "\n")[geometry.Body.Y]
	cells := []rune(line)
	navigator := string(cells[geometry.NavigatorTitle.X : geometry.NavigatorTitle.X+geometry.NavigatorTitle.Width])
	reader := string(cells[geometry.ReaderTitle.X : geometry.ReaderTitle.X+geometry.ReaderTitle.Width])
	if !strings.HasPrefix(navigator, "12 changes +345 -67") || !strings.HasSuffix(navigator, "1 [changed]") {
		t.Fatalf("navigator title = %q", navigator)
	}
	if !strings.Contains(rendered, mutedStyle.Render("12 changes")) || !strings.Contains(rendered, addedStyle.Render("+345")) || !strings.Contains(rendered, errorStyle.Render("-67")) {
		t.Fatalf("navigator change summary lacks semantic colors: %q", rendered)
	}
	if !strings.HasPrefix(reader, "src/main.go  [diff]") || !strings.HasSuffix(reader, "2 [diff]") {
		t.Fatalf("reader title = %q", reader)
	}

	paneControls := layoutPaneHeaderControls(geometry, workspace.Files, controls)
	for _, control := range []headerControl{paneControls.navigator, paneControls.reader} {
		title := geometry.NavigatorTitle
		if control.hit == HitTertiaryControl {
			title = geometry.ReaderTitle
		}
		if control.rect.X+control.rect.Width != title.X+title.Width {
			t.Fatalf("pane control is not right-aligned: %+v", control)
		}
		if hit := geometry.HitTest(control.rect.X, control.rect.Y, workspace.Files, controls, 0, 0, 0, 0); hit.Kind != control.hit {
			t.Fatalf("pane control hit = %v, want %v", hit.Kind, control.hit)
		}
	}

	model.Controls.Files = workspace.AllFiles
	frame = ansi.Strip(Render(model))
	line = strings.Split(frame, "\n")[geometry.Body.Y]
	cells = []rune(line)
	navigator = string(cells[geometry.NavigatorTitle.X : geometry.NavigatorTitle.X+geometry.NavigatorTitle.Width])
	if !strings.HasPrefix(navigator, "12 files") || !strings.HasSuffix(navigator, "1 [all]") || strings.Contains(navigator, "changes") || strings.Contains(navigator, "+345") || strings.Contains(navigator, "-67") {
		t.Fatalf("all-files navigator title = %q", navigator)
	}
}

func TestChangedFilesPaneOmitsZeroChangeTotals(t *testing.T) {
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
			geometry := Calculate(100, 14)
			frame := ansi.Strip(Render(Model{
				Geometry: geometry, Workspace: workspace.Files,
				Controls: workspace.Controls{Files: workspace.ChangedFiles},
				Changes:  test.changes,
			}))
			line := strings.Split(frame, "\n")[geometry.Body.Y]
			navigator := string([]rune(line)[geometry.NavigatorTitle.X : geometry.NavigatorTitle.X+geometry.NavigatorTitle.Width])
			if !strings.HasPrefix(navigator, test.want) || strings.Contains(navigator, test.absent) {
				t.Fatalf("navigator title = %q, want prefix %q without %q", navigator, test.want, test.absent)
			}
		})
	}
}

func TestBrowserFooterAdvertisesPaneFocus(t *testing.T) {
	t.Parallel()
	for _, active := range []workspace.Kind{workspace.Files, workspace.Git} {
		plain := ansi.Strip(renderFooter(Model{Geometry: Calculate(120, 20), Workspace: active}))
		if !strings.HasPrefix(plain, "tab focus") {
			t.Fatalf("workspace %v footer = %q, want pane-focus hint first", active, plain)
		}
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
	if len(visible) != 1 || visible[0].hit != HitComparisonControl {
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
	if got := geometry.HitTest(geometry.HeaderSwitcher.Width+11, 0, workspace.Git, controls, 0, 0, 0, 0).Kind; got != HitNone {
		t.Fatalf("Git Refs claimed absent tertiary control: %v", got)
	}
}

func TestDiffHighlightControlStaysInHeaderAndShedsCompletely(t *testing.T) {
	t.Parallel()
	geometry := Calculate(120, 12)
	controls := workspace.Controls{Reader: workspace.DiffReader, RichDiff: true}
	frame := ansi.Strip(Render(Model{
		Geometry: geometry, Workspace: workspace.Files, Controls: controls,
	}))
	if !strings.Contains(strings.Split(frame, "\n")[0], "4 [sidebar]") || strings.Contains(strings.Split(frame, "\n")[geometry.Footer.Y], "4 diff highlight") {
		t.Fatalf("diff highlight control was not confined to the header: %q", frame)
	}
	visible := layoutHeaderControls(geometry, workspace.Files, controls)
	if len(visible) != 2 || visible[1].hit != HitDiffHighlightControl {
		t.Fatalf("eligible controls = %+v", visible)
	}
	if hit := geometry.HitTest(visible[1].rect.X, visible[1].rect.Y, workspace.Files, controls, 0, 0, 0, 0); hit.Kind != HitDiffHighlightControl {
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

	narrowGeometry := Calculate(42, 12)
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
