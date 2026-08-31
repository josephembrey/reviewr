// Package session persists authored UI place state for one Git worktree.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/josephembrey/reviewr/internal/appstate"
)

const version = 2

// Identity names one checkout without exposing its paths in the state filename.
type Identity struct {
	CommonGitDir string `json:"common_git_dir"`
	Worktree     string `json:"worktree"`
}

// State contains only user-authored place and presentation choices. Repository
// data is deliberately absent and is always loaded fresh on startup.
type State struct {
	Active   string   `json:"active,omitempty"`
	Controls Controls `json:"controls,omitempty"`
	Settings Settings `json:"settings,omitempty"`
	Layout   Layout   `json:"layout,omitempty"`
	Files    Files    `json:"files,omitempty"`
	History  History  `json:"history,omitempty"`
	Stashes  Stashes  `json:"stashes,omitempty"`
	Notes    Notes    `json:"notes,omitempty"`
}

type Controls struct {
	Files         string `json:"files,omitempty"`
	Reader        string `json:"reader,omitempty"`
	Comparison    string `json:"comparison,omitempty"`
	Git           string `json:"git,omitempty"`
	Traversal     string `json:"traversal,omitempty"`
	DiffHighlight string `json:"diff_highlight,omitempty"`
}

type Settings struct {
	// Negative/default-false fields preserve the application's defaults when
	// reading sessions written before settings were persisted.
	ExcludeCommentsFromHunkNavigation bool `json:"exclude_comments_from_hunk_navigation,omitempty"`
	DiffsStartUnfolded                bool `json:"diffs_start_unfolded,omitempty"`
}

type Layout struct {
	NavigatorWidth  int  `json:"navigator_width,omitempty"`
	Customized      bool `json:"customized,omitempty"`
	Swapped         bool `json:"swapped,omitempty"`
	GitSourceWidth  int  `json:"git_source_width,omitempty"`
	GitSourceCustom bool `json:"git_source_custom,omitempty"`
	GitStashWidth   int  `json:"git_stash_width,omitempty"`
	GitStashCustom  bool `json:"git_stash_custom,omitempty"`
	GitFilesSize    int  `json:"git_files_size,omitempty"`
	GitFilesCustom  bool `json:"git_files_custom,omitempty"`
}

// Place retains the old identity sequence as well as its indices so startup
// reconciliation can use identity, nearest survivor, then clamping.
type Place struct {
	Items        []string `json:"items,omitempty"`
	Selected     int      `json:"selected,omitempty"`
	Top          int      `json:"top,omitempty"`
	Focus        string   `json:"focus,omitempty"`
	ReaderOffset int      `json:"reader_offset,omitempty"`
	ReaderColumn int      `json:"reader_column,omitempty"`
	ReaderCursor int      `json:"reader_cursor,omitempty"`
}

type Folds struct {
	Known     []string `json:"known,omitempty"`
	Collapsed []string `json:"collapsed,omitempty"`
}

type Files struct {
	Place                Place            `json:"place,omitempty"`
	ReaderPath           string           `json:"reader_path,omitempty"`
	ReaderRows           []string         `json:"reader_rows,omitempty"`
	ContextExpanded      bool             `json:"context_expanded,omitempty"`
	ContextFoldOverrides map[string]bool  `json:"context_fold_overrides,omitempty"`
	Folds                map[string]Folds `json:"folds,omitempty"`
	ReviewFull           map[string]bool  `json:"review_full,omitempty"`
	MarkdownPreviews     []string         `json:"markdown_previews,omitempty"`
}

type History struct {
	SourcePlace    Place            `json:"source_place,omitempty"`
	TimelinePlace  Place            `json:"timeline_place,omitempty"`
	SelectedSource string           `json:"selected_source,omitempty"`
	SourceFolds    map[string]bool  `json:"source_folds,omitempty"`
	Focus          string           `json:"focus,omitempty"`
	Inspecting     bool             `json:"inspecting,omitempty"`
	InspectionOID  string           `json:"inspection_oid,omitempty"`
	Inspection     ChangeInspection `json:"inspection,omitempty"`
}

