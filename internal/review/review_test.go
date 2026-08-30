package review

import (
	"fmt"
	"strings"
	"testing"
)

func endpoint(path, content string) Endpoint {
	return Endpoint{Path: path, Kind: Regular, Mode: 0o100644, ContentID: ContentIdentity([]byte(content))}
}

func comparison(scope string, old, new Endpoint) FileComparison {
	action := Modified
	if old.Path != new.Path {
		action = Renamed
	}
	return FileComparison{
		Identity:  ComparisonIdentity{Scope: scope, Basis: "basis"},
		OldSource: EndpointSource{Kind: GitTreeSource, Value: "base"},
		NewSource: EndpointSource{Kind: WorktreeSource},
		Action:    action,
		Old:       old,
		New:       new,
	}
}

func retained(value string) *string { return &value }

func markFull(t *testing.T, ledger *Ledger, edge FileComparison, snapshot *string) {
	t.Helper()
	if !ledger.Mark(edge, Bounds{Old: edge.Old, New: edge.New}, snapshot) {
		t.Fatal("mark returned false")
	}
}

func TestStateBadgesLabelsAndGapPriority(t *testing.T) {
	cases := []struct {
		state    State
		badge    string
		label    string
		priority int
		gap      bool
	}{
		{Unreviewed, "[ ]", "unreviewed", 3, true},
		{Reviewed, "[x]", "reviewed", 0, false},
		{Updated, "[~]", "updated since review", 1, true},
		{Partial, "[!]", "re-review required", 2, true},
		{BasisChanged, "[!]", "re-review required", 0, true},
	}
	for _, test := range cases {
		priority, gap := test.state.GapPriority()
		if test.state.Badge() != test.badge || test.state.Label() != test.label || priority != test.priority || gap != test.gap {
			t.Fatalf("state %v = badge %q label %q priority %d/%v", test.state, test.state.Badge(), test.state.Label(), priority, gap)
		}
		if len(test.state.Badge()) != 3 {
			t.Fatalf("badge %q is not fixed width", test.state.Badge())
		}
	}
}

func TestCoverageIsExplicitCumulativeAndExactRevertIsReviewed(t *testing.T) {
	a := endpoint("a.go", "a")
	b := endpoint("a.go", "b")
	c := endpoint("a.go", "c")
	ab := comparison("branch", a, b)
	bc := comparison("last-turn", b, c)
	ac := comparison("branch", a, c)
	var ledger Ledger
	if got := ledger.Assess(ab).State; got != Unreviewed {
		t.Fatalf("observation = %v, want unreviewed", got)
	}
	markFull(t, &ledger, ab, retained("b"))
	if got := ledger.Assess(ab).State; got != Reviewed {
		t.Fatalf("ab = %v, want reviewed", got)
	}
	if got := ledger.Assess(ac); got.State != Updated || got.Frontier == nil || *got.Frontier != b || got.Retained == nil || *got.Retained != "b" {
		t.Fatalf("ac assessment = %+v, want updated at b", got)
	}
	markFull(t, &ledger, bc, retained("c"))
	if got := ledger.Assess(ac).State; got != Reviewed {
		t.Fatalf("composed ac = %v, want reviewed", got)
	}
	if got := ledger.Assess(ab).State; got != Reviewed {
		t.Fatalf("exact revert ab = %v, want reviewed", got)
	}
}

func TestNarrowAndDisjointCoverageNeverChecksBroaderComparison(t *testing.T) {
	a := endpoint("a.go", "a")
	b := endpoint("a.go", "b")
	c := endpoint("a.go", "c")
	bc := comparison("last-turn", b, c)
	ac := comparison("branch", a, c)
	var ledger Ledger
	markFull(t, &ledger, bc, retained("c"))
	if got := ledger.Assess(ac); got.State != Partial || got.Reason == "" {
		t.Fatalf("narrow assessment = %+v, want partial", got)
	}
	unrelated := comparison("branch", endpoint("other.go", "a"), endpoint("other.go", "c"))
	if got := ledger.Assess(unrelated).State; got != Unreviewed {
		t.Fatalf("unrelated = %v, want unreviewed", got)
	}
}

