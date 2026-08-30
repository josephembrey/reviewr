package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/ui"
)

type modalKind uint8

const (
	modalNone modalKind = iota
	modalHelp
	modalSettings
)

func (m *Model) toggleModal(action ActionKind) {
	target := modalNone
	switch action {
	case ToggleHelp:
		target = modalHelp
	case ToggleSettings:
		target = modalSettings
	default:
		return
	}
	if m.modal == target {
		m.modal = modalNone
		return
	}
	m.modal = target
}

// routeModalInput owns global modal shortcuts and gives an open modal first
// refusal over every workspace input. Help remains non-global in the Notes
// editor, while comma deliberately opens Settings from every destination.
func routeModalInput(msg tea.Msg, geometry ui.Geometry, open modalKind, helpKeyboardAvailable bool) (Action, bool) {
	if open == modalNone {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case ",":
				return Action{Kind: ToggleSettings}, true
			case "?":
				if helpKeyboardAvailable {
					return Action{Kind: ToggleHelp}, true
				}
			}
		case tea.MouseClickMsg:
			mouse := msg.Mouse()
			if mouse.Button == tea.MouseLeft && geometry.FooterHelp.Contains(mouse.X, mouse.Y) {
				return Action{Kind: ToggleHelp}, true
			}
		}
		return Action{}, false
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return routeOpenModalKey(msg, open)
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		if open == modalHelp && msg.Mouse().Button == tea.MouseLeft {
			return Action{Kind: ToggleHelp}, true
		}
		return Action{Kind: ActionNone}, true
	case tea.MouseReleaseMsg, tea.MouseWheelMsg, tea.MouseMotionMsg, tea.PasteMsg:
		return Action{Kind: ActionNone}, true
	default:
		return Action{}, false
	}
}

func routeOpenModalKey(msg tea.KeyPressMsg, open modalKind) (Action, bool) {
	key := msg.String()
	if key == "q" || key == "ctrl+c" {
		return Action{Kind: Quit}, true
	}
	switch open {
	case modalHelp:
		if key == "," {
			return Action{Kind: ToggleSettings}, true
		}
		if key == "?" || key == "esc" {
			return Action{Kind: ToggleHelp}, true
		}
	case modalSettings:
		switch key {
		case ",", "esc":
			return Action{Kind: ToggleSettings}, true
		case "j", "down":
			return Action{Kind: SelectNextSetting}, true
		case "k", "up":
			return Action{Kind: SelectPreviousSetting}, true
		case " ", "space", "enter":
			return Action{Kind: ToggleSelectedSetting}, true
		}
	}
	return Action{Kind: ActionNone}, true
}
