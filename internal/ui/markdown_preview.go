package ui

import (
	"strings"

	"charm.land/glamour/v2"
	markdownansi "charm.land/glamour/v2/ansi"
	"github.com/charmbracelet/x/ansi"
)

// RenderMarkdownPreview turns trusted Markdown syntax into a bounded terminal
// document. Source controls are neutralized before Glamour is allowed to
// author the ANSI sequences retained by ReaderRow.Styled.
func RenderMarkdownPreview(source string, rows Rect) (ReaderDocument, error) {
	width := max(1, rows.Width)
	document, err := renderMarkdownPreviewWidth(source, width)
	if err != nil {
		return ReaderDocument{}, err
	}
	if rows.Height > 0 && len(document.Rows) > rows.Height && width > 1 {
		return renderMarkdownPreviewWidth(source, width-1)
	}
	return document, nil
}

func renderMarkdownPreviewWidth(source string, width int) (ReaderDocument, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownPreviewStyle()),
		glamour.WithWordWrap(max(1, width)),
		glamour.WithTableWrap(true),
	)
	if err != nil {
		return ReaderDocument{}, err
	}
	safeSource := strings.Join(SafeContentLines(source), "\n")
	output, err := renderer.Render(safeSource)
	if err != nil {
		return ReaderDocument{}, err
	}
	output = strings.Trim(output, "\n")
	lines := []string{""}
	if output != "" {
		lines = strings.Split(output, "\n")
	}
	document := ReaderDocument{Kind: ReaderMarkdownDocument, Rows: make([]ReaderRow, len(lines))}
	for index, line := range lines {
		plain := ansi.Strip(line)
		trimmed := strings.TrimRight(plain, " ")
		line = ansi.Cut(line, 0, ansi.StringWidth(trimmed))
		document.Rows[index] = ReaderRow{
			Kind: ReaderMarkdown,
			Text: trimmed, Styled: line,
		}
	}
	return document, nil
}

// markdownPreviewStyle uses the terminal's ANSI slots rather than an imposed
// RGB theme. It deliberately stays restrained: layout conveys structure while
// cyan, yellow, and BrightBlack carry the few useful semantic accents.
func markdownPreviewStyle() markdownansi.StyleConfig {
	bold, italic, underline, crossedOut := true, true, true, true
	zero, one := uint(0), uint(1)
	quoteToken := "│ "
	cyan, yellow, muted := "6", "3", "8"
	return markdownansi.StyleConfig{
		Document: markdownansi.StyleBlock{},
		BlockQuote: markdownansi.StyleBlock{
			StylePrimitive: markdownansi.StylePrimitive{Color: &muted},
			Indent:         &one, IndentToken: &quoteToken,
		},
		List: markdownansi.StyleList{LevelIndent: 2},
		Heading: markdownansi.StyleBlock{StylePrimitive: markdownansi.StylePrimitive{
			BlockSuffix: "\n", Color: &cyan, Bold: &bold,
		}},
		H1:            markdownansi.StyleBlock{StylePrimitive: markdownansi.StylePrimitive{Prefix: "▌ "}},
		H2:            markdownansi.StyleBlock{StylePrimitive: markdownansi.StylePrimitive{Prefix: "› "}},
		Emph:          markdownansi.StylePrimitive{Italic: &italic},
		Strong:        markdownansi.StylePrimitive{Bold: &bold},
		Strikethrough: markdownansi.StylePrimitive{CrossedOut: &crossedOut},
		HorizontalRule: markdownansi.StylePrimitive{
			Color: &muted, Format: "\n────────────────\n",
		},
		Item:        markdownansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: markdownansi.StylePrimitive{BlockPrefix: ". "},
		Task: markdownansi.StyleTask{
			Ticked: "[✓] ", Unticked: "[ ] ",
		},
		Link:     markdownansi.StylePrimitive{Color: &cyan, Underline: &underline},
		LinkText: markdownansi.StylePrimitive{Color: &cyan, Underline: &underline},
		ImageText: markdownansi.StylePrimitive{
			Color: &muted, Format: "image: {{.text}} →",
		},
		Code: markdownansi.StyleBlock{StylePrimitive: markdownansi.StylePrimitive{
			Color: &yellow,
		}},
		CodeBlock: markdownansi.StyleCodeBlock{
			StyleBlock: markdownansi.StyleBlock{Indent: &zero, Margin: &zero},
			Chroma:     markdownPreviewChroma(),
		},
		Table: markdownansi.StyleTable{
			CenterSeparator: stringPointer("┼"), ColumnSeparator: stringPointer("│"), RowSeparator: stringPointer("─"),
		},
		DefinitionDescription: markdownansi.StylePrimitive{BlockPrefix: "  "},
	}
}

func markdownPreviewChroma() *markdownansi.Chroma {
	color := func(value string) markdownansi.StylePrimitive {
		return markdownansi.StylePrimitive{Color: &value}
	}
	italic := true
	bold := true
	// Chroma requires RGB descriptors, so use the exact xterm base-palette
	// anchors. Its terminal256 formatter emits slots 0–8 from these values;
	// the rendered preview therefore remains ANSI/theme-owned, not truecolor.
	red, green, yellow := "#800000", "#008000", "#808000"
	blue, purple, cyan := "#000080", "#800080", "#008080"
	white, muted := "#c0c0c0", "#808080"
	return &markdownansi.Chroma{
		Text: color(white), Error: color(red), Comment: color(muted), CommentPreproc: color(yellow),
		Keyword: color(blue), KeywordReserved: color(purple), KeywordNamespace: color(cyan), KeywordType: color(green),
		Operator: color(cyan), Punctuation: color(white), Name: color(white), NameBuiltin: color(cyan),
		NameTag: color(red), NameAttribute: color(cyan), NameClass: color(green), NameConstant: color(yellow),
		NameDecorator: color(purple), NameException: color(red), NameFunction: color(cyan), NameOther: color(white),
		Literal: color(yellow), LiteralNumber: color(yellow), LiteralDate: color(yellow), LiteralString: color(green),
		LiteralStringEscape: color(cyan), GenericDeleted: color(red), GenericInserted: color(green),
		GenericEmph:       markdownansi.StylePrimitive{Italic: &italic},
		GenericStrong:     markdownansi.StylePrimitive{Bold: &bold},
		GenericSubheading: color(cyan),
	}
}

func stringPointer(value string) *string {
	return &value
}
