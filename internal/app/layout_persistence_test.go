package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/session"
)

type fakeSessionStore struct {
	generation uint64
	state      session.State
}

func (store *fakeSessionStore) Save(generation uint64, state session.State) error {
	store.generation = generation
	store.state = state
	return nil
}

func TestPaneLayoutLoadsBeforeFirstFrameAndDebouncesEachChange(t *testing.T) {
	t.Parallel()
	store := &fakeSessionStore{}
	model := NewWithSession(&fakeSource{}, herdr.Context{}, notes.NewMemoryStore(), store, session.State{
		Layout: session.Layout{NavigatorWidth: 31, Customized: true, Swapped: true},
	})
	model.apply(Action{Kind: Resize, Width: 80, Height: 24})
	if !model.layout.swapped || !model.layout.customized || model.layout.navigatorWidth != 31 || model.geometry.Reader.X != 0 {
		t.Fatalf("startup pane preference was not applied: layout=%+v geometry=%+v", model.layout, model.geometry)
	}

	pending := model.apply(Action{Kind: SwapPanes})
	if pending.kind != effectNone || model.layout.swapped {
		t.Fatalf("pane swap state = effect %+v layout %+v", pending, model.layout)
	}
	due := model.commandAfterAction(pending)().(sessionSaveDueMsg)
	next, saveCommand := model.Update(due)
	model = next.(Model)
	message := saveCommand().(sessionSavedMsg)
	if message.err != nil || store.generation != 1 || store.state.Layout.Swapped || store.state.Layout.NavigatorWidth != 31 {
		t.Fatalf("saved pane session = generation %d state %+v err %v", store.generation, store.state.Layout, message.err)
	}
}
