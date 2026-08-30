package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestSettingsRoutesGlobalShortcutAndModalActions(t *testing.T) {
	t.Parallel()
	geometry := ui.Calculate(80, 20)
	tests := []struct {
		name string
		msg  tea.Msg
		open modalKind
		want ActionKind
		ok   bool
	}{
		{name: "global comma", msg: keyPress(','), open: modalNone, want: ToggleSettings, ok: true},
		{name: "comma closes", msg: keyPress(','), open: modalSettings, want: ToggleSettings, ok: true},
		{name: "escape closes", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), open: modalSettings, want: ToggleSettings, ok: true},
		{name: "j selects next", msg: keyPress('j'), open: modalSettings, want: SelectNextSetting, ok: true},
		{name: "down selects next", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), open: modalSettings, want: SelectNextSetting, ok: true},
		{name: "k selects previous", msg: keyPress('k'), open: modalSettings, want: SelectPreviousSetting, ok: true},
		{name: "up selects previous", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), open: modalSettings, want: SelectPreviousSetting, ok: true},
		{name: "space toggles", msg: tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "}), open: modalSettings, want: ToggleSelectedSetting, ok: true},
		{name: "enter toggles", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), open: modalSettings, want: ToggleSelectedSetting, ok: true},
		{name: "workspace key consumed", msg: keyPress('g'), open: modalSettings, want: ActionNone, ok: true},
		{name: "paste consumed", msg: tea.PasteMsg{Content: "text"}, open: modalSettings, want: ActionNone, ok: true},
		{name: "ordinary key without modal", msg: keyPress('g'), open: modalNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			action, ok := routeModalInput(test.msg, geometry, test.open, true)
			if ok != test.ok || action.Kind != test.want {
				t.Fatalf("routeModalInput() = (%+v, %v), want kind %v handled %v", action, ok, test.want, test.ok)
			}
		})
	}
}

func TestSettingsToggleAndListStateAreSessionScoped(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	if !model.settings.includeCommentsInHunkNavigation {
		t.Fatal("include-comments setting did not default on")
	}
	if pending := model.apply(Action{Kind: ToggleSettings}); pending.kind != effectNone || model.modal != modalSettings {
		t.Fatalf("open Settings = effect %+v modal %v", pending, model.modal)
	}
	model.apply(Action{Kind: SelectNextSetting})
	model.apply(Action{Kind: SelectPreviousSetting})
	if model.settings.selected != 0 {
		t.Fatalf("single-entry selection escaped list: %d", model.settings.selected)
	}
	model.apply(Action{Kind: ToggleSelectedSetting})
	if model.settings.includeCommentsInHunkNavigation {
		t.Fatal("Space/Enter action did not disable selected setting")
	}
	model.apply(Action{Kind: ToggleSettings})
	model.apply(Action{Kind: ToggleSettings})
	if model.modal != modalSettings || model.settings.includeCommentsInHunkNavigation {
		t.Fatalf("reopened Settings lost session state: modal=%v settings=%+v", model.modal, model.settings)
	}
}

func TestSettingsModalPrecedencePreservesAllPlaceState(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	model.active = workspace.Notes
	model.files.place = navigation.State{Items: []string{"file-a", "file-b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 4, ReaderCursor: 5}
	model.history.place = navigation.State{Items: []string{"commit-a", "commit-b"}, Selected: 1, Top: 1, Focus: navigation.FocusReader, ReaderOffset: 3}
	model.refs.place = navigation.State{Items: []string{"all", "branch"}, Selected: 1, Focus: navigation.FocusReader, ReaderOffset: 2}
	model.stashes.place = navigation.State{Items: []string{"stash-a", "stash-b"}, Selected: 1, Focus: navigation.FocusReader, ReaderOffset: 1}
	model.note.current().editor.Load("notes stay put")

	filesPlace := model.files.place
	historyPlace := model.history.place
	refsPlace := model.refs.place
	stashesPlace := model.stashes.place
	notesPlace := model.note.current().editor.Presentation()

	update := func(msg tea.Msg) {
		next, command := model.Update(msg)
		model = next.(Model)
		if command != nil {
			t.Fatalf("modal input %T unexpectedly scheduled a command", msg)
		}
	}
	update(keyPress(','))
	if model.modal != modalSettings || !strings.Contains(model.View().Content, "include comments in hunk navigation ([/])") {
		t.Fatalf("global comma did not open rendered Settings: modal=%v", model.modal)
	}
	for _, msg := range []tea.Msg{
		keyPress('j'),
		keyPress('g'),
		tea.PasteMsg{Content: "mutating paste"},
		tea.MouseClickMsg(tea.Mouse{X: model.geometry.HeaderGit.X, Y: model.geometry.HeaderGit.Y, Button: tea.MouseLeft}),
	} {
		update(msg)
	}
	if model.active != workspace.Notes ||
		!reflect.DeepEqual(model.files.place, filesPlace) ||
		!reflect.DeepEqual(model.history.place, historyPlace) ||
		!reflect.DeepEqual(model.refs.place, refsPlace) ||
		!reflect.DeepEqual(model.stashes.place, stashesPlace) ||
		!reflect.DeepEqual(model.note.current().editor.Presentation(), notesPlace) {
		t.Fatalf("Settings input disturbed Continuity: active=%v files=%+v history=%+v refs=%+v stashes=%+v notes=%+v",
			model.active, model.files.place, model.history.place, model.refs.place, model.stashes.place, model.note.current().editor.Presentation())
	}
	update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.modal != modalNone {
		t.Fatalf("Escape left modal open: %v", model.modal)
	}
	update(keyPress(','))
	update(keyPress(','))
	if model.modal != modalNone {
		t.Fatalf("comma did not toggle Settings closed: %v", model.modal)
	}
}

func TestSettingsFilterHunkNavigationLandmarks(t *testing.T) {
	t.Parallel()
	landmarks := []readerNavigationLandmark{
		{row: 8, kind: readerCommentLandmark},
		{row: 2, kind: readerHunkLandmark},
		{row: 11, kind: readerFoldLandmark},
		{row: 5, kind: readerCommentLandmark},
		{row: 5, kind: readerHunkLandmark},
		{row: 5, kind: readerFoldLandmark},
		{row: -1, kind: readerCommentLandmark},
	}
	settings := newSettingsState()
	if got, want := settings.hunkNavigationTargets(landmarks), []int{2, 5, 8, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("enabled landmarks = %v, want %v", got, want)
	}
	settings.includeCommentsInHunkNavigation = false
	if got, want := settings.hunkNavigationTargets(landmarks), []int{2, 5, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disabled landmarks = %v, want %v", got, want)
	}
}

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)})
}
