// Package appstate resolves reviewr's private platform state directory.
package appstate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultRoot returns the application state root without creating it.
func DefaultRoot() (string, error) {
	if runtime.GOOS != "windows" {
		if root := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(root) {
			return filepath.Join(root, "reviewr"), nil
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "state", "reviewr"), nil
		}
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve application state directory: %w", err)
	}
	return filepath.Join(root, "reviewr"), nil
}
