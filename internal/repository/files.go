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
func (r *Repository) ReadDiff(comparison Comparison, entry Entry) Diff {
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
	if !comparison.Available() {
		result.Kind = DiffUnavailable
		result.Err = fmt.Errorf("%s", comparison.Reason)
		return result
	}
	var data []byte
	var err error
	if comparison.Target != "" {
		data, err = r.git.ReadDiffBetween(
			r.root,
			comparison.Basis,
			comparison.Target,
			entry.Path,
			entry.PreviousPath,
			r.maxBytes,
		)
	} else {
		data, err = r.git.ReadDiffFrom(
			r.root,
			comparison.Basis,
			entry.Path,
			entry.PreviousPath,
			entry.State == FileUntracked,
			r.maxBytes,
		)
	}
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
