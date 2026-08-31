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
	ExpandReaderFold
	CollapseReaderFold
	ToggleReaderFold
	SelectNextLandmark
	SelectPreviousLandmark
	MoveReaderSelection
	MoveReaderPage
	SelectReaderBoundary
	SelectReaderViewport
	SelectReaderLine
	StartVisualLine
	StartCharacterSelection
	DragCharacterSelection
	FinishCharacterSelection
	CancelVisualLine
	CopyVisualSelection
	ComposeComment
	ComposeCommentAtLine
	SetCommentHover
	ClearCommentHover
	FocusNavigator
	FocusReader
	FocusGitRegion
	EnterGit
	BackGit
	ActivateGitRow
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
	CommentInsert
	CommentBackspace
	CommentDelete
	CommentMoveLeft
	CommentMoveRight
	CommentMoveUp
	CommentMoveDown
	CommentMoveWordLeft
	CommentMoveWordRight
	CommentMoveHome
	CommentMoveEnd
	CommentSubmit
	CommentCancel
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
	Position   int
	Pane       navigation.Focus
	GitFocus   workspace.GitFocus
	GitDivider ui.GitDividerKind
	Grab       int
	Text       string
	X          int
	Y          int
	Selecting  bool
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
	visualSelecting   bool
	readerCommentable bool
	readerFoldable    bool
	readerLandmarks   bool
}

func routeCommentInput(msg tea.Msg) (Action, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.PasteMsg:
		return Action{Kind: CommentInsert, Text: msg.Content}, true
	case tea.KeyPressMsg:
		key := msg.Key()
		selecting := key.Mod&tea.ModShift != 0
		if key.Code == tea.KeyEscape {
			return Action{Kind: CommentCancel}, true
		}
		if key.Code == tea.KeyEnter {
			if key.Mod&(tea.ModAlt|tea.ModShift) != 0 {
				return Action{Kind: CommentInsert, Text: "\n"}, true
			}
			return Action{Kind: CommentSubmit}, true
		}
		if key.Mod&tea.ModCtrl != 0 {
			switch key.Code {
			case 'j':
				return Action{Kind: CommentInsert, Text: "\n"}, true
			case tea.KeyLeft:
				return Action{Kind: CommentMoveWordLeft, Selecting: selecting}, true
			case tea.KeyRight:
				return Action{Kind: CommentMoveWordRight, Selecting: selecting}, true
			}
		}
		switch key.Code {
		case tea.KeyLeft:
			return Action{Kind: CommentMoveLeft, Selecting: selecting}, true
		case tea.KeyRight:
			return Action{Kind: CommentMoveRight, Selecting: selecting}, true
		case tea.KeyUp:
			return Action{Kind: CommentMoveUp, Selecting: selecting}, true
		case tea.KeyDown:
			return Action{Kind: CommentMoveDown, Selecting: selecting}, true
		case tea.KeyHome:
			return Action{Kind: CommentMoveHome, Selecting: selecting}, true
		case tea.KeyEnd:
			return Action{Kind: CommentMoveEnd, Selecting: selecting}, true
		case tea.KeyBackspace:
			return Action{Kind: CommentBackspace}, true
		case tea.KeyDelete:
			return Action{Kind: CommentDelete}, true
		}
		if key.Text != "" && key.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) == 0 {
			return Action{Kind: CommentInsert, Text: key.Text}, true
		}
	}
	return Action{}, false
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
	if action, ok := routeInactivePaneNavigationKey(msg, context); ok {
		return action, true
	}
	if msg.Key().Mod&(tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) != 0 {
		return Action{}, false
	}
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

