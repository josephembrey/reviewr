package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/ui"
)

func TestHelpRoutesKeyboardAndSharedFooterTarget(t *testing.T) {
	t.Parallel()
	geometry := ui.Calculate(80, 20)
	question := tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"})
	if action, ok := routeModalInput(question, geometry, modalNone, true); !ok || action.Kind != ToggleHelp {
		t.Fatalf("question key = (%+v, %v)", action, ok)
	}
	if action, ok := routeModalInput(question, geometry, modalNone, false); ok {
		t.Fatalf("question mark stole Notes input as (%+v, true)", action)
	}

	target := geometry.FooterHelp
	click := tea.MouseClickMsg(tea.Mouse{X: target.X, Y: target.Y, Button: tea.MouseLeft})
	if action, ok := routeModalInput(click, geometry, modalNone, false); !ok || action.Kind != ToggleHelp {
		t.Fatalf("footer help click = (%+v, %v)", action, ok)
	}

	for _, test := range []struct {
		key  tea.Key
		want ActionKind
	}{
		{key: tea.Key{Code: tea.KeyEscape}, want: ToggleHelp},
		{key: tea.Key{Code: '?', Text: "?"}, want: ToggleHelp},
		{key: tea.Key{Code: ',', Text: ","}, want: ToggleSettings},
		{key: tea.Key{Code: 'q', Text: "q"}, want: Quit},
		{key: tea.Key{Code: 'r', Text: "r"}, want: ActionNone},
	} {
		action, ok := routeModalInput(tea.KeyPressMsg(test.key), geometry, modalHelp, true)
		if !ok || action.Kind != test.want {
			t.Fatalf("open help key %q = (%+v, %v), want %v", test.key.String(), action, ok, test.want)
		}
	}
}

func TestHelpIsTransientModalState(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	if pending := model.apply(Action{Kind: ToggleHelp}); pending.kind != effectNone || model.modal != modalHelp {
		t.Fatalf("open help = pending %+v modal %v", pending, model.modal)
	}
	if pending := model.apply(Action{Kind: ToggleHelp}); pending.kind != effectNone || model.modal != modalNone {
		t.Fatalf("close help = pending %+v modal %v", pending, model.modal)
	}
}

func TestBrowserQuestionKeyOpensRenderedHelp(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	model = next.(Model)
	if command != nil || model.modal != modalHelp || !strings.Contains(model.View().Content, "hotkeys") {
		t.Fatalf("question key = modal %v command=%v", model.modal, command != nil)
	}
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command != nil || model.modal != modalNone || strings.Contains(model.View().Content, "hotkeys") {
		t.Fatalf("escape close = modal %v command=%v", model.modal, command != nil)
	}
}
