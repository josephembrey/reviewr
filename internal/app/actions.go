package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/notes"
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
	CycleDestination
	ToggleNotesScope
	SelectProjectNotes
	SelectWorktreeNotes
	ToggleSecondary
	ToggleTertiary
	ToggleComparison
	ToggleDiffHighlight
	ToggleReview
	ToggleReviewBounds
	NextReviewGap
	ActivateReviewBadge
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
	Kind   ActionKind
	Index  int
	Amount int
	Width  int
	Height int
	// Position is an absolute terminal column for geometry actions.
	Position  int
	Pane      navigation.Focus
	Grab      int
	Text      string
	X         int
	Y         int
	Selecting bool
}

func routeNotesMessage(msg tea.Msg, geometry ui.Geometry, presentation notes.Presentation, selectionDragging, scrollbarDragging bool, scoped ...bool) (Action, bool) {
	hasWorktree := len(scoped) > 0 && scoped[0]
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		selecting := key.Mod&tea.ModShift != 0
		if key.Code == tea.KeyEscape {
			return Action{Kind: ShowFiles}, true
		}
		if key.Mod&tea.ModCtrl != 0 {
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
			return Action{Kind: CycleDestination}, true
		}
		if key.Text != "" && key.Mod&(tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) == 0 {
			return Action{Kind: NotesInsert, Text: key.Text}, true
		}
	case tea.PasteMsg:
		return Action{Kind: NotesInsert, Text: msg.Content}, true
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft {
			return Action{}, false
		}
		hit := geometry.NotesHitTestWithScopes(mouse.X, mouse.Y, len(presentation.Document.Rows), presentation.Top, hasWorktree)
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
			return Action{Kind: NotesBeginSelection, X: mouse.X - geometry.NotesText.X, Y: mouse.Y - geometry.NotesText.Y}, true
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		hit := geometry.NotesHitTestWithScopes(mouse.X, mouse.Y, len(presentation.Document.Rows), presentation.Top, hasWorktree)
		if hit.Kind != ui.HitNotesText && hit.Kind != ui.HitNotesScrollbar {
			return Action{}, false
		}
		switch mouse.Button {
		case tea.MouseWheelUp:
			return Action{Kind: NotesScroll, Amount: -3}, true
		case tea.MouseWheelDown:
			return Action{Kind: NotesScroll, Amount: 3}, true
		}
	case tea.MouseMotionMsg:
		mouse := msg.Mouse()
		if scrollbarDragging && mouse.Button == tea.MouseLeft {
			return Action{Kind: DragNotesScrollbar, Position: mouse.Y}, true
		}
		if selectionDragging && mouse.Button == tea.MouseLeft {
			return Action{Kind: NotesDragSelection, X: mouse.X - geometry.NotesText.X, Y: mouse.Y - geometry.NotesText.Y}, true
		}
	case tea.MouseReleaseMsg:
		if scrollbarDragging {
			return Action{Kind: FinishNotesScrollbarDrag}, true
		}
		if selectionDragging {
			return Action{Kind: NotesEndSelection}, true
		}
	}
	return Action{}, false
}

func routeMessage(msg tea.Msg, focus navigation.Focus, geometry ui.Geometry, active workspace.Kind, controls workspace.Controls, dividerDragging, scrollbarDragging bool, top, fileCount, readerOffset, readerLineCount int) (Action, bool) {
	return routeMessageWithRows(msg, focus, geometry, active, controls, dividerDragging, scrollbarDragging, top, fileCount, readerOffset, readerLineCount, nil)
}

