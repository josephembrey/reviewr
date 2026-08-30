package git

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const (
	// CommitLimit bounds startup and refresh work for the Git workspace.
	CommitLimit = 200
	// DefaultMaxHistoryBytes bounds each history metadata or parent read in memory.
	DefaultMaxHistoryBytes int64 = 1 << 20
)

// Traversal selects the commit universe and projected graph edges.
type Traversal uint8

const (
	GraphTraversal Traversal = iota
	FirstParentTraversal
)

// HistoryQuery describes one bounded, read-only history request. StartOID is
// used only by first-parent traversal; an empty value means HEAD.
type HistoryQuery struct {
	Traversal Traversal
	StartOID  string
}

// RefKind gives a decoration semantic meaning independent of its spelling.
type RefKind uint8

const (
	BranchRef RefKind = iota
	RemoteRef
	TagRef
)

// CommitRef is one public ref pointing at a commit.
type CommitRef struct {
	Kind RefKind
	Name string
}

// Commit is one row in a structured Git history traversal.
type Commit struct {
	OID          string
	ShortOID     string
	Parents      []string
	Subject      string
	Author       string
	AuthoredUnix int64
	Refs         []CommitRef
	Merge        bool
	Head         bool
}

// CommitSummary is bounded metadata and stat for one exact commit identity.
type CommitSummary struct {
	OID         string
	AuthorName  string
	AuthorEmail string
	AuthoredAt  string
	Message     string
	Stat        string
}

// ListCommits returns a bounded structured traversal. Graph mode walks every
// public branch, remote-tracking branch, and tag plus a detached HEAD. First
// parent mode walks StartOID, or HEAD when no start was supplied.
func (Client) ListCommits(root string, query HistoryQuery) ([]Commit, error) {
	head, hasCurrentHead, err := resolveHead(root)
	if err != nil {
		return nil, err
	}

	args := []string{
		"log",
		"-z",
		"--no-color",
		"--no-show-signature",
		"--max-count=" + strconv.Itoa(CommitLimit),
		"--format=%H%x00%h%x00%an%x00%at%x00%s%x00",
	}
	switch query.Traversal {
	case GraphTraversal:
		hasRefs, refErr := hasPublicRefs(root)
		if refErr != nil {
			return nil, refErr
		}
		if !hasRefs && !hasCurrentHead {
			return nil, nil
		}
		args = append(args, "--topo-order", "--branches", "--remotes", "--tags")
		if hasCurrentHead {
			// Public patterns already contain an attached HEAD. Repeating it is
			// harmless and is what keeps a detached HEAD in the graph.
			args = append(args, "HEAD")
		}
	case FirstParentTraversal:
		start := query.StartOID
		if start == "" {
			if !hasCurrentHead {
				return nil, nil
			}
			start = "HEAD"
		} else if !validObjectID(start) {
			return nil, fmt.Errorf("invalid history start object ID %q", start)
		}
		args = append(args, "--first-parent", "--date-order", "--end-of-options", start)
	default:
		return nil, fmt.Errorf("unsupported history traversal %d", query.Traversal)
	}

	out, err := runBounded(root, DefaultMaxHistoryBytes, args...)
	if err != nil {
		return nil, err
	}
	commits, err := parseCommitLog(out)
	if err != nil {
		return nil, err
	}
	parents, err := readCommitParents(root, commits)
	if err != nil {
		return nil, err
	}
	refs, err := listCommitRefs(root)
	if err != nil {
		return nil, err
	}
	for index := range commits {
		commits[index].Parents = parents[index]
		commits[index].Merge = len(parents[index]) > 1
		commits[index].Refs = append([]CommitRef(nil), refs[commits[index].OID]...)
		commits[index].Head = commits[index].OID == head
	}
	return commits, nil
}

