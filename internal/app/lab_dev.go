//go:build dev

package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/lab"
)

type labState struct {
	active bool
	model  lab.Model
}

func newLabState() labState {
	return labState{model: lab.New()}
}

func (m *Model) updateLab(msg tea.Msg) (bool, tea.Cmd) {
	key, isKey := msg.(tea.KeyPressMsg)
	if isKey && key.String() == "ctrl+l" {
		m.lab.active = !m.lab.active
		if m.lab.active {
			return true, tea.RequestBackgroundColor
		}
		return true, nil
	}
	if !m.lab.active {
		return false, nil
	}
	if isKey {
		switch key.String() {
		case "esc":
			m.lab.active = false
			return true, nil
		case "q", "ctrl+c":
			return false, nil
		}
	}
	if next, command, handled := m.lab.model.Update(msg); handled {
		m.lab.model = next
		return true, command
	}
	switch msg.(type) {
	case tea.MouseClickMsg, tea.MouseWheelMsg, tea.MouseMotionMsg:
		return true, nil
	default:
		return false, nil
	}
}

func (m Model) labView() (tea.View, bool) {
	if !m.lab.active {
		return tea.View{}, false
	}
	view := tea.NewView(m.lab.model.View(m.geometry.Screen.Width, m.geometry.Screen.Height))
	view.AltScreen = true
	view.WindowTitle = "reviewr lab"
	return view, true
}
