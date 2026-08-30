package app

import (
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestMarkdownPreviewIsAFileLocalSourceToggle(t *testing.T) {
	t.Parallel()
	state := loadedMarkdownFilesState("README.md", "# Reviewr\n\nHello **world**.")
	state.place.ReaderOffset = 4
	state.place.ReaderCursor = 5
	rows := ui.Rect{Width: 50, Height: 10}

	state.toggleMarkdownPreview(rows)
	if !state.markdownPreviewActive() || state.readerDocument().Kind != ui.ReaderMarkdownDocument {
		t.Fatalf("enabled preview = active %v document %+v", state.markdownPreviewActive(), state.readerDocument())
	}
	if state.place.ReaderOffset != 0 || state.place.ReaderCursor != 0 || !strings.Contains(state.readerTitle(), "[preview]") {
		t.Fatalf("enabled preview place/title = %+v / %q", state.place, state.readerTitle())
	}
	if plain := strings.Join(readerRowTexts(state.readerRows()), "\n"); !strings.Contains(plain, "▌ Reviewr") || strings.Contains(plain, "**world**") {
		t.Fatalf("rendered Markdown = %q", plain)
	}

	state.readerEntry = repository.Entry{Path: "notes.txt"}
	if state.markdownPreviewEligible() || state.markdownPreviewActive() {
		t.Fatal("non-Markdown selection inherited preview")
	}
	state.readerEntry = repository.Entry{Path: "README.md"}
	if !state.markdownPreviewActive() {
		t.Fatal("returning to Markdown file lost its local preview choice")
	}

	state.toggleMarkdownPreview(rows)
	if state.markdownPreviewActive() || state.readerDocument().Kind != ui.ReaderFileDocument || strings.Contains(state.readerTitle(), "[preview]") {
		t.Fatalf("disabled preview = active %v kind %v title %q", state.markdownPreviewActive(), state.readerDocument().Kind, state.readerTitle())
	}
}

func TestMarkdownPreviewRefreshReconcilesRenderedRowIdentity(t *testing.T) {
	t.Parallel()
	state := loadedMarkdownFilesState("README.md", "# Heading\n\nFirst paragraph.\n\nStable paragraph.")
	state.toggleMarkdownPreview(ui.Rect{Width: 60, Height: 10})
	stable := readerTextIndex(state.readerRows(), "Stable paragraph.")
	if stable < 0 {
		t.Fatalf("initial preview rows = %#v", readerRowTexts(state.readerRows()))
	}
	state.place.ReaderOffset = stable
	state.place.ReaderCursor = stable
	state = state.landFile(fileLoadedMsg{
		generation: state.contentGeneration,
		entry:      state.readerEntry,
		file: repository.File{
			Path: state.readerEntry.Path, Kind: repository.FileReady,
			Content: "# Heading\n\nInserted paragraph.\n\nFirst paragraph.\n\nStable paragraph.",
		},
	})
	rows := state.readerRows()
	if rows[state.place.ReaderOffset].Text != "Stable paragraph." || rows[state.place.ReaderCursor].Text != "Stable paragraph." {
		t.Fatalf("refresh moved preview place = %+v rows %#v", state.place, readerRowTexts(rows))
	}
}

func TestMarkdownPreviewOnlyAppliesToFileReader(t *testing.T) {
	t.Parallel()
	state := loadedMarkdownFilesState("README.markdown", "# Heading")
	if !state.markdownPreviewEligible() {
		t.Fatal(".markdown file is not eligible")
	}
	state.readerMode = workspace.DiffReader
	if state.markdownPreviewEligible() {
		t.Fatal("diff reader exposed Markdown preview")
	}
}

func loadedMarkdownFilesState(path, content string) filesState {
	state := newFilesState()
	state.readerEntry = repository.Entry{Path: path}
	state.readerMode = workspace.FileReader
	state.reader = repository.File{Path: path, Kind: repository.FileReady, Content: content}
	presentation := fileReaderDocument(state.reader, state.readerEntry)
	state.readerPresentation = &presentation
	return state
}

func readerRowTexts(rows []ui.ReaderRow) []string {
	result := make([]string, len(rows))
	for index, row := range rows {
		result[index] = row.Text
	}
	return result
}

func readerTextIndex(rows []ui.ReaderRow, text string) int {
	for index, row := range rows {
		if row.Text == text {
			return index
		}
	}
	return -1
}
