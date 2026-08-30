package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// ActionKind is a semantic user intent, independent of terminal input syntax.
type ActionKind uint8

const (
	ActionNone ActionKind = iota
	ShowFiles
	ShowGit
	ShowNotes
	ToggleNotesScope
	SelectProjectNotes
	SelectWorktreeNotes
	ToggleSecondary
	ToggleTertiary
	ToggleComparison
	ToggleDiffHighlight
	ToggleMarkdownPreview
	OpenEditor
	ToggleReview
	ToggleReviewBounds
	NextReviewGap
	ActivateReviewBadge
	ToggleSettings
	SelectNextSetting
	SelectPreviousSetting
	ToggleSelectedSetting
	ToggleHelp
	SelectNext
	SelectPrevious
	SelectIndex
	ActivateNavigatorRow
	SelectNextFile
	SelectPreviousFile
	ExpandNavigatorSelection
	CollapseNavigatorSelection
	ExpandReaderContext
	CollapseReaderContext
	ToggleReaderFold
	SelectNextHunk
	SelectPreviousHunk
	MoveReaderSelection
	MoveReaderPage
	SelectReaderBoundary
	SelectReaderViewport
	SelectReaderLine
	FocusNavigator
	FocusReader
	SwapPanes
	ScrollReader
	StartPaneResize
	ResizePanes
	FinishPaneResize
	StartScrollbarDrag
	DragScrollbar
	FinishScrollbarDrag
	Reload
	Resize
	Quit
	NotesInsert
	NotesBackspace
	NotesDelete
	NotesMoveLeft
	NotesMoveRight
	NotesMoveUp
	NotesMoveDown
	NotesMoveWordLeft
	NotesMoveWordRight
	NotesMoveHome
	NotesMoveEnd
	NotesPageUp
	NotesPageDown
	NotesSelectAll
	NotesUndo
	NotesRedo
	NotesBeginSelection
	NotesDragSelection
	NotesEndSelection
	NotesScroll
	StartNotesScrollbarDrag
	DragNotesScrollbar
	FinishNotesScrollbarDrag
)

// Action carries the small amount of data needed by a semantic intent.
type Action struct {
	Kind     ActionKind
	Identity string
	Index    int
	Amount   int
	Width    int
	Height   int
	// Position is an absolute terminal column for geometry actions.
	Position  int
	Pane      navigation.Focus
	Grab      int
	Text      string
	X         int
	Y         int
	Selecting bool
}

type notesRouteContext struct {
	geometry          ui.Geometry
	totalRows         int
	top               int
	selectionDragging bool
	scrollbarDragging bool
	hasWorktree       bool
}

func routeNotesInput(msg tea.Msg, context notesRouteContext) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return routeNotesKey(msg, context.hasWorktree)
	case tea.PasteMsg:
		return Action{Kind: NotesInsert, Text: msg.Content}, true
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		return routeNotesClick(msg, context)
	case tea.MouseWheelMsg:
		return routeNotesWheel(msg, context)
	case tea.MouseMotionMsg:
		return routeNotesMotion(msg, context)
	case tea.MouseReleaseMsg:
		if context.scrollbarDragging {
			return Action{Kind: FinishNotesScrollbarDrag}, true
		}
		if context.selectionDragging {
			return Action{Kind: NotesEndSelection}, true
		}
	}
	return Action{}, false
}

func routeNotesKey(msg tea.KeyPressMsg, hasWorktree bool) (Action, bool) {
	key := msg.Key()
	selecting := key.Mod&tea.ModShift != 0
	if key.Code == tea.KeyEscape {
		return Action{Kind: ShowFiles}, true
	}
	if key.Mod&tea.ModCtrl != 0 {
		return routeNotesControlKey(key, selecting, hasWorktree)
	}
	if action, ok := routeNotesEditorKey(key, selecting); ok {
		return action, true
	}
	if key.Text != "" && key.Mod&(tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) == 0 {
		return Action{Kind: NotesInsert, Text: key.Text}, true
	}
	return Action{}, false
}

