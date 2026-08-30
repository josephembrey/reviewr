package git

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RefSourceKind classifies one selectable history source. The kind is part of
// identity: a branch, tag, and worktree at the same commit remain distinct.
type RefSourceKind uint8

const (
	RefSourceAll RefSourceKind = iota + 1
	RefSourceCurrentWorktree
	RefSourceLinkedWorktree
	RefSourceLocalBranch
	RefSourceRemoteBranch
	RefSourceTag
)

// RefSourceID is a stable, typed source identity. Name is a canonical refname
// for named refs, an absolute path for worktrees, and empty for All refs.
type RefSourceID struct {
	Kind RefSourceKind
	Name string
}

// Key adapts a typed source identity to the workspace-neutral navigation seam.
// It is never shown to users.
func (id RefSourceID) Key() string {
	return strconv.Itoa(int(id.Kind)) + ":" + id.Name
}

// RefSource is one read-only source in the refs navigator.
type RefSource struct {
	ID       RefSourceID
	Label    string
	Revision string
	OID      string
	Path     string
	Branch   string
	Upstream string
	Tracking string
	Remote   string
	UnixTime int64
}

// Kind returns the source's typed kind.
func (source RefSource) Kind() RefSourceKind {
	return source.ID.Kind
}

// AllRefsSource returns the synthetic complete public-ref source.
func AllRefsSource() RefSource {
	return RefSource{ID: RefSourceID{Kind: RefSourceAll}, Label: "All refs"}
}

// RefDecorationKind classifies a public ref decorating a commit row.
type RefDecorationKind uint8

const (
	RefDecorationBranch RefDecorationKind = iota + 1
	RefDecorationRemote
	RefDecorationTag
)

// RefDecoration is one public named ref pointing at a commit.
type RefDecoration struct {
	Kind  RefDecorationKind
	Label string
}

// RefCommit is one compact source-preview row. Graph topology is deliberately
// absent here; the UI seam accepts graph cells supplied by the Log component.
type RefCommit struct {
	OID          string
	ShortOID     string
	Subject      string
	Author       string
	AuthoredUnix int64
	Decorations  []RefDecoration
	Merge        bool
}

type worktreeRecord struct {
	path   string
	oid    string
	branch string
}

type refRecord struct {
	name     string
	oid      string
	peeled   string
	upstream string
	tracking string
	unixTime int64
}

