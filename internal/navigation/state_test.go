package navigation

import (
	"reflect"
	"testing"
)

func TestReconcileSelectionByIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		state       State
		files       []string
		wantPath    string
		wantIndex   int
		wantTopPath string
	}{
		{
			name:        "identity survives reorder",
			state:       State{Files: []string{"a", "b", "c"}, Selected: 1, Top: 1, ReaderOffset: 7},
			files:       []string{"c", "a", "b"},
			wantPath:    "b",
			wantIndex:   2,
			wantTopPath: "b",
		},
		{
			name:        "removed selection prefers equally near successor",
			state:       State{Files: []string{"a", "b", "c"}, Selected: 1},
			files:       []string{"a", "c"},
			wantPath:    "c",
			wantIndex:   1,
			wantTopPath: "a",
		},
		{
			name:        "removed selection uses predecessor",
			state:       State{Files: []string{"a", "b", "c"}, Selected: 2},
			files:       []string{"a"},
			wantPath:    "a",
			wantIndex:   0,
			wantTopPath: "a",
		},
		{
			name:        "complete replacement clamps old index",
			state:       State{Files: []string{"a", "b", "c"}, Selected: 2, Top: 1},
			files:       []string{"x", "y"},
			wantPath:    "y",
			wantIndex:   1,
			wantTopPath: "y",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state := test.state
			state.Reconcile(test.files)
			path, ok := state.SelectedPath()
			if !ok || path != test.wantPath || state.Selected != test.wantIndex {
				t.Fatalf("selection = (%q, %d, %v), want (%q, %d, true)", path, state.Selected, ok, test.wantPath, test.wantIndex)
			}
			if got := state.Files[state.Top]; got != test.wantTopPath {
				t.Fatalf("top path = %q, want %q", got, test.wantTopPath)
			}
			if state.ReaderOffset != test.state.ReaderOffset {
				t.Fatalf("reader offset reset from %d to %d", test.state.ReaderOffset, state.ReaderOffset)
			}
		})
	}
}

func TestReconcileEmpty(t *testing.T) {
	t.Parallel()
	state := State{Files: []string{"a"}, Selected: 0, Top: 0, ReaderOffset: 4, Focus: FocusReader}
	state.Reconcile(nil)
	if _, ok := state.SelectedPath(); ok {
		t.Fatal("empty state retained a selected path")
	}
	if state.Selected != 0 || state.Top != 0 || state.ReaderOffset != 4 || state.Focus != FocusReader {
		t.Fatalf("unexpected empty reconciliation: %+v", state)
	}
}

func TestUserSelectionFocusAndScroll(t *testing.T) {
	t.Parallel()
	state := State{Files: []string{"a", "b", "c", "d"}, ReaderOffset: 5}
	if !state.SelectIndex(3, 2) {
		t.Fatal("selection did not change")
	}
	if state.Selected != 3 || state.Top != 2 || state.ReaderOffset != 0 {
		t.Fatalf("selection state = %+v", state)
	}
	if state.SelectDelta(1, 2) {
		t.Fatal("clamped selection reported a change")
	}
	state.ToggleFocus()
	if state.Focus != FocusReader {
		t.Fatalf("focus = %v, want reader", state.Focus)
	}
	state.ScrollReader(8, 10, 3)
	state.ScrollReader(8, 10, 3)
	if state.ReaderOffset != 7 {
		t.Fatalf("reader offset = %d, want 7", state.ReaderOffset)
	}
	state.ScrollReader(-20, 10, 3)
	if state.ReaderOffset != 0 {
		t.Fatalf("reader offset = %d, want 0", state.ReaderOffset)
	}
}

func TestViewportReconciliationPreservesValidPlace(t *testing.T) {
	t.Parallel()
	state := State{Files: []string{"a", "b", "c", "d", "e"}, Selected: 3, Top: 2, ReaderOffset: 4}
	state.EnsureSelectionVisible(3)
	state.ClampReader(20, 8)
	want := State{Files: state.Files, Selected: 3, Top: 2, ReaderOffset: 4}
	if !reflect.DeepEqual(state, want) {
		t.Fatalf("valid place changed on resize: got %+v, want %+v", state, want)
	}
	state.EnsureSelectionVisible(0)
	if state.Top != 2 {
		t.Fatalf("zero-row resize reset top to %d", state.Top)
	}
	state.EnsureSelectionVisible(5)
	state.ClampReader(6, 5)
	if state.Selected != 3 || state.Top != 0 || state.ReaderOffset != 1 {
		t.Fatalf("clamped place = %+v", state)
	}
}