func routeNotesEditorKey(key tea.Key, selecting bool) (Action, bool) {
	switch key.Code {
	case tea.KeyLeft:
		return Action{Kind: NotesMoveLeft, Selecting: selecting}, true
	case tea.KeyRight:
		return Action{Kind: NotesMoveRight, Selecting: selecting}, true
	case tea.KeyUp:
		return Action{Kind: NotesMoveUp, Selecting: selecting}, true
	case tea.KeyDown:
		return Action{Kind: NotesMoveDown, Selecting: selecting}, true
	case tea.KeyHome:
		return Action{Kind: NotesMoveHome, Selecting: selecting}, true
	case tea.KeyEnd:
		return Action{Kind: NotesMoveEnd, Selecting: selecting}, true
	case tea.KeyPgUp:
		return Action{Kind: NotesPageUp, Selecting: selecting}, true
	case tea.KeyPgDown:
		return Action{Kind: NotesPageDown, Selecting: selecting}, true
	case tea.KeyBackspace:
		return Action{Kind: NotesBackspace}, true
	case tea.KeyDelete:
		return Action{Kind: NotesDelete}, true
	case tea.KeyEnter:
		return Action{Kind: NotesInsert, Text: "\n"}, true
	case tea.KeyTab:
		if !selecting {
			return Action{Kind: NotesInsert, Text: "\t"}, true
		}
		return Action{}, false
	default:
		return Action{}, false
	}
}

func routeNotesControlKey(key tea.Key, selecting, hasWorktree bool) (Action, bool) {
	switch key.Code {
	case 'c':
		return Action{Kind: Quit}, true
	case 'a':
		return Action{Kind: NotesSelectAll}, true
	case 'z':
		if selecting {
			return Action{Kind: NotesRedo}, true
		}
		return Action{Kind: NotesUndo}, true
	case 'y':
		return Action{Kind: NotesRedo}, true
	case 't':
		if hasWorktree {
			return Action{Kind: ToggleNotesScope}, true
		}
	case tea.KeyLeft:
		return Action{Kind: NotesMoveWordLeft, Selecting: selecting}, true
	case tea.KeyRight:
		return Action{Kind: NotesMoveWordRight, Selecting: selecting}, true
	}
	return Action{}, false
}

func routeNotesClick(msg tea.MouseClickMsg, context notesRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return Action{}, false
	}
	hit := context.geometry.NotesHitTestWithScopes(
		mouse.X,
		mouse.Y,
		context.totalRows,
		context.top,
		context.hasWorktree,
	)
	switch hit.Kind {
	case ui.HitFilesWorkspace:
		return Action{Kind: ShowFiles}, true
	case ui.HitGitWorkspace:
		return Action{Kind: ShowGit}, true
	case ui.HitNotesWorkspace:
		return Action{Kind: ShowNotes}, true
	case ui.HitNotesProjectScope:
		return Action{Kind: SelectProjectNotes}, true
	case ui.HitNotesWorktreeScope:
		return Action{Kind: SelectWorktreeNotes}, true
	case ui.HitNotesScrollbar:
		return Action{Kind: StartNotesScrollbarDrag, Position: mouse.Y, Grab: hit.GrabOffset}, true
	case ui.HitNotesText:
		return Action{Kind: NotesBeginSelection, X: mouse.X - context.geometry.NotesText.X, Y: mouse.Y - context.geometry.NotesText.Y}, true
	default:
		return Action{}, false
	}
}

func routeNotesWheel(msg tea.MouseWheelMsg, context notesRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	hit := context.geometry.NotesHitTestWithScopes(
		mouse.X,
		mouse.Y,
		context.totalRows,
		context.top,
		context.hasWorktree,
	)
	if hit.Kind != ui.HitNotesText && hit.Kind != ui.HitNotesScrollbar {
		return Action{}, false
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		return Action{Kind: NotesScroll, Amount: -3}, true
	case tea.MouseWheelDown:
		return Action{Kind: NotesScroll, Amount: 3}, true
	default:
		return Action{}, false
	}
}

func routeNotesMotion(msg tea.MouseMotionMsg, context notesRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	if context.scrollbarDragging && mouse.Button == tea.MouseLeft {
		return Action{Kind: DragNotesScrollbar, Position: mouse.Y}, true
	}
	if context.selectionDragging && mouse.Button == tea.MouseLeft {
		return Action{Kind: NotesDragSelection, X: mouse.X - context.geometry.NotesText.X, Y: mouse.Y - context.geometry.NotesText.Y}, true
	}
	return Action{}, false
}

