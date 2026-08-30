package git

import (
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

	rows, occupied := worktreeSources(root, worktrees, len(refs))
	buckets := namedRefSources(rows, occupied, refs)
	rows = append(rows, buckets.branches...)
	rows = append(rows, buckets.remotes...)
	rows = append(rows, buckets.tags...)
	return rows, nil
}

func worktreeSources(root string, records []worktreeRecord, refCount int) ([]RefSource, map[string]int) {
	current := make([]RefSource, 0, 1)
	linked := make([]RefSource, 0, len(records))
	for _, record := range records {
		source := refSourceFromWorktree(record, RefSourceLinkedWorktree)
		if samePath(record.path, root) {
			source.ID.Kind = RefSourceCurrentWorktree
			current = append(current, source)
		} else {
			linked = append(linked, source)
		}
	}
	rows := make([]RefSource, 0, 1+len(records)+refCount)
	rows = append(rows, AllRefsSource())
	rows = append(rows, current...)
	rows = append(rows, linked...)
	occupied := make(map[string]int, len(records))
	for index := 1; index < len(rows); index++ {
		if rows[index].Branch != "" {
			occupied["refs/heads/"+rows[index].Branch] = index
		}
	}
	return rows, occupied
}

func refSourceFromWorktree(record worktreeRecord, kind RefSourceKind) RefSource {
	label := record.branch
	if label == "" {
		label = "detached"
		if record.oid != "" {
			label += " " + abbreviateObjectID(record.oid)
		}
	}
	return RefSource{
		ID:       RefSourceID{Kind: kind, Name: record.path},
		Label:    label,
		Revision: record.oid,
		OID:      record.oid,
		Path:     record.path,
		Branch:   record.branch,
	}
}

type refSourceBuckets struct {
	branches []RefSource
	remotes  []RefSource
	tags     []RefSource
}

func namedRefSources(rows []RefSource, occupied map[string]int, records []refRecord) refSourceBuckets {
	buckets := refSourceBuckets{
		branches: make([]RefSource, 0, len(records)),
		remotes:  make([]RefSource, 0, len(records)),
		tags:     make([]RefSource, 0, len(records)),
	}
	for _, record := range records {
		addNamedRefSource(rows, occupied, &buckets, record)
	}
	return buckets
}

func addNamedRefSource(rows []RefSource, occupied map[string]int, buckets *refSourceBuckets, record refRecord) {
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
			return
		}
		buckets.branches = append(buckets.branches, RefSource{
			ID: RefSourceID{Kind: RefSourceLocalBranch, Name: record.name}, Label: strings.TrimPrefix(record.name, "refs/heads/"),
			Revision: record.name, OID: oid, Upstream: record.upstream, Tracking: record.tracking, UnixTime: record.unixTime,
		})
	case strings.HasPrefix(record.name, "refs/remotes/"):
		label := strings.TrimPrefix(record.name, "refs/remotes/")
		if strings.HasSuffix(label, "/HEAD") {
			return
		}
		remote, _, _ := strings.Cut(label, "/")
		buckets.remotes = append(buckets.remotes, RefSource{
			ID: RefSourceID{Kind: RefSourceRemoteBranch, Name: record.name}, Label: label,
			Revision: record.name, OID: oid, Remote: remote, UnixTime: record.unixTime,
		})
	case strings.HasPrefix(record.name, "refs/tags/"):
		buckets.tags = append(buckets.tags, RefSource{
			ID: RefSourceID{Kind: RefSourceTag, Name: record.name}, Label: strings.TrimPrefix(record.name, "refs/tags/"),
			Revision: record.name, OID: oid, UnixTime: record.unixTime,
		})
	}
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
