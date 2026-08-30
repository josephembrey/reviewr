package notes

import (
	"errors"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/josephembrey/reviewr/internal/appstate"
)

// readNotesTarget distinguishes an absent pathname from an existing target
// whose content cannot be followed or read. That distinction is load-bearing
// for target-wins migration, including dangling symlinks and other error
// objects that report ENOENT through ReadFile.
func readNotesTarget(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, true, err
	}
	_, statErr := os.Lstat(path)
	if statErr == nil {
		return nil, true, err
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return nil, false, err
	}
	return nil, true, errors.Join(err, statErr)
}

func readLegacyNote(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("read legacy note: %w", err)
	}
	if !utf8.Valid(data) {
		return data, ErrInvalidUTF8
	}
	return data, nil
}

// importLegacyIfMissing makes a one-time, source-preserving copy while the
// Notes destination lock is held. Any destination object wins, even when it is
// empty, unreadable, or otherwise invalid.
func (s *PrivateStore) importLegacyIfMissing() error {
	if !s.locked || s.paths.legacyNote == "" || notesTargetExists(s.paths.Note) {
		return nil
	}
	data, err := readLegacyNote(s.paths.legacyNote)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("import legacy note: %w", err)
	}
	return s.publishLegacy(data)
}

func notesTargetExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func (s *PrivateStore) publishLegacy(data []byte) error {
	temporary, err := os.CreateTemp(s.paths.Directory, ".legacy-note-*.tmp")
	if err != nil {
		return fmt.Errorf("stage legacy note: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	fail := func(operation string, operationErr error) error {
		_ = temporary.Close()
		return fmt.Errorf("%s legacy note: %w", operation, operationErr)
	}
	if err := temporary.Chmod(0o600); err != nil {
		return fail("protect", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fail("write", err)
	}
	if err := temporary.Sync(); err != nil {
		return fail("flush", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close legacy note: %w", err)
	}
	// Link publishes without replacement: a destination created by another
	// process between Lstat and here remains authoritative.
	if err := os.Link(temporaryPath, s.paths.Note); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("publish legacy note: %w", err)
	}
	appstate.SyncDirectory(s.paths.Directory)
	return nil
}
