package app

import (
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
)

func TestFileReaderAddsSyntaxSpansWithoutChangingText(t *testing.T) {
	t.Parallel()
	rows := fileReaderDocument(repository.File{
		Path:    "cmd/reviewr/main.go",
		Kind:    repository.FileReady,
		Content: "package main\n\nfunc main() {}",
	}, repository.Entry{}).Rows
	if len(rows) != 3 {
		t.Fatalf("reader rows = %d, want 3", len(rows))
	}
	for _, row := range rows {
		if got := readerSpanText(row); len(row.Spans) != 0 && got != row.Text {
			t.Fatalf("span text = %q, want %q", got, row.Text)
		}
	}
	if !readerHasColor(rows[0]) || !readerHasColor(rows[2]) {
		t.Fatalf("Go source lacks syntax colors: %+v", rows)
	}
}

func TestUnifiedDiffHighlightsPayloadAndPreservesSemanticMarker(t *testing.T) {
	t.Parallel()
	rows := diffReaderDocument(repository.Diff{
		Entry: repository.Entry{Path: "main.go"},
		Kind:  repository.DiffReady,
		Content: "diff --git a/main.go b/main.go\n" +
			"--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n package main\n+func main() {}",
	}).Rows
	var added ui.ReaderRow
	for _, row := range rows {
		if row.Kind == ui.ReaderInsertion {
			added = row
			break
		}
	}
	if added.Text != "func main() {}" || strings.HasPrefix(added.Text, "+") {
		t.Fatalf("semantic insertion retained a raw marker: %+v", added)
	}
	if len(added.Spans) == 0 {
		t.Fatalf("added row lacks syntax spans: %+v", added)
	}
	if readerSpanText(added) != added.Text || !readerHasColor(added) {
		t.Fatalf("added line was not safely syntax highlighted: %+v", added)
	}
}

func TestReviewDiffHighlightsOldAndNewPayloads(t *testing.T) {
	t.Parallel()
	document := review.Document{Exact: true, Lines: []review.Line{
		{Text: "  package main", Kind: review.ContextLine},
		{Text: "- const answer = 41", Kind: review.RemovedLine},
		{Text: "+ const answer = 42", Kind: review.AddedLine},
	}}
	rows := reviewReaderDocument("main.go", document).Rows
	for index, wantKind := range []ui.ReaderRowKind{ui.ReaderContext, ui.ReaderDeletion, ui.ReaderInsertion} {
		if readerSpanText(rows[index]) != rows[index].Text {
			t.Fatalf("row %d spans do not preserve text: %+v", index, rows[index])
		}
		if rows[index].Kind != wantKind || strings.HasPrefix(rows[index].Text, "+ ") || strings.HasPrefix(rows[index].Text, "- ") {
			t.Fatalf("row %d semantic payload = %+v, want kind %d without marker", index, rows[index], wantKind)
		}
		if !readerHasColor(rows[index]) {
			t.Fatalf("row %d lacks syntax color: %+v", index, rows[index])
		}
	}
}

func TestReviewFileAnnotatesAddsAndDeletionBoundariesWithoutChangingSource(t *testing.T) {
	t.Parallel()
	comparison := testComparison("main.go", "head", "old", "new")
	tests := []struct {
		name          string
		oldText       string
		newText       string
		row           int
		kind          ui.ReaderRowKind
		removedBefore uint64
		removedAfter  uint64
	}{
		{name: "addition", oldText: "one\nkeep\nlast", newText: "one\nadded\nkeep\nlast", row: 1, kind: ui.ReaderInsertion},
		{name: "replacement", oldText: "old\nkeep", newText: "new\nkeep", row: 0, kind: ui.ReaderInsertion, removedBefore: 1},
		{name: "middle deletion", oldText: "one\ngone\nkeep", newText: "one\nkeep", row: 1, kind: ui.ReaderFile, removedBefore: 1},
		{name: "end deletion", oldText: "one\nkeep\ngone\n", newText: "one\nkeep\n", row: 1, kind: ui.ReaderFile, removedAfter: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			oldContent := content(comparison.Old, test.oldText)
			newContent := content(comparison.New, test.newText)
			diff := review.BuildDocument(review.Bounds{Old: comparison.Old, New: comparison.New}, oldContent, newContent)
			document := annotatedReviewFileReaderDocument(newContent, repository.Entry{Path: "main.go"}, comparison, diff)
			wantLines := ui.SafeContentLines(test.newText)
			if len(document.Rows) != len(wantLines) {
				t.Fatalf("rows = %d, want %d", len(document.Rows), len(wantLines))
			}
			for index, want := range wantLines {
				if document.Rows[index].Text != want {
					t.Fatalf("row %d text = %q, want exact current source %q", index, document.Rows[index].Text, want)
				}
			}
			row := document.Rows[test.row]
			if row.Kind != test.kind || row.RemovedBefore != test.removedBefore || row.RemovedAfter != test.removedAfter {
				t.Fatalf("annotated row = %+v", row)
			}
			if readerSpanText(row) != "" && readerSpanText(row) != row.Text {
				t.Fatalf("syntax spans changed source: %+v", row)
			}
		})
	}
}

