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
	readerRevision  uint64
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

func (m Model) activeReaderRowsRect() ui.Rect {
	if m.active == workspace.Git && m.gitDiffVisible() {
		return m.gitGeometry.ContentRows
	}
	return m.geometry.ReaderRows
}

func (m *Model) readerRowsForPlace(place *navigation.State) ui.Rect {
	if place == &m.history.inspection.place || place == &m.stashes.inspection.place {
		return m.gitGeometry.ContentRows
	}
	return m.geometry.ReaderRows
}

func (m Model) activeReaderViewportKey() (readerViewportKey, bool) {
	key := readerViewportKey{workspace: m.active, rows: m.activeReaderRowsRect()}
	switch {
	case m.active == workspace.Git && m.controls.Git == workspace.GitHistory && m.history.inspecting:
		key.source = m.history.inspection.readerPresentation
		key.contextRevision = m.history.inspection.readerContext.revision
	case m.gitStashesActive():
		key.source = m.stashes.inspection.readerPresentation
		key.contextRevision = m.stashes.inspection.readerContext.revision
	case m.active == workspace.Files:
		key.source = m.files.activeReaderPresentation()
		key.contextRevision = m.files.readerContext.revision
		key.readerRevision = m.files.readerRevision
	default:
		return readerViewportKey{}, false
	}
	// Completed loads always own an immutable presentation. Loading and empty
	// states are cheap and deliberately bypass the cache.
	return key, key.source != nil
}

