package app

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// readerViewportKey names the immutable presentation and pane width that own a
// wrapped layout. Reader presentations are replaced, never mutated, when new
// file content lands.
type readerViewportKey struct {
	workspace       workspace.Kind
	rows            ui.Rect
	source          *ui.ReaderDocument
	contextProgress int
}

// readerViewport keeps input routing and painting on one wrapped geometry.
// Scrolling changes only place state, so it must remain a cache hit.
type readerViewport struct {
	valid    bool
	key      readerViewportKey
	document ui.ReaderDocument
	layout   ui.ReaderLayout
	foldable bool
}

func (m Model) activeReaderViewportKey() (readerViewportKey, bool) {
	key := readerViewportKey{workspace: m.active, rows: m.geometry.ReaderRows}
	switch {
	case m.gitStashesActive():
		key.source = m.stashes.readerPresentation
		key.contextProgress = m.stashes.readerContextProgress
	case m.active == workspace.Files:
		key.source = m.files.readerPresentation
		key.contextProgress = m.files.readerContextProgress
	default:
		return readerViewportKey{}, false
	}
	// Completed loads always own an immutable presentation. Loading and empty
	// states are cheap and deliberately bypass the cache.
	return key, key.source != nil
}

func (m Model) activeReaderDocument() (ui.ReaderDocument, bool) {
	switch {
	case m.gitStashesActive():
		return m.stashes.readerDocument(), true
	case m.active == workspace.Files:
		return m.files.readerDocument(), true
	default:
		return ui.ReaderDocument{}, false
	}
}

func (m Model) cachedActiveReaderViewport() (readerViewport, bool) {
	key, ok := m.activeReaderViewportKey()
	if !ok || !m.readerViewport.valid || m.readerViewport.key != key {
		return readerViewport{}, false
	}
	return m.readerViewport, true
}

func (m *Model) activeReaderViewport() (readerViewport, bool) {
	if cached, ok := m.cachedActiveReaderViewport(); ok {
		return cached, true
	}
	document, ok := m.activeReaderDocument()
	if !ok || document.Kind == ui.ReaderDocumentNone {
		return readerViewport{}, false
	}
	viewport := readerViewport{
		document: document,
		layout:   ui.CalculateReaderLayout(m.geometry.ReaderRows, document),
		foldable: document.HasContextFold(),
	}
	if key, cacheable := m.activeReaderViewportKey(); cacheable {
		viewport.valid = true
		viewport.key = key
		m.readerViewport = viewport
	}
	return viewport, true
}

func (m *Model) rememberActiveReaderLayout(place *navigation.State, document ui.ReaderDocument, layout ui.ReaderLayout) {
	if place != m.activePlace() {
		return
	}
	key, ok := m.activeReaderViewportKey()
	if !ok {
		return
	}
	m.readerViewport = readerViewport{
		valid: true, key: key, document: document, layout: layout,
		foldable: document.HasContextFold(),
	}
}

func (m *Model) activeReaderLineCount() int {
	if layout, ok := m.activeReaderLayout(); ok {
		return layout.Total
	}
	if m.gitStashesActive() {
		return len(m.stashes.readerRows())
	}
	if m.gitRefsActive() {
		return len(m.refs.commits)
	}
	if m.active == workspace.Git {
		return len(commitSummaryLines(m.history.summary))
	}
	return len(m.files.readerRows())
}

func (m *Model) activeReaderLayout() (ui.ReaderLayout, bool) {
	viewport, ok := m.activeReaderViewport()
	return viewport.layout, ok
}

func (m *Model) activeReaderVisualOffset() int {
	place := m.activePlace()
	if layout, ok := m.activeReaderLayout(); ok {
		return layout.VisualOffset(place.ReaderOffset, place.ReaderColumn)
	}
	return place.ReaderOffset
}

func (m *Model) setActiveReaderVisualOffset(offset int) {
	place := m.activePlace()
	if layout, ok := m.activeReaderLayout(); ok {
		maximum := max(0, layout.Total-m.geometry.ReaderRows.Height)
		source, column := layout.SourceOffset(min(max(offset, 0), maximum))
		place.ReaderOffset = source
		place.ReaderColumn = column
		return
	}
	place.ReaderOffset = min(max(offset, 0), max(0, m.activeReaderLineCount()-m.geometry.ReaderRows.Height))
	place.ReaderColumn = 0
}

func (m *Model) scrollActiveReader(delta int) {
	m.setActiveReaderVisualOffset(m.activeReaderVisualOffset() + delta)
}

func (m *Model) clampDocumentReader(place *navigation.State, document ui.ReaderDocument) {
	if document.Kind == ui.ReaderDocumentNone {
		place.ClampReader(0, m.geometry.ReaderRows.Height)
		return
	}
	layout := ui.CalculateReaderLayout(m.geometry.ReaderRows, document)
	maximum := max(0, layout.Total-m.geometry.ReaderRows.Height)
	source, column := layout.SourceOffset(min(layout.VisualOffset(place.ReaderOffset, place.ReaderColumn), maximum))
	place.ReaderOffset = source
	place.ReaderColumn = column
	m.rememberActiveReaderLayout(place, document, layout)
}
