package app

import (
	"sort"

	"github.com/josephembrey/reviewr/internal/ui"
)

type settingID uint8

const (
	settingIncludeCommentsInHunkNavigation settingID = iota
	settingDiffsStartFolded
)

type settingDefinition struct {
	id    settingID
	label string
}

var settingDefinitions = [...]settingDefinition{
	{id: settingIncludeCommentsInHunkNavigation, label: "include comments in hunk navigation ([/])"},
	{id: settingDiffsStartFolded, label: "start diffs folded"},
}

// settingsState is intentionally worktree-session-scoped UI state, not a
// general configuration system. Definitions keep rendering independent from
// the stored fields.
type settingsState struct {
	selected                        int
	includeCommentsInHunkNavigation bool
	diffsStartFolded                bool
}

func newSettingsState() settingsState {
	return settingsState{
		includeCommentsInHunkNavigation: true,
		diffsStartFolded:                true,
	}
}

func (state *settingsState) selectDelta(delta int) {
	if len(settingDefinitions) == 0 {
		state.selected = 0
		return
	}
	state.selected = min(max(state.selected+delta, 0), len(settingDefinitions)-1)
}

func (state *settingsState) toggleSelected() (settingID, bool) {
	if state.selected < 0 || state.selected >= len(settingDefinitions) {
		return 0, false
	}
	id := settingDefinitions[state.selected].id
	switch id {
	case settingIncludeCommentsInHunkNavigation:
		state.includeCommentsInHunkNavigation = !state.includeCommentsInHunkNavigation
	case settingDiffsStartFolded:
		state.diffsStartFolded = !state.diffsStartFolded
	}
	return id, true
}

func (state settingsState) enabled(id settingID) bool {
	switch id {
	case settingIncludeCommentsInHunkNavigation:
		return state.includeCommentsInHunkNavigation
	case settingDiffsStartFolded:
		return state.diffsStartFolded
	default:
		return false
	}
}

func (m *Model) toggleSelectedSetting() {
	id, ok := m.settings.toggleSelected()
	if !ok || id != settingDiffsStartFolded {
		return
	}
	m.configureDiffContextDefaults()
}

func (m *Model) configureDiffContextDefaults() {
	expanded := !m.settings.diffsStartFolded
	m.files.readerContext.setStartExpanded(expanded)
	m.history.inspection.readerContext.setStartExpanded(expanded)
	m.stashes.inspection.readerContext.setStartExpanded(expanded)
}

func (state settingsState) presentation(open bool) ui.Settings {
	entries := make([]ui.SettingEntry, len(settingDefinitions))
	for index, definition := range settingDefinitions {
		entries[index] = ui.SettingEntry{
			Label:    definition.label,
			Enabled:  state.enabled(definition.id),
			Selected: index == state.selected,
		}
	}
	return ui.Settings{Open: open, Entries: entries}
}

type readerNavigationLandmarkKind uint8

const (
	readerHunkLandmark readerNavigationLandmarkKind = iota
	readerFoldLandmark
	readerCommentLandmark
	readerBoundaryLandmark
)

// readerNavigationLandmark is the policy boundary between reader features and
// [/]. Hunks, context folds, and stable inline-comment headers share this
// ordered stream.
type readerNavigationLandmark struct {
	row      int
	kind     readerNavigationLandmarkKind
	identity string
}

// readerNavigationLandmarks turns disposable reader positions into the one
// ordered policy stream consumed by Settings. Comment entries retain their
// stable card identity so their selectable header remains the landmark even
// when wrapping, folding, or refresh changes rendered row positions.
func readerNavigationLandmarks(document ui.ReaderDocument) []readerNavigationLandmark {
	hunks := document.HunkNavigationTargets()
	landmarks := make([]readerNavigationLandmark, 0, len(hunks)+2)
	if len(document.Rows) != 0 {
		landmarks = append(landmarks, readerNavigationLandmark{
			row: document.SelectionTarget(0), kind: readerBoundaryLandmark,
		})
	}
	for _, row := range hunks {
		landmarks = append(landmarks, readerNavigationLandmark{row: row, kind: readerHunkLandmark})
	}
	for row, candidate := range document.Rows {
		if candidate.Kind == ui.ReaderFold {
			landmarks = append(landmarks, readerNavigationLandmark{row: row, kind: readerFoldLandmark})
		}
		if identity, ok := candidate.CommentHeaderIdentity(); ok && candidate.Selectable() {
			landmarks = append(landmarks, readerNavigationLandmark{
				row:      row,
				kind:     readerCommentLandmark,
				identity: identity,
			})
		}
	}
	if len(document.Rows) != 0 {
		landmarks = append(landmarks, readerNavigationLandmark{
			row: document.SelectionTarget(len(document.Rows) - 1), kind: readerBoundaryLandmark,
		})
	}
	sort.SliceStable(landmarks, func(left, right int) bool {
		return landmarks[left].row < landmarks[right].row
	})
	return landmarks
}

func (state settingsState) hunkNavigationTargets(landmarks []readerNavigationLandmark) []int {
	targets := make([]int, 0, len(landmarks))
	for _, landmark := range landmarks {
		if landmark.row < 0 {
			continue
		}
		switch landmark.kind {
		case readerHunkLandmark, readerFoldLandmark, readerBoundaryLandmark:
			targets = append(targets, landmark.row)
		case readerCommentLandmark:
			if state.includeCommentsInHunkNavigation {
				targets = append(targets, landmark.row)
			}
		}
	}
	sort.Ints(targets)
	return compactSortedInts(targets)
}

func compactSortedInts(values []int) []int {
	if len(values) < 2 {
		return values
	}
	write := 1
	for _, value := range values[1:] {
		if value == values[write-1] {
			continue
		}
		values[write] = value
		write++
	}
	return values[:write]
}
