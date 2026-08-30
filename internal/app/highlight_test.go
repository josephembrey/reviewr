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
