package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderLineUsesLipGlossForSyntaxSpans(t *testing.T) {
	t.Parallel()
	line := Line{
		Text: "package main",
		Spans: []TextSpan{
			{Text: "package", Style: TextStyle{Foreground: "4", Bold: true}},
			{Text: " main"},
		},
	}
	rendered := renderLine(line)
	if got := ansi.Strip(rendered); got != line.Text {
		t.Fatalf("plain rendered line = %q, want %q", got, line.Text)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered line has no Lip Gloss styling: %q", rendered)
	}
}

func TestRenderLineKeepsSemanticMarkerSeparateFromSyntax(t *testing.T) {
	t.Parallel()
	line := Line{
		Text: "+return 42",
		Spans: []TextSpan{
			{Text: "+", Tone: ToneAdded},
			{Text: "return", Style: TextStyle{Foreground: "4"}},
			{Text: " 42", Style: TextStyle{Foreground: "3"}},
		},
	}
	if got := ansi.Strip(renderLine(line)); got != line.Text {
		t.Fatalf("plain rendered diff = %q, want %q", got, line.Text)
	}
}
