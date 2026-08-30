package app

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
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
		{name: "l expands directory", key: tea.Key{Code: 'l', Text: "l"}, focus: navigation.FocusNavigator, want: Action{Kind: ExpandDirectory}},
		{name: "right expands directory", key: tea.Key{Code: tea.KeyRight}, focus: navigation.FocusNavigator, want: Action{Kind: ExpandDirectory}},
		{name: "h collapses directory", key: tea.Key{Code: 'h', Text: "h"}, focus: navigation.FocusNavigator, want: Action{Kind: CollapseDirectory}},
		{name: "left collapses directory", key: tea.Key{Code: tea.KeyLeft}, focus: navigation.FocusNavigator, want: Action{Kind: CollapseDirectory}},
		{name: "tab toggles focus", key: tea.Key{Code: tea.KeyTab}, want: Action{Kind: ToggleFocus}},
		{name: "one toggles primary workspace", key: tea.Key{Code: '1', Text: "1"}, want: Action{Kind: ToggleWorkspace}},
		{name: "escape toggles scratch", key: tea.Key{Code: tea.KeyEscape}, want: Action{Kind: ToggleScratch}},
		{name: "two toggles secondary", key: tea.Key{Code: '2', Text: "2"}, want: Action{Kind: ToggleSecondary}},
		{name: "three toggles tertiary", key: tea.Key{Code: '3', Text: "3"}, want: Action{Kind: ToggleTertiary}},
		{name: "four cycles comparison", key: tea.Key{Code: '4', Text: "4"}, want: Action{Kind: ToggleComparison}},
		{name: "r reloads", key: tea.Key{Code: 'r', Text: "r"}, want: Action{Kind: Reload}},
		{name: "q quits", key: tea.Key{Code: 'q', Text: "q"}, want: Action{Kind: Quit}},
		{name: "ctrl-c quits", key: tea.Key{Code: 'c', Mod: tea.ModCtrl}, want: Action{Kind: Quit}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := routeMessage(tea.KeyPressMsg(test.key), test.focus, ui.Geometry{}, workspace.Files, workspace.Controls{}, false, false, 0, 0, 0, 0)
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeMessage() = (%+v, %v), want (%+v, true)", got, ok, test.want)
			}
		})
	}
	if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}), navigation.FocusNavigator, ui.Geometry{}, workspace.Files, workspace.Controls{}, false, false, 0, 0, 0, 0); ok {
		t.Fatalf("retired Scratch key routed as (%+v, true)", got)
	}
	for _, test := range []struct {
		name   string
		focus  navigation.Focus
		active workspace.Kind
	}{
		{name: "reader focus", focus: navigation.FocusReader, active: workspace.Files},
		{name: "Git workspace", focus: navigation.FocusNavigator, active: workspace.Git},
	} {
		t.Run("fold keys ignore "+test.name, func(t *testing.T) {
			t.Parallel()
			if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}), test.focus, ui.Geometry{}, test.active, workspace.Controls{}, false, false, 0, 0, 0, 0); ok {
				t.Fatalf("h routed as (%+v, true)", got)
			}
			if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}), test.focus, ui.Geometry{}, test.active, workspace.Controls{}, false, false, 0, 0, 0, 0); ok {
				t.Fatalf("l routed as (%+v, true)", got)
			}
		})
	}
}

