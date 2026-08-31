package repository

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCommitInspectionReadsRootAndFirstParentChangesWithoutMutation(t *testing.T) {
	root := initRepository(t)
	original := strings.Repeat("stable line\n", 12)
	writeFile(t, root, "old.go", original)
	runGit(t, root, "add", "old.go")
	runGit(t, root, "commit", "-q", "-m", "root")
	rootOID := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))

	runGit(t, root, "mv", "old.go", "new.go")
	writeFile(t, root, "new.go", original+"new line\n")
	writeFile(t, root, "added.go", "package added\n")
	runGit(t, root, "add", "new.go", "added.go")
	runGit(t, root, "commit", "-q", "-m", "rename and add")
	tipOID := strings.TrimSpace(string(runGitBytes(t, root, "rev-parse", "HEAD")))

	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before := captureGitState(t, root)
	rootFiles, err := repo.ListCommitFiles(rootOID)
	if err != nil {
		t.Fatal(err)
	}
	rootIndex := slices.IndexFunc(rootFiles, func(file ChangedFile) bool { return file.Path == "old.go" })
	if rootIndex < 0 || rootFiles[rootIndex].Kind != ChangeAdded {
		t.Fatalf("root commit files = %#v", rootFiles)
	}
	rootDocument := repo.ReadCommitFile(rootOID, rootFiles[rootIndex])
	if rootDocument.Old.Kind != FileMissing || rootDocument.New.Kind != FileReady ||
		rootDocument.New.Content != original || rootDocument.Patch.Kind != FileReady || !strings.Contains(rootDocument.Patch.Content, "+stable line") {
		t.Fatalf("root commit document = %+v", rootDocument)
	}

	tipFiles, err := repo.ListCommitFiles(tipOID)
	if err != nil {
		t.Fatal(err)
	}
	renameIndex := slices.IndexFunc(tipFiles, func(file ChangedFile) bool {
		return file.Path == "new.go" && file.PreviousPath == "old.go"
	})
	if renameIndex < 0 || tipFiles[renameIndex].Kind != ChangeRenamed {
		t.Fatalf("tip commit files = %#v", tipFiles)
	}
	renameDocument := repo.ReadCommitFile(tipOID, tipFiles[renameIndex])
	if renameDocument.Old.Kind != FileReady || renameDocument.Old.Content != original ||
		renameDocument.New.Kind != FileReady || !strings.HasSuffix(renameDocument.New.Content, "new line\n") ||
		renameDocument.Patch.Kind != FileReady || !strings.Contains(renameDocument.Patch.Content, "old.go") ||
		!strings.Contains(renameDocument.Patch.Content, "new.go") {
		t.Fatalf("renamed commit document = %+v", renameDocument)
	}
	after := captureGitState(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("commit inspection changed Git state\nbefore: %+v\nafter:  %+v", before, after)
	}
}