type browserRouteContext struct {
	focus             navigation.Focus
	geometry          ui.Geometry
	active            workspace.Kind
	controls          workspace.Controls
	dividerDragging   bool
	scrollbarDragging bool
	top               int
	navigatorCount    int
	readerOffset      int
	readerLineCount   int
	navigatorRows     []ui.NavigatorRow
}

func routeBrowserMessage(msg tea.Msg, context browserRouteContext) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return routeBrowserKey(msg, context)
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		return routeBrowserClick(msg, context)
	case tea.MouseWheelMsg:
		return routeBrowserWheel(msg, context)
	case tea.MouseMotionMsg:
		return routeBrowserMotion(msg, context)
	case tea.MouseReleaseMsg:
		return routeBrowserRelease(context)
	default:
		return Action{}, false
	}
}

func routeBrowserKey(msg tea.KeyPressMsg, context browserRouteContext) (Action, bool) {
	if action, ok := routeReaderJumpKey(msg, context); ok {
		return action, true
	}
	if action, ok := routeBrowserCommandKey(msg, context); ok {
		return action, true
	}
	if action, ok := routeBrowserReviewKey(msg, context.active); ok {
		return action, true
	}
	return routeBrowserNavigationKey(msg, context)
}

func routeReaderJumpKey(msg tea.KeyPressMsg, context browserRouteContext) (Action, bool) {
	if context.focus != navigation.FocusReader || !context.hasStructuredReader() {
		return Action{}, false
	}
	fullPage := max(1, context.geometry.ReaderRows.Height)
	switch msg.String() {
	case "home":
		return Action{Kind: SelectReaderBoundary, Amount: -1}, true
	case "end":
		return Action{Kind: SelectReaderBoundary, Amount: 1}, true
	case "H":
		return Action{Kind: SelectReaderViewport, Amount: -1}, true
	case "M":
		return Action{Kind: SelectReaderViewport}, true
	case "L":
		return Action{Kind: SelectReaderViewport, Amount: 1}, true
	case "pgup":
		return Action{Kind: MoveReaderPage, Amount: -fullPage}, true
	case "pgdown":
		return Action{Kind: MoveReaderPage, Amount: fullPage}, true
	default:
		return Action{}, false
	}
}

func (context browserRouteContext) hasStructuredReader() bool {
	return context.active == workspace.Files ||
		context.active == workspace.Git && context.controls.Git == workspace.GitStashes
}

func routeBrowserCommandKey(msg tea.KeyPressMsg, context browserRouteContext) (Action, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		return Action{Kind: Quit}, true
	case workspace.SecondaryControlKey:
		return Action{Kind: ToggleSecondary}, true
	case workspace.TertiaryControlKey:
		return Action{Kind: ToggleTertiary}, true
	case workspace.ComparisonControlKey:
		return Action{Kind: ToggleComparison}, true
	case workspace.DiffHighlightKey:
		if context.controls.RichDiff {
			return Action{Kind: ToggleDiffHighlight}, true
		}
	case "m":
		if context.active == workspace.Files && context.controls.MarkdownPreviewEligible {
			return Action{Kind: ToggleMarkdownPreview}, true
		}
	case "e":
		if context.active == workspace.Files {
			return Action{Kind: OpenEditor}, true
		}
	case "esc":
		if context.active == workspace.Git {
			return Action{Kind: ShowFiles}, true
		}
	case "g":
		return Action{Kind: ShowGit}, true
	case "n":
		return Action{Kind: ShowNotes}, true
	case "tab":
		if context.focus == navigation.FocusNavigator {
			return Action{Kind: FocusReader}, true
		}
		return Action{Kind: FocusNavigator}, true
	case "z":
		return Action{Kind: SwapPanes}, true
	case "r":
		return Action{Kind: Reload}, true
	default:
		return Action{}, false
	}
	return Action{}, false
}

func routeBrowserReviewKey(msg tea.KeyPressMsg, active workspace.Kind) (Action, bool) {
	if active != workspace.Files {
		return Action{}, false
	}
	switch msg.String() {
	case "x":
		return Action{Kind: ToggleReview, Index: -1}, true
	case "R":
		return Action{Kind: ToggleReviewBounds}, true
	case "X":
		return Action{Kind: NextReviewGap}, true
	default:
		return Action{}, false
	}
}

