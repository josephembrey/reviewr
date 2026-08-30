package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const DefaultMaxStashBytes int64 = 8 << 20

// ChangeKind describes one path stored in an immutable Git comparison.
type ChangeKind uint8

const (
	ChangeModified ChangeKind = iota + 1
	ChangeAdded
	ChangeDeleted
	ChangeRenamed
	ChangeCopied
	ChangeUntracked
)

// ChangedFile is one machine-parsed path and its line statistics.
type ChangedFile struct {
	Path         string
	PreviousPath string
	Kind         ChangeKind
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// StashEntry is one refs/stash reflog entry. OID, not Selector, is its identity.
type StashEntry struct {
	OID          string
	Selector     string
	Branch       string
	Message      string
	Timestamp    int64
	FileCount    uint64
	Additions    uint64
	Deletions    uint64
	BaseOID      string
	UntrackedOID string
}

// StashSource contains only immutable object identities needed to inspect a stash.
type StashSource struct {
	OID          string
	BaseOID      string
	UntrackedOID string
}

// ObjectKind classifies a bounded immutable Git object read.
type ObjectKind uint8

const (
	ObjectReady ObjectKind = iota + 1
	ObjectMissing
	ObjectBinary
	ObjectTooLarge
	ObjectUnreadable
)

// ObjectFile is a bounded blob or patch produced from immutable objects.
type ObjectFile struct {
	Kind ObjectKind
	Data []byte
	Size int64
	Err  error
}

// ListStashes returns the complete stash reflog, newest first, with aggregate stats.
// The reflog selector is presentation only; all comparisons use full object IDs.
func (client Client) ListStashes(root string) ([]StashEntry, error) {
	hasStashes, err := hasStashRef(root)
	if err != nil || !hasStashes {
		return nil, err
	}
	out, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"log",
		"-g",
		"-z",
		"--format=%H%x00%gD%x00%gs%x00%ct%x00%P",
		"--end-of-options",
		"refs/stash",
	)
	if err != nil {
		return nil, err
	}
	entries, err := parseStashLog(out)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		changes, listErr := client.ListStashChanges(root, StashSource{
			OID:          entries[index].OID,
			BaseOID:      entries[index].BaseOID,
			UntrackedOID: entries[index].UntrackedOID,
		})
		if listErr != nil {
			return nil, fmt.Errorf("inspect %s: %w", entries[index].Selector, listErr)
		}
		entries[index].FileCount = uint64(len(changes))
		for _, change := range changes {
			entries[index].Additions = saturatingAdd(entries[index].Additions, change.Additions)
			entries[index].Deletions = saturatingAdd(entries[index].Deletions, change.Deletions)
		}
	}
	return entries, nil
}

// ListStashChanges combines the tracked stash tree with its optional untracked parent.
func (client Client) ListStashChanges(root string, source StashSource) ([]ChangedFile, error) {
	if err := validateStashSource(source); err != nil {
		return nil, err
	}
	changes, err := client.changedBetween(root, source.BaseOID, source.OID)
	if err != nil {
		return nil, err
	}
	if source.UntrackedOID != "" {
		emptyTree, emptyErr := hashEmptyTree(root)
		if emptyErr != nil {
			return nil, emptyErr
		}
		extras, extrasErr := client.changedBetween(root, string(bytes.TrimSpace(emptyTree)), source.UntrackedOID)
		if extrasErr != nil {
			return nil, extrasErr
		}
		for index := range extras {
			extras[index].Kind = ChangeUntracked
		}
		changes = mergeChanges(changes, extras)
	}
	return changes, nil
}

// EmptyTree returns the empty tree identity for this repository's object format.
func (Client) EmptyTree(root string) (string, error) {
	out, err := hashEmptyTree(root)
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(out))
	if !validObjectID(oid) {
		return "", fmt.Errorf("Git returned an invalid empty tree identity")
	}
	return oid, nil
}

