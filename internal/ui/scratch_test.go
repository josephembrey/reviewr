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
