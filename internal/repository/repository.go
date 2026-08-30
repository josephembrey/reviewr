// Package repository exposes the read-only worktree operations used by the Go TUI.
package repository

import (
	"errors"
	"fmt"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
	"github.com/josephembrey/reviewr/internal/notes"
)

// DefaultMaxFileBytes bounds memory and render work for one reader load.
const DefaultMaxFileBytes int64 = 1 << 20

// FileKind classifies a worktree read without collapsing expected failures.
type FileKind uint8

const (
	FileReady FileKind = iota + 1
	FileMissing
	FileUnreadable
	FileBinary
	FileTooLarge
)

// File is the bounded result for one typed repository entry.
type File struct {
	Path    string
	Kind    FileKind
	Content string
	Size    int64
	Symlink bool
	Err     error
}

// CommitTraversal selects the Git Log universe.
type CommitTraversal uint8

const (
	CommitGraph CommitTraversal = iota
	CommitFirstParent
)

// CommitQuery describes one bounded history load. StartOID applies to the
// first-parent lineage; graph traversal always uses public refs.
type CommitQuery struct {
	Traversal CommitTraversal
	StartOID  string
}

// CommitRefKind gives a displayed ref its semantic role.
type CommitRefKind uint8

const (
	CommitBranchRef CommitRefKind = iota
	CommitRemoteRef
	CommitTagRef
)

// CommitRef is one public ref pointing at a commit.
type CommitRef struct {
	Kind CommitRefKind
	Name string
}

// DiffKind classifies one bounded worktree patch load.
type DiffKind uint8

const (
	DiffReady DiffKind = iota + 1
	DiffUnavailable
	DiffTooLarge
)

// Diff is the bounded patch result for one typed repository entry.
type Diff struct {
	Entry   Entry
	Kind    DiffKind
	Content string
	Size    int64
	Err     error
}

// Commit is one structured Git Log row.
type Commit struct {
	OID          string
	ShortOID     string
	Parents      []string
	Subject      string
	Author       string
	AuthoredUnix int64
	Refs         []CommitRef
	Merge        bool
	Head         bool
}

// CommitSummary is bounded metadata and changed-file stat for one commit.
type CommitSummary struct {
	OID         string
	AuthorName  string
	AuthorEmail string
	AuthoredAt  string
	Message     string
	Stat        string
}

// ChangeSummary is the aggregate current-worktree diff against HEAD.
type ChangeSummary struct {
	Files     uint64
	Additions uint64
	Deletions uint64
}

// Repository is a resolved Git worktree with read-only operations.
type Repository struct {
	root      string
	commonDir string
	git       gitadapter.Client
	maxBytes  int64
}

// StateFingerprint is the observable Git state used by the application's
// background poller.
type StateFingerprint struct {
	Worktree string
	Refs     string
}

// Open resolves path to its containing worktree.
func Open(path string) (*Repository, error) {
	client := gitadapter.New()
	root, err := client.ResolveRoot(path)
	if err != nil {
		return nil, fmt.Errorf("resolve Git worktree: %w", err)
	}
	commonDir, err := client.ResolveCommonDir(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Git common directory: %w", err)
	}
	return &Repository{root: root, commonDir: commonDir, git: client, maxBytes: DefaultMaxFileBytes}, nil
}

// Root returns the resolved absolute worktree root.
func (r *Repository) Root() string {
	return r.root
}

// PollState detects external repository changes without reading file bodies or
// including reviewr's private refs.
func (r *Repository) PollState() (StateFingerprint, error) {
	state, err := r.git.PollState(r.root)
	if err != nil {
		return StateFingerprint{}, err
	}
	return StateFingerprint{Worktree: state.Worktree, Refs: state.Refs}, nil
}

// CommonDir returns the canonical absolute Git common directory used for
// clone-scoped private state. It performs no writes and is called only during
// executable startup.
func (r *Repository) CommonDir() string {
	return r.commonDir
}

// NotesStores builds the private project-wide and checkout-local note
// sessions from canonical Git identities. Every checkout, including the
// primary one, has a distinct worktree note.
func (r *Repository) NotesStores(lookupEnv func(string) (string, bool)) notes.Stores {
	return notes.NewStores(r.commonDir, r.root, lookupEnv)
}

// Snapshot returns one typed source for the All and Changed file sets under
// the requested comparison.
func (r *Repository) Snapshot(scope string) (Snapshot, error) {
	inventory, err := r.git.Inventory(r.root)
	if err != nil {
		return Snapshot{}, err
	}
	comparison, changes, err := r.resolveComparison(scope)
	if err != nil {
		return Snapshot{}, err
	}
	return NewComparisonSnapshot(comparisonEntries(inventory, changes), comparison), nil
}

