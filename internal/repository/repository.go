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
	"strings"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
	"github.com/josephembrey/reviewr/internal/review"
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

// ReviewRepositoryID returns the canonical private-state namespace without
// enumerating or mutating repository status.
func (r *Repository) ReviewRepositoryID() (review.RepositoryID, error) {
	common, err := r.git.CommonGitDir(r.root)
	if err != nil {
		return review.RepositoryID{}, err
	}
	return review.ResolveRepositoryID(r.root, common)
}

// ReviewComparisons enriches already-enumerated typed candidates with exact
// uncommitted endpoint identities. Unsupported comparison bases remain inert
// rather than borrowing unsafe bounds.
func (r *Repository) ReviewComparisons(scope string, candidates []review.Candidate) (review.Snapshot, error) {
	snapshot := review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	if scope != "uncommitted" {
		return snapshot, fmt.Errorf("exact %s comparison is not available", scope)
	}
	head, err := r.git.HeadOID(r.root)
	unborn := false
	if errors.Is(err, gitadapter.ErrUnbornHead) {
		head, err = r.git.EmptyTreeOID(r.root)
		unborn = true
		if err != nil {
			return snapshot, fmt.Errorf("resolve review comparison HEAD: %w", err)
		}
	} else if err != nil {
		return snapshot, fmt.Errorf("resolve review comparison HEAD: %w", err)
	}
	for _, candidate := range candidates {
		basisReason := ""
		oldPath := candidate.Path
		if candidate.PreviousPath != "" {
			oldPath = candidate.PreviousPath
		}
		old := review.AbsentEndpoint(oldPath)
		if !unborn {
			old, err = r.treeEndpoint(head, oldPath)
			if err != nil {
				old = review.Endpoint{Path: oldPath, Kind: review.Regular}
				basisReason = "comparison basis content is unavailable"
			}
		}
		new := r.worktreeReviewContent(candidate.Path).Endpoint
		expectsAbsent := candidate.Action == review.Deleted
		if (expectsAbsent && new.Kind != review.Absent) || (!expectsAbsent && new.Kind == review.Absent) {
			new.ContentID = ""
			basisReason = "file changed during comparison snapshot"
		}
		if candidate.Action == review.Added && old.Kind != review.Absent {
			old.ContentID = ""
			basisReason = "comparison action and basis do not agree"
		}
		if candidate.Action != review.Added && old.Kind == review.Absent {
			old.ContentID = ""
			basisReason = "comparison action and basis do not agree"
		}
		if basisReason == "" && (!old.Exact() || !new.Exact()) {
			basisReason = "exact comparison content is unavailable"
		}
		comparison := review.FileComparison{
			Identity:  review.ComparisonIdentity{Scope: scope, Basis: head},
			OldSource: review.EndpointSource{Kind: review.GitTreeSource, Value: head},
			NewSource: review.EndpointSource{Kind: review.WorktreeSource},
			Action:    candidate.Action,
			Old:       old,
			New:       new,
		}
		comparison.BasisReason = basisReason
		if comparison.BasisReason == "" && (candidate.Action == review.Renamed || candidate.Action == review.Copied) {
			comparison.BasisReason = "rename or copy lineage requires a full review"
		}
		snapshot.Comparisons[candidate.Path] = comparison
	}
	return snapshot, nil
}

