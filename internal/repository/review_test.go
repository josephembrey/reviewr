package repository

import (
	"reflect"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/review"
)

func TestReviewComparisonsPreserveGitStateIncludingUnreachableObjects(t *testing.T) {
	root := initRepository(t)
	writeFile(t, root, "tracked.txt", "base\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	writeFile(t, root, "tracked.txt", "changed\n")

	before := captureGitState(t, root)
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.ReviewComparisons(ComparisonUncommitted, comparisonBasis(t, repo, ComparisonUncommitted), []review.Candidate{{Path: "tracked.txt", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := snapshot.Comparisons["tracked.txt"]
	if !comparison.Exact() {
		t.Fatalf("comparison = %+v", comparison)
	}
	if content := repo.ReadReviewContent(comparison.OldSource, comparison.Old); content.State != review.ContentText || content.Text != "base\n" {
		t.Fatalf("old content = %+v", content)
	}
	if content := repo.ReadReviewContent(comparison.NewSource, comparison.New); content.State != review.ContentText || content.Text != "changed\n" {
		t.Fatalf("new content = %+v", content)
	}
	if after := captureGitState(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("review reads changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestSHA256ReviewComparisonsUseRepositoryNativeObjectIDs(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q", "--object-format=sha256")
	runGit(t, root, "config", "user.name", "Reviewr Tests")
	runGit(t, root, "config", "user.email", "reviewr@example.invalid")
	writeFile(t, root, "tracked.txt", "base\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-qm", "base")
	writeFile(t, root, "tracked.txt", "changed\n")

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.ReviewComparisons(ComparisonUncommitted, comparisonBasis(t, repo, ComparisonUncommitted), []review.Candidate{{Path: "tracked.txt", Action: review.Modified}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := snapshot.Comparisons["tracked.txt"]
	if !comparison.Exact() || len(strings.TrimPrefix(comparison.Identity.Basis, "git:")) != 64 ||
		len(strings.TrimPrefix(comparison.Old.ContentID, "git:")) != 64 ||
		len(strings.TrimPrefix(comparison.New.ContentID, "git:")) != 64 {
		t.Fatalf("SHA-256 comparison = %+v", comparison)
	}
}
