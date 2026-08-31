package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
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
		{name: "down moves reader selection", key: tea.Key{Code: tea.KeyDown}, focus: navigation.FocusReader, want: Action{Kind: MoveReaderSelection, Amount: 1}},
		{name: "k selects previous", key: tea.Key{Code: 'k', Text: "k"}, focus: navigation.FocusNavigator, want: Action{Kind: SelectPrevious}},
		{name: "up moves reader selection", key: tea.Key{Code: tea.KeyUp}, focus: navigation.FocusReader, want: Action{Kind: MoveReaderSelection, Amount: -1}},
		{name: "l expands navigator selection", key: tea.Key{Code: 'l', Text: "l"}, focus: navigation.FocusNavigator, want: Action{Kind: ExpandNavigatorSelection}},
		{name: "right expands navigator selection", key: tea.Key{Code: tea.KeyRight}, focus: navigation.FocusNavigator, want: Action{Kind: ExpandNavigatorSelection}},
		{name: "h collapses navigator selection", key: tea.Key{Code: 'h', Text: "h"}, focus: navigation.FocusNavigator, want: Action{Kind: CollapseNavigatorSelection}},
		{name: "left collapses navigator selection", key: tea.Key{Code: tea.KeyLeft}, focus: navigation.FocusNavigator, want: Action{Kind: CollapseNavigatorSelection}},
		{name: "tab focuses reader", key: tea.Key{Code: tea.KeyTab}, focus: navigation.FocusNavigator, want: Action{Kind: FocusReader}},
		{name: "tab focuses navigator", key: tea.Key{Code: tea.KeyTab}, focus: navigation.FocusReader, want: Action{Kind: FocusNavigator}},
		{name: "g opens Git", key: tea.Key{Code: 'g', Text: "g"}, want: Action{Kind: ShowGit}},
		{name: "reader home selects start", key: tea.Key{Code: tea.KeyHome}, focus: navigation.FocusReader, want: Action{Kind: SelectReaderBoundary, Amount: -1}},
		{name: "reader end selects end", key: tea.Key{Code: tea.KeyEnd}, focus: navigation.FocusReader, want: Action{Kind: SelectReaderBoundary, Amount: 1}},
		{name: "reader H selects viewport top", key: tea.Key{Code: 'H', Text: "H"}, focus: navigation.FocusReader, want: Action{Kind: SelectReaderViewport, Amount: -1}},
		{name: "reader M selects viewport middle", key: tea.Key{Code: 'M', Text: "M"}, focus: navigation.FocusReader, want: Action{Kind: SelectReaderViewport}},
		{name: "reader L selects viewport bottom", key: tea.Key{Code: 'L', Text: "L"}, focus: navigation.FocusReader, want: Action{Kind: SelectReaderViewport, Amount: 1}},
		{name: "reader page up moves page", key: tea.Key{Code: tea.KeyPgUp}, focus: navigation.FocusReader, want: Action{Kind: MoveReaderPage, Amount: -1}},
		{name: "reader page down moves page", key: tea.Key{Code: tea.KeyPgDown}, focus: navigation.FocusReader, want: Action{Kind: MoveReaderPage, Amount: 1}},
		{name: "n opens Notes", key: tea.Key{Code: 'n', Text: "n"}, want: Action{Kind: ShowNotes}},
		{name: "e opens selected file in editor", key: tea.Key{Code: 'e', Text: "e"}, want: Action{Kind: OpenEditor}},
		{name: "z swaps panes", key: tea.Key{Code: 'z', Text: "z"}, want: Action{Kind: SwapPanes}},
		{name: "one toggles secondary", key: tea.Key{Code: '1', Text: "1"}, want: Action{Kind: ToggleSecondary}},
		{name: "two toggles tertiary", key: tea.Key{Code: '2', Text: "2"}, want: Action{Kind: ToggleTertiary}},
		{name: "three cycles comparison", key: tea.Key{Code: '3', Text: "3"}, want: Action{Kind: ToggleComparison}},
		{name: "r reloads", key: tea.Key{Code: 'r', Text: "r"}, want: Action{Kind: Reload}},
		{name: "q quits", key: tea.Key{Code: 'q', Text: "q"}, want: Action{Kind: Quit}},
		{name: "ctrl-c quits", key: tea.Key{Code: 'c', Mod: tea.ModCtrl}, want: Action{Kind: Quit}},
	}
	for _, key := range []tea.Key{
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: 'G', Text: "G"},
		{Code: 'u', Mod: tea.ModCtrl},
		{Code: 'd', Mod: tea.ModCtrl},
	} {
		if got, ok := routeMessage(
			tea.KeyPressMsg(key), navigation.FocusReader, ui.Geometry{}, workspace.Files,
			workspace.Controls{}, false, false, 0, 0, 0, 1,
		); ok {
			t.Errorf("retired key %q routed as %+v", key.String(), got)
		}
	}
	if got, ok := routeMessage(
		tea.KeyPressMsg(tea.Key{Code: '4', Text: "4"}), navigation.FocusReader, ui.Geometry{}, workspace.Files,
		workspace.Controls{RichDiff: true}, false, false, 0, 0, 0, 1,
	); !ok || got.Kind != ToggleDiffHighlight {
		t.Fatalf("eligible 4 routed as (%+v, %v)", got, ok)
	}
	if got, ok := routeMessage(
		tea.KeyPressMsg(tea.Key{Code: '4', Text: "4"}), navigation.FocusReader, ui.Geometry{}, workspace.Files,
		workspace.Controls{}, false, false, 0, 0, 0, 1,
	); ok {
		t.Fatalf("ineligible 4 routed as (%+v, true)", got)
	}
	if got, ok := routeMessage(
		tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}), navigation.FocusReader, ui.Geometry{}, workspace.Files,
		workspace.Controls{MarkdownPreviewEligible: true}, false, false, 0, 0, 0, 1,
	); !ok || got.Kind != ToggleMarkdownPreview {
		t.Fatalf("eligible Markdown m routed as (%+v, %v)", got, ok)
	}
	for _, active := range []workspace.Kind{workspace.Files, workspace.Git} {
		if got, ok := routeMessage(
			tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}), navigation.FocusReader, ui.Geometry{}, active,
			workspace.Controls{}, false, false, 0, 0, 0, 1,
		); ok {
			t.Fatalf("ineligible workspace %v m routed as (%+v, true)", active, got)
		}
	}
	for _, test := range []struct {
		key  tea.Key
		want ActionKind
	}{
		{key: tea.Key{Code: 'l', Text: "l"}, want: ExpandReaderFold},
		{key: tea.Key{Code: tea.KeyRight}, want: ExpandReaderFold},
		{key: tea.Key{Code: 'h', Text: "h"}, want: CollapseReaderFold},
		{key: tea.Key{Code: tea.KeyLeft}, want: CollapseReaderFold},
		{key: tea.Key{Code: ']', Text: "]"}, want: SelectNextLandmark},
		{key: tea.Key{Code: '[', Text: "["}, want: SelectPreviousLandmark},
	} {
		got, ok := routeMessage(
			tea.KeyPressMsg(test.key), navigation.FocusReader, ui.Geometry{}, workspace.Git,
			workspace.Controls{RichDiff: true}, false, false, 0, 0, 0, 1,
		)
		if !ok || got.Kind != test.want {
			t.Fatalf("rich reader %v routed as (%+v, %v), want %v", test.key, got, ok, test.want)
		}
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
		t.Fatalf("retired Notes key routed as (%+v, true)", got)
	}
	if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), navigation.FocusNavigator, ui.Geometry{}, workspace.Files, workspace.Controls{}, false, false, 0, 0, 0, 0); ok {
		t.Fatalf("Files Escape was consumed as (%+v, true)", got)
	}
	if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), navigation.FocusNavigator, ui.Geometry{}, workspace.Git, workspace.Controls{}, false, false, 0, 0, 0, 0); !ok || got.Kind != ShowFiles {
		t.Fatalf("Git Escape = (%+v, %v), want ShowFiles", got, ok)
	}
	if got, ok := routeMessage(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}), navigation.FocusNavigator, ui.Geometry{}, workspace.Git, workspace.Controls{}, false, false, 0, 0, 0, 0); ok {
		t.Fatalf("Git e routed as (%+v, true)", got)
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

func TestAltMovementRoutesToTheUnfocusedFilesPane(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		focus navigation.Focus
		key   tea.Key
		want  Action
	}{
		{name: "tree focus alt j", focus: navigation.FocusNavigator, key: tea.Key{Code: 'j', Text: "j", Mod: tea.ModAlt}, want: Action{Kind: MoveReaderSelection, Amount: 1}},
		{name: "tree focus alt up", focus: navigation.FocusNavigator, key: tea.Key{Code: tea.KeyUp, Mod: tea.ModAlt}, want: Action{Kind: MoveReaderSelection, Amount: -1}},
		{name: "tree focus alt l", focus: navigation.FocusNavigator, key: tea.Key{Code: 'l', Text: "l", Mod: tea.ModAlt}, want: Action{Kind: ExpandReaderFold}},
		{name: "tree focus alt left", focus: navigation.FocusNavigator, key: tea.Key{Code: tea.KeyLeft, Mod: tea.ModAlt}, want: Action{Kind: CollapseReaderFold}},
		{name: "reader focus alt down", focus: navigation.FocusReader, key: tea.Key{Code: tea.KeyDown, Mod: tea.ModAlt}, want: Action{Kind: SelectNext}},
		{name: "reader focus alt k", focus: navigation.FocusReader, key: tea.Key{Code: 'k', Text: "k", Mod: tea.ModAlt}, want: Action{Kind: SelectPrevious}},
		{name: "reader focus alt right", focus: navigation.FocusReader, key: tea.Key{Code: tea.KeyRight, Mod: tea.ModAlt}, want: Action{Kind: ExpandNavigatorSelection}},
		{name: "reader focus alt h", focus: navigation.FocusReader, key: tea.Key{Code: 'h', Text: "h", Mod: tea.ModAlt}, want: Action{Kind: CollapseNavigatorSelection}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := routeBrowserKey(tea.KeyPressMsg(test.key), browserRouteContext{
				active: workspace.Files, focus: test.focus,
				controls: workspace.Controls{RichDiff: true},
			})
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeBrowserKey() = (%+v, %v), want (%+v, true)", got, ok, test.want)
			}
		})
	}

	for _, context := range []browserRouteContext{
		{active: workspace.Git, focus: navigation.FocusNavigator, controls: workspace.Controls{RichDiff: true}},
		{active: workspace.Files, focus: navigation.FocusNavigator, controls: workspace.Controls{RichDiff: true}, visualSelecting: true},
	} {
		if got, ok := routeBrowserKey(
			tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j", Mod: tea.ModAlt}), context,
		); ok {
			t.Fatalf("ineligible inactive-pane movement routed as %+v", got)
		}
	}
	if got, ok := routeBrowserKey(
		tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j", Mod: tea.ModAlt | tea.ModShift}),
		browserRouteContext{active: workspace.Files, focus: navigation.FocusNavigator},
	); ok {
		t.Fatalf("combined modifier routed as %+v", got)
	}
}