func routeBrowserNavigationKey(msg tea.KeyPressMsg, context browserRouteContext) (Action, bool) {
	switch msg.String() {
	case "f":
		return Action{Kind: SelectNextFile}, true
	case "F":
		return Action{Kind: SelectPreviousFile}, true
	case "]":
		if context.controls.RichDiff {
			return Action{Kind: SelectNextHunk}, true
		}
	case "[":
		if context.controls.RichDiff {
			return Action{Kind: SelectPreviousHunk}, true
		}
	case "j", "down":
		if context.focus == navigation.FocusNavigator {
			return Action{Kind: SelectNext}, true
		}
		return Action{Kind: MoveReaderSelection, Amount: 1}, true
	case "k", "up":
		if context.focus == navigation.FocusNavigator {
			return Action{Kind: SelectPrevious}, true
		}
		return Action{Kind: MoveReaderSelection, Amount: -1}, true
	case "l", "right":
		if context.active == workspace.Files && context.focus == navigation.FocusNavigator {
			return Action{Kind: ExpandNavigatorSelection}, true
		}
		if context.focus == navigation.FocusReader && context.controls.RichDiff {
			return Action{Kind: ExpandReaderContext}, true
		}
	case "h", "left":
		if context.active == workspace.Files && context.focus == navigation.FocusNavigator {
			return Action{Kind: CollapseNavigatorSelection}, true
		}
		if context.focus == navigation.FocusReader && context.controls.RichDiff {
			return Action{Kind: CollapseReaderContext}, true
		}
	default:
		return Action{}, false
	}
	return Action{}, false
}

func routeBrowserClick(msg tea.MouseClickMsg, context browserRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return Action{}, false
	}
	if context.active == workspace.Files {
		if index, ok := context.geometry.HitNavigatorReview(mouse.X, mouse.Y, context.top, context.navigatorRows); ok {
			return Action{Kind: ActivateReviewBadge, Index: index}, true
		}
	}
	hit := context.hit(mouse.X, mouse.Y)
	switch hit.Kind {
	case ui.HitFilesWorkspace:
		return Action{Kind: ShowFiles}, true
	case ui.HitGitWorkspace:
		return Action{Kind: ShowGit}, true
	case ui.HitNotesWorkspace:
		return Action{Kind: ShowNotes}, true
	case ui.HitSecondaryControl:
		return Action{Kind: ToggleSecondary}, true
	case ui.HitTertiaryControl:
		return Action{Kind: ToggleTertiary}, true
	case ui.HitComparisonControl:
		return Action{Kind: ToggleComparison}, true
	case ui.HitDiffHighlightControl:
		return Action{Kind: ToggleDiffHighlight}, true
	case ui.HitDivider:
		return Action{Kind: StartPaneResize}, true
	case ui.HitNavigatorScrollbar:
		return Action{Kind: StartScrollbarDrag, Pane: navigation.FocusNavigator, Position: mouse.Y, Grab: hit.GrabOffset}, true
	case ui.HitReaderScrollbar:
		return Action{Kind: StartScrollbarDrag, Pane: navigation.FocusReader, Position: mouse.Y, Grab: hit.GrabOffset}, true
	case ui.HitNavigatorRow:
		return Action{Kind: ActivateNavigatorRow, Index: hit.Index}, true
	case ui.HitNavigator:
		return Action{Kind: FocusNavigator}, true
	case ui.HitReader:
		return Action{Kind: FocusReader}, true
	default:
		return Action{}, false
	}
}

func routeBrowserWheel(msg tea.MouseWheelMsg, context browserRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	hit := context.hit(mouse.X, mouse.Y)
	if ignoresBrowserWheel(hit.Kind) {
		return Action{}, false
	}
	direction := 0
	switch mouse.Button {
	case tea.MouseWheelUp:
		direction = -1
	case tea.MouseWheelDown:
		direction = 1
	default:
		return Action{}, false
	}
	if hit.Kind == ui.HitNavigator || hit.Kind == ui.HitNavigatorRow || hit.Kind == ui.HitNavigatorScrollbar {
		if direction < 0 {
			return Action{Kind: SelectPrevious}, true
		}
		return Action{Kind: SelectNext}, true
	}
	return Action{Kind: ScrollReader, Amount: direction * 3}, true
}

