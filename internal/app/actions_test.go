package app

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
)

func TestKeyRoutingProducesSemanticActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		key   tea.Key
		focus navigation.Focus
		want  Action
	}{
		{name: "j selects next", key: tea.Key{Code: 'j', Text: "j"}, focus: navigation.FocusNavigator, want: Action{Kind: SelectNext}},
		{name: "down scrolls reader", key: tea.Key{Code: tea.KeyDown}, focus: navigation.FocusReader, want: Action{Kind: ScrollReader, Amount: 1}},
		{name: "k selects previous", key: tea.Key{Code: 'k', Text: "k"}, focus: navigation.FocusNavigator, want: Action{Kind: SelectPrevious}},
		{name: "up scrolls reader", key: tea.Key{Code: tea.KeyUp}, focus: navigation.FocusReader, want: Action{Kind: ScrollReader, Amount: -1}},
		{name: "tab toggles focus", key: tea.Key{Code: tea.KeyTab}, want: Action{Kind: ToggleFocus}},
		{name: "r reloads", key: tea.Key{Code: 'r', Text: "r"}, want: Action{Kind: Reload}},
		{name: "q quits", key: tea.Key{Code: 'q', Text: "q"}, want: Action{Kind: Quit}},
		{name: "ctrl-c quits", key: tea.Key{Code: 'c', Mod: tea.ModCtrl}, want: Action{Kind: Quit}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := routeMessage(tea.KeyPressMsg(test.key), test.focus, ui.Geometry{}, 0, 0)
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeMessage() = (%+v, %v), want (%+v, true)", got, ok, test.want)
			}
		})
	}
}

func TestMouseRoutingPrecedence(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 20)
	rowX, rowY := g.NavigatorRows.X, g.NavigatorRows.Y+1
	tests := []struct {
		name string
		msg  tea.Msg
		want Action
		ok   bool
	}{
		{name: "left click selects visible row", msg: tea.MouseClickMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseLeft}), want: Action{Kind: SelectIndex, Index: 3}, ok: true},
		{name: "wheel on row navigates instead of clicking", msg: tea.MouseWheelMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseWheelDown}), want: Action{Kind: SelectNext}, ok: true},
		{name: "wheel in reader scrolls three", msg: tea.MouseWheelMsg(tea.Mouse{X: g.Reader.X, Y: g.Reader.Y, Button: tea.MouseWheelUp}), want: Action{Kind: ScrollReader, Amount: -3}, ok: true},
		{name: "shared boundary belongs reader", msg: tea.MouseClickMsg(tea.Mouse{X: g.Reader.X, Y: g.Reader.Y, Button: tea.MouseLeft}), want: Action{Kind: FocusReader}, ok: true},
		{name: "right click ignored", msg: tea.MouseClickMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseRight})},
		{name: "right half-open boundary ignored", msg: tea.MouseClickMsg(tea.Mouse{X: g.Screen.Width, Y: g.Reader.Y, Button: tea.MouseLeft})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := routeMessage(test.msg, navigation.FocusNavigator, g, 2, 10)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeMessage() = (%+v, %v), want (%+v, %v)", got, ok, test.want, test.ok)
			}
		})
	}
}
