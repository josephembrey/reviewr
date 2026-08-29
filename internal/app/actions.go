package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
)

// ActionKind is a semantic user intent, independent of terminal input syntax.
type ActionKind uint8

const (
	ActionNone ActionKind = iota
	SelectNext
	SelectPrevious
	SelectIndex
	FocusNavigator
	FocusReader
	ToggleFocus
	ScrollReader
	Reload
	Resize
	Quit
)

// Action carries the small amount of data needed by a semantic intent.
type Action struct {
	Kind   ActionKind
	Index  int
	Amount int
	Width  int
	Height int
}

func routeMessage(msg tea.Msg, focus navigation.Focus, geometry ui.Geometry, top, fileCount int) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return Action{Kind: Quit}, true
		case "tab":
			return Action{Kind: ToggleFocus}, true
		case "r":
			return Action{Kind: Reload}, true
		case "j", "down":
			if focus == navigation.FocusNavigator {
				return Action{Kind: SelectNext}, true
			}
			return Action{Kind: ScrollReader, Amount: 1}, true
		case "k", "up":
			if focus == navigation.FocusNavigator {
				return Action{Kind: SelectPrevious}, true
			}
			return Action{Kind: ScrollReader, Amount: -1}, true
		}
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft {
			return Action{}, false
		}
		switch hit := geometry.HitTest(mouse.X, mouse.Y, top, fileCount); hit.Kind {
		case ui.HitNavigatorRow:
			return Action{Kind: SelectIndex, Index: hit.Index}, true
		case ui.HitNavigator:
			return Action{Kind: FocusNavigator}, true
		case ui.HitReader:
			return Action{Kind: FocusReader}, true
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		hit := geometry.HitTest(mouse.X, mouse.Y, top, fileCount)
		if hit.Kind == ui.HitNone {
			return Action{}, false
		}
		direction := 0
		switch mouse.Button {
		case tea.MouseWheelUp:
			direction = -1
		case tea.MouseWheelDown:
			direction = 1
		default:
			return Action{}, false
		}
		if hit.Kind == ui.HitNavigator || hit.Kind == ui.HitNavigatorRow {
			if direction < 0 {
				return Action{Kind: SelectPrevious}, true
			}
			return Action{Kind: SelectNext}, true
		}
		return Action{Kind: ScrollReader, Amount: direction * 3}, true
	}
	return Action{}, false
}