func (client Client) changedBetween(root, oldOID, newOID string) ([]ChangedFile, error) {
	if !validObjectID(oldOID) || !validObjectID(newOID) {
		return nil, fmt.Errorf("invalid comparison object identity")
	}
	numstat, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"diff",
		"-z",
		"--numstat",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		oldOID,
		newOID,
		"--",
	)
	if err != nil {
		return nil, err
	}
	stats, err := parseNumstatDetails(numstat)
	if err != nil {
		return nil, err
	}
	status, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"diff",
		"-z",
		"--name-status",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		oldOID,
		newOID,
		"--",
	)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(status, stats)
}

// ReadObjectFile reads one exact path from an exact tree-ish without consulting the worktree.
func (Client) ReadObjectFile(root, oid, path string, maxBytes int64) ObjectFile {
	if !validObjectID(oid) || !validGitPath(path) {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid immutable file identity")}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	expression := oid + ":" + path
	sizeOutput, err := runBounded(root, 128, "cat-file", "-s", expression)
	if err != nil {
		return ObjectFile{Kind: ObjectMissing, Err: err}
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
	if err != nil || size < 0 {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("parse Git object size for %q", path)}
	}
	if size > maxBytes {
		return ObjectFile{Kind: ObjectTooLarge, Size: size}
	}
	data, err := runBounded(root, maxBytes, "cat-file", "blob", expression)
	if err != nil {
		kind := ObjectUnreadable
		if errors.Is(err, ErrOutputTooLarge) {
			kind = ObjectTooLarge
		}
		return ObjectFile{Kind: kind, Size: size, Err: err}
	}
	kind := ObjectReady
	if bytes.IndexByte(data, 0) >= 0 {
		kind = ObjectBinary
	}
	return ObjectFile{Kind: kind, Data: data, Size: size}
}

// DiffObjects returns a bounded patch between exact object IDs for the selected paths.
func (Client) DiffObjects(root, oldOID, newOID string, paths []string, maxBytes int64) ObjectFile {
	if !validObjectID(oldOID) || !validObjectID(newOID) {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid comparison object identity")}
	}
	if len(paths) == 0 {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("comparison has no path")}
	}
	for _, path := range paths {
		if !validGitPath(path) {
			return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid comparison path")}
		}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	args := []string{
		"diff",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		oldOID,
		newOID,
		"--",
	}
	args = append(args, paths...)
	data, err := runBounded(root, maxBytes, args...)
	if err != nil {
		kind := ObjectUnreadable
		if errors.Is(err, ErrOutputTooLarge) {
			kind = ObjectTooLarge
		}
		return ObjectFile{Kind: kind, Size: maxBytes + 1, Err: err}
	}
	return ObjectFile{Kind: ObjectReady, Data: data, Size: int64(len(data))}
}

func hasStashRef(root string) (bool, error) {
	_, err := runBounded(root, 128, "rev-parse", "--verify", "--quiet", "refs/stash^{commit}")
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func parseStashLog(data []byte) ([]StashEntry, error) {
	fields := bytes.Split(data, []byte{0})
	if len(fields) > 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	if len(fields)%5 != 0 {
		return nil, fmt.Errorf("parse stash reflog: got %d fields", len(fields))
	}
	entries := make([]StashEntry, 0, len(fields)/5)
	for index := 0; index < len(fields); index += 5 {
		oid := string(fields[index])
		selector := strings.TrimPrefix(string(fields[index+1]), "refs/")
		if !validObjectID(oid) || !strings.HasPrefix(selector, "stash@{") {
			return nil, fmt.Errorf("parse stash reflog: invalid identity")
		}
		timestamp, err := strconv.ParseInt(string(fields[index+3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse stash reflog timestamp: %w", err)
		}
		parents := strings.Fields(string(fields[index+4]))
		if len(parents) < 1 || !validObjectID(parents[0]) {
			return nil, fmt.Errorf("parse stash reflog: missing base object")
		}
		untracked := ""
		if len(parents) >= 3 {
			if !validObjectID(parents[2]) {
				return nil, fmt.Errorf("parse stash reflog: invalid untracked object")
			}
			untracked = parents[2]
		}
		branch, message := parseStashSubject(string(fields[index+2]))
		entries = append(entries, StashEntry{
			OID: oid, Selector: selector, Branch: branch, Message: message,
			Timestamp: timestamp, BaseOID: parents[0], UntrackedOID: untracked,
		})
	}
	return entries, nil
}

func parseStashSubject(subject string) (string, string) {
	rest := subject
	for _, prefix := range []string{"WIP on ", "On ", "index on "} {
		if strings.HasPrefix(subject, prefix) {
			rest = strings.TrimPrefix(subject, prefix)
			break
		}
	}
	if rest == subject {
		return "", subject
	}
	branch, message, ok := strings.Cut(rest, ": ")
	if !ok {
		return "", subject
	}
	return branch, message
}

type changeStat struct {
	additions uint64
	deletions uint64
	binary    bool
}

func parseNumstatDetails(data []byte) (map[string]changeStat, error) {
	fields := bytes.Split(data, []byte{0})
	stats := make(map[string]changeStat)
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if len(field) == 0 {
			continue
		}
		parts := bytes.SplitN(field, []byte{'\t'}, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse Git numstat record %q", field)
		}
		path := string(parts[2])
		if path == "" {
			if index+2 >= len(fields) {
				return nil, fmt.Errorf("parse truncated Git numstat rename")
			}
			index += 2
			path = string(fields[index])
		}
		if path == "" {
			continue
		}
		stats[path] = changeStat{
			additions: parseCount(parts[0]),
			deletions: parseCount(parts[1]),
			binary:    bytes.Equal(parts[0], []byte{'-'}) || bytes.Equal(parts[1], []byte{'-'}),
		}
	}
	return stats, nil
}

func parseNameStatus(data []byte, stats map[string]changeStat) ([]ChangedFile, error) {
	fields := bytes.Split(data, []byte{0})
	changes := make([]ChangedFile, 0, len(fields)/2)
	for index := 0; index < len(fields); {
		if len(fields[index]) == 0 {
			index++
			continue
		}
		status := string(fields[index])
		index++
		if index >= len(fields) || status == "" {
			return nil, fmt.Errorf("parse truncated Git name-status record")
		}
		kind := ChangeModified
		previous := ""
		var path string
		switch status[0] {
		case 'A':
			kind = ChangeAdded
		case 'D':
			kind = ChangeDeleted
		case 'R', 'C':
			if index+1 >= len(fields) {
				return nil, fmt.Errorf("parse truncated Git rename record")
			}
			previous = string(fields[index])
			index++
			if status[0] == 'R' {
				kind = ChangeRenamed
			} else {
				kind = ChangeCopied
			}
		}
		path = string(fields[index])
		index++
		if path == "" {
			return nil, fmt.Errorf("parse Git name-status: empty path")
		}
		stat := stats[path]
		changes = append(changes, ChangedFile{
			Path: path, PreviousPath: previous, Kind: kind,
			Additions: stat.additions, Deletions: stat.deletions, Binary: stat.binary,
		})
	}
	return changes, nil
}

func mergeChanges(tracked, extras []ChangedFile) []ChangedFile {
	combined := make(map[string]ChangedFile, len(tracked)+len(extras))
	for _, change := range tracked {
		combined[change.Path] = change
	}
	for _, change := range extras {
		if _, exists := combined[change.Path]; !exists {
			combined[change.Path] = change
		}
	}
	paths := make([]string, 0, len(combined))
	for path := range combined {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]ChangedFile, 0, len(paths))
	for _, path := range paths {
		result = append(result, combined[path])
	}
	return result
}

func validateStashSource(source StashSource) error {
	if !validObjectID(source.OID) || !validObjectID(source.BaseOID) ||
		(source.UntrackedOID != "" && !validObjectID(source.UntrackedOID)) {
		return fmt.Errorf("invalid stash object identity")
	}
	return nil
}

func validGitPath(path string) bool {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return false
	}
	return filepath.IsLocal(filepath.FromSlash(path))
}
