//go:build !dev

package app

import tea "charm.land/bubbletea/v2"

type labState struct{}

func newLabState() labState {
	return labState{}
}

func (*Model) updateLab(tea.Msg) (bool, tea.Cmd) {
	return false, nil
}

func (Model) labView() (tea.View, bool) {
	return tea.View{}, false
}
