package notes

import "errors"

var (
	ErrReadOnly    = errors.New("notes is read-only")
	ErrInvalidUTF8 = errors.New("notes content contains invalid UTF-8")
)

// Store is the narrow persistence session consumed by the application.
type Store interface {
	Load() (text string, readOnly bool, err error)
	Save(text string) error
	Close() error
}

// Scope identifies one independently edited and persisted Notes note.
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