func TestReviewFileAnnotatesAddedAndDeletedFiles(t *testing.T) {
	t.Parallel()
	added := testComparison("new.go", "head", "absent", "new")
	added.Action = review.Added
	added.Old = review.AbsentEndpoint("new.go")
	oldContent := review.AbsentContent("new.go")
	newContent := content(added.New, "one\ntwo")
	diff := review.BuildDocument(review.Bounds{Old: added.Old, New: added.New}, oldContent, newContent)
	document := annotatedReviewFileReaderDocument(newContent, repository.Entry{Path: "new.go"}, added, diff)
	for _, row := range document.Rows {
		if row.Kind != ui.ReaderInsertion {
			t.Fatalf("added file row is not marked added: %+v", row)
		}
	}

	deleted := testComparison("old.go", "head", "old", "absent")
	deleted.Action = review.Deleted
	deleted.New = review.AbsentEndpoint("old.go")
	oldContent = content(deleted.Old, "one\ntwo")
	deletedContent := review.AbsentContent("old.go")
	diff = review.BuildDocument(review.Bounds{Old: deleted.Old, New: deleted.New}, oldContent, deletedContent)
	document = annotatedReviewFileReaderDocument(deletedContent, repository.Entry{Path: "old.go"}, deleted, diff)
	if len(document.Rows) != 1 || document.Rows[0].RemovedBefore != 2 {
		t.Fatalf("deleted file marker = %+v", document.Rows)
	}
}

func TestReviewFreshnessProjectsNewLinesRemovalsAndReversions(t *testing.T) {
	t.Parallel()
	frontier := review.Endpoint{Path: "main.go", Kind: review.Regular, Mode: 0o100644, ContentID: "frontier"}
	current := review.Endpoint{Path: "main.go", Kind: review.Regular, Mode: 0o100644, ContentID: "current"}
	freshness := review.BuildDocument(
		review.Bounds{Old: frontier, New: current},
		content(frontier, "one\nreviewed\nremove-me\nlast\n"),
		content(current, "one\nnew\nlast\n"),
	)
	presentation := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "one", Kind: ui.ReaderContext, Text: "one", NewLine: 1},
		{Identity: "new", Kind: ui.ReaderInsertion, Text: "new", NewLine: 2},
		{Identity: "last", Kind: ui.ReaderContext, Text: "last", NewLine: 3},
	}}
	annotated := annotateReviewFreshness(presentation, freshness)
	if !annotated.Rows[1].ReviewFresh || annotated.Rows[1].ReviewRemovedBefore != 2 ||
		annotated.Rows[0].ReviewFresh || annotated.Rows[2].ReviewFresh {
		t.Fatalf("projected freshness = %+v", annotated.Rows)
	}
	if presentation.HasReviewFreshness() {
		t.Fatal("freshness annotation mutated its source presentation")
	}

	// A reversion is context in the full branch diff but remains new review work.
	reversion := review.BuildDocument(
		review.Bounds{Old: frontier, New: current},
		content(frontier, "changed\n"),
		content(current, "base\n"),
	)
	context := annotateReviewFreshness(ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "base", Kind: ui.ReaderContext, Text: "base", NewLine: 1},
	}}, reversion)
	if !context.Rows[0].ReviewFresh || context.Rows[0].ReviewRemovedBefore != 1 {
		t.Fatalf("reverted full-diff context lost freshness = %+v", context.Rows[0])
	}
}

func TestReviewFreshnessAnchorsDeletionWithoutCurrentLines(t *testing.T) {
	t.Parallel()
	old := review.Endpoint{Path: "gone.go", Kind: review.Regular, Mode: 0o100644, ContentID: "old"}
	missing := review.AbsentEndpoint("gone.go")
	freshness := review.BuildDocument(
		review.Bounds{Old: old, New: missing},
		content(old, "one\ntwo\n"),
		review.AbsentContent("gone.go"),
	)
	document := annotateReviewFreshness(ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: []ui.ReaderRow{
		{Identity: "notice", Kind: ui.ReaderNotice, Text: "File was deleted."},
	}}, freshness)
	if document.Rows[0].ReviewRemovedAfter != 2 || !document.HasReviewFreshness() {
		t.Fatalf("deleted-file freshness = %+v", document.Rows)
	}
}

func TestReviewFreshnessMarksExactMetadataOnlyChanges(t *testing.T) {
	t.Parallel()
	old := review.Endpoint{Path: "script.sh", Kind: review.Regular, Mode: 0o100644, ContentID: "same"}
	current := old
	current.Mode = 0o100755
	freshness := review.Document{
		Bounds: review.Bounds{Old: old, New: current}, Exact: true,
		Lines: []review.Line{{Identity: "notice:mode", Kind: review.NoticeLine, Text: "mode changed"}},
	}
	document := annotateReviewFreshness(ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "notice:mode", Kind: ui.ReaderNotice, Text: "mode changed"},
	}}, freshness)
	if !document.Rows[0].ReviewFresh {
		t.Fatalf("metadata-only freshness = %+v", document.Rows)
	}
}

func readerSpanText(line ui.ReaderRow) string {
	var text strings.Builder
	for _, span := range line.Spans {
		text.WriteString(span.Text)
	}
	return text.String()
}

func readerHasColor(line ui.ReaderRow) bool {
	for _, span := range line.Spans {
		if span.Style.Foreground != "" {
			return true
		}
	}
	return false
}