// ListRefSources returns All refs, the current worktree, linked worktrees,
// unoccupied local branches, remote-tracking branches, and tags in that order.
// Every command is read-only and bounded.
func (Client) ListRefSources(root string) ([]RefSource, error) {
	worktreeOutput, err := runBounded(root, DefaultMaxHistoryBytes, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	worktrees, err := parseWorktreeList(worktreeOutput)
	if err != nil {
		return nil, err
	}

	refOutput, err := runBounded(
		root,
		DefaultMaxHistoryBytes,
		"for-each-ref",
		"--sort=refname",
		"--sort=-creatordate",
		"--format=%(refname)%00%(objectname)%00%(*objectname)%00%(upstream:short)%00%(upstream:trackshort)%00%(creatordate:unix)%00",
		"refs/heads",
		"refs/remotes",
		"refs/tags",
	)
	if err != nil {
		return nil, err
	}
	refs, err := parseRefList(refOutput)
	if err != nil {
		return nil, err
	}

	cleanRoot := filepath.Clean(root)
	current := make([]RefSource, 0, 1)
	linked := make([]RefSource, 0, len(worktrees))
	for _, record := range worktrees {
		kind := RefSourceLinkedWorktree
		bucket := &linked
		if samePath(record.path, cleanRoot) {
			kind = RefSourceCurrentWorktree
			bucket = &current
		}
		label := record.branch
		if label == "" {
			label = "detached"
			if record.oid != "" {
				label += " " + abbreviateObjectID(record.oid)
			}
		}
		*bucket = append(*bucket, RefSource{
			ID:       RefSourceID{Kind: kind, Name: record.path},
			Label:    label,
			Revision: record.oid,
			OID:      record.oid,
			Path:     record.path,
			Branch:   record.branch,
		})
	}

	rows := make([]RefSource, 0, 1+len(worktrees)+len(refs))
	rows = append(rows, AllRefsSource())
	rows = append(rows, current...)
	rows = append(rows, linked...)
	occupied := make(map[string]int, len(worktrees))
	for index := 1; index < len(rows); index++ {
		if rows[index].Branch != "" {
			occupied["refs/heads/"+rows[index].Branch] = index
		}
	}

	branches := make([]RefSource, 0, len(refs))
	remotes := make([]RefSource, 0, len(refs))
	tags := make([]RefSource, 0, len(refs))
	for _, record := range refs {
		oid := record.oid
		if record.peeled != "" {
			oid = record.peeled
		}
		switch {
		case strings.HasPrefix(record.name, "refs/heads/"):
			if index, ok := occupied[record.name]; ok {
				rows[index].OID = oid
				rows[index].Revision = oid
				rows[index].Upstream = record.upstream
				rows[index].Tracking = record.tracking
				rows[index].UnixTime = record.unixTime
				continue
			}
			branches = append(branches, RefSource{
				ID:       RefSourceID{Kind: RefSourceLocalBranch, Name: record.name},
				Label:    strings.TrimPrefix(record.name, "refs/heads/"),
				Revision: record.name,
				OID:      oid,
				Upstream: record.upstream,
				Tracking: record.tracking,
				UnixTime: record.unixTime,
			})
		case strings.HasPrefix(record.name, "refs/remotes/"):
			label := strings.TrimPrefix(record.name, "refs/remotes/")
			if strings.HasSuffix(label, "/HEAD") {
				continue
			}
			remote, _, _ := strings.Cut(label, "/")
			remotes = append(remotes, RefSource{
				ID:       RefSourceID{Kind: RefSourceRemoteBranch, Name: record.name},
				Label:    label,
				Revision: record.name,
				OID:      oid,
				Remote:   remote,
				UnixTime: record.unixTime,
			})
		case strings.HasPrefix(record.name, "refs/tags/"):
			tags = append(tags, RefSource{
				ID:       RefSourceID{Kind: RefSourceTag, Name: record.name},
				Label:    strings.TrimPrefix(record.name, "refs/tags/"),
				Revision: record.name,
				OID:      oid,
				UnixTime: record.unixTime,
			})
		}
	}
	rows = append(rows, branches...)
	rows = append(rows, remotes...)
	rows = append(rows, tags...)
	return rows, nil
}

// ListRefCommits returns a bounded compact history for one exact source.
func (Client) ListRefCommits(root string, source RefSource) ([]RefCommit, error) {
	args := []string{
		"log",
		"-z",
		"--topo-order",
		"--max-count=" + strconv.Itoa(CommitLimit),
		"--format=%H%x00%h%x00%s%x00%an%x00%at%x00",
	}
	switch source.Kind() {
	case RefSourceAll:
		args = append(args, "--branches", "--remotes", "--tags")
		_, hasCurrentHead, err := resolveHead(root)
		if err != nil {
			return nil, err
		}
		if hasCurrentHead {
			args = append(args, "HEAD")
		}
	case RefSourceCurrentWorktree, RefSourceLinkedWorktree:
		if source.Revision == "" {
			return nil, nil
		}
		if source.Revision != source.OID || !validObjectID(source.Revision) {
			return nil, fmt.Errorf("invalid worktree history source")
		}
		args = append(args, "--end-of-options", source.Revision)
	case RefSourceLocalBranch:
		if !validNamedSource(source, "refs/heads/") {
			return nil, fmt.Errorf("invalid local branch history source")
		}
		args = append(args, "--end-of-options", source.Revision)
	case RefSourceRemoteBranch:
		if !validNamedSource(source, "refs/remotes/") {
			return nil, fmt.Errorf("invalid remote branch history source")
		}
		args = append(args, "--end-of-options", source.Revision)
	case RefSourceTag:
		if !validNamedSource(source, "refs/tags/") {
			return nil, fmt.Errorf("invalid tag history source")
		}
		args = append(args, "--end-of-options", source.Revision)
	default:
		return nil, fmt.Errorf("invalid ref history source kind %d", source.Kind())
	}

	out, err := runBounded(root, DefaultMaxHistoryBytes, args...)
	if err != nil {
		return nil, err
	}
	commits, err := parseRefCommitLog(out)
	if err != nil {
		return nil, err
	}
	parentInput := make([]Commit, len(commits))
	for index, commit := range commits {
		parentInput[index].OID = commit.OID
	}
	parents, err := readCommitParents(root, parentInput)
	if err != nil {
		return nil, err
	}
	decorations, err := listCommitRefs(root)
	if err != nil {
		return nil, err
	}
	for index := range commits {
		commits[index].Merge = len(parents[index]) > 1
		for _, reference := range decorations[commits[index].OID] {
			kind := RefDecorationBranch
			switch reference.Kind {
			case RemoteRef:
				kind = RefDecorationRemote
			case TagRef:
				kind = RefDecorationTag
			}
			commits[index].Decorations = append(commits[index].Decorations, RefDecoration{
				Kind:  kind,
				Label: reference.Name,
			})
		}
	}
	return commits, nil
}

func parseWorktreeList(data []byte) ([]worktreeRecord, error) {
	data = bytes.TrimSuffix(data, []byte{0})
	if len(data) == 0 {
		return nil, nil
	}
	records := bytes.Split(data, []byte{0, 0})
	result := make([]worktreeRecord, 0, len(records))
	for _, raw := range records {
		var record worktreeRecord
		for _, field := range bytes.Split(raw, []byte{0}) {
			switch {
			case bytes.HasPrefix(field, []byte("worktree ")):
				record.path = string(bytes.TrimPrefix(field, []byte("worktree ")))
			case bytes.HasPrefix(field, []byte("HEAD ")):
				oid := string(bytes.TrimPrefix(field, []byte("HEAD ")))
				if validObjectID(oid) && strings.Trim(oid, "0") != "" {
					record.oid = oid
				}
			case bytes.HasPrefix(field, []byte("branch refs/heads/")):
				record.branch = string(bytes.TrimPrefix(field, []byte("branch refs/heads/")))
			}
		}
		if record.path == "" {
			return nil, fmt.Errorf("parse git worktree list: record has no path")
		}
		record.path = filepath.Clean(record.path)
		result = append(result, record)
	}
	return result, nil
}

func parseRefList(data []byte) ([]refRecord, error) {
	lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil, nil
	}
	result := make([]refRecord, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte{0})
		fields := bytes.Split(line, []byte{0})
		if len(fields) != 6 {
			return nil, fmt.Errorf("parse git for-each-ref: record has %d fields", len(fields))
		}
		unixTime, _ := strconv.ParseInt(string(fields[5]), 10, 64)
		result = append(result, refRecord{
			name:     string(fields[0]),
			oid:      string(fields[1]),
			peeled:   string(fields[2]),
			upstream: string(fields[3]),
			tracking: string(fields[4]),
			unixTime: unixTime,
		})
	}
	return result, nil
}