func TestMouseRoutingPrecedence(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 20)
	rowX, rowY := g.NavigatorRows.X, g.NavigatorRows.Y+1
	navigatorBar, _ := ui.CalculateScrollbar(g.NavigatorRows, 30, 2)
	readerBar, _ := ui.CalculateScrollbar(g.ReaderRows, 40, 5)
	tests := []struct {
		name       string
		msg        tea.Msg
		want       Action
		ok         bool
		drag       bool
		scrollDrag bool
		files      int
	}{
		{name: "left click activates visible row", msg: tea.MouseClickMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseLeft}), want: Action{Kind: ActivateNavigatorRow, Index: 3}, ok: true},
		{name: "files label selects Files", msg: tea.MouseClickMsg(tea.Mouse{X: g.HeaderFiles.X, Y: g.HeaderFiles.Y, Button: tea.MouseLeft}), want: Action{Kind: ShowFiles}, ok: true},
		{name: "git label selects Git", msg: tea.MouseClickMsg(tea.Mouse{X: g.HeaderGit.X, Y: g.HeaderGit.Y, Button: tea.MouseLeft}), want: Action{Kind: ShowGit}, ok: true},
		{name: "scratch label selects Scratch", msg: tea.MouseClickMsg(tea.Mouse{X: g.HeaderScratch.X, Y: g.HeaderScratch.Y, Button: tea.MouseLeft}), want: Action{Kind: ShowScratch}, ok: true},
		{name: "secondary control cycles", msg: tea.MouseClickMsg(tea.Mouse{X: 31, Y: g.Header.Y, Button: tea.MouseLeft}), want: Action{Kind: ToggleSecondary}, ok: true},
		{name: "tertiary control cycles", msg: tea.MouseClickMsg(tea.Mouse{X: 37, Y: g.Header.Y, Button: tea.MouseLeft}), want: Action{Kind: ToggleTertiary}, ok: true},
		{name: "comparison control cycles", msg: tea.MouseClickMsg(tea.Mouse{X: 44, Y: g.Header.Y, Button: tea.MouseLeft}), want: Action{Kind: ToggleComparison}, ok: true},
		{name: "switcher separator is neutral", msg: tea.MouseClickMsg(tea.Mouse{X: 15, Y: g.Header.Y, Button: tea.MouseLeft})},
		{name: "header gap is neutral", msg: tea.MouseClickMsg(tea.Mouse{X: 30, Y: g.Header.Y, Button: tea.MouseLeft})},
		{name: "wheel on workspace label is neutral", msg: tea.MouseWheelMsg(tea.Mouse{X: g.HeaderGit.X, Y: g.HeaderGit.Y, Button: tea.MouseWheelDown})},
		{name: "wheel on row navigates instead of clicking", msg: tea.MouseWheelMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseWheelDown}), want: Action{Kind: SelectNext}, ok: true},
		{name: "navigator title focuses navigator", msg: tea.MouseClickMsg(tea.Mouse{X: g.NavigatorTitle.X, Y: g.NavigatorTitle.Y, Button: tea.MouseLeft}), want: Action{Kind: FocusNavigator}, ok: true},
		{name: "empty navigator surface focuses navigator", msg: tea.MouseClickMsg(tea.Mouse{X: g.NavigatorRows.X, Y: g.NavigatorRows.Y + 11, Button: tea.MouseLeft}), want: Action{Kind: FocusNavigator}, ok: true},
		{name: "reader title focuses reader", msg: tea.MouseClickMsg(tea.Mouse{X: g.ReaderTitle.X, Y: g.ReaderTitle.Y, Button: tea.MouseLeft}), want: Action{Kind: FocusReader}, ok: true},
		{name: "wheel in reader surface scrolls three", msg: tea.MouseWheelMsg(tea.Mouse{X: g.ReaderRows.X, Y: g.ReaderRows.Y, Button: tea.MouseWheelUp}), want: Action{Kind: ScrollReader, Amount: -3}, ok: true},
		{name: "divider starts pane resize", msg: tea.MouseClickMsg(tea.Mouse{X: g.Divider.X, Y: g.Divider.Y, Button: tea.MouseLeft}), want: Action{Kind: StartPaneResize}, ok: true},
		{name: "navigator scrollbar starts drag", msg: tea.MouseClickMsg(tea.Mouse{X: navigatorBar.Thumb.X, Y: navigatorBar.Thumb.Y, Button: tea.MouseLeft}), want: Action{Kind: StartScrollbarDrag, Pane: navigation.FocusNavigator, Position: navigatorBar.Thumb.Y, Grab: 0}, ok: true, files: 30},
		{name: "reader scrollbar starts drag", msg: tea.MouseClickMsg(tea.Mouse{X: readerBar.Thumb.X, Y: readerBar.Thumb.Y, Button: tea.MouseLeft}), want: Action{Kind: StartScrollbarDrag, Pane: navigation.FocusReader, Position: readerBar.Thumb.Y, Grab: 0}, ok: true},
		{name: "left drag moves divider", msg: tea.MouseMotionMsg(tea.Mouse{X: 47, Y: g.Divider.Y, Button: tea.MouseLeft}), want: Action{Kind: ResizePanes, Position: 47}, ok: true, drag: true},
		{name: "left drag moves scrollbar", msg: tea.MouseMotionMsg(tea.Mouse{X: readerBar.Thumb.X, Y: 12, Button: tea.MouseLeft}), want: Action{Kind: DragScrollbar, Position: 12}, ok: true, scrollDrag: true},
		{name: "motion without drag is neutral", msg: tea.MouseMotionMsg(tea.Mouse{X: 47, Y: g.Divider.Y, Button: tea.MouseLeft})},
		{name: "release finishes pane resize", msg: tea.MouseReleaseMsg(tea.Mouse{X: 47, Y: g.Divider.Y, Button: tea.MouseLeft}), want: Action{Kind: FinishPaneResize}, ok: true, drag: true},
		{name: "release finishes scrollbar drag", msg: tea.MouseReleaseMsg(tea.Mouse{X: readerBar.Thumb.X, Y: 12, Button: tea.MouseLeft}), want: Action{Kind: FinishScrollbarDrag}, ok: true, scrollDrag: true},
		{name: "release without drag is neutral", msg: tea.MouseReleaseMsg(tea.Mouse{X: 47, Y: g.Divider.Y, Button: tea.MouseLeft})},
		{name: "wheel on divider is neutral", msg: tea.MouseWheelMsg(tea.Mouse{X: g.Divider.X, Y: g.Divider.Y, Button: tea.MouseWheelDown})},
		{name: "right click ignored", msg: tea.MouseClickMsg(tea.Mouse{X: rowX, Y: rowY, Button: tea.MouseRight})},
		{name: "right half-open boundary ignored", msg: tea.MouseClickMsg(tea.Mouse{X: g.Screen.Width, Y: g.Reader.Y, Button: tea.MouseLeft})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileCount := test.files
			if fileCount == 0 {
				fileCount = 10
			}
			got, ok := routeMessage(test.msg, navigation.FocusNavigator, g, workspace.Files, workspace.Controls{}, test.drag, test.scrollDrag, 2, fileCount, 5, 40)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeMessage() = (%+v, %v), want (%+v, %v)", got, ok, test.want, test.ok)
			}
		})
	}
}
