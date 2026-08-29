//go:build dev

// Package lab contains development-only interface experiments.
package lab

import tea "charm.land/bubbletea/v2"

// Model is the place state for the switcher comparison page.
type Model struct {
	selected    int
	destination int
	fileSet     int
	reader      int
	comparison  int
}

// New returns the initial lab state.
func New() Model {
	return Model{}
}

// Update handles only lab-local controls.
func (model Model) Update(msg tea.KeyPressMsg) Model {
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
		if model.destination == destinationFiles {
			model.destination = destinationGit
		} else {
			model.destination = destinationFiles
		}
	case "2":
		model.fileSet = 1 - model.fileSet
		if model.fileSet == fileSetAll {
			model.reader = readerFile
		}
	case "3":
		model.reader = 1 - model.reader
	case "4":
		model.comparison = (model.comparison + 1) % len(comparisonLabels)
	case "0", "`", "n":
		model.destination = destinationScratch
	}
	return model
}
