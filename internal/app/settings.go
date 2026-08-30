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
	readerCommentLandmark
)

// readerNavigationLandmark is the policy boundary between reader features and
// [/]. Current readers produce hunk landmarks; inline comment headers can join
// the same ordered stream once that feature lands.
type readerNavigationLandmark struct {
	row  int
	kind readerNavigationLandmarkKind
}

func (state settingsState) hunkNavigationTargets(landmarks []readerNavigationLandmark) []int {
	targets := make([]int, 0, len(landmarks))
	for _, landmark := range landmarks {
		if landmark.row < 0 {
			continue
		}
		switch landmark.kind {
		case readerHunkLandmark:
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