// ReadReviewContent materializes one exact comparison endpoint through bounded,
// entry-aware repository reads.
func (r *Repository) ReadReviewContent(source review.EndpointSource, endpoint review.Endpoint) review.Content {
	if endpoint.Kind == review.Absent {
		if source.Kind == review.WorktreeSource {
			return r.worktreeReviewContent(endpoint.Path)
		}
		return review.AbsentContent(endpoint.Path)
	}
	if source.Kind == review.GitTreeSource {
		return r.treeReviewContent(source.Value, endpoint)
	}
	return r.worktreeReviewContent(endpoint.Path)
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

// ReadFile reads one typed root-relative entry without following a final symlink.
func (r *Repository) ReadFile(entry Entry) File {
	result := File{Path: entry.Path}
	fullPath, err := r.resolvePath(entry.Path)
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

// ReadDiff renders one bounded patch without allowing Git pathspec magic.
func (r *Repository) ReadDiff(entry Entry) Diff {
	result := Diff{Entry: entry}
	if err := validatePath(entry.Path); err != nil {
		result.Kind = DiffUnavailable
		result.Err = err
		return result
	}
	if entry.PreviousPath != "" {
		if err := validatePath(entry.PreviousPath); err != nil {
			result.Kind = DiffUnavailable
			result.Err = err
			return result
		}
	}
	if entry.State == FileUnchanged || entry.State == FileIgnored {
		result.Kind = DiffReady
		return result
	}
	data, err := r.git.ReadDiff(
		r.root,
		entry.Path,
		entry.PreviousPath,
		entry.State == FileUntracked,
		r.maxBytes,
	)
	result.Size = int64(len(data))
	if err != nil {
		if errors.Is(err, gitadapter.ErrOutputTooLarge) {
			result.Kind = DiffTooLarge
		} else {
			result.Kind = DiffUnavailable
		}
		result.Err = err
		return result
	}
	result.Kind = DiffReady
	result.Content = string(data)
	return result
}

func (r *Repository) treeEndpoint(revision, path string) (review.Endpoint, error) {
	entry, exists, err := r.git.ReadTreeEntry(r.root, revision, path)
	if err != nil {
		return review.Endpoint{}, err
	}
	if !exists {
		return review.AbsentEndpoint(path), nil
	}
	kind := review.Regular
	switch entry.Mode {
	case 0o120000:
		kind = review.Symlink
	case 0o160000:
		kind = review.Submodule
	}
	return review.Endpoint{Path: path, Kind: kind, Mode: entry.Mode, ContentID: "git:" + entry.OID}, nil
}

func (r *Repository) treeReviewContent(_ string, endpoint review.Endpoint) review.Content {
	if endpoint.Kind == review.Submodule {
		return review.Content{Endpoint: endpoint, State: review.ContentText, Text: strings.TrimPrefix(endpoint.ContentID, "git:"), Size: int64(len(endpoint.ContentID) - len("git:"))}
	}
	oid := strings.TrimPrefix(endpoint.ContentID, "git:")
	data, err := r.git.ReadObjectContent(r.root, oid, r.maxBytes)
	if errors.Is(err, gitadapter.ErrOutputTooLarge) {
		return review.Content{Endpoint: endpoint, State: review.ContentTooLarge, Size: r.maxBytes + 1}
	}
	if err != nil {
		return review.UnavailableContent(endpoint.Path, endpoint.Kind, endpoint.Mode, err)
	}
	state := review.ContentText
	text := string(data)
	if bytes.IndexByte(data, 0) >= 0 {
		state = review.ContentBinary
		text = ""
	}
	return review.Content{Endpoint: endpoint, State: state, Text: text, Size: int64(len(data))}
}

func (r *Repository) worktreeReviewContent(path string) review.Content {
	fullPath, info, missing, err := r.resolveReviewPath(path)
	if missing {
		return review.AbsentContent(path)
	}
	if err != nil {
		return review.UnavailableContent(path, review.Regular, 0, err)
	}
	kind := review.Regular
	mode := uint32(0o100644)
	if info.Mode()&fs.ModeSymlink != 0 {
		kind = review.Symlink
		mode = 0o120000
		target, readErr := os.Readlink(fullPath)
		if readErr != nil {
			return review.UnavailableContent(path, kind, mode, readErr)
		}
		return r.hashWorktreeContent(path, kind, mode, strings.NewReader(target))
	}
	if info.IsDir() {
		oid, headErr := r.git.HeadOID(fullPath)
		if headErr != nil {
			return review.UnavailableContent(path, review.Submodule, 0o160000, headErr)
		}
		endpoint := review.Endpoint{Path: path, Kind: review.Submodule, Mode: 0o160000, ContentID: "git:" + oid}
		return review.Content{Endpoint: endpoint, State: review.ContentText, Text: oid, Size: int64(len(oid))}
	}
	if !info.Mode().IsRegular() {
		return review.UnavailableContent(path, kind, mode, fmt.Errorf("unsupported worktree file type"))
	}
	if info.Mode()&0o111 != 0 {
		mode = 0o100755
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return review.UnavailableContent(path, kind, mode, err)
	}
	defer file.Close()
	return r.hashWorktreeContent(path, kind, mode, file)
}

// resolveReviewPath verifies every existing parent without following symlinks,
// while still allowing a missing parent chain to prove an absent endpoint.
func (r *Repository) resolveReviewPath(path string) (string, fs.FileInfo, bool, error) {
	if err := validatePath(path); err != nil {
		return "", nil, false, err
	}
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return "", nil, false, err
	}
	segments := strings.Split(filepath.ToSlash(path), "/")
	current := root
	for _, segment := range segments[:len(segments)-1] {
		current = filepath.Join(current, filepath.FromSlash(segment))
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			return filepath.Join(root, filepath.FromSlash(path)), nil, true, nil
		}
		if statErr != nil {
			return "", nil, false, statErr
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return "", nil, false, fmt.Errorf("path escapes the worktree through a symlink")
		}
	}
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(fullPath)
	if errors.Is(err, fs.ErrNotExist) {
		return fullPath, nil, true, nil
	}
	return fullPath, info, false, err
}

// hashWorktreeContent lets Git identify the exact same byte stream that the
// bounded reader classifies. This avoids a read/hash race without retaining
// binary or oversized bodies in memory.
func (r *Repository) hashWorktreeContent(path string, kind review.FileKind, mode uint32, reader io.Reader) review.Content {
	observer := &reviewContentObserver{limit: r.maxBytes}
	oid, err := r.git.HashObject(r.root, io.TeeReader(reader, observer))
	if err != nil {
		return review.UnavailableContent(path, kind, mode, err)
	}
	content := review.Content{
		Endpoint: review.Endpoint{Path: path, Kind: kind, Mode: mode, ContentID: "git:" + oid},
		Size:     observer.size,
	}
	switch {
	case observer.binary:
		content.State = review.ContentBinary
	case observer.size > observer.limit:
		content.State = review.ContentTooLarge
	default:
		content.State = review.ContentText
		content.Text = observer.retained.String()
	}
	return content
}

type reviewContentObserver struct {
	limit    int64
	size     int64
	binary   bool
	retained bytes.Buffer
}

func (observer *reviewContentObserver) Write(data []byte) (int, error) {
	observer.size += int64(len(data))
	observer.binary = observer.binary || bytes.IndexByte(data, 0) >= 0
	remaining := observer.limit + 1 - int64(observer.retained.Len())
	if remaining > 0 {
		keep := int64(len(data))
		if keep > remaining {
			keep = remaining
		}
		_, _ = observer.retained.Write(data[:keep])
	}
	return len(data), nil
}

func (r *Repository) resolvePath(path string) (string, error) {
	if err := validatePath(path); err != nil {
		return "", err
	}
	relative := filepath.FromSlash(path)
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

func validatePath(path string) error {
	relative := filepath.FromSlash(path)
	if !filepath.IsLocal(relative) || relative == "." {
		return fmt.Errorf("path is not worktree-relative")
	}
	return nil
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
