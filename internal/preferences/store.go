// Package preferences persists small application-wide UI preferences.
package preferences

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/josephembrey/reviewr/internal/appstate"
)

const version = 1

// Values contains preferences needed before the first frame is painted.
type Values struct {
	PanesSwapped bool
}

type diskState struct {
	Version      int  `json:"version"`
	PanesSwapped bool `json:"panes_swapped"`
}

// Store atomically replaces one application-wide preference file. Generation
// ordering prevents concurrent Bubble Tea commands from persisting an older
// pane choice after a newer one.
type Store struct {
	mu         sync.Mutex
	path       string
	generation uint64
}

// Open reads preferences from root. An empty root selects reviewr's default
// application-state directory. A returned store remains usable after corrupt
// or incompatible content so the next explicit user change can repair it.
func Open(root string) (*Store, Values, error) {
	if root == "" {
		var err error
		root, err = appstate.DefaultRoot()
		if err != nil {
			return nil, Values{}, err
		}
	}
	store := &Store{path: filepath.Join(root, "preferences.json")}
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return store, Values{}, nil
	}
	if err != nil {
		return store, Values{}, fmt.Errorf("read preferences: %w", err)
	}
	var state diskState
	if err := json.Unmarshal(data, &state); err != nil {
		return store, Values{}, fmt.Errorf("decode preferences: %w", err)
	}
	if state.Version != version {
		return store, Values{}, fmt.Errorf("preferences version %d is unsupported", state.Version)
	}
	return store, Values{PanesSwapped: state.PanesSwapped}, nil
}

// SavePaneSwapped persists the newest authored pane-side choice.
func (store *Store) SavePaneSwapped(generation uint64, swapped bool) error {
	if store == nil || store.path == "" {
		return errors.New("preferences location is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if generation < store.generation {
		return nil
	}
	state := diskState{Version: version, PanesSwapped: swapped}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	data = append(data, '\n')
	if err := replace(store.path, data); err != nil {
		return err
	}
	store.generation = generation
	return nil
}

func replace(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("prepare preferences: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("protect preferences directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".preferences-*.tmp")
	if err != nil {
		return fmt.Errorf("stage preferences: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect preferences: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write preferences: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush preferences: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close preferences: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace preferences: %w", err)
	}
	return nil
}
