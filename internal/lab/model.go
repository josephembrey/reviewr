//go:build dev

// Package lab contains development-only interface experiments.
package lab

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
)

// Model is the place state for the switcher comparison page.
type Model struct {
	selected             int
	destination          int
	fileSet              int
	reader               int
	comparison           int
	page                 int
	foldSelected         int
	foldExpanded         [3]bool
	foldMotionVisible    int
	foldMotionTarget     int
	foldMotionSpeed      int
	foldMotionGeneration uint64
	terminalBackground   color.RGBA
	backgroundReported   bool
}

// New returns the initial lab state.
func New() Model {
	return Model{terminalBackground: fallbackTerminalBackground}
}

// Update handles only lab-local controls and animation frames. Unknown messages
// remain available to the real application behind the development overlay.
func (model Model) Update(msg tea.Msg) (Model, tea.Cmd, bool) {
	if background, ok := msg.(tea.BackgroundColorMsg); ok {
		model.setTerminalBackground(background.Color)
		return model, nil, true
	}
	if tick, ok := msg.(foldMotionTick); ok {
		return model.updateFoldMotionTick(tick)
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return model, nil, false
	}
	if key.String() == "tab" {
		if model.page == labPageFoldMotion {
			model.foldMotionTarget = model.foldMotionVisible
			model.foldMotionGeneration++
		}
		model.page = (model.page + 1) % labPageCount
		return model, nil, true
	}
	if model.page == labPageFolds {
		return model.updateFolds(key), nil, true
	}
	if model.page == labPageFoldMotion {
		model, command := model.updateFoldMotion(key)
		return model, command, true
	}
	if model.page == labPageANSIPalette || model.page == labPageDiffTints {
		return model, nil, true
	}
	switch key.String() {
	case "j", "down":
		model.selected = min(model.selected+1, len(variants)-1)
	case "k", "up":
		model.selected = max(model.selected-1, 0)
	case "h", "left":
		model.destination = (model.destination + 2) % 3
	case "l", "right":
		model.destination = (model.destination + 1) % 3
	case "1":
		model.fileSet = 1 - model.fileSet
		if model.fileSet == fileSetAll {
			model.reader = readerFile
		}
	case "2":
		model.reader = 1 - model.reader
	case "3":
		model.comparison = (model.comparison + 1) % len(comparisonLabels)
	}
	return model, nil, true
}

func (model Model) updateFolds(msg tea.KeyPressMsg) Model {
	switch msg.String() {
	case "j", "down":
		model.foldSelected = min(model.foldSelected+1, len(foldVariants)-1)
	case "k", "up":
		model.foldSelected = max(model.foldSelected-1, 0)
	case "h", "left":
		model.foldExpanded[model.foldSelected] = false
	case "l", "right":
		model.foldExpanded[model.foldSelected] = true
	case "enter", " ":
		model.foldExpanded[model.foldSelected] = !model.foldExpanded[model.foldSelected]
	}
	return model
}