func TestNotesRoutingIsModelessAndSemantic(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 12)
	editor := notes.NewEditor()
	editor.Load(strings.Repeat("line\n", 30))
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	presentation := editor.Presentation()
	tests := []struct {
		name string
		msg  tea.Msg
		want Action
		ok   bool
	}{
		{name: "h inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}), want: Action{Kind: NotesInsert, Text: "h"}, ok: true},
		{name: "z inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}), want: Action{Kind: NotesInsert, Text: "z"}, ok: true},
		{name: "q inserts", msg: tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}), want: Action{Kind: NotesInsert, Text: "q"}, ok: true},
		{name: "one inserts", msg: tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}), want: Action{Kind: NotesInsert, Text: "1"}, ok: true},
		{name: "two inserts", msg: tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}), want: Action{Kind: NotesInsert, Text: "2"}, ok: true},
		{name: "three inserts", msg: tea.KeyPressMsg(tea.Key{Code: '3', Text: "3"}), want: Action{Kind: NotesInsert, Text: "3"}, ok: true},
		{name: "escape returns Files", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), want: Action{Kind: ShowFiles}, ok: true},
		{name: "ctrl c quits", msg: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), want: Action{Kind: Quit}, ok: true},
		{name: "shift left selects", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft, Mod: tea.ModShift}), want: Action{Kind: NotesMoveLeft, Selecting: true}, ok: true},
		{name: "ctrl right words", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}), want: Action{Kind: NotesMoveWordRight}, ok: true},
		{name: "home", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), want: Action{Kind: NotesMoveHome}, ok: true},
		{name: "page down", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}), want: Action{Kind: NotesPageDown}, ok: true},
		{name: "enter", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), want: Action{Kind: NotesInsert, Text: "\n"}, ok: true},
		{name: "tab inserts indentation", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), want: Action{Kind: NotesInsert, Text: "\t"}, ok: true},
		{name: "shift tab is reserved", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})},
		{name: "delete", msg: tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}), want: Action{Kind: NotesDelete}, ok: true},
		{name: "undo", msg: tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}), want: Action{Kind: NotesUndo}, ok: true},
		{name: "redo", msg: tea.KeyPressMsg(tea.Key{Code: 'y', Mod: tea.ModCtrl}), want: Action{Kind: NotesRedo}, ok: true},
		{name: "paste", msg: tea.PasteMsg{Content: "a\nb"}, want: Action{Kind: NotesInsert, Text: "a\nb"}, ok: true},
		{name: "click text", msg: tea.MouseClickMsg(tea.Mouse{X: g.NotesText.X + 3, Y: g.NotesText.Y + 2, Button: tea.MouseLeft}), want: Action{Kind: NotesBeginSelection, X: 3, Y: 2}, ok: true},
		{name: "wheel text", msg: tea.MouseWheelMsg(tea.Mouse{X: g.NotesText.X, Y: g.NotesText.Y, Button: tea.MouseWheelDown}), want: Action{Kind: NotesScroll, Amount: 3}, ok: true},
		{name: "drag selection", msg: tea.MouseMotionMsg(tea.Mouse{X: g.NotesText.X + 4, Y: g.NotesText.Y + 3, Button: tea.MouseLeft}), want: Action{Kind: NotesDragSelection, X: 4, Y: 3}, ok: true},
		{name: "release selection", msg: tea.MouseReleaseMsg(tea.Mouse{Button: tea.MouseLeft}), want: Action{Kind: NotesEndSelection}, ok: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selectionDragging := strings.Contains(test.name, "selection")
			got, ok := routeNotesMessage(test.msg, g, presentation, selectionDragging, false)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeNotesMessage() = (%+v, %v), want (%+v, %v)", got, ok, test.want, test.ok)
			}
		})
	}

	bar, ok := ui.CalculateScrollbar(g.NotesRows, len(presentation.Document.Rows), presentation.Top)
	if !ok {
		t.Fatal("long Notes note has no scrollbar")
	}
	got, routed := routeNotesMessage(tea.MouseClickMsg(tea.Mouse{X: bar.Thumb.X, Y: bar.Thumb.Y, Button: tea.MouseLeft}), g, presentation, false, false)
	if !routed || got.Kind != StartNotesScrollbarDrag || got.Position != bar.Thumb.Y {
		t.Fatalf("notes scrollbar click = (%+v, %v)", got, routed)
	}
}

