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
