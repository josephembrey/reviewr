package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMarkdownPreviewRendersStructureWithTerminalPalette(t *testing.T) {
	t.Parallel()
	source := "# Heading\n\nA **strong** and *emphasized* [link](https://example.com).\n\n" +
		"> quoted\n\n- item\n\n```go\nfunc main() {}\n```\n\nunsafe \x1b[31mcontrol"
	document, err := RenderMarkdownPreview(source, Rect{Width: 42, Height: 30})
	if err != nil {
		t.Fatal(err)
	}
	if document.Kind != ReaderMarkdownDocument || len(document.Rows) == 0 {
		t.Fatalf("preview document = %+v", document)
	}
	var plain, styled strings.Builder
	for _, row := range document.Rows {
		if row.Kind != ReaderMarkdown || row.Text != ansi.Strip(row.Styled) {
			t.Fatalf("preview row lost its styled/plain pairing: %+v", row)
		}
		if width := lipgloss.Width(row.Styled); width > 42 {
			t.Fatalf("preview row width = %d, want <= 42: %q", width, row.Styled)
		}
		plain.WriteString(row.Text)
		plain.WriteByte('\n')
		styled.WriteString(row.Styled)
	}
	plainText := plain.String()
	for _, expected := range []string{"▌ Heading", "strong", "emphasized", "link", "│ quoted", "• item", "func main()", "␛[31mcontrol"} {
		if !strings.Contains(plainText, expected) {
			t.Errorf("rendered preview is missing %q: %q", expected, plainText)
		}
	}
	if strings.Contains(plainText, "**strong**") || strings.Contains(plainText, "[link](") {
		t.Fatalf("preview retained Markdown source delimiters: %q", plainText)
	}
	if strings.Contains(styled.String(), "38;2;") || strings.Contains(styled.String(), "48;2;") {
		t.Fatalf("preview imposed truecolor styling: %q", styled.String())
	}
	if !strings.Contains(styled.String(), "38;5;6") {
		t.Fatalf("preview lacks terminal cyan role: %q", styled.String())
	}
}

func TestMarkdownPreviewReservesScrollbarBeforeWrapping(t *testing.T) {
	t.Parallel()
	document, err := RenderMarkdownPreview(
		"A paragraph with enough words to wrap onto several visual rows in a narrow reader.",
		Rect{Width: 20, Height: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Rows) <= 2 {
		t.Fatalf("preview did not overflow: %+v", document.Rows)
	}
	for _, row := range document.Rows {
		if width := lipgloss.Width(row.Styled); width > 19 {
			t.Fatalf("overflow row width = %d, want <= 19: %q", width, row.Styled)
		}
	}
	geometry := CalculateReaderGeometry(Rect{Width: 20, Height: 2}, document, true)
	if geometry.Prefix != 0 || geometry.Code.Width != 19 || geometry.Scrollbar.Width != 1 {
		t.Fatalf("Markdown reader geometry = %+v", geometry)
	}
}
