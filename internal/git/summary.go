package git

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

// ChangeSummary is the aggregate current-worktree diff against HEAD.
type ChangeSummary struct {
	Files     uint64
	Additions uint64
	Deletions uint64
}

// WorktreeSummary counts tracked, staged, unstaged, and untracked changes
// without writing the worktree, index, object database, or refs.
func (client Client) WorktreeSummary(root string) (ChangeSummary, error) {
	untracked, err := run(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return ChangeSummary{}, err
	}
	changes, err := client.worktreeStats(root, ParseNUL(untracked))
	if err != nil {
		return ChangeSummary{}, err
	}

	summary := ChangeSummary{Files: uint64(len(changes))}
	for _, counts := range changes {
		summary.Additions = saturatingAdd(summary.Additions, counts.additions)
		summary.Deletions = saturatingAdd(summary.Deletions, counts.deletions)
	}
	return summary, nil
}

func (Client) worktreeStats(root string, untracked []string) (map[string]changeStat, error) {
	base, err := run(root, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		base, err = hashEmptyTree(root)
		if err != nil {
			return nil, err
		}
	}
	base = bytes.TrimSpace(base)

	numstat, err := run(root, "diff", "-M", "-C", string(base), "--numstat", "-z", "--")
	if err != nil {
		return nil, err
	}
	changes, err := parseNumstatDetails(numstat)
	if err != nil {
		return nil, err
	}

	for _, path := range untracked {
		if _, exists := changes[path]; !exists {
			changes[path] = untrackedFileStat(root, path)
		}
	}
	return changes, nil
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

func untrackedFileStat(root, path string) changeStat {
	relative := filepath.FromSlash(path)
	if !filepath.IsLocal(relative) {
		return changeStat{}
	}
	fullPath := filepath.Join(root, relative)
	info, err := os.Lstat(fullPath)
	if err != nil || !info.Mode().IsRegular() {
		return changeStat{}
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return changeStat{}
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
				return changeStat{binary: true}
			}
			seen = true
			last = chunk[len(chunk)-1]
			lines = saturatingAdd(lines, uint64(bytes.Count(chunk, []byte{'\n'})))
		}
		if readErr != nil {
			if readErr != io.EOF {
				return changeStat{}
			}
			break
		}
	}
	if seen && last != '\n' {
		lines = saturatingAdd(lines, 1)
	}
	return changeStat{additions: lines}
}

func saturatingAdd(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}
