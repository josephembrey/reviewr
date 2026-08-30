// Package git provides reviewr's read-only Git CLI boundary.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	return filepath.Clean(string(out)), nil
}

// CommonGitDir returns the absolute repository directory shared by linked worktrees.
func (Client) CommonGitDir(root string) (string, error) {
	out, err := run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", errors.New("git returned an empty common directory")
	}
	return filepath.Clean(value), nil
}

// HeadOID returns the exact current HEAD object identity.
func (Client) HeadOID(root string) (string, error) {
	out, err := runBoundedAllowExit(root, 128, map[int]struct{}{1: {}}, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", ErrUnbornHead
	}
	return value, nil
}

// TreeEntry is exact immutable metadata for one path at a revision.
type TreeEntry struct {
	Mode uint32
	Type string
	OID  string
	Path string
}

// ReadTreeEntry reads one literal path from an immutable tree.
func (Client) ReadTreeEntry(root, revision, path string) (TreeEntry, bool, error) {
	out, err := run(root, "ls-tree", "-z", "--full-tree", revision, "--", path)
	if err != nil {
		return TreeEntry{}, false, err
	}
	if len(out) == 0 {
		return TreeEntry{}, false, nil
	}
	record := bytes.TrimSuffix(out, []byte{0})
	tab := bytes.IndexByte(record, '\t')
	if tab < 0 {
		return TreeEntry{}, false, errors.New("malformed git ls-tree record")
	}
	fields := strings.Fields(string(record[:tab]))
	if len(fields) != 3 {
		return TreeEntry{}, false, errors.New("malformed git ls-tree metadata")
	}
	mode, err := strconv.ParseUint(fields[0], 8, 32)
	if err != nil {
		return TreeEntry{}, false, fmt.Errorf("parse git tree mode: %w", err)
	}
	return TreeEntry{Mode: uint32(mode), Type: fields[1], OID: fields[2], Path: string(record[tab+1:])}, true, nil
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
	commandArgs := []string{"-C", root, "hash-object", "-t", objectType, "--stdin"}
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	cmd.Stdin = reader
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git hash-object %s: %s: %w", objectType, bytes.TrimSpace(stderr.Bytes()), err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", errors.New("git hash-object returned an empty identity")
	}
	return value, nil
}

// ParseNUL splits NUL-delimited Git output. A final unterminated field is kept
// so a nonconforming producer cannot silently drop the last path.
func ParseNUL(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	parts := bytes.Split(data, []byte{0})
	if len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			paths = append(paths, string(part))
		}
	}
	return paths
}

func run(root string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"--literal-pathspecs", "-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := bytes.TrimSpace(stderr.Bytes())
		if len(message) == 0 {
			return nil, fmt.Errorf("git %s: %w", args[0], err)
		}
		return nil, fmt.Errorf("git %s: %s: %w", args[0], message, err)
	}
	return out, nil
}

func runBounded(root string, maxBytes int64, args ...string) ([]byte, error) {
	return runBoundedAllowExit(root, maxBytes, nil, args...)
}

func runBoundedAllowExit(root string, maxBytes int64, allowedExitCodes map[int]struct{}, args ...string) ([]byte, error) {
	commandArgs := append([]string{"--literal-pathspecs", "-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	stdout := boundedBuffer{limit: max(0, maxBytes)}
	stderr := boundedBuffer{limit: 64 << 10}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.truncated {
		return nil, fmt.Errorf("git %s: %w (%d bytes)", args[0], ErrOutputTooLarge, maxBytes)
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if _, allowed := allowedExitCodes[exitError.ExitCode()]; allowed {
			err = nil
		}
	}
	if err != nil {
		message := bytes.TrimSpace(stderr.Bytes())
		if len(message) == 0 {
			return nil, fmt.Errorf("git %s: %w", args[0], err)
		}
		return nil, fmt.Errorf("git %s: %s: %w", args[0], message, err)
	}
	return stdout.Bytes(), nil
}

type boundedBuffer struct {
	data      bytes.Buffer
	limit     int64
	written   int64
	truncated bool
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	length := len(data)
	remaining := b.limit - b.written
	if remaining > 0 {
		keep := int64(length)
		if keep > remaining {
			keep = remaining
		}
		_, _ = b.data.Write(data[:keep])
		b.written += keep
	}
	if int64(length) > remaining {
		b.truncated = true
	}
	return length, nil
}

func (b *boundedBuffer) Bytes() []byte {
	return b.data.Bytes()
}

func withOptionalLocksDisabled(env []string) []string {
	const setting = "GIT_OPTIONAL_LOCKS=0"
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if len(entry) >= len("GIT_OPTIONAL_LOCKS=") && entry[:len("GIT_OPTIONAL_LOCKS=")] == "GIT_OPTIONAL_LOCKS=" {
			continue
		}
		result = append(result, entry)
	}
	return append(result, setting)
}
