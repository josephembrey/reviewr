package appstate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalPath resolves one required identity path without treating an empty
// value as the process working directory.
func CanonicalPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(canonical), nil
}

// FileKey returns a stable SHA-256 key for an ordered identity tuple.
func FileKey(parts ...string) string {
	hash := sha256.New()
	for index, part := range parts {
		if index > 0 {
			_, _ = hash.Write([]byte{0})
		}
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// EnsurePrivateSubdirectory creates and protects every directory below a
// trusted state base. Existing descendant symlinks and non-directories are
// rejected so a state namespace cannot escape the base lexically or by link.
func EnsurePrivateSubdirectory(base, directory string) error {
	if base == "" {
		return errors.New("private state base is unavailable")
	}
	relative, err := filepath.Rel(filepath.Clean(base), filepath.Clean(directory))
	if err != nil || relative == "." || pathEscapes(relative) {
		return errors.New("private state directory is outside its base")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	current := filepath.Clean(base)
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := ensurePrivateDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func pathEscapes(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative)
}

func ensurePrivateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("private state path %q is not a directory", path)
	}
	return os.Chmod(path, 0o700)
}

// ReplaceFile stages, flushes, and atomically renames a private file in its
// destination directory. A directory flush is best-effort after the rename:
// once published, an error must not imply that the prior file is still active.
func ReplaceFile(path, pattern string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return fmt.Errorf("create temporary private state: %w", err)
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		_ = temporary.Close()
		if !published {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("protect temporary private state: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary private state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush temporary private state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary private state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish private state: %w", err)
	}
	published = true
	SyncDirectory(directory)
	return nil
}

// SyncDirectory best-effort flushes a directory after a successful publish.
// Callers must not report failure after the new state is already authoritative.
func SyncDirectory(directory string) {
	file, err := os.Open(directory)
	if err != nil {
		return
	}
	defer file.Close()
	_ = file.Sync()
}
