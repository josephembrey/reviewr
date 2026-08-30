package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/scratch"
)

type fakePaneStateStore struct {
	generation uint64
	swapped    bool
}

func (store *fakePaneStateStore) SavePaneSwapped(generation uint64, swapped bool) error {
	store.generation = generation
	store.swapped = swapped
	return nil
}

func TestPaneSideLoadsBeforeLayoutAndPersistsEachSwap(t *testing.T) {
	t.Parallel()
	store := &fakePaneStateStore{}
	model := NewWithPaneState(&fakeSource{}, herdr.Context{}, scratch.NewMemoryStore(), store, true)
	model.apply(Action{Kind: Resize, Width: 80, Height: 24})
	if !model.layout.swapped || model.geometry.Reader.X != 0 {
		t.Fatalf("startup pane preference was not applied: layout=%+v geometry=%+v", model.layout, model.geometry)
	}

	pending := model.apply(Action{Kind: SwapPanes})
	if pending.kind != effectSavePaneState || pending.generation != 1 || pending.swapped {
		t.Fatalf("pane swap persistence effect = %+v", pending)
	}
	message := model.command(pending)().(paneStateSavedMsg)
	if message.err != nil || store.generation != 1 || store.swapped {
		t.Fatalf("saved pane preference = generation %d swapped %v err %v", store.generation, store.swapped, message.err)
	}
}
