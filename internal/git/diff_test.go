package git

import (
	"fmt"
	"strings"
	"testing"
)

func TestDiffsIncludeContextTheReaderCanFoldAndRestore(t *testing.T) {
	root := initGitTestRepository(t)
	lines := make([]string, 40)
	for index := range lines {
		lines[index] = fmt.Sprintf("line %02d", index+1)
	}
	writeGitFixture(t, root, "fixture.txt", strings.Join(lines, "\n")+"\n")
	runGitTest(t, root, "add", "fixture.txt")
	runGitTest(t, root, "commit", "-q", "-m", "base")
	baseOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))

	lines[19] = "line 20 changed"
	writeGitFixture(t, root, "fixture.txt", strings.Join(lines, "\n")+"\n")
	client := New()
	worktreePatch, err := client.ReadDiff(root, "fixture.txt", "", false, 16<<10)
	if err != nil {
		t.Fatal(err)
	}
	assertExpandableContext(t, worktreePatch)

	runGitTest(t, root, "add", "fixture.txt")
	runGitTest(t, root, "commit", "-q", "-m", "change")
	changedOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	objectPatch := client.DiffObjects(root, baseOID, changedOID, []string{"fixture.txt"}, 16<<10)
	if objectPatch.Kind != ObjectReady {
		t.Fatalf("DiffObjects() = %+v", objectPatch)
	}
	assertExpandableContext(t, objectPatch.Data)
}

func assertExpandableContext(t *testing.T, patch []byte) {
	t.Helper()
	text := string(patch)
	for _, line := range []string{" line 01", " line 40"} {
		if !strings.Contains(text, line) {
			t.Fatalf("patch omits expandable context %q:\n%s", line, text)
		}
	}
}
