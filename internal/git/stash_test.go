package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseStashLogUsesOIDIdentityAndMachineFields(t *testing.T) {
	t.Parallel()
	oidA := strings.Repeat("a", 40)
	oidB := strings.Repeat("b", 40)
	baseA := strings.Repeat("c", 40)
	baseB := strings.Repeat("d", 40)
	untracked := strings.Repeat("e", 40)
	data := []byte(strings.Join([]string{
		oidA, "refs/stash@{0}", "On feature/reader: hostile\nmessage \x1b[31m", "1728000000", baseA + " " + oidB + " " + untracked,
		oidB, "refs/stash@{1}", "subject without a prefix", "1727990000", baseB + " " + oidA,
		"",
	}, "\x00"))
	got, err := parseStashLog(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []StashEntry{
		{OID: oidA, Selector: "stash@{0}", Branch: "feature/reader", Message: "hostile\nmessage \x1b[31m", Timestamp: 1728000000, BaseOID: baseA, UntrackedOID: untracked},
		{OID: oidB, Selector: "stash@{1}", Message: "subject without a prefix", Timestamp: 1727990000, BaseOID: baseB},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseStashLog() = %#v, want %#v", got, want)
	}
	if _, err := parseStashLog([]byte("truncated\x00record")); err == nil {
		t.Fatal("parseStashLog() accepted a truncated machine record")
	}
}

func TestListStashesCombinesTrackedUntrackedAndRenumberedSelectors(t *testing.T) {
	root := initGitTestRepository(t)
	writeGitFixture(t, root, "modified.txt", "old\n")
	writeGitFixture(t, root, "deleted.txt", "delete me\n")
	writeGitFixture(t, root, "rename-old.txt", "rename me\n")
	writeGitFixtureBytes(t, root, "binary.dat", []byte{0, 1, 2})
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "commit", "-q", "-m", "base")
	branch := strings.TrimSpace(runGitTest(t, root, "branch", "--show-current"))

	writeGitFixture(t, root, "modified.txt", "new\nwith tail\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, root, "mv", "rename-old.txt", "rename-new.txt")
	writeGitFixtureBytes(t, root, "binary.dat", []byte{0, 9, 8})
	writeGitFixture(t, root, "untracked path.txt", "saved untracked\n")
	runGitTest(t, root, "stash", "push", "-u", "-m", "complete snapshot")
	completeOID := strings.TrimSpace(runGitTest(t, root, "rev-parse", "stash@{0}"))

	writeGitFixture(t, root, "modified.txt", "newer stash\n")
	runGitTest(t, root, "stash", "push", "-m", "newer snapshot")
	client := New()
	rows, err := client.ListStashes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Selector != "stash@{0}" || rows[1].Selector != "stash@{1}" || rows[1].OID != completeOID {
		t.Fatalf("ListStashes() = %#v", rows)
	}
	if rows[1].Branch != branch || rows[1].Message != "complete snapshot" || rows[1].FileCount < 5 || rows[1].Additions == 0 || rows[1].Deletions == 0 {
		t.Fatalf("complete stash metadata = %+v", rows[1])
	}
	source := StashSource{OID: rows[1].OID, BaseOID: rows[1].BaseOID, UntrackedOID: rows[1].UntrackedOID}
	files, err := client.ListStashChanges(root, source)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]ChangedFile, len(files))
	for _, file := range files {
		byPath[file.Path] = file
	}
	for _, path := range []string{"modified.txt", "deleted.txt", "rename-new.txt", "binary.dat", "untracked path.txt"} {
		if _, ok := byPath[path]; !ok {
			t.Fatalf("stash files omit %q: %#v", path, files)
		}
	}
	if byPath["deleted.txt"].Kind != ChangeDeleted || byPath["rename-new.txt"].Kind != ChangeRenamed ||
		byPath["rename-new.txt"].PreviousPath != "rename-old.txt" || byPath["untracked path.txt"].Kind != ChangeUntracked || !byPath["binary.dat"].Binary {
		t.Fatalf("stash change kinds = %#v", byPath)
	}
	old := client.ReadObjectFile(root, source.BaseOID, "modified.txt", 1024)
	new := client.ReadObjectFile(root, source.OID, "modified.txt", 1024)
	if string(old.Data) != "old\n" || string(new.Data) != "new\nwith tail\n" {
		t.Fatalf("immutable sides = old %q new %q", old.Data, new.Data)
	}
	patch := client.DiffObjects(root, source.BaseOID, source.OID, []string{"modified.txt"}, 4096)
	if patch.Kind != ObjectReady || !strings.Contains(string(patch.Data), "+with tail") {
		t.Fatalf("immutable patch = %+v %q", patch, patch.Data)
	}

	runGitTest(t, root, "stash", "drop", "-q", "stash@{0}")
	renumbered, err := client.ListStashes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(renumbered) != 1 || renumbered[0].OID != completeOID || renumbered[0].Selector != "stash@{0}" {
		t.Fatalf("renumbered stashes = %#v", renumbered)
	}
	runGitTest(t, root, "stash", "drop", "-q", "stash@{0}")
	empty, err := client.ListStashes(root)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty ListStashes() = (%#v, %v)", empty, err)
	}
}

