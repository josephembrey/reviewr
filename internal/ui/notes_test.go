package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/highlight"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestNotesRenderUsesSharedWrapSelectionCursorAndStatus(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 12)
	editor := notes.NewEditor()
	editor.Load("a\t界e\u0301 " + strings.Repeat("wrapped ", 100))
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	editor.MoveHorizontal(5, true)
	frame := Render(Model{
		Geometry:    g,
		Workspace:   workspace.Notes,
		Notes:       editor.Presentation(),
		NotesStatus: "Ln 1, Col 4  •  modified",
	})
	width, height := lipgloss.Size(frame)
	if width != g.Screen.Width || height != g.Screen.Height {
		t.Fatalf("Notes frame = %dx%d, want %dx%d", width, height, g.Screen.Width, g.Screen.Height)
	}
	plain := ansi.Strip(frame)
	if !strings.Contains(plain, "Notes") || !strings.Contains(plain, "a   界é") || !strings.Contains(plain, "Ln 1, Col 4  •  modified") {
		t.Fatalf("Notes frame = %q", plain)
	}
	if !strings.Contains(frame, ";7m") {
		t.Fatalf("Notes selection is not visibly highlighted: %q", frame)
	}
	if !strings.Contains(plain, "▐") && !strings.Contains(plain, "▕") {
		t.Fatalf("wrapped Notes note lacks scrollbar: %q", plain)
	}
}

func TestNotesRenderClipsNarrowUnicodeWithoutControls(t *testing.T) {
	t.Parallel()
	for width := 1; width <= 4; width++ {
		g := Calculate(width, 4)
		editor := notes.NewEditor()
		editor.Load("界\x1b\ne\u0301")
		editor.Resize(g.NotesText.Width, g.NotesText.Height)
		frame := Render(Model{Geometry: g, Workspace: workspace.Notes, Notes: editor.Presentation()})
		gotWidth, gotHeight := lipgloss.Size(frame)
		if gotWidth != width || gotHeight != 4 {
			t.Fatalf("width %d rendered %dx%d: %q", width, gotWidth, gotHeight, frame)
		}
		if strings.Contains(ansi.Strip(frame), "\x1b") {
			t.Fatalf("control survived width %d: %q", width, frame)
		}
	}
}

func TestNotesFittingContentUsesLastColumnWithoutScrollbarPaint(t *testing.T) {
	t.Parallel()
	g := Calculate(40, 8)
	editor := notes.NewEditor()
	editor.Load(strings.Repeat("x", g.NotesRows.Width))
	editor.Resize(g.NotesRows.Width, g.NotesRows.Height)
	frame := Render(Model{Geometry: g, Workspace: workspace.Notes, Notes: editor.Presentation()})
	lines := strings.Split(ansi.Strip(frame), "\n")
	row := []rune(lines[g.NotesRows.Y])
	if got := string(row[g.NotesRows.X : g.NotesRows.X+g.NotesRows.Width]); got != strings.Repeat("x", g.NotesRows.Width) {
		t.Fatalf("fitting Notes row did not use its final column: %q", got)
	}
	if strings.ContainsAny(string(row), "▕▐") {
		t.Fatalf("fitting Notes row painted scrollbar chrome: %q", string(row))
	}
}

func TestNotesOverflowPaintAndHitsUseSharedGeometry(t *testing.T) {
	t.Parallel()
	g := Calculate(50, 10)
	editor := notes.NewEditor()
	editor.Load(strings.Repeat("notes line\n", 30))
	editor.Resize(g.NotesRows.Width, g.NotesRows.Height)
	presentation := editor.Presentation()
	bar, ok := CalculateScrollbar(g.NotesRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("overflowing Notes note produced no scrollbar")
	}
	editor.Resize(bar.Content.Width, bar.Content.Height)
	presentation = editor.Presentation()
	bar, ok = CalculateScrollbar(g.NotesRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("reflowed Notes note produced no scrollbar")
	}
	frame := Render(Model{Geometry: g, Workspace: workspace.Notes, Notes: presentation})
	lines := strings.Split(ansi.Strip(frame), "\n")
	for y := bar.Track.Y; y < bar.Track.Y+bar.Track.Height; y++ {
		cell := []rune(lines[y])[bar.Track.X]
		want := '▕'
		if bar.Thumb.Contains(bar.Thumb.X, y) {
			want = '▐'
		}
		if cell != want {
			t.Fatalf("Notes scrollbar cell (%d,%d) = %q, want %q", bar.Track.X, y, cell, want)
		}
		hit := g.NotesHitTest(bar.Track.X, y, len(presentation.Document.Rows), presentation.Top)
		if hit.Kind != HitNotesScrollbar || hit.GrabOffset != bar.GrabOffset(y) {
			t.Fatalf("Notes scrollbar hit (%d,%d) = %+v", bar.Track.X, y, hit)
		}
	}
}

