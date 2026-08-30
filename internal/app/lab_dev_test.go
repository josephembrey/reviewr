//go:build dev

package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDevLabTogglesWithoutChangingApplicationPlace(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 24})
	before := model.controls

	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Mod: tea.ModCtrl}))
	model = next.(Model)
	if command == nil || !model.lab.active || !strings.Contains(model.View().Content, "lab / switchers") {
		t.Fatalf("opening lab = active %v background request=%v", model.lab.active, command != nil)
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	model = next.(Model)
	if model.controls != before {
		t.Fatalf("lab input changed application controls: before %+v after %+v", before, model.controls)
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = next.(Model)
	if model.lab.active || strings.Contains(model.View().Content, "lab / switchers") {
		t.Fatal("escape did not close lab")
	}
}
