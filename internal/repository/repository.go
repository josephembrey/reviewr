// Package repository exposes the read-only worktree operations used by the Go TUI.
package repository

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
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

// File is the bounded result for one raw Git path identity.
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

// ListFiles returns tracked and untracked, nonignored raw path identities.
func (r *Repository) ListFiles() ([]string, error) {
	return r.git.ListFiles(r.root)
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
			refs[refIndex] = CommitRef{Kind: CommitRefKind(reference.Kind), Name: reference.Name}
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

// ReadFile reads a single root-relative path without following a final symlink.
func (r *Repository) ReadFile(path string) File {
	result := File{Path: path}
	fullPath, err := r.resolvePath(path)
	if err != nil {
		return classifyReadError(result, err)
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return classifyReadError(result, err)
	}
	result.Size = info.Size()

	if info.Mode()&fs.ModeSymlink != 0 {
		target, readErr := os.Readlink(fullPath)
		if readErr != nil {
			return classifyReadError(result, readErr)
		}
		result.Kind = FileReady
		result.Content = target
		result.Size = int64(len(target))
		result.Symlink = true
		return result
	}
	if !info.Mode().IsRegular() {
		result.Kind = FileUnreadable
		result.Err = fmt.Errorf("not a regular file")
		return result
	}
	if info.Size() > r.maxBytes {
		result.Kind = FileTooLarge
		return result
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return classifyReadError(result, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, r.maxBytes+1))
	if err != nil {
		return classifyReadError(result, err)
	}
	result.Size = int64(len(data))
	if result.Size > r.maxBytes {
		result.Kind = FileTooLarge
		result.Content = ""
		return result
	}
	if bytes.IndexByte(data, 0) >= 0 {
		result.Kind = FileBinary
		return result
	}

	result.Kind = FileReady
	result.Content = string(data)
	return result
}

func (r *Repository) resolvePath(path string) (string, error) {
	relative := filepath.FromSlash(path)
	if !filepath.IsLocal(relative) || relative == "." {
		return "", fmt.Errorf("path is not worktree-relative")
	}
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Join(root, relative)))
	if err != nil {
		return "", err
	}
	contained, err := filepath.Rel(root, parent)
	if err != nil || !filepath.IsLocal(contained) {
		return "", fmt.Errorf("path escapes the worktree through a symlink")
	}
	return filepath.Join(parent, filepath.Base(relative)), nil
}

func classifyReadError(result File, err error) File {
	if errors.Is(err, fs.ErrNotExist) {
		result.Kind = FileMissing
	} else {
		result.Kind = FileUnreadable
	}
	result.Err = err
	return result
}
