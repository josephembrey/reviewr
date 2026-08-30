package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/ui"
)

func TestReaderContextStateReconcilesOverrideToNearestSurvivingGap(t *testing.T) {
	t.Parallel()
	document := foldableDiffDocument()
	identities := document.ContextFoldIdentities()
	if len(identities) != 2 {
		t.Fatalf("fold identities = %#v, want two gaps", identities)
	}

	state := readerContextState{}
	if changed, _ := state.setFold(document, identities[1], true); !changed {
		t.Fatal("local fold did not change")
	}
	shifted := document
	shifted.Rows = append([]ui.ReaderRow(nil), document.Rows...)
	for index := range shifted.Rows {
		if shifted.Rows[index].Kind != ui.ReaderContext {
			continue
		}
		shifted.Rows[index].Identity = "shifted:" + shifted.Rows[index].Identity
		shifted.Rows[index].OldLine += 100
		shifted.Rows[index].NewLine += 100
	}
	shiftedIdentities := shifted.ContextFoldIdentities()
	state.reconcile(shifted)

	if state.target(shiftedIdentities[0]) || !state.target(shiftedIdentities[1]) {
		t.Fatalf("reconciled targets = first %v second %v, want only nearest gap expanded",
			state.target(shiftedIdentities[0]), state.target(shiftedIdentities[1]))
	}
	if _, stale := state.folds[identities[1]]; stale {
		t.Fatalf("stale fold identity %q survived refresh", identities[1])
	}
}

func TestReaderContextStateKeepsExactIdentityAheadOfNearestFallback(t *testing.T) {
	t.Parallel()
	document := foldableDiffDocument()
	identities := document.ContextFoldIdentities()
	state := readerContextState{}
	state.setFold(document, identities[0], true)

	changed := document
	changed.Rows = append([]ui.ReaderRow(nil), document.Rows...)
	for index := range changed.Rows {
		if changed.Rows[index].Kind == ui.ReaderContext && changed.Rows[index].OldLine >= 15 {
			changed.Rows[index].Identity = "changed:" + changed.Rows[index].Identity
			changed.Rows[index].OldLine += 100
			changed.Rows[index].NewLine += 100
		}
	}
	changedIdentities := changed.ContextFoldIdentities()
	state.reconcile(changed)

	if changedIdentities[0] != identities[0] || !state.target(identities[0]) || state.target(changedIdentities[1]) {
		t.Fatalf("exact fold was displaced: old=%#v new=%#v targets=%#v", identities, changedIdentities, state.folds)
	}
}
