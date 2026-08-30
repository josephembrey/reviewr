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
	ToggleWorkspace
	ToggleScratch
	ShowFiles
	ShowGit
	ShowScratch
	ToggleSecondary
	ToggleTertiary
	ToggleComparison
	SelectNext
	SelectPrevious
	SelectIndex
	ActivateNavigatorRow
	SelectNextFile
	SelectPreviousFile
	ExpandDirectory
	CollapseDirectory
	FocusNavigator
	FocusReader
	ToggleFocus
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
)

// Action carries the small amount of data needed by a semantic intent.
type Action struct {
	Kind   ActionKind
	Index  int
	Amount int
	Width  int
	Height int
	// Position is an absolute terminal column for geometry actions.
	Position int
	Pane     navigation.Focus
	Grab     int
}

func routeMessage(msg tea.Msg, focus navigation.Focus, geometry ui.Geometry, active workspace.Kind, controls workspace.Controls, dividerDragging, scrollbarDragging bool, top, fileCount, readerOffset, readerLineCount int) (Action, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return Action{Kind: Quit}, true
		case "1":
			return Action{Kind: ToggleWorkspace}, true
		case "esc":
			return Action{Kind: ToggleScratch}, true
		case "2":
			return Action{Kind: ToggleSecondary}, true
		case "3":
			return Action{Kind: ToggleTertiary}, true
		case "4":
			return Action{Kind: ToggleComparison}, true
		case "tab":
			return Action{Kind: ToggleFocus}, true
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
				return Action{Kind: ExpandDirectory}, true
			}
		case "h", "left":
			if active == workspace.Files && focus == navigation.FocusNavigator {
				return Action{Kind: CollapseDirectory}, true
			}
		}
	case tea.WindowSizeMsg:
		return Action{Kind: Resize, Width: msg.Width, Height: msg.Height}, true
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft {
			return Action{}, false
		}
		switch hit := geometry.HitTest(mouse.X, mouse.Y, active, controls, top, fileCount, readerOffset, readerLineCount); hit.Kind {
		case ui.HitFilesWorkspace:
			return Action{Kind: ShowFiles}, true
		case ui.HitGitWorkspace:
			return Action{Kind: ShowGit}, true
		case ui.HitScratchWorkspace:
			return Action{Kind: ShowScratch}, true
		case ui.HitSecondaryControl:
			return Action{Kind: ToggleSecondary}, true
		case ui.HitTertiaryControl:
			return Action{Kind: ToggleTertiary}, true
		case ui.HitComparisonControl:
			return Action{Kind: ToggleComparison}, true
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
		if hit.Kind == ui.HitNone || hit.Kind == ui.HitFilesWorkspace || hit.Kind == ui.HitGitWorkspace || hit.Kind == ui.HitScratchWorkspace || hit.Kind == ui.HitDivider ||
			hit.Kind == ui.HitSecondaryControl || hit.Kind == ui.HitTertiaryControl || hit.Kind == ui.HitComparisonControl {
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
