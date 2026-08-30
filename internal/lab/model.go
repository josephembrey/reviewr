//go:build dev

// Package lab contains development-only interface experiments.
package lab

import tea "charm.land/bubbletea/v2"

// Model is the place state for the switcher comparison page.
type Model struct {
	selected     int
	destination  int
	fileSet      int
	reader       int
	comparison   int
	page         int
	foldSelected int
	foldExpanded [3]bool
}

// New returns the initial lab state.
func New() Model {
	return Model{}
}

// Update handles only lab-local controls.
func (model Model) Update(msg tea.KeyPressMsg) Model {
	if msg.String() == "tab" {
		model.page = (model.page + 1) % labPageCount
		return model
	}
	if model.page == labPageFolds {
		return model.updateFolds(msg)
	}
	switch msg.String() {
	case "j", "down":
		model.selected = min(model.selected+1, len(variants)-1)
	case "k", "up":
		model.selected = max(model.selected-1, 0)
	case "h", "left":
		model.destination = (model.destination + 2) % 3
	case "l", "right":
		model.destination = (model.destination + 1) % 3
	case "1":
		model.destination = destinationFiles
	case "2":
		model.destination = destinationGit
	case "3":
		model.destination = destinationNotes
	case "4":
		model.fileSet = 1 - model.fileSet
		if model.fileSet == fileSetAll {
			model.reader = readerFile
		}
	case "5":
		model.reader = 1 - model.reader
	case "6":
		model.comparison = (model.comparison + 1) % len(comparisonLabels)
	}
	return model
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