func parseRefCommitLog(data []byte) ([]RefCommit, error) {
	records := bytes.Split(data, []byte{0, 0})
	result := make([]RefCommit, 0, min(len(records), CommitLimit))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		if len(fields) != 5 || !validObjectID(string(fields[0])) || len(fields[1]) == 0 {
			return nil, fmt.Errorf("parse git ref log: invalid record")
		}
		authoredUnix, err := strconv.ParseInt(string(fields[4]), 10, 64)
		if err != nil || authoredUnix < 0 {
			return nil, fmt.Errorf("parse git ref log: invalid authored timestamp")
		}
		result = append(result, RefCommit{
			OID:          string(fields[0]),
			ShortOID:     string(fields[1]),
			Subject:      string(fields[2]),
			Author:       string(fields[3]),
			AuthoredUnix: authoredUnix,
		})
		if len(result) == CommitLimit {
			break
		}
	}
	return result, nil
}

func validNamedSource(source RefSource, prefix string) bool {
	return source.ID.Name == source.Revision && strings.HasPrefix(source.ID.Name, prefix)
}

func samePath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil && rightErr == nil {
		return filepath.Clean(leftResolved) == filepath.Clean(rightResolved)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func abbreviateObjectID(oid string) string {
	if len(oid) <= 7 {
		return oid
	}
	return oid[:7]
}