func (r *Repository) resolveComparison(scope string) (Comparison, []gitadapter.FileEntry, error) {
	comparison := Comparison{Scope: scope}
	switch scope {
	case ComparisonBranch:
		_, base, err := r.git.DefaultBranch(r.root)
		if err != nil {
			return comparison, nil, err
		}
		if base == "" {
			comparison.Reason = "origin/HEAD does not name a default branch"
			return comparison, nil, nil
		}
		basis, err := r.git.MergeBase(r.root, base)
		if err != nil {
			return comparison, nil, err
		}
		if basis == "" {
			comparison.Reason = "the default branch and HEAD have no merge base"
			return comparison, nil, nil
		}
		comparison.Basis = basis
		changes, err := r.git.WorktreeChanges(r.root, basis)
		return comparison, changes, err
	case ComparisonLastTurn:
		basis, exists, err := r.git.TurnBaseline(r.root)
		if err != nil {
			return comparison, nil, err
		}
		if !exists {
			comparison.Reason = "no agent turn has been observed in this worktree"
			return comparison, nil, nil
		}
		current, err := r.git.SnapshotWorktree(r.root)
		if err != nil {
			return comparison, nil, err
		}
		comparison.Basis = basis
		comparison.Target = current
		changes, err := r.git.TreeChanges(r.root, basis, current)
		return comparison, changes, err
	case ComparisonUncommitted, "":
		comparison.Scope = ComparisonUncommitted
		basis, err := r.git.HeadOID(r.root)
		if errors.Is(err, gitadapter.ErrUnbornHead) {
			basis, err = r.git.EmptyTreeOID(r.root)
		}
		if err != nil {
			return comparison, nil, err
		}
		comparison.Basis = basis
		changes, err := r.git.WorktreeChanges(r.root, basis)
		return comparison, changes, err
	default:
		comparison.Reason = "unsupported comparison " + scope
		return comparison, nil, nil
	}
}

func comparisonEntries(inventory, changes []gitadapter.FileEntry) []Entry {
	byPath := make(map[string]Entry, len(inventory)+len(changes))
	for _, entry := range inventory {
		state := FileUnchanged
		if entry.State == gitadapter.FileIgnored {
			state = FileIgnored
		}
		byPath[entry.Path] = Entry{Path: entry.Path, State: state}
	}
	for _, entry := range changes {
		byPath[entry.Path] = repositoryEntry(entry)
	}
	result := make([]Entry, 0, len(byPath))
	for _, entry := range byPath {
		result = append(result, entry)
	}
	return result
}

func repositoryEntry(entry gitadapter.FileEntry) Entry {
	return Entry{
		Path:         entry.Path,
		PreviousPath: entry.PreviousPath,
		State:        repositoryFileState(entry.State),
		Additions:    entry.Additions,
		Deletions:    entry.Deletions,
		Binary:       entry.Binary,
	}
}

func repositoryFileState(state gitadapter.FileState) FileState {
	switch state {
	case gitadapter.FileModified:
		return FileModified
	case gitadapter.FileAdded:
		return FileAdded
	case gitadapter.FileDeleted:
		return FileDeleted
	case gitadapter.FileRenamed:
		return FileRenamed
	case gitadapter.FileUntracked:
		return FileUntracked
	case gitadapter.FileIgnored:
		return FileIgnored
	default:
		return FileUnchanged
	}
}

// WorktreeSummary returns aggregate tracked and untracked change counts.
func (r *Repository) WorktreeSummary() (ChangeSummary, error) {
	summary, err := r.git.WorktreeSummary(r.root)
	if err != nil {
		return ChangeSummary{}, err
	}
	return ChangeSummary{
		Files:     summary.Files,
		Additions: summary.Additions,
		Deletions: summary.Deletions,
	}, nil
}

// ListCommits returns a bounded structured history traversal.
func (r *Repository) ListCommits(query CommitQuery) ([]Commit, error) {
	traversal := gitadapter.GraphTraversal
	if query.Traversal == CommitFirstParent {
		traversal = gitadapter.FirstParentTraversal
	}
	commits, err := r.git.ListCommits(r.root, gitadapter.HistoryQuery{
		Traversal: traversal,
		StartOID:  query.StartOID,
	})
	if err != nil {
		return nil, err
	}
	result := make([]Commit, len(commits))
	for index, commit := range commits {
		refs := make([]CommitRef, len(commit.Refs))
		for refIndex, reference := range commit.Refs {
			kind := CommitBranchRef
			switch reference.Kind {
			case gitadapter.RemoteRef:
				kind = CommitRemoteRef
			case gitadapter.TagRef:
				kind = CommitTagRef
			}
			refs[refIndex] = CommitRef{Kind: kind, Name: reference.Name}
		}
		result[index] = Commit{
			OID:          commit.OID,
			ShortOID:     commit.ShortOID,
			Parents:      append([]string(nil), commit.Parents...),
			Subject:      commit.Subject,
			Author:       commit.Author,
			AuthoredUnix: commit.AuthoredUnix,
			Refs:         refs,
			Merge:        commit.Merge,
			Head:         commit.Head,
		}
	}
	return result, nil
}

// ReadCommit reads one exact full object identity without mutating Git state.
func (r *Repository) ReadCommit(oid string) (CommitSummary, error) {
	summary, err := r.git.ReadCommit(r.root, oid, r.maxBytes)
	if err != nil {
		return CommitSummary{}, err
	}
	return CommitSummary{
		OID:         summary.OID,
		AuthorName:  summary.AuthorName,
		AuthorEmail: summary.AuthorEmail,
		AuthoredAt:  summary.AuthoredAt,
		Message:     summary.Message,
		Stat:        summary.Stat,
	}, nil
}
