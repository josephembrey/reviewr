package repository

import (
	"errors"
	"fmt"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
	"github.com/josephembrey/reviewr/internal/review"
)

// ReviewRepositoryID returns the canonical private-state namespace without
// enumerating or mutating repository status.
func (r *Repository) ReviewRepositoryID() (review.RepositoryID, error) {
	common, err := r.git.ResolveCommonDir(r.root)
	if err != nil {
		return review.RepositoryID{}, err
	}
	return review.ResolveRepositoryID(r.root, common)
}

// ReviewComparisons enriches already-enumerated typed candidates with exact
// uncommitted endpoint identities. Unsupported comparison bases remain inert
// rather than borrowing unsafe bounds.
func (r *Repository) ReviewComparisons(scope string, candidates []review.Candidate) (review.Snapshot, error) {
	snapshot := review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	if scope != "uncommitted" {
		return snapshot, fmt.Errorf("exact %s comparison is not available", scope)
	}
	head, unborn, err := r.reviewHead()
	if err != nil {
		return snapshot, err
	}
	treeEntries, treeErr := r.reviewTreeEntries(head, unborn, candidates)
	for _, candidate := range candidates {
		snapshot.Comparisons[candidate.Path] = r.reviewComparison(
			scope,
			head,
			unborn,
			treeEntries,
			treeErr,
			candidate,
		)
	}
	return snapshot, nil
}

func (r *Repository) reviewHead() (string, bool, error) {
	head, err := r.git.HeadOID(r.root)
	if errors.Is(err, gitadapter.ErrUnbornHead) {
		head, err = r.git.EmptyTreeOID(r.root)
		if err != nil {
			return "", true, fmt.Errorf("resolve review comparison HEAD: %w", err)
		}
		return head, true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve review comparison HEAD: %w", err)
	}
	return head, false, nil
}

func (r *Repository) reviewTreeEntries(head string, unborn bool, candidates []review.Candidate) (map[string]gitadapter.TreeEntry, error) {
	if unborn {
		return nil, nil
	}
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		path := candidateOldPath(candidate)
		if _, exists := seen[path]; !exists {
			paths = append(paths, path)
			seen[path] = struct{}{}
		}
	}
	return r.git.ReadTreeEntries(r.root, head, paths)
}

func (r *Repository) reviewComparison(
	scope, head string,
	unborn bool,
	treeEntries map[string]gitadapter.TreeEntry,
	treeErr error,
	candidate review.Candidate,
) review.FileComparison {
	oldPath := candidateOldPath(candidate)
	old, basisReason := reviewOldEndpoint(oldPath, unborn, treeEntries, treeErr)
	new := r.worktreeReviewContent(candidate.Path).Endpoint
	basisReason = validateReviewEndpoints(candidate.Action, &old, &new, basisReason)
	comparison := review.FileComparison{
		Identity:    review.ComparisonIdentity{Scope: scope, Basis: head},
		OldSource:   review.EndpointSource{Kind: review.GitTreeSource, Value: head},
		NewSource:   review.EndpointSource{Kind: review.WorktreeSource},
		Action:      candidate.Action,
		Old:         old,
		New:         new,
		BasisReason: basisReason,
	}
	if comparison.BasisReason == "" && (candidate.Action == review.Renamed || candidate.Action == review.Copied) {
		comparison.BasisReason = "rename or copy lineage requires a full review"
	}
	return comparison
}

func candidateOldPath(candidate review.Candidate) string {
	if candidate.PreviousPath != "" {
		return candidate.PreviousPath
	}
	return candidate.Path
}

func reviewOldEndpoint(
	path string,
	unborn bool,
	entries map[string]gitadapter.TreeEntry,
	treeErr error,
) (review.Endpoint, string) {
	if unborn {
		return review.AbsentEndpoint(path), ""
	}
	if treeErr != nil {
		return review.Endpoint{Path: path, Kind: review.Regular}, "comparison basis content is unavailable"
	}
	entry, exists := entries[path]
	if !exists {
		return review.AbsentEndpoint(path), ""
	}
	kind := review.Regular
	switch entry.Mode {
	case 0o120000:
		kind = review.Symlink
	case 0o160000:
		kind = review.Submodule
	}
	return review.Endpoint{Path: path, Kind: kind, Mode: entry.Mode, ContentID: "git:" + entry.OID}, ""
}

func validateReviewEndpoints(action review.FileAction, old, new *review.Endpoint, reason string) string {
	expectsAbsent := action == review.Deleted
	if (expectsAbsent && new.Kind != review.Absent) || (!expectsAbsent && new.Kind == review.Absent) {
		new.ContentID = ""
		return "file changed during comparison snapshot"
	}
	if action == review.Added && old.Kind != review.Absent {
		old.ContentID = ""
		return "comparison action and basis do not agree"
	}
	if action != review.Added && old.Kind == review.Absent {
		old.ContentID = ""
		return "comparison action and basis do not agree"
	}
	if reason == "" && (!old.Exact() || !new.Exact()) {
		return "exact comparison content is unavailable"
	}
	return reason
}
