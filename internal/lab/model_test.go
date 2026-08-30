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
		var handled bool
		model, _, handled = model.Update(tea.KeyPressMsg(key))
		if !handled {
			t.Fatalf("lab did not handle key %q", key.String())
		}
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
	model, _, _ := New().Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	frame := model.View(100, 24)
	plain := ansi.Strip(frame)
	for _, want := range []string{"lab / diff folds", "unchanged gaps", "hunk accordion", "whole-file context", "6 unchanged lines", "return previous", "return current"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("fold lab misses %q:\n%s", want, plain)
		}
	}

	model, _, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model, _, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}))
	if model.foldSelected != 1 || !model.foldExpanded[1] {
		t.Fatalf("fold lab state = %+v", model)
	}
	plain = ansi.Strip(model.View(100, 24))
	if !strings.Contains(plain, "return previous") || !strings.Contains(plain, "return current") || !strings.Contains(plain, "@@ 108-116") {
		t.Fatalf("expanded hunk preview is incomplete:\n%s", plain)
	}
	model, _, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	if model.foldExpanded[1] {
		t.Fatal("h did not collapse selected fold preview")
	}
	plain = ansi.Strip(model.View(100, 24))
	if !strings.Contains(plain, "3 collapsed hunks") || strings.Contains(plain, "func value() string") {
		t.Fatalf("collapsed hunk preview is incoherent:\n%s", plain)
	}
}

func TestLabFoldMotionAnimatesAndReversesTowardLatestTarget(t *testing.T) {
	t.Parallel()
	model := New()
	for range 2 {
		model, _, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
	plain := ansi.Strip(model.View(100, 24))
	for _, want := range []string{"lab / fold motion", "fast · 20ms/row", "8 unchanged lines", "compact 0/8", "return previous", "nextReviewGap"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("fold-motion lab misses %q:\n%s", want, plain)
		}
	}

	var command tea.Cmd
	model, command, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	if model.foldMotionVisible != 1 || model.foldMotionTarget != foldMotionLineCount || command == nil {
		t.Fatalf("opening start = visible %d target %d command=%v", model.foldMotionVisible, model.foldMotionTarget, command != nil)
	}
	openingGeneration := model.foldMotionGeneration
	for model.foldMotionVisible < 4 {
		model, _, _ = model.Update(foldMotionTick{generation: openingGeneration})
	}
	model, command, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if model.foldMotionTarget != 0 || model.foldMotionVisible != 3 || command == nil {
		t.Fatalf("reversal = visible %d target %d command=%v", model.foldMotionVisible, model.foldMotionTarget, command != nil)
	}
	stale := model
	stale, staleCommand, _ := stale.Update(foldMotionTick{generation: openingGeneration})
	if stale.foldMotionVisible != model.foldMotionVisible || staleCommand != nil {
		t.Fatalf("stale opening frame survived reversal: visible %d command=%v", stale.foldMotionVisible, staleCommand != nil)
	}
	closingGeneration := model.foldMotionGeneration
	for model.foldMotionVisible > 0 {
		model, _, _ = model.Update(foldMotionTick{generation: closingGeneration})
	}
	if plain = ansi.Strip(model.View(100, 24)); !strings.Contains(plain, "compact 0/8") || strings.Contains(plain, "frontier := state.Frontier()") {
		t.Fatalf("collapsed motion preview is incoherent:\n%s", plain)
	}
}

func TestLabANSIPaletteShowsEveryTerminalSlot(t *testing.T) {
	t.Parallel()
	model := New()
	for range 3 {
		model, _, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
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
