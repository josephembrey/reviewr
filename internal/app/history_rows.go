package app

import (
	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func buildCommitRows(commits []repository.Commit, traversal workspace.GitTraversal) []commitrow.Row {
	graphCommits := make([]commitgraph.Commit, len(commits))
	for index, commit := range commits {
		parents := commit.Parents
		if traversal == workspace.GitFirstParent && len(parents) > 1 {
			parents = parents[:1]
		}
		graphCommits[index] = commitgraph.Commit{
			OID:     commit.OID,
			Parents: parents,
			Merge:   commit.Merge,
		}
	}
	graphs := commitgraph.Layout(graphCommits)
	rows := make([]commitrow.Row, len(commits))
	for index, commit := range commits {
		refs := make([]commitrow.Ref, len(commit.Refs))
		for refIndex, reference := range commit.Refs {
			kind := commitrow.Branch
			switch reference.Kind {
			case repository.CommitRemoteRef:
				kind = commitrow.Remote
			case repository.CommitTagRef:
				kind = commitrow.Tag
			}
			refs[refIndex] = commitrow.Ref{Kind: kind, Name: reference.Name}
		}
		rows[index] = commitrow.Row{
			Graph:        graphs[index],
			OID:          commit.OID,
			ShortOID:     commit.ShortOID,
			Subject:      commit.Subject,
			Author:       commit.Author,
			AuthoredUnix: commit.AuthoredUnix,
			Refs:         refs,
			Merge:        commit.Merge,
		}
	}
	return rows
}

func commitQuery(traversal workspace.GitTraversal, startOID string) repository.CommitQuery {
	query := repository.CommitQuery{}
	if traversal == workspace.GitFirstParent {
		query.Traversal = repository.CommitFirstParent
		query.StartOID = startOID
	}
	return query
}

func traversalForQuery(query repository.CommitQuery) workspace.GitTraversal {
	if query.Traversal == repository.CommitFirstParent {
		return workspace.GitFirstParent
	}
	return workspace.GitGraph
}
