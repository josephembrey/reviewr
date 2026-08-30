package repository

import (
	"fmt"

	gitadapter "github.com/josephembrey/reviewr/internal/git"
	"github.com/josephembrey/reviewr/internal/review"
)

// ReviewRepositoryID returns the canonical private-state namespace without
// enumerating or mutating repository status.
func (r *Repository) ReviewRepositoryID() (review.RepositoryID, error) {
	return review.RepositoryID{CommonGitDir: r.commonDir, Worktree: r.root}, nil
}

// ReviewComparisons enriches already-enumerated typed candidates with exact
// endpoint identities from the snapshot's pinned basis. Unsupported comparison
// bases remain inert rather than borrowing unsafe bounds.
func (r *Repository) ReviewComparisons(scope, basis string, candidates []review.Candidate) (review.Snapshot, error) {
	snapshot := review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
	if scope != ComparisonUncommitted && scope != ComparisonBranch && scope != ComparisonLastTurn {
		return snapshot, fmt.Errorf("exact %s comparison is not available", scope)
	}
	if basis == "" {
		return snapshot, fmt.Errorf("exact %s comparison has no basis", scope)
	}
	treeEntries, treeErr := r.reviewTreeEntries(basis, candidates)
	for _, candidate := range candidates {
		snapshot.Comparisons[candidate.Path] = r.reviewComparison(
			scope,
			basis,
			treeEntries,
			treeErr,
			candidate,
		)
	}
	return snapshot, nil
}

func (r *Repository) reviewTreeEntries(basis string, candidates []review.Candidate) (map[string]gitadapter.TreeEntry, error) {
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		path := candidateOldPath(candidate)
		if _, exists := seen[path]; !exists {
			paths = append(paths, path)
			seen[path] = struct{}{}
		}
	}
	return r.git.ReadTreeEntries(r.root, basis, paths)
}

func (r *Repository) reviewComparison(
	scope, basis string,
	treeEntries map[string]gitadapter.TreeEntry,
	treeErr error,
	candidate review.Candidate,
) review.FileComparison {
	oldPath := candidateOldPath(candidate)
	old, basisReason := reviewOldEndpoint(oldPath, treeEntries, treeErr)
	new := r.worktreeReviewContent(candidate.Path).Endpoint
	basisReason = validateReviewEndpoints(candidate.Action, &old, &new, basisReason)
	comparison := review.FileComparison{
		Identity:    review.ComparisonIdentity{Scope: scope, Basis: basis},
		OldSource:   review.EndpointSource{Kind: review.GitTreeSource, Value: basis},
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
	entries map[string]gitadapter.TreeEntry,
	treeErr error,
) (review.Endpoint, string) {
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
