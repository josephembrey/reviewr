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
	if action, ok := routeHelpInput(question, geometry, false, true); !ok || action.Kind != ToggleHelp {
		t.Fatalf("question key = (%+v, %v)", action, ok)
	}
	if action, ok := routeHelpInput(question, geometry, false, false); ok {
		t.Fatalf("question mark stole Notes input as (%+v, true)", action)
	}

	target := geometry.FooterHelp
	click := tea.MouseClickMsg(tea.Mouse{X: target.X, Y: target.Y, Button: tea.MouseLeft})
	if action, ok := routeHelpInput(click, geometry, false, false); !ok || action.Kind != ToggleHelp {
		t.Fatalf("footer help click = (%+v, %v)", action, ok)
	}

	for _, test := range []struct {
		key  tea.Key
		want ActionKind
	}{
		{key: tea.Key{Code: tea.KeyEscape}, want: ToggleHelp},
		{key: tea.Key{Code: '?', Text: "?"}, want: ToggleHelp},
		{key: tea.Key{Code: 'q', Text: "q"}, want: Quit},
		{key: tea.Key{Code: 'r', Text: "r"}, want: ActionNone},
	} {
		action, ok := routeHelpInput(tea.KeyPressMsg(test.key), geometry, true, true)
		if !ok || action.Kind != test.want {
			t.Fatalf("open help key %q = (%+v, %v), want %v", test.key.String(), action, ok, test.want)
		}
	}
}

func TestHelpIsTransientModalState(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	if pending := model.apply(Action{Kind: ToggleHelp}); pending.kind != effectNone || !model.helpOpen {
		t.Fatalf("open help = pending %+v open %v", pending, model.helpOpen)
	}
	if pending := model.apply(Action{Kind: ToggleHelp}); pending.kind != effectNone || model.helpOpen {
		t.Fatalf("close help = pending %+v open %v", pending, model.helpOpen)
	}
}

func TestBrowserQuestionKeyOpensRenderedHelp(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 80, Height: 20})
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	model = next.(Model)
	if command != nil || !model.helpOpen || !strings.Contains(model.View().Content, "hotkeys") {
		t.Fatalf("question key = open %v command=%v", model.helpOpen, command != nil)
	}
	next, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if command != nil || model.helpOpen || strings.Contains(model.View().Content, "hotkeys") {
		t.Fatalf("escape close = open %v command=%v", model.helpOpen, command != nil)
	}
}
