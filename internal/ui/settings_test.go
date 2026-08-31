package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestSettingsPopupRendersCenteredCheckboxAtMinimumSize(t *testing.T) {
	t.Parallel()
	geometry := Calculate(MinimumWidth, MinimumHeight)
	settings := Settings{Open: true, Entries: []SettingEntry{
		{Label: "include comments in hunk navigation ([/])", Enabled: true, Selected: true},
		{Label: "start diffs folded", Enabled: true},
	}}
	frame := Render(Model{Geometry: geometry, Workspace: workspace.Files, Settings: settings})
	width, height := lipgloss.Size(frame)
	if width != MinimumWidth || height != MinimumHeight {
		t.Fatalf("Settings frame = %dx%d, want %dx%d", width, height, MinimumWidth, MinimumHeight)
	}
	plain := ansi.Strip(frame)
	for _, expected := range []string{
		"Settings · ,/esc close",
		"[x] include comments in hunk navigation ([/])",
		"[x] start diffs folded",
	} {
		if !strings.Contains(plain, expected) {
			t.Errorf("Settings popup is missing %q:\n%s", expected, plain)
		}
	}
	footer := strings.TrimSpace(strings.Split(plain, "\n")[geometry.Footer.Y])
	if footer != ",/Esc close" {
		t.Fatalf("open Settings footer = %q, want only close controls", footer)
	}

	popup := renderSettingsPopup(settingsPopupWidth, settings)
	popupWidth, popupHeight := lipgloss.Size(popup)
	if popupWidth != settingsPopupWidth || popupHeight != 4 {
		t.Fatalf("Settings popup = %dx%d, want %dx4", popupWidth, popupHeight, settingsPopupWidth)
	}
	if got, want := centeredPopupRect(geometry.Screen, popupWidth, popupHeight), (Rect{X: 3, Y: 4, Width: 54, Height: 4}); got != want {
		t.Fatalf("centered Settings geometry = %+v, want %+v", got, want)
	}
}

func TestSettingsCheckboxAndSelectionRendering(t *testing.T) {
	t.Parallel()
	selected := renderSettingEntry(SettingEntry{Label: "setting", Enabled: true, Selected: true})
	if selected != selectionStyle(true).Render("[x] setting") {
		t.Fatalf("selected enabled setting = %q", selected)
	}
	unselected := renderSettingEntry(SettingEntry{Label: "setting", Enabled: false})
	if unselected != chromeStyle.Render("[ ] setting") {
		t.Fatalf("unselected disabled setting = %q", unselected)
	}
}