func routeMessageWithRows(msg tea.Msg, focus navigation.Focus, geometry ui.Geometry, active workspace.Kind, controls workspace.Controls, dividerDragging, scrollbarDragging bool, top, fileCount, readerOffset, readerLineCount int, rows []ui.NavigatorRow) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return Action{Kind: Quit}, true
		case "1":
			return Action{Kind: ShowFiles}, true
		case "2":
			return Action{Kind: ShowGit}, true
		case "3":
			return Action{Kind: ShowNotes}, true
		case workspace.SecondaryControlKey:
			return Action{Kind: ToggleSecondary}, true
		case workspace.TertiaryControlKey:
			return Action{Kind: ToggleTertiary}, true
		case workspace.ComparisonControlKey:
			return Action{Kind: ToggleComparison}, true
		case workspace.DiffHighlightKey:
			if controls.RichDiff {
				return Action{Kind: ToggleDiffHighlight}, true
			}
		case "esc":
			if active == workspace.Git {
				return Action{Kind: ShowFiles}, true
			}
		case "x":
			if active == workspace.Files {
				return Action{Kind: ToggleReview, Index: -1}, true
			}
		case "R":
			if active == workspace.Files {
				return Action{Kind: ToggleReviewBounds}, true
			}
		case "X":
			if active == workspace.Files {
				return Action{Kind: NextReviewGap}, true
			}
		case "tab":
			return Action{Kind: CycleDestination}, true
		case "z":
			return Action{Kind: SwapPanes}, true
		case "r":
			return Action{Kind: Reload}, true
		case "f":
			return Action{Kind: SelectNextFile}, true
		case "F":
			return Action{Kind: SelectPreviousFile}, true
		case "j", "down":
			if focus == navigation.FocusNavigator {
				return Action{Kind: SelectNext}, true
			}
			return Action{Kind: ScrollReader, Amount: 1}, true
		case "k", "up":
			if focus == navigation.FocusNavigator {
				return Action{Kind: SelectPrevious}, true
			}
			return Action{Kind: ScrollReader, Amount: -1}, true
		case "l", "right":
			if active == workspace.Files && focus == navigation.FocusNavigator {
				return Action{Kind: ExpandNavigatorSelection}, true
			}
			if focus == navigation.FocusReader && controls.RichDiff {
				return Action{Kind: ExpandReaderContext}, true
			}
		case "h", "left":
			if active == workspace.Files && focus == navigation.FocusNavigator {
				return Action{Kind: CollapseNavigatorSelection}, true
			}
			if focus == navigation.FocusReader && controls.RichDiff {
				return Action{Kind: CollapseReaderContext}, true
			}
		}
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft {
			return Action{}, false
		}
		if active == workspace.Files {
			if index, ok := geometry.HitNavigatorReview(mouse.X, mouse.Y, top, rows); ok {
				return Action{Kind: ActivateReviewBadge, Index: index}, true
			}
		}
		switch hit := geometry.HitTest(mouse.X, mouse.Y, active, controls, top, fileCount, readerOffset, readerLineCount); hit.Kind {
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
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		hit := geometry.HitTest(mouse.X, mouse.Y, active, controls, top, fileCount, readerOffset, readerLineCount)
		if hit.Kind == ui.HitNone || hit.Kind == ui.HitFilesWorkspace || hit.Kind == ui.HitGitWorkspace || hit.Kind == ui.HitNotesWorkspace || hit.Kind == ui.HitDivider ||
			hit.Kind == ui.HitSecondaryControl || hit.Kind == ui.HitTertiaryControl || hit.Kind == ui.HitComparisonControl || hit.Kind == ui.HitDiffHighlightControl {
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
	case tea.MouseMotionMsg:
		mouse := msg.Mouse()
		if dividerDragging && mouse.Button == tea.MouseLeft {
			return Action{Kind: ResizePanes, Position: mouse.X}, true
		}
		if scrollbarDragging && mouse.Button == tea.MouseLeft {
			return Action{Kind: DragScrollbar, Position: mouse.Y}, true
		}
	case tea.MouseReleaseMsg:
		if dividerDragging {
			return Action{Kind: FinishPaneResize}, true
		}
		if scrollbarDragging {
			return Action{Kind: FinishScrollbarDrag}, true
		}
	}
	return Action{}, false
}
