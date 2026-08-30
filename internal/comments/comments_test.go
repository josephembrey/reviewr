package comments

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
)

type recordingExporter struct {
	text string
	err  error
}

func (target *recordingExporter) Export(text string) error {
	target.text = text
	return target.err
}

func TestRangeLocationAndExportFormatting(t *testing.T) {
	t.Parallel()
	store := Store{}
	store.Add(Draft{
		File: "b.go", Range: Range{Side: NewSide, Start: SourceLine{Number: 9}, End: SourceLine{Number: 7}},
		Snippet: "+seven\n+eight\n+nine", Text: "keep this\n\n  \nplease  ",
	})
	store.Add(Draft{
		File: "a.go", Range: Range{Side: OldSide, Start: SourceLine{Number: 3}, End: SourceLine{Number: 3}},
		Snippet: "-gone", Text: "still needed",
	})
	want := "a.go:3 (removed)\n-gone\nstill needed\n\nb.go:7-9\n+seven\n+eight\n+nine\nkeep this\nplease"
	if got := FormatAll(store.Items()); got != want {
		t.Fatalf("formatted comments = %q, want %q", got, want)
	}
}

func TestExportConsumesOnlyAfterExplicitSuccess(t *testing.T) {
	t.Parallel()
	store := Store{}
	store.Add(Draft{File: "a.go", Range: Range{Start: SourceLine{Number: 1}, End: SourceLine{Number: 2}}, Text: "note"})
	failure := &recordingExporter{err: errors.New("closed")}
	if err := store.Export(failure); err == nil || store.Len() != 1 {
		t.Fatalf("failed export = %v comments=%d, want retained comment", err, store.Len())
	}
	success := &recordingExporter{}
	if err := store.Export(success); err != nil || store.Len() != 0 || success.text == "" {
		t.Fatalf("successful export = %v comments=%d text=%q", err, store.Len(), success.text)
	}
}

func TestFormatPreservesVerbatimContextAndDiffMarkers(t *testing.T) {
	t.Parallel()
	comment := Comment{
		File:    "src/a.go",
		Range:   Range{Side: NewSide, Start: SourceLine{Number: 4}, End: SourceLine{Number: 5}},
		Snippet: "  context keeps its marker and indent  \n+added()",
		Text:    "explain this",
	}
	want := "src/a.go:4-5\n  context keeps its marker and indent  \n+added()\nexplain this"
	if got := Format(comment); got != want {
		t.Fatalf("Format() = %q, want exact self-contained block %q", got, want)
	}
}

func TestReconcileRelocatesOnlyOneConfidentCurrentSideMatch(t *testing.T) {
	t.Parallel()
	comment := Comment{
		ID: "comment:1", FileIdentity: "file:src/a.go", File: "src/a.go",
		SourceIdentity: "before", Range: Range{
			Side:  CurrentSide,
			Start: SourceLine{Identity: "old:2", Number: 2},
			End:   SourceLine{Identity: "old:3", Number: 3},
		},
		PreferredLine: 3,
		Snippet:       " target\n next",
		Fingerprint: ContextFingerprint{
			Before: []string{" before"}, After: []string{" after"},
		},
		Text: "move with the code",
	}
	snapshot := SourceSnapshot{Identity: "after", Lines: snapshotLines(
		" intro", " inserted", " before", " target", " next", " after",
	)}

	got := Reconcile(comment, snapshot)
	if got.Stale || got.SourceIdentity != "after" ||
		got.Range.Start.Number != 4 || got.Range.Start.Identity != "line:4" ||
		got.Range.End.Number != 5 || got.Range.End.Identity != "line:5" || got.PreferredLine != 5 {
		t.Fatalf("unique relocation = %+v", got)
	}
	if got.Snippet != comment.Snippet || got.Text != comment.Text {
		t.Fatalf("relocation changed authored evidence: %+v", got)
	}
}

func TestReconcileRetainsAuthoredAnchorWhenAmbiguousMissingOrUnmatched(t *testing.T) {
	t.Parallel()
	base := Comment{
		SourceIdentity: "authored", Range: Range{
			Side:  NewSide,
			Start: SourceLine{Identity: "old:7", Number: 7},
			End:   SourceLine{Identity: "old:7", Number: 7},
		},
		PreferredLine: 7,
		Snippet:       "+same",
		Fingerprint: ContextFingerprint{
			Before: []string{" before"}, After: []string{" after"},
		},
	}
	for name, snapshot := range map[string]SourceSnapshot{
		"ambiguous": {Identity: "ambiguous", Lines: snapshotLines(
			" before", "+same", " after", " gap", " before", "+same", " after",
		)},
		"no match": {Identity: "different", Lines: snapshotLines(" before", "+other", " after")},
		"missing":  {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := Reconcile(base, snapshot)
			if !got.Stale || got.SourceIdentity != base.SourceIdentity || got.Range != base.Range || got.PreferredLine != base.PreferredLine {
				t.Fatalf("unresolved reconciliation = %+v, want authored anchor %+v marked stale", got, base)
			}
		})
	}
}

func TestOldSideAnchorRemainsTiedToItsAuthoredRevision(t *testing.T) {
	t.Parallel()
	comment := Comment{
		SourceIdentity: "base-oid", Stale: false,
		Range:   Range{Side: OldSide, Start: SourceLine{Identity: "old:9", Number: 9}, End: SourceLine{Identity: "old:10", Number: 10}},
		Snippet: "-gone one\n-gone two",
	}
	if got := Reconcile(comment, SourceSnapshot{Identity: "another-base", Lines: snapshotLines("-gone one", "-gone two")}); !reflect.DeepEqual(got, comment) {
		t.Fatalf("old-side reconciliation = %+v, want immutable %+v", got, comment)
	}
}

func TestStoreMatchesSemanticFileBeforeReconciliationAndRetainsMissingFiles(t *testing.T) {
	t.Parallel()
	store := Store{}
	store.Add(Draft{
		FileIdentity: "file:a", File: "a.go", Context: "file", SourceIdentity: "old",
		Range:   Range{Side: CurrentSide, Start: SourceLine{Number: 2}, End: SourceLine{Number: 2}},
		Snippet: " selected", Text: "keep me",
	})
	store.Add(Draft{
		FileIdentity: "file:b", File: "b.go", Context: "file", SourceIdentity: "old",
		Range:   Range{Side: CurrentSide, Start: SourceLine{Number: 3}, End: SourceLine{Number: 3}},
		Snippet: " other", Text: "other file",
	})
	if !store.Reconcile("file:a", "file", CurrentSide, SourceSnapshot{}) {
		t.Fatal("missing semantic file did not update its unresolved state")
	}
	items := store.Items()
	if store.Len() != 2 || !items[0].Stale || items[1].Stale {
		t.Fatalf("missing-file reconcile lost or crossed comments: %+v", items)
	}
}

func snapshotLines(text ...string) []SnapshotLine {
	lines := make([]SnapshotLine, len(text))
	for index, value := range text {
		lines[index] = SnapshotLine{Identity: "line:" + strconv.Itoa(index+1), Number: uint64(index + 1), Text: value}
	}
	return lines
}
