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

// Commit is one recent current-HEAD history row.
type Commit struct {
	OID      string
	ShortOID string
	Subject  string
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

// ChangeKind describes how one immutable comparison path changed.
type ChangeKind uint8

const (
	ChangeModified ChangeKind = iota + 1
	ChangeAdded
	ChangeDeleted
	ChangeRenamed
	ChangeCopied
	ChangeUntracked
)

// ChangedFile is one selectable file in a commit-like comparison.
type ChangedFile struct {
	Path         string
	PreviousPath string
	Kind         ChangeKind
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// Identity is stable across refreshes of the same immutable comparison.
func (file ChangedFile) Identity() string {
	return file.PreviousPath + "\x00" + file.Path
}

// ChangeSource names the immutable objects that make up a stash comparison.
// OID is also the stable source identity; Selector deliberately does not appear here.
type ChangeSource struct {
	OID          string
	BaseOID      string
	UntrackedOID string
}

// Stash is one read-only stash reflog row with a display selector and immutable source.
type Stash struct {
	OID       string
	Selector  string
	Branch    string
	Message   string
	Timestamp int64
	Files     uint64
	Additions uint64
	Deletions uint64
	Source    ChangeSource
}

// ChangeDocument is the shared immutable file/diff reader input used by stash
// browsing and, later, commit readers.
type ChangeDocument struct {
	Change ChangedFile
	Old    File
	New    File
	Patch  File
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

// ListCommits returns bounded current-HEAD history ordered newest first.
func (r *Repository) ListCommits() ([]Commit, error) {
	commits, err := r.git.ListCommits(r.root)
	if err != nil {
		return nil, err
	}
	result := make([]Commit, len(commits))
	for index, commit := range commits {
		result[index] = Commit{OID: commit.OID, ShortOID: commit.ShortOID, Subject: commit.Subject}
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

// ListStashes returns every refs/stash reflog entry with OID-backed aggregate stats.
func (r *Repository) ListStashes() ([]Stash, error) {
	entries, err := r.git.ListStashes(r.root)
	if err != nil {
		return nil, err
	}
	stashes := make([]Stash, len(entries))
	for index, entry := range entries {
		stashes[index] = Stash{
			OID: entry.OID, Selector: entry.Selector, Branch: entry.Branch,
			Message: entry.Message, Timestamp: entry.Timestamp, Files: entry.FileCount,
			Additions: entry.Additions, Deletions: entry.Deletions,
			Source: ChangeSource{OID: entry.OID, BaseOID: entry.BaseOID, UntrackedOID: entry.UntrackedOID},
		}
	}
	return stashes, nil
}

// ListStashFiles enumerates the combined tracked and untracked paths stored by a stash.
func (r *Repository) ListStashFiles(source ChangeSource) ([]ChangedFile, error) {
	changes, err := r.git.ListStashChanges(r.root, gitStashSource(source))
	if err != nil {
		return nil, err
	}
	files := make([]ChangedFile, len(changes))
	for index, change := range changes {
		files[index] = fromGitChangedFile(change)
	}
	return files, nil
}

// ReadStashFile reads exact old/new blobs and their patch without consulting
// the index, worktree, HEAD, selectors, or mutable refs.
func (r *Repository) ReadStashFile(source ChangeSource, change ChangedFile) ChangeDocument {
	document := ChangeDocument{Change: change}
	oldOID := source.BaseOID
	newOID := source.OID
	oldPath := change.Path
	if change.PreviousPath != "" {
		oldPath = change.PreviousPath
	}
	if change.Kind == ChangeUntracked {
		empty, err := r.git.EmptyTree(r.root)
		if err != nil {
			document.Patch = File{Path: change.Path, Kind: FileUnreadable, Err: err}
			return document
		}
		oldOID = empty
		newOID = source.UntrackedOID
		document.Old = File{Path: oldPath, Kind: FileMissing}
	} else {
		document.Old = fromGitObject(oldPath, r.git.ReadObjectFile(r.root, oldOID, oldPath, r.maxBytes))
	}
	if change.Kind == ChangeDeleted {
		document.New = File{Path: change.Path, Kind: FileMissing}
	} else {
		document.New = fromGitObject(change.Path, r.git.ReadObjectFile(r.root, newOID, change.Path, r.maxBytes))
	}
	paths := []string{change.Path}
	if oldPath != change.Path {
		paths = append([]string{oldPath}, paths...)
	}
	document.Patch = fromGitObject(change.Path, r.git.DiffObjects(r.root, oldOID, newOID, paths, r.maxBytes))
	return document
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

func gitStashSource(source ChangeSource) gitadapter.StashSource {
	return gitadapter.StashSource{
		OID: source.OID, BaseOID: source.BaseOID, UntrackedOID: source.UntrackedOID,
	}
}

func fromGitChangedFile(change gitadapter.ChangedFile) ChangedFile {
	return ChangedFile{
		Path: change.Path, PreviousPath: change.PreviousPath,
		Kind: fromGitChangeKind(change.Kind), Additions: change.Additions,
		Deletions: change.Deletions, Binary: change.Binary,
	}
}

func fromGitChangeKind(kind gitadapter.ChangeKind) ChangeKind {
	switch kind {
	case gitadapter.ChangeAdded:
		return ChangeAdded
	case gitadapter.ChangeDeleted:
		return ChangeDeleted
	case gitadapter.ChangeRenamed:
		return ChangeRenamed
	case gitadapter.ChangeCopied:
		return ChangeCopied
	case gitadapter.ChangeUntracked:
		return ChangeUntracked
	default:
		return ChangeModified
	}
}

func fromGitObject(path string, object gitadapter.ObjectFile) File {
	file := File{Path: path, Content: string(object.Data), Size: object.Size, Err: object.Err}
	switch object.Kind {
	case gitadapter.ObjectReady:
		file.Kind = FileReady
	case gitadapter.ObjectMissing:
		file.Kind = FileMissing
	case gitadapter.ObjectBinary:
		file.Kind = FileBinary
	case gitadapter.ObjectTooLarge:
		file.Kind = FileTooLarge
	default:
		file.Kind = FileUnreadable
	}
	return file
}
