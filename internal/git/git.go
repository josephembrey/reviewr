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
)

// Client invokes the Git executable without permitting optional lock writes.
type Client struct{}

// ErrOutputTooLarge reports that bounded Git output exceeded its memory budget.
var ErrOutputTooLarge = errors.New("git output exceeds limit")

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
	return runBoundedInputAllowExit(root, maxBytes, nil, nil, args...)
}

func runBoundedInput(root string, maxBytes int64, input io.Reader, args ...string) ([]byte, error) {
	return runBoundedInputAllowExit(root, maxBytes, input, nil, args...)
}

func runBoundedAllowExit(root string, maxBytes int64, allowedExitCodes map[int]struct{}, args ...string) ([]byte, error) {
	return runBoundedInputAllowExit(root, maxBytes, nil, allowedExitCodes, args...)
}

func runBoundedInputAllowExit(root string, maxBytes int64, input io.Reader, allowedExitCodes map[int]struct{}, args ...string) ([]byte, error) {
	commandArgs := append([]string{"--literal-pathspecs", "-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	cmd.Stdin = input
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