// ReadCommit returns bounded metadata and a first-parent changed-file stat.
func (Client) ReadCommit(root, oid string, maxBytes int64) (CommitSummary, error) {
	if !validObjectID(oid) {
		return CommitSummary{}, fmt.Errorf("invalid commit object ID %q", oid)
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	metadata, err := runBounded(
		root,
		maxBytes,
		"show",
		"-s",
		"--no-color",
		"--format=%H%x00%an%x00%ae%x00%aI%x00%B",
		"--end-of-options",
		oid,
	)
	if err != nil {
		return CommitSummary{}, err
	}
	remaining := maxBytes - int64(len(metadata))
	if remaining <= 0 {
		return CommitSummary{}, fmt.Errorf("git show: %w (%d bytes)", ErrOutputTooLarge, maxBytes)
	}
	stat, err := runBounded(
		root,
		remaining,
		"show",
		"--first-parent",
		"--format=",
		"--stat",
		"--no-renames",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		"--end-of-options",
		oid,
		"--",
	)
	if err != nil {
		return CommitSummary{}, err
	}
	return parseCommitSummary(metadata, stat)
}

func resolveHead(root string) (string, bool, error) {
	out, err := runBounded(root, 128, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err == nil {
		oid := string(bytes.TrimSpace(out))
		if !validObjectID(oid) {
			return "", false, fmt.Errorf("git rev-parse returned invalid HEAD")
		}
		return oid, true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return "", false, nil
	}
	return "", false, err
}

func hasPublicRefs(root string) (bool, error) {
	out, err := runBounded(
		root,
		256,
		"for-each-ref",
		"--count=1",
		"--format=%(objectname)",
		"refs/heads",
		"refs/remotes",
		"refs/tags",
	)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

func parseCommitLog(data []byte) ([]Commit, error) {
	records := bytes.Split(data, []byte{0, 0})
	commits := make([]Commit, 0, min(len(records), CommitLimit))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		if len(fields) != 5 {
			return nil, fmt.Errorf("parse git log: record has %d fields", len(fields))
		}
		if !validObjectID(string(fields[0])) || len(fields[1]) == 0 {
			return nil, fmt.Errorf("parse git log: invalid object identity")
		}
		timestamp, err := strconv.ParseInt(string(fields[3]), 10, 64)
		if err != nil || timestamp < 0 {
			return nil, fmt.Errorf("parse git log: invalid authored timestamp")
		}
		commits = append(commits, Commit{
			OID:          string(fields[0]),
			ShortOID:     string(fields[1]),
			Author:       string(fields[2]),
			AuthoredUnix: timestamp,
			Subject:      string(fields[4]),
		})
		if len(commits) == CommitLimit {
			break
		}
	}
	return commits, nil
}

// readCommitParents reads raw object headers so a shallow boundary retains the
// parent identity hidden by Git's revision walker.
func readCommitParents(root string, commits []Commit) ([][]string, error) {
	if len(commits) == 0 {
		return nil, nil
	}
	var input strings.Builder
	for _, commit := range commits {
		input.WriteString(commit.OID)
		input.WriteByte('\n')
	}
	out, err := runBoundedInput(
		root,
		DefaultMaxHistoryBytes,
		strings.NewReader(input.String()),
		"cat-file",
		"--batch",
	)
	if err != nil {
		return nil, err
	}
	return parseCommitParents(out, commits)
}

func parseCommitParents(data []byte, commits []Commit) ([][]string, error) {
	cursor := 0
	result := make([][]string, 0, len(commits))
	for _, commit := range commits {
		if cursor >= len(data) {
			return nil, fmt.Errorf("parse git cat-file: missing header for %s", commit.OID)
		}
		relativeHeaderEnd := bytes.IndexByte(data[cursor:], '\n')
		if relativeHeaderEnd < 0 {
			return nil, fmt.Errorf("parse git cat-file: truncated header for %s", commit.OID)
		}
		headerEnd := cursor + relativeHeaderEnd
		header := strings.Fields(string(data[cursor:headerEnd]))
		if len(header) != 3 || header[0] != commit.OID || header[1] != "commit" {
			return nil, fmt.Errorf("parse git cat-file: invalid header for %s", commit.OID)
		}
		size, sizeErr := strconv.Atoi(header[2])
		bodyStart := headerEnd + 1
		bodyEnd := bodyStart + size
		if sizeErr != nil || size < 0 || bodyEnd >= len(data) || data[bodyEnd] != '\n' {
			return nil, fmt.Errorf("parse git cat-file: invalid body for %s", commit.OID)
		}
		parents := make([]string, 0, 2)
		for _, line := range bytes.Split(data[bodyStart:bodyEnd], []byte{'\n'}) {
			if len(line) == 0 {
				break
			}
			if parent, ok := bytes.CutPrefix(line, []byte("parent ")); ok {
				if !validObjectID(string(parent)) {
					return nil, fmt.Errorf("parse git cat-file: invalid parent for %s", commit.OID)
				}
				parents = append(parents, string(parent))
			}
		}
		result = append(result, parents)
		cursor = bodyEnd + 1
	}
	return result, nil
}

func listCommitRefs(root string) (map[string][]CommitRef, error) {
	out, err := runBounded(
		root,
		DefaultMaxHistoryBytes,
		"for-each-ref",
		"--format=%(objectname)%00%(*objectname)%00%(refname)",
		"refs/heads",
		"refs/remotes",
		"refs/tags",
	)
	if err != nil {
		return nil, err
	}
	refs, err := parseCommitRefs(out)
	if err != nil {
		return nil, err
	}
	for oid := range refs {
		sort.Slice(refs[oid], func(left, right int) bool {
			if refs[oid][left].Kind != refs[oid][right].Kind {
				return refs[oid][left].Kind < refs[oid][right].Kind
			}
			return refs[oid][left].Name < refs[oid][right].Name
		})
	}
	return refs, nil
}

func parseCommitRefs(data []byte) (map[string][]CommitRef, error) {
	result := make(map[string][]CommitRef)
	for _, record := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse git refs: record has %d fields", len(fields))
		}
		oid := string(fields[0])
		if len(fields[1]) != 0 {
			oid = string(fields[1])
		}
		if !validObjectID(oid) {
			// A public ref can legally point at a non-commit object. It cannot
			// decorate a commit row, so leave it out without failing history.
			continue
		}
		name := string(fields[2])
		var reference CommitRef
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			reference = CommitRef{Kind: BranchRef, Name: strings.TrimPrefix(name, "refs/heads/")}
		case strings.HasPrefix(name, "refs/remotes/"):
			reference = CommitRef{Kind: RemoteRef, Name: strings.TrimPrefix(name, "refs/remotes/")}
		case strings.HasPrefix(name, "refs/tags/"):
			reference = CommitRef{Kind: TagRef, Name: strings.TrimPrefix(name, "refs/tags/")}
		default:
			continue
		}
		if reference.Name != "" {
			result[oid] = append(result[oid], reference)
		}
	}
	return result, nil
}

func parseCommitSummary(metadata, stat []byte) (CommitSummary, error) {
	fields := bytes.SplitN(metadata, []byte{0}, 5)
	if len(fields) != 5 {
		return CommitSummary{}, fmt.Errorf("parse git show: metadata has %d fields", len(fields))
	}
	return CommitSummary{
		OID:         string(fields[0]),
		AuthorName:  string(fields[1]),
		AuthorEmail: string(fields[2]),
		AuthoredAt:  string(fields[3]),
		Message:     string(bytes.TrimSuffix(fields[4], []byte{'\n'})),
		Stat:        string(bytes.TrimSuffix(stat, []byte{'\n'})),
	}, nil
}

func validObjectID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}
