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
	contextRevision uint64
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
		key.contextRevision = m.stashes.readerContext.revision
	case m.active == workspace.Files:
		key.source = m.files.readerPresentation
		key.contextRevision = m.files.readerContext.revision
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

func (m *Model) moveActiveReaderSelection(delta int) {
	document, ok := m.activeReaderDocument()
	if !ok || document.Kind == ui.ReaderDocumentNone {
		m.scrollActiveReader(delta)
		return
	}
	m.selectActiveReaderLine(m.activePlace().ReaderCursor + delta)
}

func (m *Model) moveActiveReaderPage(delta int) {
	if delta == 0 {
		return
	}
	layout, ok := m.activeReaderLayout()
	if !ok || layout.Total == 0 {
		m.moveActiveReaderSelection(delta)
		return
	}
	place := m.activePlace()
	visualCursor := layout.VisualOffset(place.ReaderCursor, 0)
	targetVisual := max(0, min(visualCursor+delta, layout.Total-1))
	target, _ := layout.SourceOffset(targetVisual)
	m.setActiveReaderVisualOffset(m.activeReaderVisualOffset() + delta)
	place.ReaderCursor = target
}

func (m *Model) selectActiveReaderBoundary(end bool) {
	document, ok := m.activeReaderDocument()
	if !ok || len(document.Rows) == 0 {
		return
	}
	target := 0
	if end {
		target = len(document.Rows) - 1
	}
	m.selectActiveReaderLine(target)
}

func (m *Model) selectActiveReaderViewport(position int) {
	layout, ok := m.activeReaderLayout()
	if !ok || layout.Total == 0 || m.geometry.ReaderRows.Height <= 0 {
		return
	}
	top := m.activeReaderVisualOffset()
	bottom := min(layout.Total-1, top+m.geometry.ReaderRows.Height-1)
	targetVisual := top + (bottom-top)/2
	if position < 0 {
		targetVisual = top
	} else if position > 0 {
		targetVisual = bottom
	}
	target, _ := layout.SourceOffset(targetVisual)
	m.activePlace().ReaderCursor = target
}

func (m *Model) selectActiveReaderLine(index int) {
	document, ok := m.activeReaderDocument()
	if !ok || len(document.Rows) == 0 {
		return
	}
	place := m.activePlace()
	place.ReaderCursor = max(0, min(index, len(document.Rows)-1))
	m.ensureActiveReaderSelectionVisible()
}

func (m *Model) ensureActiveReaderSelectionVisible() {
	layout, ok := m.activeReaderLayout()
	if !ok || layout.Total == 0 || m.geometry.ReaderRows.Height <= 0 {
		return
	}
	cursor := layout.VisualOffset(m.activePlace().ReaderCursor, 0)
	top := m.activeReaderVisualOffset()
	bottom := top + m.geometry.ReaderRows.Height
	switch {
	case cursor < top:
		m.setActiveReaderVisualOffset(cursor)
	case cursor >= bottom:
		m.setActiveReaderVisualOffset(cursor - m.geometry.ReaderRows.Height + 1)
	}
}

func (m *Model) selectActiveReaderHunk(delta int) {
	document, ok := m.activeReaderDocument()
	if !ok || delta == 0 {
		return
	}
	targets := document.HunkNavigationTargets()
	if len(targets) == 0 {
		return
	}
	current := m.activePlace().ReaderCursor
	target := -1
	if delta > 0 {
		for _, candidate := range targets {
			if candidate > current {
				target = candidate
				break
			}
		}
	} else {
		for index := len(targets) - 1; index >= 0; index-- {
			if targets[index] < current {
				target = targets[index]
				break
			}
		}
	}
	if target < 0 {
		return
	}
	m.selectActiveReaderLine(target)
}

func (m *Model) clampDocumentReader(place *navigation.State, document ui.ReaderDocument) {
	if document.Kind == ui.ReaderDocumentNone {
		place.ClampReader(0, m.geometry.ReaderRows.Height)
		return
	}
	place.ClampReaderSource(len(document.Rows))
	layout := ui.CalculateReaderLayout(m.geometry.ReaderRows, document)
	maximum := max(0, layout.Total-m.geometry.ReaderRows.Height)
	source, column := layout.SourceOffset(min(layout.VisualOffset(place.ReaderOffset, place.ReaderColumn), maximum))
	place.ReaderOffset = source
	place.ReaderColumn = column
	m.rememberActiveReaderLayout(place, document, layout)
}