func TestNotesStatusSanitizesTerminalControlsAndRows(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 6)
	editor := notes.NewEditor()
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	frame := Render(Model{
		Geometry:    g,
		Workspace:   workspace.Notes,
		Notes:       editor.Presentation(),
		NotesStatus: "error\x1b\nrecoverable",
		NotesError:  true,
	})
	plain := ansi.Strip(frame)
	if !strings.Contains(plain, "error␛↵recoverable") {
		t.Fatalf("unsafe Notes status = %q", plain)
	}
	if width, height := lipgloss.Size(frame); width != 60 || height != 6 {
		t.Fatalf("status changed frame size to %dx%d", width, height)
	}
}

func TestNotesScopeTitleAndFooterCollapseOrExposeTogether(t *testing.T) {
	t.Parallel()
	g := Calculate(60, 7)
	editor := notes.NewEditor()
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	base := Model{
		Geometry:    g,
		Workspace:   workspace.Notes,
		Notes:       editor.Presentation(),
		NotesStatus: "Ln 1, Col 1  •  saved",
	}

	primary := Render(base)
	primaryPlain := ansi.Strip(primary)
	if strings.Contains(primaryPlain, "project") || strings.Contains(primaryPlain, "worktree") || strings.Contains(primaryPlain, "ctrl+t") {
		t.Fatalf("primary Notes exposed redundant scopes: %q", primaryPlain)
	}

	base.NotesHasWorktree = true
	base.NotesScope = notes.Project
	project := Render(base)
	projectPlain := ansi.Strip(project)
	if !strings.Contains(projectPlain, "Notes  [project] worktree") || !strings.Contains(projectPlain, "ctrl+t scope") {
		t.Fatalf("project-scoped frame = %q", projectPlain)
	}
	if !strings.Contains(project, headerStyle.Render("[project]")) {
		t.Fatalf("project scope styles are not active/readable: %q", project)
	}

	base.NotesScope = notes.Worktree
	worktree := Render(base)
	worktreePlain := ansi.Strip(worktree)
	if !strings.Contains(worktreePlain, "Notes   project [worktree]") || !strings.Contains(worktreePlain, "ctrl+t scope") {
		t.Fatalf("worktree-scoped frame = %q", worktreePlain)
	}
	if !strings.Contains(worktree, headerStyle.Render("[worktree]")) {
		t.Fatalf("worktree scope styles are not active/readable: %q", worktree)
	}
}

func TestNotesScopeRenderingStaysBoundedAtHostileWidths(t *testing.T) {
	t.Parallel()
	for width := 1; width <= 32; width++ {
		for _, scope := range []notes.Scope{notes.Project, notes.Worktree} {
			g := Calculate(width, 4)
			editor := notes.NewEditor()
			editor.Resize(g.NotesText.Width, g.NotesText.Height)
			frame := Render(Model{
				Geometry:         g,
				Workspace:        workspace.Notes,
				Notes:            editor.Presentation(),
				NotesStatus:      "saved",
				NotesScope:       scope,
				NotesHasWorktree: true,
			})
			gotWidth, gotHeight := lipgloss.Size(frame)
			if gotWidth != width || gotHeight != 4 {
				t.Fatalf("width %d scope %v rendered %dx%d: %q", width, scope, gotWidth, gotHeight, frame)
			}
		}
	}
}

func TestNotesCursorAndSelectionOverrideMarkdownInk(t *testing.T) {
	t.Parallel()
	g := Calculate(20, 5)
	editor := notes.NewEditor()
	editor.Load("abc")
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	editor.MoveHorizontal(1, true)
	presentation := editor.Presentation()
	presentation.Styles = []highlight.Style{
		{Foreground: "1", Bold: true},
		{Foreground: "2", Italic: true},
		{Foreground: "4", Underline: true},
	}
	frame := Render(Model{Geometry: g, Workspace: workspace.Notes, Notes: presentation})
	if !strings.Contains(frame, selectionStyle(true).Render("a")) {
		t.Fatalf("selection did not override syntax ink: %q", frame)
	}
	if !strings.Contains(frame, headerStyle.Reverse(true).Render("b")) {
		t.Fatalf("cursor did not override syntax ink: %q", frame)
	}
	wantPlainInk := renderTextStyle("c", TextStyle{Foreground: "4", Underline: true})
	if !strings.Contains(frame, wantPlainInk) {
		t.Fatalf("unselected Markdown ink missing: %q", frame)
	}
}
