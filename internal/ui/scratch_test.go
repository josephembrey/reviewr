package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/scratch"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestScratchRenderUsesSharedWrapSelectionCursorAndStatus(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 12)
	editor := scratch.NewEditor()
	editor.Load("a\t界e\u0301 " + strings.Repeat("wrapped ", 100))
	editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
	editor.MoveHorizontal(5, true)
	frame := Render(Model{
		Geometry:         g,
		Workspace:        workspace.Scratch,
		PrimaryWorkspace: workspace.Files,
		Scratch:          editor.Presentation(),
		ScratchStatus:    "Ln 1, Col 4  •  modified",
	})
	width, height := lipgloss.Size(frame)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("Scratch frame = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	plain := ansi.Strip(frame)
	if !strings.Contains(plain, "Scratch") || !strings.Contains(plain, "a   界é") || !strings.Contains(plain, "Ln 1, Col 4  •  modified") {
		t.Fatalf("Scratch frame = %q", plain)
	}
	if !strings.Contains(frame, ";7m") {
		t.Fatalf("Scratch selection is not visibly highlighted: %q", frame)
	}
	if !strings.Contains(plain, "▐") && !strings.Contains(plain, "▕") {
		t.Fatalf("wrapped Scratch note lacks scrollbar: %q", plain)
	}
}

func TestScratchRenderClipsNarrowUnicodeWithoutControls(t *testing.T) {
	t.Parallel()
	for width := 1; width <= 4; width++ {
		g := Calculate(width, 4)
		editor := scratch.NewEditor()
		editor.Load("界\x1b\ne\u0301")
		editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
		frame := Render(Model{Geometry: g, Workspace: workspace.Scratch, Scratch: editor.Presentation()})
		gotWidth, gotHeight := lipgloss.Size(frame)
		if gotWidth != width || gotHeight != 4 {
			t.Fatalf("width %d rendered %dx%d: %q", width, gotWidth, gotHeight, frame)
		}
		if strings.Contains(ansi.Strip(frame), "\x1b") {
			t.Fatalf("control survived width %d: %q", width, frame)
		}
	}
}

func TestScratchFittingContentUsesLastColumnWithoutScrollbarPaint(t *testing.T) {
	t.Parallel()
	g := Calculate(40, 8)
	editor := scratch.NewEditor()
	editor.Load(strings.Repeat("x", g.ScratchRows.Width))
	editor.Resize(g.ScratchRows.Width, g.ScratchRows.Height)
	frame := Render(Model{Geometry: g, Workspace: workspace.Scratch, Scratch: editor.Presentation()})
	lines := strings.Split(ansi.Strip(frame), "\n")
	row := []rune(lines[g.ScratchRows.Y])
	if got := string(row[g.ScratchRows.X : g.ScratchRows.X+g.ScratchRows.Width]); got != strings.Repeat("x", g.ScratchRows.Width) {
		t.Fatalf("fitting Scratch row did not use its final column: %q", got)
	}
	if strings.ContainsAny(string(row), "▕▐") {
		t.Fatalf("fitting Scratch row painted scrollbar chrome: %q", string(row))
	}
}

func TestScratchOverflowPaintAndHitsUseSharedGeometry(t *testing.T) {
	t.Parallel()
	g := Calculate(50, 10)
	editor := scratch.NewEditor()
	editor.Load(strings.Repeat("scratch line\n", 30))
	editor.Resize(g.ScratchRows.Width, g.ScratchRows.Height)
	presentation := editor.Presentation()
	bar, ok := CalculateScrollbar(g.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("overflowing Scratch note produced no scrollbar")
	}
	editor.Resize(bar.Content.Width, bar.Content.Height)
	presentation = editor.Presentation()
	bar, ok = CalculateScrollbar(g.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("reflowed Scratch note produced no scrollbar")
	}
	frame := Render(Model{Geometry: g, Workspace: workspace.Scratch, Scratch: presentation})
	lines := strings.Split(ansi.Strip(frame), "\n")
	for y := bar.Track.Y; y < bar.Track.Y+bar.Track.Height; y++ {
		cell := []rune(lines[y])[bar.Track.X]
		want := '▕'
		if bar.Thumb.Contains(bar.Thumb.X, y) {
			want = '▐'
		}
		if cell != want {
			t.Fatalf("Scratch scrollbar cell (%d,%d) = %q, want %q", bar.Track.X, y, cell, want)
		}
		hit := g.ScratchHitTest(bar.Track.X, y, len(presentation.Document.Rows), presentation.Top)
		if hit.Kind != HitScratchScrollbar || hit.GrabOffset != bar.GrabOffset(y) {
			t.Fatalf("Scratch scrollbar hit (%d,%d) = %+v", bar.Track.X, y, hit)
		}
	}
}