func TestIncrementalMarkUsesFrontierProvenanceAndClearRemovesApplicableProof(t *testing.T) {
	a := endpoint("a.go", "a")
	b := endpoint("a.go", "b")
	c := endpoint("a.go", "c")
	ab := comparison("branch", a, b)
	ac := comparison("branch", a, c)
	bc := comparison("last-turn", b, c)
	var ledger Ledger
	markFull(t, &ledger, ab, retained("b"))
	markFull(t, &ledger, comparison("branch", endpoint("other", "a"), endpoint("other", "b")), retained("other"))
	if !ledger.Mark(ac, Bounds{Old: b, New: c}, retained("c")) {
		t.Fatal("incremental mark failed")
	}
	last := ledger.ReceiptData[len(ledger.ReceiptData)-1]
	if last.Action != Modified || last.OldSource.Kind != WorktreeSource {
		t.Fatalf("incremental receipt = %+v", last)
	}
	if got := ledger.Assess(ac).State; got != Reviewed {
		t.Fatalf("incremental composition = %v, want reviewed", got)
	}

	var clearing Ledger
	markFull(t, &clearing, ac, retained("c"))
	markFull(t, &clearing, bc, retained("c"))
	if !clearing.Clear(ac) {
		t.Fatal("clear failed")
	}
	if got := clearing.Assess(ac).State; got == Reviewed {
		t.Fatal("clear left active proof reviewed")
	}
	foundNarrow := false
	for _, receipt := range clearing.ReceiptData {
		foundNarrow = foundNarrow || receipt.Comparison.Scope == "last-turn"
	}
	if !foundNarrow {
		t.Fatal("clear removed unrelated narrow edge")
	}
}

func TestCrossPathActionsCannotComposeIntoOrdinaryModification(t *testing.T) {
	a := endpoint("a", "a")
	b := endpoint("b", "b")
	a2 := endpoint("a", "a2")
	rename := comparison("branch", a, b)
	rename.Action = Renamed
	back := comparison("branch", b, a2)
	back.Action = Renamed
	var ledger Ledger
	markFull(t, &ledger, rename, retained("b"))
	markFull(t, &ledger, back, retained("a2"))
	modified := comparison("branch", a, a2)
	if got := ledger.Assess(modified).State; got == Reviewed {
		t.Fatal("rename excursion falsely covered an ordinary modification")
	}
}

func TestAssessAllMatchesExactAssessmentAcrossRenameComponents(t *testing.T) {
	a := endpoint("a", "a")
	b := endpoint("b", "b")
	c := endpoint("c", "c")
	ab := comparison("branch", a, b)
	ab.Action = Renamed
	bc := comparison("branch", b, c)
	bc.Action = Renamed
	ac := comparison("branch", a, c)
	ac.Action = Renamed
	unrelated := comparison("branch", endpoint("other", "old"), endpoint("other", "new"))
	var ledger Ledger
	markFull(t, &ledger, ab, retained("b"))
	markFull(t, &ledger, bc, retained("c"))
	markFull(t, &ledger, unrelated, retained("new"))
	comparisons := map[string]FileComparison{"c": ac, "other": unrelated}
	all := ledger.AssessAll(comparisons)
	for path, candidate := range comparisons {
		if got, want := all[path], ledger.Assess(candidate); got.State != want.State || got.Reason != want.Reason {
			t.Fatalf("AssessAll(%s) = %+v, want %+v", path, got, want)
		}
	}
}

func TestEveryExactFileTransitionIsReviewableOnlyAsItsAction(t *testing.T) {
	regular := endpoint("a", "bytes")
	executable := regular
	executable.Mode = 0o100755
	symlink := Endpoint{Path: "a", Kind: Symlink, Mode: 0o120000, ContentID: ContentIdentity([]byte("target"))}
	submoduleOld := Endpoint{Path: "sub", Kind: Submodule, Mode: 0o160000, ContentID: "git:111"}
	submoduleNew := Endpoint{Path: "sub", Kind: Submodule, Mode: 0o160000, ContentID: "git:222"}
	cases := []struct {
		name     string
		action   FileAction
		old, new Endpoint
		snapshot *string
	}{
		{"content", Modified, regular, endpoint("a", "new"), retained("new")},
		{"mode", Modified, regular, executable, retained("bytes")},
		{"rename", Renamed, regular, endpoint("renamed", "bytes"), retained("bytes")},
		{"copy", Copied, regular, endpoint("copy", "bytes"), retained("bytes")},
		{"delete", Deleted, regular, AbsentEndpoint("a"), retained("")},
		{"symlink", Modified, regular, symlink, retained("target")},
		{"submodule", Modified, submoduleOld, submoduleNew, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			edge := comparison("branch", test.old, test.new)
			edge.Action = test.action
			var ledger Ledger
			if ledger.Assess(edge).State == Reviewed {
				t.Fatal("transition inherited reviewed")
			}
			markFull(t, &ledger, edge, test.snapshot)
			if got := ledger.Assess(edge).State; got != Reviewed {
				t.Fatalf("explicit edge = %v, want reviewed", got)
			}
		})
	}
}

