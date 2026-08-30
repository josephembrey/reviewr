package scratch

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const stateVersion = "v1"

var (
	ErrReadOnly    = errors.New("scratch is read-only")
	ErrInvalidUTF8 = errors.New("scratch note contains invalid UTF-8")
)

// Store is the narrow persistence session consumed by the application.
type Store interface {
	Load() (text string, readOnly bool, err error)
	Save(text string) error
	Close() error
}

// Scope identifies one independently edited and persisted Scratch note.
type Scope uint8

const (
	Project Scope = iota
	Worktree
)

func (scope Scope) String() string {
	if scope == Worktree {
		return "worktree"
	}
	return "project"
}

// Stores contains the independently locked project-wide and checkout-local
// persistence sessions.
type Stores struct {
	Project  Store
	Worktree Store
}

// HasWorktree reports whether the current checkout has a distinct local note.
func (stores Stores) HasWorktree() bool { return stores.Worktree != nil }

// Store returns the session for scope, collapsing unsupported worktree scope
// to the project session.
func (stores Stores) Store(scope Scope) Store {
	if scope == Worktree && stores.Worktree != nil {
		return stores.Worktree
	}
	return stores.Project
}

// Close releases every constructed session and joins independent failures.
func (stores Stores) Close() error {
	var projectErr, worktreeErr error
	if stores.Project != nil {
		projectErr = stores.Project.Close()
	}
	if stores.Worktree != nil {
		worktreeErr = stores.Worktree.Close()
	}
	return errors.Join(projectErr, worktreeErr)
}

// Paths identifies one clone's opaque private state without exposing the Git
// common directory in filenames.
type Paths struct {
	Directory string
	Note      string
	Lock      string
	stateBase string
}

// StatePaths derives the private versioned path for a canonical Git common
// directory. Relative XDG_STATE_HOME values are ignored per the XDG spec.
func StatePaths(commonDir string, lookupEnv func(string) (string, bool)) (Paths, error) {
	if commonDir == "" || !filepath.IsAbs(commonDir) {
		return Paths{}, fmt.Errorf("Git common directory must be absolute")
	}
	base, err := stateBase(lookupEnv)
	if err != nil {
		return Paths{}, err
	}
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(commonDir))))
	directory := filepath.Join(base, "reviewr", stateVersion, "scratch", identity)
	return Paths{
		Directory: directory,
		Note:      filepath.Join(directory, "note.txt"),
		Lock:      filepath.Join(directory, "edit.lock"),
		stateBase: base,
	}, nil
}

// WorktreeStatePaths derives a second opaque path below the existing project
// state directory. StatePaths remains unchanged so every existing note stays
// at its original project path.
func WorktreeStatePaths(commonDir, worktreeRoot string, lookupEnv func(string) (string, bool)) (Paths, error) {
	project, err := StatePaths(commonDir, lookupEnv)
	if err != nil {
		return Paths{}, err
	}
	if worktreeRoot == "" || !filepath.IsAbs(worktreeRoot) {
		return Paths{}, fmt.Errorf("Git worktree root must be absolute")
	}
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(worktreeRoot))))
	directory := filepath.Join(project.Directory, "worktrees", identity)
	return Paths{
		Directory: directory,
		Note:      filepath.Join(directory, "note.txt"),
		Lock:      filepath.Join(directory, "edit.lock"),
		stateBase: project.stateBase,
	}, nil
}

// NewStores constructs both the project-wide and checkout-local sessions.
// The primary checkout is a worktree too; treating it specially would make
// the same scope selector mean different things in different checkouts.
func NewStores(commonDir, worktreeRoot string, lookupEnv func(string) (string, bool)) Stores {
	return Stores{
		Project:  NewPrivateStore(commonDir, lookupEnv),
		Worktree: NewWorktreePrivateStore(commonDir, worktreeRoot, lookupEnv),
	}
}

func stateBase(lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if runtime.GOOS == "linux" {
		if value, ok := lookupEnv("XDG_STATE_HOME"); ok && filepath.IsAbs(value) {
			return filepath.Clean(value), nil
		}
		home, ok := lookupEnv("HOME")
		if !ok || !filepath.IsAbs(home) {
			return "", fmt.Errorf("resolve private state: HOME is unavailable")
		}
		return filepath.Join(home, ".local", "state"), nil
	}
	if value, ok := lookupEnv("LOCALAPPDATA"); ok && filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	home, ok := lookupEnv("HOME")
	if !ok || !filepath.IsAbs(home) {
		return "", fmt.Errorf("resolve private state: home is unavailable")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	return filepath.Join(home, ".local", "state"), nil
}

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
		return "", true, fmt.Errorf("load scratch: store is closed")
	}
	if s.pathErr != nil {
		return "", true, s.pathErr
	}
	if err := ensurePrivateDirectory(s.paths.stateBase, s.paths.Directory); err != nil {
		return "", true, fmt.Errorf("prepare scratch state: %w", err)
	}
	var lockErr error
	if !s.locked {
		lockErr = s.tryLock()
	}
	data, readErr := os.ReadFile(s.paths.Note)
	if errors.Is(readErr, os.ErrNotExist) {
		readErr = nil
		data = nil
	}
	if readErr == nil && !utf8.Valid(data) {
		readErr = ErrInvalidUTF8
	}
	return string(data), !s.locked, errors.Join(lockErr, readErr)
}

func (s *PrivateStore) tryLock() error {
	file, err := os.OpenFile(s.paths.Lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open scratch lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect scratch lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil
		}
		return fmt.Errorf("lock scratch: %w", err)
	}
	s.lockFile = file
	s.locked = true
	return nil
}

func (s *PrivateStore) Save(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("save scratch: store is closed")
	}
	if !s.locked {
		return ErrReadOnly
	}
	temporary, err := os.CreateTemp(s.paths.Directory, ".note-*.tmp")
	if err != nil {
		return fmt.Errorf("stage scratch note: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	fail := func(operation string, operationErr error) error {
		_ = temporary.Close()
		return fmt.Errorf("%s scratch note: %w", operation, operationErr)
	}
	if err := temporary.Chmod(0o600); err != nil {
		return fail("protect", err)
	}
	if _, err := io.WriteString(temporary, text); err != nil {
		return fail("write", err)
	}
	if err := temporary.Sync(); err != nil {
		return fail("flush", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close scratch note: %w", err)
	}
	if err := os.Rename(temporaryPath, s.paths.Note); err != nil {
		return fmt.Errorf("replace scratch note: %w", err)
	}
	removeTemporary = false
	// The staged file already has mode 0600. Once atomic replacement succeeds,
	// the new note is valid; directory sync is best-effort because reporting a
	// post-rename failure would falsely imply the prior note remained current.
	_ = syncDirectory(s.paths.Directory)
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

func ensurePrivateDirectory(base, directory string) error {
	if base == "" {
		return fmt.Errorf("private state base is unavailable")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	relative, err := filepath.Rel(base, directory)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("scratch state directory is outside private state base")
	}
	current := base
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := os.Chmod(current, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(directory string) error {
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

// MemoryStore is a deterministic in-memory session used when embedding the
// app without executable persistence wiring.
type MemoryStore struct {
	mu       sync.Mutex
	Text     string
	ReadOnly bool
	LoadErr  error
	SaveErr  error
	Closed   bool
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (s *MemoryStore) Load() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Text, s.ReadOnly, s.LoadErr
}

func (s *MemoryStore) Save(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ReadOnly {
		return ErrReadOnly
	}
	if s.SaveErr != nil {
		return s.SaveErr
	}
	s.Text = text
	return nil
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Closed = true
	return nil
}