type ChangeReaderPlace struct {
	FileIdentity string `json:"file_identity,omitempty"`
	FileTop      int    `json:"file_top,omitempty"`
	ReaderOffset int    `json:"reader_offset,omitempty"`
	ReaderColumn int    `json:"reader_column,omitempty"`
	ReaderCursor int    `json:"reader_cursor,omitempty"`
}

type ChangeInspection struct {
	Place                Place                        `json:"place,omitempty"`
	ReaderRows           []string                     `json:"reader_rows,omitempty"`
	ContextExpanded      bool                         `json:"context_expanded,omitempty"`
	ContextFoldOverrides map[string]bool              `json:"context_fold_overrides,omitempty"`
	ReaderPlaces         map[string]ChangeReaderPlace `json:"reader_places,omitempty"`
}

type Stashes struct {
	Place      Place            `json:"place,omitempty"`
	Focus      string           `json:"focus,omitempty"`
	Inspection ChangeInspection `json:"inspection,omitempty"`
}

type Notes struct {
	Scope    string    `json:"scope,omitempty"`
	Project  NotePlace `json:"project,omitempty"`
	Worktree NotePlace `json:"worktree,omitempty"`
}

type NotePlace struct {
	Valid        bool `json:"valid,omitempty"`
	Cursor       int  `json:"cursor,omitempty"`
	Anchor       int  `json:"anchor,omitempty"`
	PreferredCol int  `json:"preferred_column,omitempty"`
	Scroll       int  `json:"scroll,omitempty"`
}

type diskState struct {
	Version  int      `json:"version"`
	Identity Identity `json:"identity"`
	Session  State    `json:"session"`
}

// Store atomically replaces one worktree session. Generation ordering keeps
// delayed Bubble Tea commands from replacing a newer authored snapshot.
type Store struct {
	mu         sync.Mutex
	root       string
	path       string
	identity   Identity
	generation uint64
}

// Open resolves and reads one worktree session. A store returned alongside a
// corrupt file remains writable so a later authored snapshot can repair it.
func Open(root, commonGitDir, worktree string) (*Store, State, error) {
	identity, err := resolveIdentity(commonGitDir, worktree)
	if err != nil {
		return nil, State{}, err
	}
	if root == "" {
		root, err = appstate.DefaultRoot()
		if err != nil {
			return nil, State{}, err
		}
	}
	store := &Store{
		root:     root,
		path:     filepath.Join(root, "sessions", identity.fileKey()+".json"),
		identity: identity,
	}
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return store, State{}, nil
	}
	if err != nil {
		return store, State{}, fmt.Errorf("read session: %w", err)
	}
	var disk diskState
	if err := json.Unmarshal(data, &disk); err != nil {
		return store, State{}, fmt.Errorf("decode session: %w", err)
	}
	if disk.Version != version {
		return store, State{}, fmt.Errorf("session version %d is unsupported", disk.Version)
	}
	if disk.Identity != identity {
		return store, State{}, errors.New("session identity does not match this worktree")
	}
	return store, disk.Session, nil
}

// Save persists the newest authored snapshot.
func (store *Store) Save(generation uint64, state State) error {
	if store == nil || store.path == "" {
		return errors.New("session location is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if generation < store.generation {
		return nil
	}
	data, err := json.MarshalIndent(diskState{
		Version: version, Identity: store.identity, Session: state,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	data = append(data, '\n')
	if err := replace(store.root, store.path, data); err != nil {
		return err
	}
	store.generation = generation
	return nil
}

func resolveIdentity(commonGitDir, worktree string) (Identity, error) {
	common, err := appstate.CanonicalPath(commonGitDir)
	if err != nil {
		return Identity{}, fmt.Errorf("canonicalize common Git directory: %w", err)
	}
	checkout, err := appstate.CanonicalPath(worktree)
	if err != nil {
		return Identity{}, fmt.Errorf("canonicalize worktree: %w", err)
	}
	return Identity{CommonGitDir: common, Worktree: checkout}, nil
}

func (identity Identity) fileKey() string {
	return appstate.FileKey(identity.CommonGitDir, identity.Worktree)
}

func replace(root, path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := appstate.EnsurePrivateSubdirectory(root, directory); err != nil {
		return fmt.Errorf("prepare session directory: %w", err)
	}
	if err := appstate.ReplaceFile(path, ".session-*.tmp", data); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	return nil
}
