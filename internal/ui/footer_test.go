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

func TestFilesFooterAddsOnlyImmediatelyRelevantCommentActions(t *testing.T) {
	t.Parallel()
	plain := func(model Model) string {
		model.Geometry = Calculate(120, 20)
		model.Workspace = workspace.Files
		return ansi.Strip(renderFooter(model))
	}

	source := plain(Model{ReaderCommentable: true, FileActions: FileFooterActions{Review: true}})
	for _, expected := range []string{"V select lines", "c comment", "x review"} {
		if !strings.Contains(source, expected) {
			t.Errorf("commentable source footer lacks %q: %q", expected, source)
		}
	}
	for _, routine := range []string{"tab", "j/k move", "h/l less", "e edit", "[/]"} {
		if strings.Contains(source, routine) {
			t.Errorf("commentable source footer restored routine hint %q: %q", routine, source)
		}
	}

	visual := plain(Model{ReaderVisualSelection: true})
	for _, expected := range []string{"c comment range", "esc cancel selection", "j/k extend"} {
		if !strings.Contains(visual, expected) {
			t.Errorf("Visual footer lacks %q: %q", expected, visual)
		}
	}

	composer := plain(Model{ReaderComposingComment: true})
	for _, expected := range []string{"enter save comment", "esc cancel", "alt+enter newline"} {
		if !strings.Contains(composer, expected) {
			t.Errorf("composer footer lacks %q: %q", expected, composer)
		}
	}

	collapsed := plain(Model{ReaderCommentHeader: true})
	expanded := plain(Model{ReaderCommentHeader: true, ReaderCommentExpanded: true})
	if !strings.Contains(collapsed, "l expand comment") || !strings.Contains(expanded, "h collapse comment") {
		t.Fatalf("comment-card footers = collapsed %q expanded %q", collapsed, expanded)
	}
}
