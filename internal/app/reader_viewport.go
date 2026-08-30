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
	workspace workspace.Kind
	rows      ui.Rect
	source    *ui.ReaderDocument
	expanded  bool
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
		key.expanded = m.stashes.readerContextExpanded
	case m.active == workspace.Files:
		key.source = m.files.readerPresentation
		key.expanded = m.files.readerContextExpanded
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