// routeInactivePaneNavigationKey applies ordinary Files navigation to the
// other pane without transferring focus. Alt is intentionally exact: adding
// Shift or another modifier must not steal selection or terminal shortcuts.
func routeInactivePaneNavigationKey(msg tea.KeyPressMsg, context browserRouteContext) (Action, bool) {
	if context.active != workspace.Files || context.visualSelecting {
		return Action{}, false
	}
	key := msg.Key()
	if key.Mod != tea.ModAlt {
		return Action{}, false
	}
	switch key.Code {
	case 'j', tea.KeyDown:
		if context.focus == navigation.FocusNavigator {
			return Action{Kind: MoveReaderSelection, Amount: 1}, true
		}
		return Action{Kind: SelectNext}, true
	case 'k', tea.KeyUp:
		if context.focus == navigation.FocusNavigator {
			return Action{Kind: MoveReaderSelection, Amount: -1}, true
		}
		return Action{Kind: SelectPrevious}, true
	case 'l', tea.KeyRight:
		if context.focus == navigation.FocusReader {
			return Action{Kind: ExpandNavigatorSelection}, true
		}
		if context.controls.RichDiff || context.readerFoldable {
			return Action{Kind: ExpandReaderFold}, true
		}
	case 'h', tea.KeyLeft:
		if context.focus == navigation.FocusReader {
			return Action{Kind: CollapseNavigatorSelection}, true
		}
		if context.controls.RichDiff || context.readerFoldable {
			return Action{Kind: CollapseReaderFold}, true
		}
	}
	return Action{}, false
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
	case "V":
		if context.active == workspace.Files && context.focus == navigation.FocusReader && context.readerCommentable && !context.visualSelecting {
			return Action{Kind: StartVisualLine}, true
		}
	case "c":
		if context.active == workspace.Files && context.focus == navigation.FocusReader && context.readerCommentable {
			return Action{Kind: ComposeComment}, true
		}
	case "y":
		if context.active == workspace.Files && context.visualSelecting {
			return Action{Kind: CopyVisualSelection}, true
		}
	case "esc":
		if context.active == workspace.Files && context.visualSelecting {
			return Action{Kind: CancelVisualLine}, true
		}
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
		if context.controls.RichDiff || context.readerLandmarks {
			return Action{Kind: SelectNextLandmark}, true
		}
	case "[":
		if context.controls.RichDiff || context.readerLandmarks {
			return Action{Kind: SelectPreviousLandmark}, true
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
		if context.focus == navigation.FocusReader && (context.controls.RichDiff || context.readerFoldable) {
			return Action{Kind: ExpandReaderFold}, true
		}
	case "h", "left":
		if context.active == workspace.Files && context.focus == navigation.FocusNavigator {
			return Action{Kind: CollapseNavigatorSelection}, true
		}
		if context.focus == navigation.FocusReader && (context.controls.RichDiff || context.readerFoldable) {
			return Action{Kind: CollapseReaderFold}, true
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
	if m.files.composingComment() {
		return routeCommentInput(msg)
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
	if m.active == workspace.Git {
		return m.routeGitMessage(msg)
	}
	place := m.activePlace()
	readerOffset := place.ReaderOffset
	readerLineCount := 0
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		if m.active == workspace.Files && !m.layout.dragging && !m.scrollbar.active {
			mouse := msg.Mouse()
			if layout, ok := m.activeReaderLayout(); ok {
				if m.files.readerSelectionDragging && mouse.Button == tea.MouseLeft {
					if point, hit := layout.ClampedCodePointAt(mouse.X, mouse.Y, m.activeReaderVisualOffset()); hit {
						return Action{Kind: DragCharacterSelection, Index: point.Source, Position: point.Column}, true
					}
				}
				if source, hit := layout.CommentGutterSourceAt(mouse.X, mouse.Y, m.activeReaderVisualOffset()); hit {
					if !m.files.hoveredCommentLine(source) {
						return Action{Kind: SetCommentHover, Index: source}, true
					}
				} else if m.files.commentHover != nil {
					return Action{Kind: ClearCommentHover}, true
				}
			}
		}
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if m.geometry.ReaderRows.Contains(mouse.X, mouse.Y) {
			readerOffset = m.activeReaderVisualOffset()
			readerLineCount = m.activeReaderLineCount()
			if mouse.Button == tea.MouseLeft {
				if layout, ok := m.activeReaderLayout(); ok {
					if m.active == workspace.Files {
						if source, hit := layout.CommentGutterSourceAt(mouse.X, mouse.Y, readerOffset); hit {
							return Action{Kind: ComposeCommentAtLine, Index: source}, true
						}
					}
					if source, hit := layout.SourceAt(mouse.X, mouse.Y, readerOffset); hit {
						row, _ := layout.Row(readerOffset + mouse.Y - layout.Geometry.Rows.Y)
						if identity, fold := row.CommentHeaderIdentity(); fold {
							return Action{Kind: ToggleReaderFold, Identity: identity, Index: source}, true
						}
						if identity, fold := row.ContextFoldIdentity(); fold {
							return Action{Kind: ToggleReaderFold, Identity: identity, Index: source}, true
						}
						if m.active == workspace.Files {
							if point, code := layout.CodePointAt(mouse.X, mouse.Y, readerOffset); code && documentRowCommentable(layout, point.Source) {
								return Action{Kind: StartCharacterSelection, Index: point.Source, Position: point.Column}, true
							}
						}
						return Action{Kind: SelectReaderLine, Index: source}, true
					}
				}
			}
		}
	case tea.MouseReleaseMsg:
		if m.active == workspace.Files && m.files.readerSelectionDragging {
			return Action{Kind: FinishCharacterSelection}, true
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if m.geometry.ReaderRows.Contains(mouse.X, mouse.Y) {
			readerOffset = m.activeReaderVisualOffset()
			readerLineCount = m.activeReaderLineCount()
		}
	}
	document, structured := m.activeReaderDocument()
	readerCommentable := false
	readerFoldable := false
	readerLandmarks := false
	if structured && len(document.Rows) != 0 {
		cursor := max(0, min(place.ReaderCursor, len(document.Rows)-1))
		readerCommentable = m.active == workspace.Files && document.Rows[cursor].Commentable() && !m.files.markdownPreviewActive()
		_, readerFoldable = document.Rows[cursor].CommentHeaderIdentity()
		readerLandmarks = len(m.settings.hunkNavigationTargets(readerNavigationLandmarks(document))) != 0
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
		visualSelecting:   m.files.visualSelection != nil,
		readerCommentable: readerCommentable,
		readerFoldable:    readerFoldable,
		readerLandmarks:   readerLandmarks,
	})
}

func documentRowCommentable(layout ui.ReaderLayout, source int) bool {
	row, rowSource, _ := layout.RowWithSource(layout.VisualOffset(source, 0))
	return rowSource == source && row.Commentable()
}
