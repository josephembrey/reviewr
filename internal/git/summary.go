package git

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// ChangeSummary is the aggregate current-worktree diff against HEAD.
type ChangeSummary struct {
	Files     uint64
	Additions uint64
	Deletions uint64
}

// WorktreeSummary counts tracked, staged, unstaged, and untracked changes
// without writing the worktree, index, object database, or refs.
func (Client) WorktreeSummary(root string) (ChangeSummary, error) {
	base, err := run(root, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		base, err = hashEmptyTree(root)
		if err != nil {
			return ChangeSummary{}, err
		}
	}
	base = bytes.TrimSpace(base)

	numstat, err := run(root, "diff", "-M", "-C", string(base), "--numstat", "-z", "--")
	if err != nil {
		return ChangeSummary{}, err
	}
	changes, err := parseNumstat(numstat)
	if err != nil {
		return ChangeSummary{}, err
	}

	untracked, err := run(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return ChangeSummary{}, err
	}
	for _, path := range ParseNUL(untracked) {
		if _, exists := changes[path]; !exists {
			changes[path] = [2]uint64{untrackedAdditions(root, path), 0}
		}
	}

	summary := ChangeSummary{Files: uint64(len(changes))}
	for _, counts := range changes {
		summary.Additions = saturatingAdd(summary.Additions, counts[0])
		summary.Deletions = saturatingAdd(summary.Deletions, counts[1])
	}
	return summary, nil
}

func hashEmptyTree(root string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "hash-object", "-t", "tree", "--stdin")
	cmd.Env = withOptionalLocksDisabled(os.Environ())
	cmd.Stdin = bytes.NewReader(nil)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git hash-object: %s: %w", bytes.TrimSpace(stderr.Bytes()), err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, fmt.Errorf("git hash-object returned an empty tree identity")
	}
	return out, nil
}

func parseNumstat(data []byte) (map[string][2]uint64, error) {
	fields := bytes.Split(data, []byte{0})
	changes := make(map[string][2]uint64)
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if len(field) == 0 {
			continue
		}
		parts := bytes.SplitN(field, []byte{'\t'}, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse git numstat record %q", field)
		}
		additions := parseCount(parts[0])
		deletions := parseCount(parts[1])
		path := string(parts[2])
		if path == "" {
			if index+2 >= len(fields) {
				return nil, fmt.Errorf("parse truncated git numstat rename")
			}
			index += 2
			path = string(fields[index])
		}
		if path != "" {
			changes[path] = [2]uint64{additions, deletions}
		}
	}
	return changes, nil
}

func parseCount(value []byte) uint64 {
	count, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		return 0
	}
	return count
}

func untrackedAdditions(root, path string) uint64 {
	relative := filepath.FromSlash(path)
	if !filepath.IsLocal(relative) {
		return 0
	}
	fullPath := filepath.Join(root, relative)
	info, err := os.Lstat(fullPath)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	buffer := make([]byte, 32<<10)
	var lines uint64
	var last byte
	seen := false
	for {
		read, readErr := file.Read(buffer)
		if read > 0 {
			chunk := buffer[:read]
			if bytes.IndexByte(chunk, 0) >= 0 {
				return 0
			}
			seen = true
			last = chunk[len(chunk)-1]
			lines = saturatingAdd(lines, uint64(bytes.Count(chunk, []byte{'\n'})))
		}
		if readErr != nil {
			if readErr != io.EOF {
				return 0
			}
			break
		}
	}
	if seen && last != '\n' {
		lines = saturatingAdd(lines, 1)
	}
	return lines
}

func saturatingAdd(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}
