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
	lines := fileReaderLines(repository.File{
		Path:    "cmd/reviewr/main.go",
		Kind:    repository.FileReady,
		Content: "package main\n\nfunc main() {}",
	}, repository.Entry{})
	if len(lines) != 3 {
		t.Fatalf("reader lines = %d, want 3", len(lines))
	}
	for _, line := range lines {
		if got := readerSpanText(line); len(line.Spans) != 0 && got != line.Text {
			t.Fatalf("span text = %q, want %q", got, line.Text)
		}
	}
	if !readerHasColor(lines[0]) || !readerHasColor(lines[2]) {
		t.Fatalf("Go source lacks syntax colors: %+v", lines)
	}
}

func TestUnifiedDiffHighlightsPayloadAndPreservesSemanticMarker(t *testing.T) {
	t.Parallel()
	lines := diffReaderLines(repository.Diff{
		Entry: repository.Entry{Path: "main.go"},
		Kind:  repository.DiffReady,
		Content: "diff --git a/main.go b/main.go\n" +
			"--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n package main\n+func main() {}",
	})
	var added ui.Line
	for _, line := range lines {
		if strings.HasPrefix(line.Text, "+func") {
			added = line
			break
		}
	}
	if len(added.Spans) < 2 {
		t.Fatalf("added line lacks marker and syntax spans: %+v", added)
	}
	if added.Spans[0].Text != "+" || added.Spans[0].Tone != ui.ToneAdded {
		t.Fatalf("added marker = %+v, want semantic add marker", added.Spans[0])
	}
	if added.Tone != ui.ToneDefault || readerSpanText(added) != added.Text || !readerHasColor(added) {
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
	lines := reviewReaderLines("main.go", document)
	for index, wantTone := range []ui.Tone{ui.ToneDefault, ui.ToneRemoved, ui.ToneAdded} {
		if readerSpanText(lines[index]) != lines[index].Text {
			t.Fatalf("line %d spans do not preserve text: %+v", index, lines[index])
		}
		if lines[index].Spans[0].Tone != wantTone {
			t.Fatalf("line %d marker tone = %d, want %d", index, lines[index].Spans[0].Tone, wantTone)
		}
		if !readerHasColor(lines[index]) {
			t.Fatalf("line %d lacks syntax color: %+v", index, lines[index])
		}
	}
}

func readerSpanText(line ui.Line) string {
	var text strings.Builder
	for _, span := range line.Spans {
		text.WriteString(span.Text)
	}
	return text.String()
}

func readerHasColor(line ui.Line) bool {
	for _, span := range line.Spans {
		if span.Style.Foreground != "" {
			return true
		}
	}
	return false
}