func TestScratchStatusSanitizesTerminalControlsAndRows(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 6)
	editor := scratch.NewEditor()
	editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
	frame := Render(Model{
		Geometry:      g,
		Workspace:     workspace.Scratch,
		Scratch:       editor.Presentation(),
		ScratchStatus: "error\x1b\nrecoverable",
		ScratchError:  true,
	})
	plain := ansi.Strip(frame)
	if !strings.Contains(plain, "error␛↵recoverable") {
		t.Fatalf("unsafe Scratch status = %q", plain)
	}
	if width, height := lipgloss.Size(frame); width != 60 || height != 6 {
		t.Fatalf("status changed frame size to %dx%d", width, height)
	}
}

func TestScratchScopeTitleAndFooterCollapseOrExposeTogether(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 7)
	editor := scratch.NewEditor()
	editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
	base := Model{
		Geometry:      g,
		Workspace:     workspace.Scratch,
		Scratch:       editor.Presentation(),
		ScratchStatus: "Ln 1, Col 1  •  saved",
	}

	primary := Render(base)
	primaryPlain := ansi.Strip(primary)
	if strings.Contains(primaryPlain, "project") || strings.Contains(primaryPlain, "worktree") || strings.Contains(primaryPlain, "ctrl+t") {
		t.Fatalf("primary Scratch exposed redundant scopes: %q", primaryPlain)
	}

	base.ScratchHasWorktree = true
	base.ScratchScope = scratch.Project
	project := Render(base)
	projectPlain := ansi.Strip(project)
	if !strings.Contains(projectPlain, "Scratch  [project] worktree") || !strings.Contains(projectPlain, "ctrl+t scope") {
		t.Fatalf("project-scoped frame = %q", projectPlain)
	}
	if !strings.Contains(project, headerStyle.Render("[project]")) {
		t.Fatalf("project scope styles are not active/readable: %q", project)
	}

	base.ScratchScope = scratch.Worktree
	worktree := Render(base)
	worktreePlain := ansi.Strip(worktree)
	if !strings.Contains(worktreePlain, "Scratch   project [worktree]") || !strings.Contains(worktreePlain, "ctrl+t scope") {
		t.Fatalf("worktree-scoped frame = %q", worktreePlain)
	}
	if !strings.Contains(worktree, headerStyle.Render("[worktree]")) {
		t.Fatalf("worktree scope styles are not active/readable: %q", worktree)
	}
}

func TestScratchScopeRenderingStaysBoundedAtHostileWidths(t *testing.T) {
	t.Parallel()
	for width := 1; width <= 32; width++ {
		for _, scope := range []scratch.Scope{scratch.Project, scratch.Worktree} {
			g := Calculate(width, 4)
			editor := scratch.NewEditor()
			editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
			frame := Render(Model{
				Geometry:           g,
				Workspace:          workspace.Scratch,
				Scratch:            editor.Presentation(),
				ScratchStatus:      "saved",
				ScratchScope:       scope,
				ScratchHasWorktree: true,
			})
			gotWidth, gotHeight := lipgloss.Size(frame)
			if gotWidth != width || gotHeight != 4 {
				t.Fatalf("width %d scope %v rendered %dx%d: %q", width, scope, gotWidth, gotHeight, frame)
			}
		}
	}
}
