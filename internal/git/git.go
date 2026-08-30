// Package git provides reviewr's read-only Git CLI boundary.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Client invokes the Git executable without permitting optional lock writes.
type Client struct{}

// ErrOutputTooLarge reports that bounded Git output exceeded its memory budget.
var ErrOutputTooLarge = errors.New("git output exceeds limit")

// ErrUnbornHead reports a repository that has no first commit yet.
var ErrUnbornHead = errors.New("git HEAD is unborn")

// New returns a read-only Git CLI client.
func New() Client {
	return Client{}
}

// ResolveRoot returns the absolute root of the worktree containing path.
func (Client) ResolveRoot(path string) (string, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repository path %q: %w", path, err)
	}
	out, err := run(absPath, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return "", err
	}
	out = bytes.TrimSuffix(out, []byte{'\n'})
	if len(out) == 0 {
		return "", fmt.Errorf("git returned an empty worktree root for %q", path)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(string(out)))
	if err != nil {
		return "", fmt.Errorf("canonicalize Git worktree root: %w", err)
	}
	return canonical, nil
}

// ResolveCommonDir returns the canonical absolute Git common directory. Git's
// machine-oriented rev-parse output is decoded when it quotes an unusual path.
// Linked worktrees of one clone resolve to the same identity.
func (Client) ResolveCommonDir(root string) (string, error) {
	out, err := run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return canonicalGitPath(out, "common directory")
}

// ResolveGitDir returns the canonical per-worktree Git directory. It equals
// ResolveCommonDir only for the primary checkout; linked worktrees have their
// own administrative directory below the common directory.
func (Client) ResolveGitDir(root string) (string, error) {
	out, err := run(root, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", err
	}
	return canonicalGitPath(out, "Git directory")
}

func canonicalGitPath(out []byte, label string) (string, error) {
	value := strings.TrimSuffix(string(out), "\n")
	if value == "" || strings.ContainsRune(value, '\n') || strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("git returned an invalid %s", label)
	}
	if strings.HasPrefix(value, "\"") {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("decode Git %s: %w", label, err)
		}
		value = decoded
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("git returned a non-absolute %s %q", label, value)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(value))
	if err != nil {
		return "", fmt.Errorf("canonicalize Git %s: %w", label, err)
	}
	return canonical, nil
}

// HeadOID returns the exact current HEAD object identity.
func (Client) HeadOID(root string) (string, error) {
	value, exists, err := resolveCommitOID(root, "HEAD")
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrUnbornHead
	}
	return value, nil
}

func resolveCommitOID(root, revision string) (string, bool, error) {
	out, err := runBoundedAllowExit(
		root,
		128,
		allowExitCodeOne,
		"rev-parse",
		"--verify",
		"--quiet",
		revision+"^{commit}",
	)
	if err != nil {
		return "", false, err
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", false, nil
	}
	if !validObjectID(value) {
		return "", false, fmt.Errorf("git returned an invalid commit identity for %s", revision)
	}
	return value, true, nil
}

// ReadObjectContent returns bounded bytes for one already-resolved immutable
// blob identity, avoiding revision/path parsing at content-read time.
func (Client) ReadObjectContent(root, oid string, maxBytes int64) ([]byte, error) {
	return runBounded(root, maxBytes, "cat-file", "blob", oid)
}

// HashObject streams one worktree state through Git's configured object hash
// without writing it to the object database.
func (Client) HashObject(root string, reader io.Reader) (string, error) {
	return hashObject(root, "blob", reader)
}

// EmptyTreeOID computes Git's configured empty-tree identity without writing
// an object, allowing exact added-file comparisons in an unborn repository.
func (Client) EmptyTreeOID(root string) (string, error) {
	return hashObject(root, "tree", bytes.NewReader(nil))
}

func hashObject(root, objectType string, reader io.Reader) (string, error) {
	out, err := runBoundedInput(root, 128, reader, "hash-object", "-t", objectType, "--stdin")
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", errors.New("git hash-object returned an empty identity")
	}
	return value, nil
}