func TestListStashesIsCalmForUnbornAndNoStashRepositories(t *testing.T) {
	root := initGitTestRepository(t)
	client := New()
	if rows, err := client.ListStashes(root); err != nil || len(rows) != 0 {
		t.Fatalf("unborn ListStashes() = (%#v, %v)", rows, err)
	}
	runGitTest(t, root, "commit", "--allow-empty", "-q", "-m", "base")
	if rows, err := client.ListStashes(root); err != nil || len(rows) != 0 {
		t.Fatalf("no-stash ListStashes() = (%#v, %v)", rows, err)
	}
}

func TestSHA256StashUsesRepositoryNativeEmptyTreeForUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q", "--object-format=sha256", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("installed Git lacks SHA-256 repository support: %v: %s", err, out)
	}
	runGitTest(t, root, "config", "user.name", "Reviewr Tests")
	runGitTest(t, root, "config", "user.email", "reviewr@example.invalid")
	writeGitFixture(t, root, "tracked.txt", "base\n")
	runGitTest(t, root, "add", "tracked.txt")
	runGitTest(t, root, "commit", "-q", "-m", "base")
	writeGitFixture(t, root, "untracked.txt", "stored in sha256 stash\n")
	runGitTest(t, root, "stash", "push", "-u", "-m", "native empty tree")

	client := New()
	empty, err := client.EmptyTree(root)
	if err != nil || len(empty) != 64 {
		t.Fatalf("SHA-256 EmptyTree() = (%q, %v)", empty, err)
	}
	rows, err := client.ListStashes(root)
	if err != nil || len(rows) != 1 || len(rows[0].OID) != 64 {
		t.Fatalf("SHA-256 ListStashes() = (%#v, %v)", rows, err)
	}
	files, err := client.ListStashChanges(root, StashSource{
		OID: rows[0].OID, BaseOID: rows[0].BaseOID, UntrackedOID: rows[0].UntrackedOID,
	})
	if err != nil || len(files) != 1 || files[0].Path != "untracked.txt" || files[0].Kind != ChangeUntracked {
		t.Fatalf("SHA-256 stash files = (%#v, %v)", files, err)
	}
	content := client.ReadObjectFile(root, rows[0].UntrackedOID, "untracked.txt", 1024)
	if content.Kind != ObjectReady || string(content.Data) != "stored in sha256 stash\n" {
		t.Fatalf("SHA-256 untracked content = %+v", content)
	}
}

func writeGitFixture(t *testing.T, root, path, content string) {
	t.Helper()
	writeGitFixtureBytes(t, root, path, []byte(content))
}

func writeGitFixtureBytes(t *testing.T, root, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), content, 0o644); err != nil {
		t.Fatal(err)
	}
}