func TestNotesScopeKeyboardAndMouseRouting(t *testing.T) {
	t.Parallel()
	g := ui.Calculate(80, 12)
	editor := notes.NewEditor()
	editor.Resize(g.NotesText.Width, g.NotesText.Height)
	presentation := editor.Presentation()
	ctrlT := tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl})
	if got, ok := routeNotesMessage(ctrlT, g, presentation, false, false); ok {
		t.Fatalf("primary ctrl+t routed as (%+v, true)", got)
	}
	if got, ok := routeNotesMessage(ctrlT, g, presentation, false, false, true); !ok || got.Kind != ToggleNotesScope {
		t.Fatalf("linked ctrl+t = (%+v, %v)", got, ok)
	}
	if got, ok := routeNotesMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), g, presentation, false, false, true); !ok || got != (Action{Kind: NotesInsert, Text: "\t"}) {
		t.Fatalf("linked Tab = (%+v, %v)", got, ok)
	}
	if got, ok := routeNotesMessage(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}), g, presentation, false, false, true); ok {
		t.Fatalf("linked Shift+Tab = (%+v, %v)", got, ok)
	}
	for _, test := range []struct {
		name string
		rect ui.Rect
		want ActionKind
	}{
		{name: "project", rect: g.NotesProjectScope, want: SelectProjectNotes},
		{name: "worktree", rect: g.NotesWorktreeScope, want: SelectWorktreeNotes},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			msg := tea.MouseClickMsg(tea.Mouse{X: test.rect.X + test.rect.Width/2, Y: test.rect.Y, Button: tea.MouseLeft})
			if got, ok := routeNotesMessage(msg, g, presentation, false, false, true); !ok || got.Kind != test.want {
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
	controlClick := func(kind ui.HitKind) tea.MouseClickMsg {
		for y := g.Header.Y; y <= g.ReaderTitle.Y; y++ {
			for x := 0; x < g.Screen.Width; x++ {
				if g.HitTest(x, y, workspace.Files, workspace.Controls{}, 0, 0, 0, 0).Kind == kind {
					return tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
				}
			}
		}
		t.Fatalf("missing mouse target for %v", kind)
		return tea.MouseClickMsg{}
	}
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
		{name: "notes label selects Notes", msg: tea.MouseClickMsg(tea.Mouse{X: g.HeaderNotes.X, Y: g.HeaderNotes.Y, Button: tea.MouseLeft}), want: Action{Kind: ShowNotes}, ok: true},
		{name: "secondary control cycles", msg: controlClick(ui.HitSecondaryControl), want: Action{Kind: ToggleSecondary}, ok: true},
		{name: "tertiary control cycles", msg: controlClick(ui.HitTertiaryControl), want: Action{Kind: ToggleTertiary}, ok: true},
		{name: "comparison control cycles", msg: controlClick(ui.HitComparisonControl), want: Action{Kind: ToggleComparison}, ok: true},
		{name: "switcher separator is neutral", msg: tea.MouseClickMsg(tea.Mouse{X: 7, Y: g.Header.Y, Button: tea.MouseLeft})},
		{name: "header gap is neutral", msg: tea.MouseClickMsg(tea.Mouse{X: g.HeaderSwitcher.X + g.HeaderSwitcher.Width, Y: g.Header.Y, Button: tea.MouseLeft})},
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

func TestDiffHighlightMouseTargetRoutesOnlyWhenPainted(t *testing.T) {
	t.Parallel()
	geometry := ui.Calculate(120, 12)
	controls := workspace.Controls{Reader: workspace.DiffReader, RichDiff: true}
	targetX := -1
	for x := 0; x < geometry.Header.Width; x++ {
		if geometry.HitTest(x, geometry.Header.Y, workspace.Files, controls, 0, 0, 0, 1).Kind == ui.HitDiffHighlightControl {
			targetX = x
			break
		}
	}
	if targetX < 0 {
		t.Fatal("eligible rich diff painted no mouse target")
	}
	action, ok := routeMessage(
		tea.MouseClickMsg(tea.Mouse{X: targetX, Y: geometry.Header.Y, Button: tea.MouseLeft}),
		navigation.FocusReader, geometry, workspace.Files, controls, false, false, 0, 0, 0, 1,
	)
	if !ok || action.Kind != ToggleDiffHighlight {
		t.Fatalf("eligible highlight click = (%+v, %v)", action, ok)
	}
	controls.RichDiff = false
	if hit := geometry.HitTest(targetX, geometry.Header.Y, workspace.Files, controls, 0, 0, 0, 1); hit.Kind == ui.HitDiffHighlightControl {
		t.Fatalf("ineligible reader retained target: %+v", hit)
	}
}
