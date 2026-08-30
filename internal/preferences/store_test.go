package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaneSwapPreferenceRoundTripsAndKeepsNewestGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, initial, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if initial.PanesSwapped {
		t.Fatal("missing preferences started swapped")
	}
	if err := store.SavePaneSwapped(2, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePaneSwapped(1, false); err != nil {
		t.Fatal(err)
	}

	_, loaded, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.PanesSwapped {
		t.Fatal("older save replaced the newest pane preference")
	}
	info, err := os.Stat(filepath.Join(root, "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("preferences mode = %o, want 600", info.Mode().Perm())
	}
}

func TestCorruptPreferenceCanBeRepairedByExplicitSave(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "preferences.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _, err := Open(root)
	if err == nil {
		t.Fatal("corrupt preferences loaded without an error")
	}
	if err := store.SavePaneSwapped(1, true); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := Open(root)
	if err != nil || !loaded.PanesSwapped {
		t.Fatalf("repaired preferences = %+v, %v", loaded, err)
	}
}
