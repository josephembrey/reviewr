package ui

import "charm.land/lipgloss/v2"

var (
	// Structural and semantic roles use the terminal's basic palette. This
	// keeps reviewr coherent with both generated palettes and conventional
	// ANSI themes; file-type icons are the deliberate truecolor exception.
	accentColor    = lipgloss.Cyan
	secondaryColor = lipgloss.White
	mutedColor     = lipgloss.BrightBlack
	errorColor     = lipgloss.Red
	addedColor     = lipgloss.Green
	purpleColor    = lipgloss.Magenta
	yellowColor    = lipgloss.Yellow

	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	focusedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	chromeStyle       = lipgloss.NewStyle().Foreground(secondaryColor)
	mutedStyle        = lipgloss.NewStyle().Foreground(mutedColor)
	readerFoldStyle   = lipgloss.NewStyle().Foreground(accentColor)
	// Scrollbars follow Herdr's restrained three-level hierarchy while staying
	// within reviewr's terminal ANSI roles.
	scrollbarTrackStyle          = mutedStyle.Faint(true)
	scrollbarUnfocusedThumbStyle = mutedStyle
	scrollbarFocusedThumbStyle   = lipgloss.NewStyle().Foreground(accentColor)
	errorStyle                   = lipgloss.NewStyle().Foreground(errorColor)
	addedStyle                   = lipgloss.NewStyle().Foreground(addedColor)
	purpleStyle                  = lipgloss.NewStyle().Foreground(purpleColor)
	yellowStyle                  = lipgloss.NewStyle().Foreground(yellowColor)
)
