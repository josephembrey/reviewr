package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestBuildCommitRowsMapsRefKindsExplicitly(t *testing.T) {
	rows := buildCommitRows([]repository.Commit{{
		OID: "commit",
		Refs: []repository.CommitRef{
			{Kind: repository.CommitBranchRef, Name: "branch"},
			{Kind: repository.CommitRemoteRef, Name: "remote"},
			{Kind: repository.CommitTagRef, Name: "tag"},
		},
	}}, workspace.GitGraph)
	want := []commitrow.Ref{
		{Kind: commitrow.Branch, Name: "branch"},
		{Kind: commitrow.Remote, Name: "remote"},
		{Kind: commitrow.Tag, Name: "tag"},
	}
	if len(rows) != 1 || len(rows[0].Refs) != len(want) {
		t.Fatalf("commit rows = %#v", rows)
	}
	for index := range want {
		if rows[0].Refs[index] != want[index] {
			t.Fatalf("ref %d = %+v, want %+v", index, rows[0].Refs[index], want[index])
		}
	}
}
