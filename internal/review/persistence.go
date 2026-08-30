package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephembrey/reviewr/internal/appstate"
)

// StateVersion is the persisted review-ledger schema version.
const StateVersion = 1

// RepositoryID is the collision guard and private-state namespace.
type RepositoryID struct {
	CommonGitDir string `json:"common_git_dir"`
	Worktree     string `json:"worktree"`
}

// ResolveRepositoryID canonicalizes the read-only Git identity supplied by the
// repository adapter. It does not invoke Git or enumerate status.
func ResolveRepositoryID(worktree, commonGitDir string) (RepositoryID, error) {
	canonicalWorktree, err := canonicalPath(worktree)
	if err != nil {
		return RepositoryID{}, fmt.Errorf("canonicalize worktree: %w", err)
	}
	canonicalCommon, err := canonicalPath(commonGitDir)
	if err != nil {
		return RepositoryID{}, fmt.Errorf("canonicalize common Git directory: %w", err)
	}
	return RepositoryID{CommonGitDir: canonicalCommon, Worktree: canonicalWorktree}, nil
}

// FileKey returns a stable collision-resistant name for one worktree ledger.
func (id RepositoryID) FileKey() string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(id.CommonGitDir))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(id.Worktree))
	return hex.EncodeToString(hash.Sum(nil))
}

// DefaultStateRoot returns reviewr's platform application-state directory.
func DefaultStateRoot() (string, error) {
	return appstate.DefaultRoot()
}

type stateFile struct {
	Version    int          `json:"version"`
	Repository RepositoryID `json:"repository"`
	Ledger     Ledger       `json:"ledger"`
}

type stateProblem uint8

const (
	stateUnreadable stateProblem = iota + 1
	stateCorrupt
	stateNewer
	stateIdentity
)

func (problem stateProblem) warning() string {
	switch problem {
	case stateCorrupt:
		return "review state is corrupt; starting unreviewed"
	case stateNewer:
		return "review state is from a newer version; starting unreviewed"
	case stateIdentity:
		return "review state identity mismatch; starting unreviewed"
	default:
		return "review state unreadable; marks will not survive restart"
	}
}

func (problem stateProblem) transactionError() string {
	switch problem {
	case stateCorrupt:
		return "review state is corrupt; refusing to overwrite it"
	case stateNewer:
		return "review state is from a newer version; refusing to overwrite it"
	case stateIdentity:
		return "review state identity mismatch; refusing to overwrite it"
	default:
		return "cannot reload review state"
	}
}

// Store atomically persists one repository/worktree ledger outside the repository.
type Store struct {
	path       string
	repository RepositoryID
	writable   bool
	replace    func(string, string, RepositoryID, Ledger) error
}

// OpenStore loads one ledger. Passing an empty root selects DefaultStateRoot;
// tests and embeddings may inject an isolated application-state root.
func OpenStore(repository RepositoryID, root string) (Ledger, *Store, string) {
	if root == "" {
		var err error
		root, err = DefaultStateRoot()
		if err != nil {
			return Ledger{}, &Store{repository: repository}, "review state unavailable; marks will not survive restart"
		}
	}
	path := filepath.Join(root, "reviews", repository.FileKey()+".json")
	store := &Store{path: path, repository: repository, writable: true, replace: replaceState}
	ledger, exists, problem := readLedger(path, repository)
	if problem != 0 {
		store.writable = false
		return Ledger{}, store, problem.warning()
	}
	if !exists {
		return Ledger{}, store, "review state missing; starting unreviewed"
	}
	return ledger, store, ""
}

// Path returns the external state filename.
func (store *Store) Path() string {
	if store == nil {
		return ""
	}
	return store.path
}

// Writable reports whether unknown state recovery has made the handle read-only.
func (store *Store) Writable() bool { return store != nil && store.writable }

// Apply applies delta locally first, then replays it against locked current
// state. Persistence failure never invalidates the local review action.
func (store *Store) Apply(ledger *Ledger, delta Delta) (bool, error) {
	local := ledger.Clone()
	if !delta.Apply(&local) {
		return false, nil
	}
	if store == nil || !store.writable {
		*ledger = local
		return true, errors.New("review state is read-only after recovery")
	}
	current, err := store.commit(delta)
	if err != nil {
		*ledger = local
		return true, err
	}
	*ledger = current
	return true, nil
}

// Replay applies one authored delta to locked current disk state and returns
// the merged ledger. Callers that already applied the delta locally use this
// to avoid manufacturing a second local mark sequence.
func (store *Store) Replay(delta Delta) (Ledger, error) {
	if store == nil || !store.writable {
		return Ledger{}, errors.New("review state is read-only after recovery")
	}
	return store.commit(delta)
}

func (store *Store) commit(delta Delta) (Ledger, error) {
	if store.path == "" {
		return Ledger{}, errors.New("review state location is unavailable")
	}
	parent := filepath.Dir(store.path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return Ledger{}, fmt.Errorf("cannot create review state directory: %w", err)
	}
	if err := os.Chmod(parent, 0o700); err != nil && !errors.Is(err, os.ErrPermission) {
		return Ledger{}, fmt.Errorf("cannot make review state directory private: %w", err)
	}

	lockPath := store.path[:len(store.path)-len(filepath.Ext(store.path))] + ".lock"
	lock, err := openPrivateLock(lockPath)
	if err != nil {
		return Ledger{}, err
	}
	defer lock.Close()
	if err := lockExclusive(lock); err != nil {
		return Ledger{}, fmt.Errorf("cannot lock review state: %w", err)
	}
	defer unlock(lock)

	current, _, problem := readLedger(store.path, store.repository)
	if problem != 0 {
		return Ledger{}, errors.New(problem.transactionError())
	}
	if delta.Apply(&current) {
		current.Compact()
		if err := store.replace(store.path, parent, store.repository, current); err != nil {
			return Ledger{}, err
		}
	}
	return current, nil
}

func readLedger(path string, repository RepositoryID) (Ledger, bool, stateProblem) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ledger{}, false, 0
	}
	if err != nil {
		return Ledger{}, false, stateUnreadable
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Ledger{}, true, stateCorrupt
	}
	if header.Version > StateVersion {
		return Ledger{}, true, stateNewer
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil || state.Version != StateVersion {
		return Ledger{}, true, stateCorrupt
	}
	if state.Repository != repository {
		return Ledger{}, true, stateIdentity
	}
	state.Ledger.Compact()
	return state.Ledger, true, 0
}

func replaceState(path, parent string, repository RepositoryID, ledger Ledger) error {
	state := stateFile{Version: StateVersion, Repository: repository, Ledger: ledger}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode review state: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(parent, ".review-state-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create review state: %w", err)
	}
	tempPath := temp.Name()
	keep := false
	defer func() {
		_ = temp.Close()
		if !keep {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("cannot make review state private: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("cannot write review state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("cannot sync review state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cannot close review state: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("cannot replace review state: %w", err)
	}
	keep = true
	directory, err := os.Open(parent)
	if err != nil {
		return fmt.Errorf("cannot open review state directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("cannot sync review state directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("cannot close review state directory: %w", err)
	}
	return nil
}

func openPrivateLock(path string) (*os.File, error) {
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open review state lock: %w", err)
	}
	if err := lock.Chmod(0o600); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("cannot make review state lock private: %w", err)
	}
	return lock, nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
