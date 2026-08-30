//go:build dev

package lab

import (
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
	for _, want := range []string{"lab / switchers", "numbered auxiliary", "drawer key", "notes key", "minimal rail", "2 [all]", "3 [file]", "4 [uncommitted]"} {
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
	press(tea.Key{Code: '2', Text: "2"})
	press(tea.Key{Code: '3', Text: "3"})
	press(tea.Key{Code: '4', Text: "4"})
	if model.selected != 1 || model.destination != destinationGit || model.fileSet != fileSetChanged ||
		model.reader != readerDiff || model.comparison != 1 {
		t.Fatalf("lab state = %+v", model)
	}
	press(tea.Key{Code: '0', Text: "0"})
	if model.destination != destinationScratch {
		t.Fatalf("Scratch preview destination = %d", model.destination)
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
