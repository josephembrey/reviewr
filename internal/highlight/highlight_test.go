package highlight

import (
	"strings"
	"testing"
)

func TestLinesDetectsLexerAndPreservesSource(t *testing.T) {
	t.Parallel()
	input := []string{
		"package main",
		"",
		"// a useful comment",
		`const answer = 42`,
		`var greeting = "hello"`,
	}
	lines := New(0).Lines("cmd/reviewr/main.go", input)
	if len(lines) != len(input) {
		t.Fatalf("highlighted lines = %d, want %d", len(lines), len(input))
	}
	for index, spans := range lines {
		if text := spanText(spans); text != input[index] {
			t.Fatalf("line %d text = %q, want %q", index, text, input[index])
		}
	}
	if got := styleForText(lines[0], "package").Foreground; got != "6" {
		t.Fatalf("package color = %q, want terminal cyan", got)
	}
	comment := styleForText(lines[2], "comment")
	if comment.Foreground != "8" || !comment.Italic {
		t.Fatalf("comment style = %+v, want italic comment color", comment)
	}
	if got := styleForText(lines[3], "42").Foreground; got != "3" {
		t.Fatalf("number color = %q, want terminal yellow", got)
	}
	if got := styleForText(lines[4], `"hello"`).Foreground; got != "2" {
		t.Fatalf("string color = %q, want terminal green", got)
	}
}

func TestLinesSupportsNixByExtension(t *testing.T) {
	t.Parallel()
	input := []string{`{ pkgs ? import <nixpkgs> {} }:`, `pkgs.mkShell { packages = [ pkgs.go ]; }`}
	lines := New(0).Lines("flake.nix", input)
	if len(lines) != len(input) || !hasForeground(lines) {
		t.Fatalf("Nix source was not highlighted: %+v", lines)
	}
	for index := range input {
		if got := spanText(lines[index]); got != input[index] {
			t.Fatalf("line %d text = %q, want %q", index, got, input[index])
		}
	}
}

func TestCacheReturnsIndependentDocuments(t *testing.T) {
	t.Parallel()
	highlighter := New(1)
	input := []string{"package main"}
	first := highlighter.Lines("main.go", input)
	if len(first) == 0 || len(first[0]) == 0 {
		t.Fatalf("first result is empty: %+v", first)
	}
	first[0][0].Text = "mutated"
	second := highlighter.Lines("main.go", input)
	if got := spanText(second[0]); got != input[0] {
		t.Fatalf("cached source = %q, want %q", got, input[0])
	}
}

func spanText(spans []Span) string {
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.Text)
	}
	return text.String()
}

func styleForText(spans []Span, text string) Style {
	for _, span := range spans {
		if strings.Contains(span.Text, text) {
			return span.Style
		}
	}
	return Style{}
}

func hasForeground(lines [][]Span) bool {
	for _, line := range lines {
		for _, span := range line {
			if span.Style.Foreground != "" {
				return true
			}
		}
	}
	return false
}
