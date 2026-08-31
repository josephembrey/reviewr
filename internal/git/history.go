package git

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// CommitLimit bounds startup and refresh work for the Git workspace.
	CommitLimit = 200
	// DefaultMaxHistoryBytes bounds each history metadata or parent read in memory.
	DefaultMaxHistoryBytes int64 = 1 << 20
	commitLogFormat              = "--format=%H%x00%h%x00%an%x00%at%x00%s%x00"
)

// Traversal selects the commit universe and projected graph edges.
type Traversal uint8

const (
	GraphTraversal Traversal = iota
	FirstParentTraversal
)

// HistoryQuery describes one bounded, read-only history request. SourceOID
// limits either traversal to one immutable source tip. StartOID is retained as
// the user-selected first-parent starting point when no source is selected.
type HistoryQuery struct {
	Traversal Traversal
	StartOID  string
	SourceOID string
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

// ListCommits returns a bounded structured traversal. Graph mode walks every
// public branch, remote-tracking branch, and tag plus a detached HEAD. First
// parent mode walks StartOID, or HEAD when no start was supplied.
func (Client) ListCommits(root string, query HistoryQuery) ([]Commit, error) {
	head, hasCurrentHead, err := resolveHead(root)
	if err != nil {
		return nil, err
	}
	args, refs, err := historyArgs(root, query, hasCurrentHead)
	if err != nil || len(args) == 0 {
		return nil, err
	}
	commandArgs := []string{
		"log",
		"-z",
		"--no-color",
		"--no-show-signature",
		"--max-count=" + strconv.Itoa(CommitLimit),
		commitLogFormat,
	}
	commandArgs = append(commandArgs, args...)
	out, err := runBounded(root, DefaultMaxHistoryBytes, commandArgs...)
	if err != nil {
		return nil, err
	}
	commits, err := parseCommitLog(out)
	if err != nil {
		return nil, err
	}
	parents, err := readCommitParents(root, commitOIDs(commits))
	if err != nil {
		return nil, err
	}
	if refs == nil {
		refs, _, err = listCommitRefs(root)
		if err != nil {
			return nil, err
		}
	}
	for index := range commits {
		commits[index].Parents = parents[index]
		commits[index].Merge = len(parents[index]) > 1
		commits[index].Refs = append([]CommitRef(nil), refs[commits[index].OID]...)
		commits[index].Head = commits[index].OID == head
	}
	return commits, nil
}

func historyArgs(root string, query HistoryQuery, hasCurrentHead bool) ([]string, map[string][]CommitRef, error) {
	switch query.Traversal {
	case GraphTraversal:
		refs, hasRefs, err := listCommitRefs(root)
		if err != nil {
			return nil, nil, err
		}
		if query.SourceOID != "" {
			if !validObjectID(query.SourceOID) {
				return nil, nil, fmt.Errorf("invalid history source object ID %q", query.SourceOID)
			}
			return []string{"--topo-order", "--end-of-options", query.SourceOID}, refs, nil
		}
		if !hasRefs && !hasCurrentHead {
			return nil, refs, nil
		}
		args := []string{"--topo-order", "--branches", "--remotes", "--tags"}
		if hasCurrentHead {
			// Public patterns already contain an attached HEAD. Repeating it is
			// harmless and is what keeps a detached HEAD in the graph.
			args = append(args, "HEAD")
		}
		return args, refs, nil
	case FirstParentTraversal:
		start := query.SourceOID
		if start == "" {
			start = query.StartOID
		}
		if start == "" {
			if !hasCurrentHead {
				return nil, nil, nil
			}
			start = "HEAD"
		} else if !validObjectID(start) {
			return nil, nil, fmt.Errorf("invalid history start object ID %q", start)
		}
		return []string{"--first-parent", "--date-order", "--end-of-options", start}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported history traversal %d", query.Traversal)
	}
}

func resolveHead(root string) (string, bool, error) {
	return resolveCommitOID(root, "HEAD")
}

// readCommitParents reads raw object headers so a shallow boundary retains the
// parent identity hidden by Git's revision walker.
func readCommitParents(root string, oids []string) ([][]string, error) {
	if len(oids) == 0 {
		return nil, nil
	}
	var input strings.Builder
	for _, oid := range oids {
		input.WriteString(oid)
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
	return parseCommitParents(out, oids)
}

func commitOIDs(commits []Commit) []string {
	oids := make([]string, len(commits))
	for index, commit := range commits {
		oids[index] = commit.OID
	}
	return oids
}

func listCommitRefs(root string) (map[string][]CommitRef, bool, error) {
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
		return nil, false, err
	}
	refs, err := parseCommitRefs(out)
	if err != nil {
		return nil, false, err
	}
	for oid := range refs {
		sort.Slice(refs[oid], func(left, right int) bool {
			if refs[oid][left].Kind != refs[oid][right].Kind {
				return refs[oid][left].Kind < refs[oid][right].Kind
			}
			return refs[oid][left].Name < refs[oid][right].Name
		})
	}
	return refs, len(out) > 0, nil
}
