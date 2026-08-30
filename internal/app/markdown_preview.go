package app

import (
	"path/filepath"
	"strings"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

var markdownExtensions = map[string]struct{}{
	".md": {}, ".markdown": {}, ".mdown": {}, ".mkd": {}, ".mkdn": {},
}

func (state filesState) markdownPreviewEligible() bool {
	if state.readerMode != workspace.FileReader || state.readerEntry.Path == "" {
		return false
	}
	_, ok := markdownExtensions[strings.ToLower(filepath.Ext(state.readerEntry.Path))]
	return ok
}

func (state filesState) markdownPreviewActive() bool {
	return state.markdownPreviewEligible() && state.markdownPreviewPaths[state.readerEntry.Path]
}

func (state filesState) markdownSource() (string, bool) {
	if state.displayedComparison != nil {
		return state.reviewFile.Text, state.reviewFile.State == review.ContentText
	}
	return state.reader.Content, state.reader.Kind == repository.FileReady && !state.reader.Symlink
}

func (state *filesState) toggleMarkdownPreview(rows ui.Rect) {
	if !state.markdownPreviewEligible() {
		return
	}
	path := state.readerEntry.Path
	state.resetReaderInteraction()
	if state.markdownPreviewPaths[path] {
		delete(state.markdownPreviewPaths, path)
	} else {
		state.markdownPreviewPaths[path] = true
	}
	state.markdownPresentation = nil
	state.rebuildMarkdownPreview(rows)
	state.place.ReaderOffset = 0
	state.place.ReaderColumn = 0
	state.place.ReaderCursor = 0
}

func (state *filesState) resizeMarkdownPreview(rows ui.Rect) {
	if !state.markdownPreviewActive() {
		state.markdownRows = rows
		return
	}
	if state.markdownPresentation != nil && state.markdownRows.Width == rows.Width && state.markdownRows.Height == rows.Height {
		return
	}
	oldRows := readerRowIdentities(state.readerRows())
	oldOffset, oldCursor := state.place.ReaderOffset, state.place.ReaderCursor
	state.rebuildMarkdownPreview(rows)
	state.reconcileReaderPlace(oldRows, oldOffset, oldCursor)
}

func (state *filesState) rebuildMarkdownPreview(rows ui.Rect) {
	state.markdownRows = rows
	state.markdownPresentation = nil
	if !state.markdownPreviewActive() || rows.Width <= 0 {
		return
	}
	source, ok := state.markdownSource()
	if !ok {
		return
	}
	document, err := ui.RenderMarkdownPreview(source, rows)
	if err != nil {
		document = ui.ReaderDocument{Kind: ui.ReaderMarkdownDocument, Rows: []ui.ReaderRow{{
			Kind: ui.ReaderMarkdown, Text: "Markdown preview failed: " + ui.SafeSingleLine(err.Error()), Tone: ui.ToneError,
		}}}
	}
	state.markdownPresentation = &document
}

func (state filesState) activeReaderPresentation() *ui.ReaderDocument {
	if state.markdownPreviewActive() && state.markdownPresentation != nil {
		return state.markdownPresentation
	}
	return state.readerPresentation
}
