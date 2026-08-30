package notes

import (
	"strings"
	"testing"
)

func TestMarkdownStylesCoverSyntaxWithoutChangingGraphemeAuthority(t *testing.T) {
	t.Parallel()
	text := "# Heading\n\n**bold** and *emphasis* and `code`\n[link](https://example.com)"
	styles := MarkdownStyles(text)
	if len(styles) != len(splitGraphemes(text)) {
		t.Fatalf("styles = %d, graphemes = %d", len(styles), len(splitGraphemes(text)))
	}
	var bold, italic, colored bool
	for _, style := range styles {
		bold = bold || style.Bold
		italic = italic || style.Italic
		colored = colored || style.Foreground != ""
	}
	link := styles[strings.Index(text, "link")]
	code := styles[strings.Index(text, "code")]
	if !bold || !italic || !colored || (link.Foreground == "" && !link.Underline) || code.Foreground == "" {
		t.Fatalf("Markdown styles lack expected heading/emphasis/code/link treatment: bold=%v italic=%v color=%v link=%+v code=%+v", bold, italic, colored, link, code)
	}

	editor := NewEditor()
	editor.Load(text + "\n\t界e\u0301")
	editor.Resize(13, 5)
	document := editor.Presentation().Document
	styled := editor.Presentation()
	styled.Styles = MarkdownStyles(editor.Text())
	if styled.Document.Width != document.Width || len(styled.Document.Rows) != len(document.Rows) {
		t.Fatalf("Markdown ink changed wrap authority: plain %+v styled %+v", document, styled.Document)
	}
}
