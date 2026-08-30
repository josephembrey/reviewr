//go:build dev

package lab

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLabViewShowsSwitcherAlternativesAtFixedSize(t *testing.T) {
	t.Parallel()
	model := New()
	frame := model.View(100, 24)
	width, height := lipgloss.Size(frame)
	if width != 100 || height != 24 {
		t.Fatalf("lab size = %dx%d, want 100x24", width, height)
	}
	plain := ansi.Strip(frame)
	for _, want := range []string{"lab / switchers", "top-level destinations", "Files controls", "Git controls", "Notes help", "[files|git|notes]", "1 [all]", "2 [file]", "3 [uncommitted]", "4 [sidebar]"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("lab frame lacks %q:\n%s", want, plain)
		}
	}
}

func TestLabControlsExploreWithoutLeavingThePage(t *testing.T) {
	t.Parallel()
	model := New()
	press := func(key tea.Key) {
		model = model.Update(tea.KeyPressMsg(key))
	}
	press(tea.Key{Code: 'j', Text: "j"})
	press(tea.Key{Code: tea.KeyRight})
	press(tea.Key{Code: tea.KeyRight})
	press(tea.Key{Code: '1', Text: "1"})
	press(tea.Key{Code: '2', Text: "2"})
	press(tea.Key{Code: '3', Text: "3"})
	if model.selected != 1 || model.destination != destinationNotes || model.fileSet != fileSetChanged ||
		model.reader != readerDiff || model.comparison != 1 {
		t.Fatalf("lab state = %+v", model)
	}
	press(tea.Key{Code: tea.KeyLeft})
	if model.destination != destinationGit {
		t.Fatalf("Git preview destination = %d", model.destination)
	}
}

func TestLabDiffFoldPagePreviewsThreeInteractiveModels(t *testing.T) {
	t.Parallel()
	model := New().Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	frame := model.View(100, 24)
	plain := ansi.Strip(frame)
	for _, want := range []string{"lab / diff folds", "unchanged gaps", "hunk accordion", "whole-file context", "6 unchanged lines", "return previous", "return current"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("fold lab misses %q:\n%s", want, plain)
		}
	}

	model = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = model.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}))
	if model.foldSelected != 1 || !model.foldExpanded[1] {
		t.Fatalf("fold lab state = %+v", model)
	}
	plain = ansi.Strip(model.View(100, 24))
	if !strings.Contains(plain, "return previous") || !strings.Contains(plain, "return current") || !strings.Contains(plain, "@@ 108-116") {
		t.Fatalf("expanded hunk preview is incomplete:\n%s", plain)
	}
	model = model.Update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	if model.foldExpanded[1] {
		t.Fatal("h did not collapse selected fold preview")
	}
	plain = ansi.Strip(model.View(100, 24))
	if !strings.Contains(plain, "3 collapsed hunks") || strings.Contains(plain, "func value() string") {
		t.Fatalf("collapsed hunk preview is incoherent:\n%s", plain)
	}
}

func TestLabANSIPaletteShowsEveryTerminalSlot(t *testing.T) {
	t.Parallel()
	model := New()
	model = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	frame := model.View(100, 24)
	width, height := lipgloss.Size(frame)
	if width != 100 || height != 24 {
		t.Fatalf("palette lab size = %dx%d, want 100x24", width, height)
	}
	plain := ansi.Strip(frame)
	for index, name := range ansiColorNames {
		want := fmt.Sprintf("%2d %-14s", index, name)
		if occurrences := strings.Count(plain, want); occurrences != 2 {
			t.Fatalf("palette slot %q occurs %d times, want foreground and background:\n%s", want, occurrences, plain)
		}
	}
	if !strings.Contains(frame, "\x1b[90m") || !strings.Contains(frame, "\x1b[100m") {
		t.Fatalf("palette does not emit bright-black foreground/background slots: %q", frame)
	}
}
