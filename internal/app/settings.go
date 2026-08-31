package app

import (
	"sort"

	"github.com/josephembrey/reviewr/internal/ui"
)

type settingID uint8

const (
	settingIncludeCommentsInHunkNavigation settingID = iota
)

type settingDefinition struct {
	id    settingID
	label string
}

var settingDefinitions = [...]settingDefinition{
	{id: settingIncludeCommentsInHunkNavigation, label: "include comments in hunk navigation ([/])"},
}

// settingsState is intentionally session-scoped UI state, not a general
// configuration system. The definitions slice keeps the small list ready for
// another concrete entry without coupling rendering to its stored fields.
type settingsState struct {
	selected                        int
	includeCommentsInHunkNavigation bool
}

func newSettingsState() settingsState {
	return settingsState{includeCommentsInHunkNavigation: true}
}

func (state *settingsState) selectDelta(delta int) {
	if len(settingDefinitions) == 0 {
		state.selected = 0
		return
	}
	state.selected = min(max(state.selected+delta, 0), len(settingDefinitions)-1)
}

func (state *settingsState) toggleSelected() {
	if state.selected < 0 || state.selected >= len(settingDefinitions) {
		return
	}
	switch settingDefinitions[state.selected].id {
	case settingIncludeCommentsInHunkNavigation:
		state.includeCommentsInHunkNavigation = !state.includeCommentsInHunkNavigation
	}
}

func (state settingsState) enabled(id settingID) bool {
	switch id {
	case settingIncludeCommentsInHunkNavigation:
		return state.includeCommentsInHunkNavigation
	default:
		return false
	}
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
