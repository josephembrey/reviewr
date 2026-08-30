package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	defaultMaxCommandBytes int64 = 64 << 20
	maxStderrBytes         int64 = 64 << 10
)

var allowExitCodeOne = map[int]struct{}{1: {}}

func run(root string, args ...string) ([]byte, error) {
	return runBounded(root, defaultMaxCommandBytes, args...)
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
	if len(args) == 0 {
		return nil, fmt.Errorf("run git: no command")
	}
	commandArgs := append([]string{"--literal-pathspecs", "-C", root}, args...)
	cmd := exec.Command("git", commandArgs...)
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	cmd.Stdin = input
	stdout := boundedBuffer{limit: max(0, maxBytes)}
	stderr := boundedBuffer{limit: maxStderrBytes}
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
		keep := min(int64(length), remaining)
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
	const name = "GIT_OPTIONAL_LOCKS="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if len(entry) >= len(name) && entry[:len(name)] == name {
			continue
		}
		result = append(result, entry)
	}
	return append(result, name+"0")
}
