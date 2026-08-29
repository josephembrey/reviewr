//go:build !dev

package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestReleaseBuildHasNoLabRoute(t *testing.T) {
	t.Parallel()
	model := New(&fakeSource{})
	if handled, command := model.updateLab(tea.KeyPressMsg(tea.Key{Code: 'l', Mod: tea.ModCtrl})); handled || command != nil {
		t.Fatalf("release lab hook = handled %v command=%v", handled, command != nil)
	}
	if _, active := model.labView(); active {
		t.Fatal("release build exposed lab view")
	}
}