func (m Model) activeReaderDocument() (ui.ReaderDocument, bool) {
	switch {
	case m.active == workspace.Git && m.controls.Git == workspace.GitHistory && m.history.inspecting:
		return m.history.inspection.readerDocument(), true
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
		layout:   ui.CalculateReaderLayout(m.activeReaderRowsRect(), document),
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
	if m.active == workspace.Git && m.history.inspecting {
		return len(m.history.inspection.readerRows())
	}
	if m.active == workspace.Git {
		return 0
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
	rows := m.activeReaderRowsRect()
	if layout, ok := m.activeReaderLayout(); ok {
		maximum := max(0, layout.Total-rows.Height)
		source, column := layout.SourceOffset(min(max(offset, 0), maximum))
		place.ReaderOffset = source
		place.ReaderColumn = column
		return
	}
	place.ReaderOffset = min(max(offset, 0), max(0, m.activeReaderLineCount()-rows.Height))
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
	m.selectActiveReaderLine(target)
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
	rows := m.activeReaderRowsRect()
	layout, ok := m.activeReaderLayout()
	if !ok || layout.Total == 0 || rows.Height <= 0 {
		return
	}
	top := m.activeReaderVisualOffset()
	bottom := min(layout.Total-1, top+rows.Height-1)
	targetVisual := top + (bottom-top)/2
	if position < 0 {
		targetVisual = top
	} else if position > 0 {
		targetVisual = bottom
	}
	target, _ := layout.SourceOffset(targetVisual)
	m.selectActiveReaderLine(target)
}

func (m *Model) selectActiveReaderLine(index int) {
	document, ok := m.activeReaderDocument()
	if !ok || len(document.Rows) == 0 {
		return
	}
	place := m.activePlace()
	index = max(0, min(index, len(document.Rows)-1))
	if m.active == workspace.Files && m.files.visualSelection != nil {
		m.selectFilesVisualLine(document, index)
		return
	}
	place.ReaderCursor = selectableReaderTarget(document, place.ReaderCursor, index)
	m.ensureActiveReaderSelectionVisible()
}

func selectableReaderTarget(document ui.ReaderDocument, current, target int) int {
	if len(document.Rows) == 0 {
		return 0
	}
	current = max(0, min(current, len(document.Rows)-1))
	target = max(0, min(target, len(document.Rows)-1))
	if document.Rows[target].Selectable() {
		return target
	}
	direction := 1
	if target < current {
		direction = -1
	}
	for index := target; index >= 0 && index < len(document.Rows); index += direction {
		if document.Rows[index].Selectable() {
			return index
		}
	}
	return document.SelectionTarget(target)
}

// selectFilesVisualLine moves only the active source endpoint. A diff-side
// selection fixes its side from the anchor; presentation rows are skipped and
// motion stops before the first real source row that lacks that side. This is
// the intentionally simple mixed deletion/addition boundary rule.
func (m *Model) selectFilesVisualLine(document ui.ReaderDocument, target int) {
	selection := m.files.visualSelection
	if selection == nil || len(document.Rows) == 0 {
		return
	}
	current, ok := findReaderSource(document, selection.Side, selection.Active)
	if !ok {
		return
	}
	target = max(0, min(target, len(document.Rows)-1))
	direction := 1
	if target < current {
		direction = -1
	} else if target == current {
		m.files.place.ReaderCursor = current
		return
	}
	last := current
	advance := func(index int) (stop bool) {
		row := document.Rows[index]
		if !row.Commentable() {
			return false
		}
		_, line, compatible := readerSourceAt(document, index, &selection.Side)
		if !compatible {
			return true
		}
		last = index
		selection.Active = line
		return false
	}
	for index := current + direction; ; index += direction {
		if index < 0 || index >= len(document.Rows) || advance(index) {
			break
		}
		if index == target {
			break
		}
	}
	// A hunk target or inline card is presentation-only. When no source row was
	// reached, continue to the nearest source in the requested direction.
	if last == current && !document.Rows[target].Commentable() {
		for index := target + direction; index >= 0 && index < len(document.Rows); index += direction {
			if advance(index) || document.Rows[index].Commentable() {
				break
			}
		}
	}
	m.files.place.ReaderCursor = last
	m.files.readerRevision++
	m.ensureActiveReaderSelectionVisible()
}

func (m *Model) ensureActiveReaderSelectionVisible() {
	rows := m.activeReaderRowsRect()
	layout, ok := m.activeReaderLayout()
	if !ok || layout.Total == 0 || rows.Height <= 0 {
		return
	}
	cursor := layout.VisualOffset(m.activePlace().ReaderCursor, 0)
	top := m.activeReaderVisualOffset()
	bottom := top + rows.Height
	switch {
	case cursor < top:
		m.setActiveReaderVisualOffset(cursor)
	case cursor >= bottom:
		m.setActiveReaderVisualOffset(cursor - rows.Height + 1)
	}
}

func (m *Model) selectActiveReaderLandmark(delta int) {
	document, ok := m.activeReaderDocument()
	if !ok || delta == 0 {
		return
	}
	targets := m.settings.hunkNavigationTargets(readerNavigationLandmarks(document))
	if m.active == workspace.Files && m.files.visualSelection != nil {
		targets = visualHunkTargets(document)
	}
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

func visualHunkTargets(document ui.ReaderDocument) []int {
	starts := document.HunkStarts()
	targets := make([]int, 0, len(starts))
	for index, start := range starts {
		end := len(document.Rows)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for row := start; row < end; row++ {
			if document.Rows[row].Commentable() {
				targets = append(targets, row)
				break
			}
		}
	}
	return targets
}

func (m *Model) clampDocumentReader(place *navigation.State, document ui.ReaderDocument) {
	rows := m.readerRowsForPlace(place)
	if document.Kind == ui.ReaderDocumentNone {
		place.ClampReader(0, rows.Height)
		return
	}
	place.ClampReaderSource(len(document.Rows))
	layout := ui.CalculateReaderLayout(rows, document)
	maximum := max(0, layout.Total-rows.Height)
	source, column := layout.SourceOffset(min(layout.VisualOffset(place.ReaderOffset, place.ReaderColumn), maximum))
	place.ReaderOffset = source
	place.ReaderColumn = column
	m.rememberActiveReaderLayout(place, document, layout)
}
