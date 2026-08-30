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
		return r.treeReviewContent(endpoint)
	}
	return r.worktreeReviewContent(endpoint.Path)
}

func (r *Repository) treeReviewContent(endpoint review.Endpoint) review.Content {
	if endpoint.Kind == review.Submodule {
		oid := strings.TrimPrefix(endpoint.ContentID, "git:")
		return review.Content{Endpoint: endpoint, State: review.ContentText, Text: oid, Size: int64(len(oid))}
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
		keep := min(int64(len(data)), remaining)
		_, _ = observer.retained.Write(data[:keep])
	}
	return len(data), nil
}
