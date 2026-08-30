package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/josephembrey/reviewr/internal/appstate"
)

const stateVersion = "v1"

// Paths identifies one clone's opaque private state without exposing the Git
// common directory in filenames.
type Paths struct {
	Directory  string
	Note       string
	Lock       string
	stateBase  string
	legacyNote string
}

// StatePaths derives the private versioned path for a canonical Git common
// directory. Relative XDG_STATE_HOME values are ignored per the XDG spec.
func StatePaths(commonDir string, lookupEnv func(string) (string, bool)) (Paths, error) {
	if commonDir == "" || !filepath.IsAbs(commonDir) {
		return Paths{}, fmt.Errorf("git common directory must be absolute")
	}
	base, err := stateBase(lookupEnv)
	if err != nil {
		return Paths{}, err
	}
	identity := appstate.FileKey(filepath.Clean(commonDir))
	directory := filepath.Join(base, "reviewr", stateVersion, "notes", identity)
	return Paths{
		Directory:  directory,
		Note:       filepath.Join(directory, "note.txt"),
		Lock:       filepath.Join(directory, "edit.lock"),
		stateBase:  base,
		legacyNote: filepath.Join(base, "reviewr", stateVersion, "scratch", identity, "note.txt"),
	}, nil
}

// WorktreeStatePaths derives a second opaque Notes path below the project
// state directory while retaining the matching legacy source identity.
func WorktreeStatePaths(commonDir, worktreeRoot string, lookupEnv func(string) (string, bool)) (Paths, error) {
	project, err := StatePaths(commonDir, lookupEnv)
	if err != nil {
		return Paths{}, err
	}
	if worktreeRoot == "" || !filepath.IsAbs(worktreeRoot) {
		return Paths{}, fmt.Errorf("git worktree root must be absolute")
	}
	identity := appstate.FileKey(filepath.Clean(worktreeRoot))
	directory := filepath.Join(project.Directory, "worktrees", identity)
	return Paths{
		Directory:  directory,
		Note:       filepath.Join(directory, "note.txt"),
		Lock:       filepath.Join(directory, "edit.lock"),
		stateBase:  project.stateBase,
		legacyNote: filepath.Join(filepath.Dir(project.legacyNote), "worktrees", identity, "note.txt"),
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
		return linuxStateBase(lookupEnv)
	}
	return platformStateBase(lookupEnv)
}

func linuxStateBase(lookupEnv func(string) (string, bool)) (string, error) {
	if value, ok := absoluteEnvironmentPath(lookupEnv, "XDG_STATE_HOME"); ok {
		return value, nil
	}
	home, ok := absoluteEnvironmentPath(lookupEnv, "HOME")
	if !ok {
		return "", fmt.Errorf("resolve private state: HOME is unavailable")
	}
	return filepath.Join(home, ".local", "state"), nil
}

func platformStateBase(lookupEnv func(string) (string, bool)) (string, error) {
	if value, ok := absoluteEnvironmentPath(lookupEnv, "LOCALAPPDATA"); ok {
		return value, nil
	}
	home, ok := absoluteEnvironmentPath(lookupEnv, "HOME")
	if !ok {
		return "", fmt.Errorf("resolve private state: home is unavailable")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	return filepath.Join(home, ".local", "state"), nil
}

func absoluteEnvironmentPath(lookupEnv func(string) (string, bool), name string) (string, bool) {
	value, ok := lookupEnv(name)
	if !ok || !filepath.IsAbs(value) {
		return "", false
	}
	return filepath.Clean(value), true
}
