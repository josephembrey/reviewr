// Package repository exposes the read-only worktree operations used by the Go TUI.
package repository

import (
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
	root     string
	git      gitadapter.Client
	maxBytes int64
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
	return &Repository{root: root, git: client, maxBytes: DefaultMaxFileBytes}, nil
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
func (r *Repository) CommonDir() (string, error) {
	return r.git.ResolveCommonDir(r.root)
}

// NotesStores builds the private project-wide and checkout-local note
// sessions from canonical Git identities. Every checkout, including the
// primary one, has a distinct worktree note.
func (r *Repository) NotesStores(lookupEnv func(string) (string, bool)) (notes.Stores, error) {
	commonDir, err := r.git.ResolveCommonDir(r.root)
	if err != nil {
		return notes.Stores{}, fmt.Errorf("resolve Notes project identity: %w", err)
	}
	return notes.NewStores(commonDir, r.root, lookupEnv), nil
}

// Snapshot returns one typed source for the All and Changed file scopes.
func (r *Repository) Snapshot() (Snapshot, error) {
	entries, err := r.git.Snapshot(r.root)
	if err != nil {
		return Snapshot{}, err
	}
	result := make([]Entry, len(entries))
	for index, entry := range entries {
		result[index] = Entry{
			Path:         entry.Path,
			PreviousPath: entry.PreviousPath,
			State:        repositoryFileState(entry.State),
			Additions:    entry.Additions,
			Deletions:    entry.Deletions,
			Binary:       entry.Binary,
		}
	}
	return NewSnapshot(result), nil
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
