package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/scratch"
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
		{name: "z swaps panes", key: tea.Key{Code: 'z', Text: "z"}, want: Action{Kind: SwapPanes}},
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

func TestScratchRoutingIsModelessAndSemantic(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 12)
	editor := scratch.NewEditor()
	editor.Load(strings.Repeat("line\n", 30))
	editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
	presentation := editor.Presentation()
	tests := []struct {
		name string
		msg  tea.Msg
		want Action
		ok   bool
	}{
		{name: "h inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}), want: Action{Kind: ScratchInsert, Text: "h"}, ok: true},
		{name: "z inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}), want: Action{Kind: ScratchInsert, Text: "z"}, ok: true},
		{name: "q inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}), want: Action{Kind: ScratchInsert, Text: "q"}, ok: true},
		{name: "two inserts", msg: tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}), want: Action{Kind: ScratchInsert, Text: "2"}, ok: true},
		{name: "one closes", msg: tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}), want: Action{Kind: ToggleWorkspace}, ok: true},
		{name: "escape closes", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), want: Action{Kind: ToggleScratch}, ok: true},
		{name: "ctrl c quits", msg: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), want: Action{Kind: Quit}, ok: true},
		{name: "shift left selects", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft, Mod: tea.ModShift}), want: Action{Kind: ScratchMoveLeft, Selecting: true}, ok: true},
		{name: "ctrl right words", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}), want: Action{Kind: ScratchMoveWordRight}, ok: true},
		{name: "home", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), want: Action{Kind: ScratchMoveHome}, ok: true},
		{name: "page down", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}), want: Action{Kind: ScratchPageDown}, ok: true},
		{name: "enter", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), want: Action{Kind: ScratchInsert, Text: "\n"}, ok: true},
		{name: "tab", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), want: Action{Kind: ScratchInsert, Text: "\t"}, ok: true},
		{name: "delete", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}), want: Action{Kind: ScratchDelete}, ok: true},
		{name: "undo", msg: tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}), want: Action{Kind: ScratchUndo}, ok: true},
		{name: "redo", msg: tea.KeyPressMsg(tea.Key{Code: 'y', Mod: tea.ModCtrl}), want: Action{Kind: ScratchRedo}, ok: true},
		{name: "paste", msg: tea.PasteMsg{Content: "a\nb"}, want: Action{Kind: ScratchInsert, Text: "a\nb"}, ok: true},
		{name: "click text", msg: tea.MouseClickMsg(tea.Mouse{X: g.ScratchText.X + 3, Y: g.ScratchText.Y + 2, Button: tea.MouseLeft}), want: Action{Kind: ScratchBeginSelection, X: 3, Y: 2}, ok: true},
		{name: "wheel text", msg: tea.MouseWheelMsg(tea.Mouse{X: g.ScratchText.X, Y: g.ScratchText.Y, Button: tea.MouseWheelDown}), want: Action{Kind: ScratchScroll, Amount: 3}, ok: true},
		{name: "drag selection", msg: tea.MouseMotionMsg(tea.Mouse{X: g.ScratchText.X + 4, Y: g.ScratchText.Y + 3, Button: tea.MouseLeft}), want: Action{Kind: ScratchDragSelection, X: 4, Y: 3}, ok: true},
		{name: "release selection", msg: tea.MouseReleaseMsg(tea.Mouse{Button: tea.MouseLeft}), want: Action{Kind: ScratchEndSelection}, ok: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selectionDragging := strings.Contains(test.name, "selection")
			got, ok := routeScratchMessage(test.msg, g, presentation, selectionDragging, false)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeScratchMessage() = (%+v, %v), want (%+v, %v)", got, ok, test.want, test.ok)
			}
		})
	}

	bar, ok := ui.CalculateScrollbar(g.ScratchRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("long Scratch note has no scrollbar")
	}
	got, routed := routeScratchMessage(tea.MouseClickMsg(tea.Mouse{X: bar.Thumb.X, Y: bar.Thumb.Y, Button: tea.MouseLeft}), g, presentation, false, false)
	if !routed || got.Kind != StartScratchScrollbarDrag || got.Position != bar.Thumb.Y {
		t.Fatalf("scratch scrollbar click = (%+v, %v)", got, routed)
	}
}

func TestScratchScopeKeyboardAndMouseRouting(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 12)
	editor := scratch.NewEditor()
	editor.Resize(g.ScratchText.Width, g.ScratchText.Height)
	presentation := editor.Presentation()
	ctrlT := tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl})
	if got, ok := routeScratchMessage(ctrlT, g, presentation, false, false); ok {
		t.Fatalf("primary ctrl+t routed as (%+v, true)", got)
	}
	if got, ok := routeScratchMessage(ctrlT, g, presentation, false, false, true); !ok || got.Kind != ToggleScratchScope {
		t.Fatalf("linked ctrl+t = (%+v, %v)", got, ok)
	}
	if got, ok := routeScratchMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), g, presentation, false, false, true); !ok || got != (Action{Kind: ScratchInsert, Text: "\t"}) {
		t.Fatalf("linked Tab = (%+v, %v)", got, ok)
	}
	for _, test := range []struct {
		name string
		rect ui.Rect
		want ActionKind
	}{
		{name: "project", rect: g.ScratchProjectScope, want: SelectProjectScratch},
		{name: "worktree", rect: g.ScratchWorktreeScope, want: SelectWorktreeScratch},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			msg := tea.MouseClickMsg(tea.Mouse{X: test.rect.X + test.rect.Width/2, Y: test.rect.Y, Button: tea.MouseLeft})
			if got, ok := routeScratchMessage(msg, g, presentation, false, false, true); !ok || got.Kind != test.want {
				t.Fatalf("scope click = (%+v, %v), want %v", got, ok, test.want)
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
