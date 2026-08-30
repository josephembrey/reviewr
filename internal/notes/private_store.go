package notes

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/josephembrey/reviewr/internal/appstate"
	"golang.org/x/sys/unix"
)

// PrivateStore persists one note and owns its process edit lock.
type PrivateStore struct {
	mu       sync.Mutex
	paths    Paths
	pathErr  error
	lockFile *os.File
	locked   bool
	closed   bool
}

// NewPrivateStore creates a lazy session. Filesystem work and lock acquisition
// happen on Load, not while rendering or routing input.
func NewPrivateStore(commonDir string, lookupEnv func(string) (string, bool)) *PrivateStore {
	paths, err := StatePaths(commonDir, lookupEnv)
	return &PrivateStore{paths: paths, pathErr: err}
}

// NewWorktreePrivateStore creates a lazy session whose identity includes both
// the project common directory and canonical current worktree root.
func NewWorktreePrivateStore(commonDir, worktreeRoot string, lookupEnv func(string) (string, bool)) *PrivateStore {
	paths, err := WorktreeStatePaths(commonDir, worktreeRoot, lookupEnv)
	return &PrivateStore{paths: paths, pathErr: err}
}

func (s *PrivateStore) Load() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", true, fmt.Errorf("load notes: store is closed")
	}
	if s.pathErr != nil {
		return "", true, s.pathErr
	}
	if err := appstate.EnsurePrivateSubdirectory(s.paths.stateBase, s.paths.Directory); err != nil {
		return "", true, fmt.Errorf("prepare notes state: %w", err)
	}
	lockErr := s.acquireLock()
	migrationErr := s.importLegacyIfMissing()
	data, readErr := s.loadData()
	return string(data), !s.locked, errors.Join(lockErr, migrationErr, readErr)
}

func (s *PrivateStore) acquireLock() error {
	if s.locked {
		return nil
	}
	return s.tryLock()
}

func (s *PrivateStore) loadData() ([]byte, error) {
	data, exists, readErr := readNotesTarget(s.paths.Note)
	if !exists && !s.locked {
		data, readErr = readLegacyNote(s.paths.legacyNote)
		if targetData, appeared, targetErr := readNotesTarget(s.paths.Note); appeared {
			data, exists, readErr = targetData, true, targetErr
		}
	}
	return validateNoteData(data, exists, readErr)
}

func validateNoteData(data []byte, exists bool, err error) ([]byte, error) {
	if !exists && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err == nil && !utf8.Valid(data) {
		return data, ErrInvalidUTF8
	}
	return data, err
}

func (s *PrivateStore) tryLock() error {
	file, err := os.OpenFile(s.paths.Lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open notes lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect notes lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil
		}
		return fmt.Errorf("lock notes: %w", err)
	}
	s.lockFile = file
	s.locked = true
	return nil
}

func (s *PrivateStore) Save(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("save notes: store is closed")
	}
	if !s.locked {
		return ErrReadOnly
	}
	if !utf8.ValidString(text) {
		return ErrInvalidUTF8
	}
	if err := appstate.ReplaceFile(s.paths.Note, ".note-*.tmp", []byte(text)); err != nil {
		return fmt.Errorf("save notes: %w", err)
	}
	return nil
}

func (s *PrivateStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.lockFile == nil {
		return nil
	}
	unlockErr := unix.Flock(int(s.lockFile.Fd()), unix.LOCK_UN)
	closeErr := s.lockFile.Close()
	s.lockFile = nil
	s.locked = false
	return errors.Join(unlockErr, closeErr)
}