func TestCopyRenameModeKindAndUnavailableIdentityAreConservative(t *testing.T) {
	source := endpoint("old", "same")
	destination := endpoint("new", "same")
	rename := comparison("branch", source, destination)
	rename.Action = Renamed
	var ledger Ledger
	markFull(t, &ledger, rename, retained("same"))
	copy := rename
	copy.Action = Copied
	if got := ledger.Assess(copy).State; got == Reviewed {
		t.Fatal("rename receipt covered copy")
	}
	mode := destination
	mode.Mode = 0o100755
	modeComparison := comparison("branch", source, mode)
	if got := ledger.Assess(modeComparison).State; got != Updated {
		t.Fatalf("mode edit = %v, want updated", got)
	}
	kind := destination
	kind.Kind = Symlink
	if got := ledger.Assess(comparison("branch", source, kind)).State; got != Updated {
		t.Fatalf("kind edit = %v, want updated", got)
	}
	unavailable := destination
	unavailable.ContentID = ""
	if got := ledger.Assess(comparison("branch", source, unavailable)).State; got != BasisChanged {
		t.Fatalf("unavailable = %v, want basis changed", got)
	}
}

func TestAmbiguousAndMovedBasisRequireFullReview(t *testing.T) {
	a := endpoint("a", "a")
	b := endpoint("a", "b")
	var ledger Ledger
	markFull(t, &ledger, comparison("branch", a, b), retained("b"))
	moved := comparison("branch", endpoint("a", "new base"), endpoint("a", "new current"))
	moved.Identity.Basis = "rebased"
	moved.BasisReason = "ambiguous rename lineage"
	assessment := ledger.Assess(moved)
	if assessment.State != BasisChanged || assessment.Reason != moved.BasisReason {
		t.Fatalf("ambiguous = %+v", assessment)
	}
	markFull(t, &ledger, moved, retained("new current"))
	if got := ledger.Assess(moved).State; got != Reviewed {
		t.Fatalf("direct full review = %v", got)
	}
}

func TestBinaryAndOversizedReceiptsKeepIdentityButLoseIncrementalSnapshot(t *testing.T) {
	a := endpoint("blob", "a")
	b := endpoint("blob", "b")
	c := endpoint("blob", "c")
	ab := comparison("branch", a, b)
	var binary Ledger
	markFull(t, &binary, ab, nil)
	if got := binary.Assess(comparison("branch", a, c)).State; got != BasisChanged {
		t.Fatalf("binary edit = %v, want basis changed", got)
	}
	if got := binary.Assess(ab).State; got != Reviewed {
		t.Fatalf("binary exact revert = %v, want reviewed", got)
	}

	large := strings.Repeat("x", MaxRetainedBytes+1)
	var oversized Ledger
	markFull(t, &oversized, ab, &large)
	if oversized.ReceiptData[0].Retained != nil {
		t.Fatal("oversized retained body was kept")
	}
	if got := oversized.Assess(comparison("branch", a, c)).State; got != BasisChanged {
		t.Fatalf("oversized edit = %v, want basis changed", got)
	}
}

func TestCompactionDeduplicatesAndBoundsHistoryAndRetainedTotal(t *testing.T) {
	var ledger Ledger
	base := endpoint("a", "base")
	duplicate := comparison("branch", base, endpoint("a", "one"))
	markFull(t, &ledger, duplicate, retained("old"))
	markFull(t, &ledger, duplicate, retained("new"))
	if len(ledger.ReceiptData) != 1 || ledger.ReceiptData[0].Retained == nil || *ledger.ReceiptData[0].Retained != "new" {
		t.Fatalf("duplicate compaction = %+v", ledger.ReceiptData)
	}

	for index := 0; index < MaxReceipts+10; index++ {
		old := endpoint("f", fmt.Sprintf("%d", index))
		new := endpoint("f", fmt.Sprintf("%d", index+1))
		ledger.ReceiptData = append(ledger.ReceiptData, Receipt{
			Comparison: ComparisonIdentity{Scope: "branch", Basis: "basis"},
			Action:     Modified, Old: old, New: new, Retained: retained("x"), Sequence: uint64(index + 10),
		})
	}
	ledger.Compact()
	if len(ledger.ReceiptData) != MaxReceipts {
		t.Fatalf("receipt count = %d, want %d", len(ledger.ReceiptData), MaxReceipts)
	}

	chunk := strings.Repeat("z", 1_000_000)
	for index := range ledger.ReceiptData[:20] {
		ledger.ReceiptData[index].Retained = retained(chunk)
	}
	ledger.Compact()
	total := 0
	missing := 0
	for _, receipt := range ledger.ReceiptData {
		if receipt.Retained == nil {
			missing++
		} else {
			total += len(*receipt.Retained)
		}
	}
	if total > MaxRetainedTotal || missing == 0 {
		t.Fatalf("retained total = %d missing = %d", total, missing)
	}
}
