package highlight

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
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
	if comment.Foreground != "5" || !comment.Italic {
		t.Fatalf("comment style = %+v, want italic terminal magenta", comment)
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

func TestTokenStylesRetainTerminalColorSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		token chroma.TokenType
		want  Style
	}{
		{token: chroma.Error, want: Style{Foreground: "1"}},
		{token: chroma.NameException, want: Style{Foreground: "1"}},
		{token: chroma.GenericDeleted, want: Style{Foreground: "1"}},
		{token: chroma.GenericInserted, want: Style{Foreground: "2"}},
		{token: chroma.LiteralNumberInteger, want: Style{Foreground: "3"}},
		{token: chroma.LiteralStringInterpol, want: Style{Foreground: "3"}},
		{token: chroma.NameFunction, want: Style{Foreground: "6"}},
		{token: chroma.LiteralStringEscape, want: Style{Foreground: "6"}},
		{token: chroma.LiteralStringDouble, want: Style{Foreground: "2"}},
		{token: chroma.KeywordType, want: Style{Foreground: "2"}},
		{token: chroma.NameClass, want: Style{Foreground: "2"}},
		{token: chroma.Keyword, want: Style{Foreground: "4"}},
		{token: chroma.NameDecorator, want: Style{Foreground: "4"}},
		{token: chroma.CommentSingle, want: Style{Foreground: "5", Italic: true}},
		{token: chroma.Operator, want: Style{Foreground: "8"}},
		{token: chroma.GenericHeading, want: Style{Foreground: "4", Bold: true}},
		{token: chroma.GenericStrong, want: Style{Bold: true}},
		{token: chroma.GenericEmph, want: Style{Italic: true}},
		{token: chroma.GenericUnderline, want: Style{Underline: true}},
		{token: chroma.Text, want: Style{}},
	}
	for _, test := range tests {
		if got := tokenStyle(test.token); got != test.want {
			t.Errorf("tokenStyle(%v) = %+v, want %+v", test.token, got, test.want)
		}
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