func routeBrowserMotion(msg tea.MouseMotionMsg, context browserRouteContext) (Action, bool) {
	mouse := msg.Mouse()
	if context.dividerDragging && mouse.Button == tea.MouseLeft {
		return Action{Kind: ResizePanes, Position: mouse.X}, true
	}
	if context.scrollbarDragging && mouse.Button == tea.MouseLeft {
		return Action{Kind: DragScrollbar, Position: mouse.Y}, true
	}
	return Action{}, false
}

func routeBrowserRelease(context browserRouteContext) (Action, bool) {
	if context.dividerDragging {
		return Action{Kind: FinishPaneResize}, true
	}
	if context.scrollbarDragging {
		return Action{Kind: FinishScrollbarDrag}, true
	}
	return Action{}, false
}

func (context browserRouteContext) hit(x, y int) ui.Hit {
	return context.geometry.HitTest(
		x,
		y,
		context.active,
		context.controls,
		context.top,
		context.navigatorCount,
		context.readerOffset,
		context.readerLineCount,
	)
}

func ignoresBrowserWheel(kind ui.HitKind) bool {
	switch kind {
	case ui.HitNone,
		ui.HitFilesWorkspace,
		ui.HitGitWorkspace,
		ui.HitNotesWorkspace,
		ui.HitDivider,
		ui.HitSecondaryControl,
		ui.HitTertiaryControl,
		ui.HitComparisonControl,
		ui.HitDiffHighlightControl:
		return true
	default:
		return false
	}
}

func (m *Model) route(msg tea.Msg) (Action, bool) {
	if action, handled := routeModalInput(msg, m.geometry, m.modal, m.active != workspace.Notes); handled {
		return action, true
	}
	if m.active == workspace.Notes {
		note := m.note.current()
		presentation := note.presentation()
		return routeNotesInput(msg, notesRouteContext{
			geometry:          m.geometry,
			totalRows:         len(presentation.Document.Rows),
			top:               presentation.Top,
			selectionDragging: note.editor.Dragging(),
			scrollbarDragging: note.scrollbarDragging,
			hasWorktree:       m.note.hasWorktree(),
		})
	}
	place := m.activePlace()
	readerOffset := place.ReaderOffset
	readerLineCount := 0
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if m.geometry.ReaderRows.Contains(mouse.X, mouse.Y) {
			readerOffset = m.activeReaderVisualOffset()
			readerLineCount = m.activeReaderLineCount()
			if mouse.Button == tea.MouseLeft {
				if layout, ok := m.activeReaderLayout(); ok {
					if source, hit := layout.SourceAt(mouse.X, mouse.Y, readerOffset); hit {
						row, _ := layout.Row(readerOffset + mouse.Y - layout.Geometry.Rows.Y)
						if identity, fold := row.ContextFoldIdentity(); fold {
							return Action{Kind: ToggleReaderFold, Identity: identity, Index: source}, true
						}
						return Action{Kind: SelectReaderLine, Index: source}, true
					}
				}
			}
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if m.geometry.ReaderRows.Contains(mouse.X, mouse.Y) {
			readerOffset = m.activeReaderVisualOffset()
			readerLineCount = m.activeReaderLineCount()
		}
	}
	var navigatorRows []ui.NavigatorRow
	if click, ok := msg.(tea.MouseClickMsg); ok && m.active == workspace.Files {
		mouse := click.Mouse()
		if m.geometry.NavigatorRows.Contains(mouse.X, mouse.Y) {
			navigatorRows = m.files.navigatorRows()
		}
	}
	return routeBrowserMessage(msg, browserRouteContext{
		focus:             place.Focus,
		geometry:          m.geometry,
		active:            m.active,
		controls:          m.presentationControls(),
		dividerDragging:   m.layout.dragging,
		scrollbarDragging: m.scrollbar.active,
		top:               place.Top,
		navigatorCount:    len(place.Items),
		readerOffset:      readerOffset,
		readerLineCount:   readerLineCount,
		navigatorRows:     navigatorRows,
	})
}
