package notes

import (
	"strings"
	"testing"
)

func TestInsertionDeletionNewlineTabsAndSanitization(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	if !editor.Insert("hjkl\t界\r\ne\u0301\x1b") {
		t.Fatal("insert reported no edit")
	}
	if got, want := editor.Text(), "hjkl\t界\né␛"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if editor.Len() != 9 { // e + combining accent is one grapheme.
		t.Fatalf("grapheme length = %d", editor.Len())
	}
	editor.MoveHorizontal(-1, false)
	if !editor.Backspace() || strings.Contains(editor.Text(), "é") {
		t.Fatalf("backspace did not delete combining grapheme: %q", editor.Text())
	}
	editor.SelectAll()
	editor.MoveHorizontal(-1, false)
	if !editor.Delete() || strings.Contains(editor.Text(), "h") {
		t.Fatalf("forward delete = %q", editor.Text())
	}
}

func TestSelectionReplacementAndSelectAll(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Insert("alpha beta")
	editor.MoveHorizontal(-4, true)
	if got := editor.SelectedText(); got != "beta" {
		t.Fatalf("selected = %q", got)
	}
	editor.Insert("two")
	if got := editor.Text(); got != "alpha two" {
		t.Fatalf("replacement = %q", got)
	}
	editor.SelectAll()
	if got := editor.SelectedText(); got != "alpha two" {
		t.Fatalf("select all = %q", got)
	}
}

func TestWordMovementUndoRedoAndBound(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Insert("one  two!")
	editor.MoveWord(-1, false)
	if editor.Cursor() != 8 {
		t.Fatalf("word left over punctuation = %d", editor.Cursor())
	}
	editor.MoveWord(-1, false)
	if editor.Cursor() != 5 {
		t.Fatalf("word left over word = %d", editor.Cursor())
	}
	editor.Insert("new")
	if !editor.Undo() || editor.Text() != "one  two!" {
		t.Fatalf("undo = %q", editor.Text())
	}
	if !editor.Redo() || editor.Text() != "one  newtwo!" {
		t.Fatalf("redo = %q", editor.Text())
	}
	for range UndoLimit + 10 {
		editor.Insert("x")
	}
	for count := 0; count < UndoLimit; count++ {
		if !editor.Undo() {
			t.Fatalf("undo stopped at %d", count)
		}
	}
	if editor.Undo() {
		t.Fatal("undo history exceeded bound")
	}
}

func TestPasteIsSingleUndoableEdit(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Insert("before")
	editor.Insert("\nwide 界\nnext")
	if !editor.Undo() || editor.Text() != "before" {
		t.Fatalf("paste undo = %q", editor.Text())
	}
}

func TestWrapTabsWideCombiningAndClipping(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Load("a\t界e\u0301z")
	editor.Resize(5, 3)
	doc := editor.Document()
	if len(doc.Rows) != 2 {
		t.Fatalf("rows = %+v", doc.Rows)
	}
	if got := doc.Rows[0].Cells[1]; got.Column != 1 || got.Width != 3 || got.Display != "   " {
		t.Fatalf("tab cell = %+v", got)
	}
	if doc.Rows[1].Cells[0].Width != 2 || doc.Rows[1].Cells[1].Width != 1 {
		t.Fatalf("unicode widths = %+v", doc.Rows[1].Cells)
	}
	editor.Resize(1, 2)
	doc = editor.Document()
	for _, row := range doc.Rows {
		for _, cell := range row.Cells {
			if cell.Width > 1 || cell.Width < 1 {
				t.Fatalf("narrow cell = %+v", cell)
			}
		}
	}
}

func TestPreferredVisualColumnAndPageMovement(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Load("abcd\nx\nabcdef")
	editor.Resize(10, 2)
	editor.MoveEnd(false)
	editor.MoveVertical(1, false)
	if editor.Cursor() != 6 {
		t.Fatalf("short-line cursor = %d", editor.Cursor())
	}
	editor.MoveVertical(1, false)
	if editor.Cursor() != 11 { // preferred column 4 on the final line.
		t.Fatalf("preferred column cursor = %d", editor.Cursor())
	}
	editor.MovePage(-1, true)
	if _, _, ok := editor.Selection(); !ok {
		t.Fatal("shift page movement did not select")
	}
}

func TestClickDragScrollAndResizeContinuity(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Load("abcd efgh ijkl\nsecond")
	editor.Resize(5, 2)
	editor.BeginDrag(1, 0)
	editor.DragTo(3, 1)
	editor.EndDrag()
	start, end, ok := editor.Selection()
	if !ok || start != 1 || end != 8 {
		t.Fatalf("drag selection = %d..%d, %v", start, end, ok)
	}
	editor.Scroll(100)
	oldTop := editor.Document().Rows[editor.ScrollOffset()].Start
	cursor := editor.Cursor()
	editor.Resize(8, 2)
	if editor.Cursor() != cursor {
		t.Fatalf("resize moved cursor: %d -> %d", cursor, editor.Cursor())
	}
	newTop := editor.Document().Rows[editor.ScrollOffset()].Start
	if newTop > oldTop {
		t.Fatalf("resize skipped past scroll anchor: %d -> %d", oldTop, newTop)
	}
}

func TestDocumentPointUsesWideCellMidpointAndClamps(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Load("a界b")
	editor.Resize(8, 1)
	doc := editor.Document()
	if got := doc.Point(1, 0, 0, 1); got != 1 {
		t.Fatalf("wide left edge = %d", got)
	}
	if got := doc.Point(2, 0, 0, 1); got != 2 {
		t.Fatalf("wide right edge = %d", got)
	}
	if got := doc.Point(99, 99, 0, 1); got != editor.Len() {
		t.Fatalf("clamped point = %d", got)
	}
}

func TestReconcilePreservesPlaceByNearestGrapheme(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Load(strings.Repeat("abcdef\n", 10))
	editor.Resize(4, 3)
	editor.MoveVertical(8, false)
	editor.MoveHorizontal(2, true)
	editor.Scroll(2)
	cursor := editor.Cursor()
	start, end, selected := editor.Selection()
	top := editor.Document().Rows[editor.ScrollOffset()].Start
	editor.Reconcile(editor.Text())
	gotStart, gotEnd, gotSelected := editor.Selection()
	if editor.Cursor() != cursor || gotStart != start || gotEnd != end || gotSelected != selected {
		t.Fatalf("reconcile place = cursor %d selection %d..%d/%v", editor.Cursor(), gotStart, gotEnd, gotSelected)
	}
	if gotTop := editor.Document().Rows[editor.ScrollOffset()].Start; gotTop != top {
		t.Fatalf("reconcile top = %d, want %d", gotTop, top)
	}
	editor.Reconcile("short")
	if editor.Cursor() > editor.Len() {
		t.Fatalf("short reconcile cursor = %d len %d", editor.Cursor(), editor.Len())
	}
}

func TestCursorLineColumnUsesLogicalTabAndWideWidths(t *testing.T) {
	t.Parallel()
	editor := NewEditor()
	editor.Resize(80, 4)
	editor.Insert("a\t界\nnext")
	line, column := editor.CursorLineColumn()
	if line != 2 || column != 5 {
		t.Fatalf("cursor position = %d:%d, want 2:5", line, column)
	}
	editor.Undo()
	editor.Insert("a\t界")
	line, column = editor.CursorLineColumn()
	if line != 1 || column != 7 {
		t.Fatalf("wide/tab cursor position = %d:%d, want 1:7", line, column)
	}
}
