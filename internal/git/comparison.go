package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const turnBaselineRef = "refs/worktree/reviewr/turn-base"

// DefaultBranch resolves the branch named by origin/HEAD without contacting a
// remote. The returned object identity is pinned for the caller's comparison.
func (Client) DefaultBranch(root string) (string, string, error) {
	target, err := runBoundedAllowExit(
		root, 1024, allowExitCodeOne,
		"symbolic-ref", "--quiet", "refs/remotes/origin/HEAD",
	)
	if err != nil {
		return "", "", err
	}
	ref := strings.TrimSpace(string(target))
	if ref == "" {
		oid, exists, resolveErr := resolveCommitOID(root, "refs/remotes/origin/HEAD")
		if resolveErr != nil || !exists {
			return "", "", resolveErr
		}
		return "HEAD", oid, nil
	}
	const prefix = "refs/remotes/origin/"
	if !strings.HasPrefix(ref, prefix) || ref == prefix+"HEAD" {
		return "", "", fmt.Errorf("origin/HEAD points outside origin branches")
	}
	oid, exists, err := resolveCommitOID(root, ref)
	if err != nil || !exists {
		return "", "", err
	}
	return strings.TrimPrefix(ref, prefix), oid, nil
}

// MergeBase returns the best common ancestor of HEAD and a pinned base tip.
func (Client) MergeBase(root, baseOID string) (string, error) {
	if !validObjectID(baseOID) {
		return "", fmt.Errorf("invalid base branch identity")
	}
	output, err := runBoundedAllowExit(
		root, 128, allowExitCodeOne,
		"merge-base", baseOID, "HEAD",
	)
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(output))
	if oid == "" {
		return "", nil
	}
	if !validObjectID(oid) {
		return "", fmt.Errorf("git returned an invalid merge-base identity")
	}
	return oid, nil
}

// WorktreeChanges compares one immutable tree with the live worktree and
// includes untracked, non-ignored files.
func (client Client) WorktreeChanges(root, basis string) ([]FileEntry, error) {
	if !validObjectID(basis) {
		return nil, fmt.Errorf("invalid worktree comparison identity")
	}
	changes, err := worktreeTrackedChanges(root, basis)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]FileEntry, len(changes))
	for _, change := range changes {
		byPath[change.Path] = changedFileEntry(change)
	}
	untrackedOutput, err := run(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range ParseNUL(untrackedOutput) {
		if _, exists := byPath[path]; exists {
			continue
		}
		stat := untrackedFileStat(root, path)
		byPath[path] = FileEntry{
			Path: path, State: FileUntracked,
			Additions: stat.additions, Deletions: stat.deletions, Binary: stat.binary,
		}
	}
	return sortedFileEntries(byPath), nil
}

func worktreeTrackedChanges(root, basis string) ([]ChangedFile, error) {
	numstat, err := run(
		root, "diff", "-z", "--numstat", "-M", "--no-ext-diff", "--no-textconv", "--no-color", basis, "--",
	)
	if err != nil {
		return nil, err
	}
	stats, err := parseNumstatDetails(numstat)
	if err != nil {
		return nil, err
	}
	status, err := run(
		root, "diff", "-z", "--name-status", "-M", "--no-ext-diff", "--no-textconv", "--no-color", basis, "--",
	)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(status, stats)
}

// TreeChanges compares two immutable worktree snapshots.
func (client Client) TreeChanges(root, oldTree, newTree string) ([]FileEntry, error) {
	changes, err := client.changedBetween(root, oldTree, newTree)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]FileEntry, len(changes))
	for _, change := range changes {
		entries[change.Path] = changedFileEntry(change)
	}
	return sortedFileEntries(entries), nil
}

func changedFileEntry(change ChangedFile) FileEntry {
	state := FileModified
	switch change.Kind {
	case ChangeAdded:
		state = FileAdded
	case ChangeDeleted:
		state = FileDeleted
	case ChangeRenamed:
		state = FileRenamed
	case ChangeCopied:
		state = FileAdded
	case ChangeUntracked:
		state = FileUntracked
	}
	return FileEntry{
		Path: change.Path, PreviousPath: change.PreviousPath, State: state,
		Additions: change.Additions, Deletions: change.Deletions, Binary: change.Binary,
	}
}

func sortedFileEntries(entries map[string]FileEntry) []FileEntry {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]FileEntry, 0, len(paths))
	for _, path := range paths {
		result = append(result, entries[path])
	}
	return result
}

// SnapshotWorktree writes a tree through an isolated temporary index. It
// never changes the real index, worktree, or a public ref.
func (client Client) SnapshotWorktree(root string) (string, error) {
	gitDir, err := client.ResolveGitDir(root)
	if err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(gitDir, "reviewr-turn-")
	if err != nil {
		return "", fmt.Errorf("create turn snapshot index: %w", err)
	}
	defer os.RemoveAll(temporary)

	index := filepath.Join(temporary, "index")
	realIndex := filepath.Join(gitDir, "index")
	if _, statErr := os.Stat(realIndex); statErr == nil {
		if copyErr := copyFile(realIndex, index); copyErr != nil {
			return "", fmt.Errorf("seed turn snapshot index: %w", copyErr)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect worktree index: %w", statErr)
	}
	environment := map[string]string{"GIT_INDEX_FILE": index}
	if _, err := runBoundedWithEnv(root, defaultMaxCommandBytes, environment, "add", "-A", "--"); err != nil {
		return "", err
	}
	output, err := runBoundedWithEnv(root, 128, environment, "write-tree")
	if err != nil {
		return "", err
	}
	tree := strings.TrimSpace(string(output))
	if !validObjectID(tree) {
		return "", fmt.Errorf("git returned an invalid worktree tree identity")
	}
	return tree, nil
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
}

// TurnBaseline reads this worktree's private last-turn tree.
func (Client) TurnBaseline(root string) (string, bool, error) {
	output, err := runBoundedAllowExit(
		root, 128, allowExitCodeOne,
		"rev-parse", "--verify", "--quiet", turnBaselineRef+"^{tree}",
	)
	if err != nil {
		return "", false, err
	}
	tree := strings.TrimSpace(string(output))
	if tree == "" {
		return "", false, nil
	}
	if !validObjectID(tree) {
		return "", false, fmt.Errorf("git returned an invalid turn baseline identity")
	}
	return tree, true, nil
}

// WriteTurnBaseline atomically updates only reviewr's worktree-private ref.
func (Client) WriteTurnBaseline(root, tree string) error {
	if !validObjectID(tree) {
		return fmt.Errorf("invalid turn baseline identity")
	}
	_, err := run(root, "update-ref", turnBaselineRef, tree)
	return err
}
