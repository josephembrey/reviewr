package ui

import "charm.land/lipgloss/v2"

var (
	// Structural and semantic roles use the terminal's basic palette:
	// cyan is focus/action/info, white is readable chrome, BrightBlack is
	// secondary content, red/green are negative/positive, magenta is a special
	// identity, and yellow is warning/attention. File-type icons are the sole
	// production truecolor exception; their complete catalog lives together in
	// file_tree_icons.go.
	accentColor    = lipgloss.Cyan
	secondaryColor = lipgloss.White
	mutedColor     = lipgloss.BrightBlack
	errorColor     = lipgloss.Red
	addedColor     = lipgloss.Green
	specialColor   = lipgloss.Magenta
	warningColor   = lipgloss.Yellow

	headerStyle        = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	focusedTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	chromeStyle        = lipgloss.NewStyle().Foreground(secondaryColor)
	mutedStyle         = lipgloss.NewStyle().Foreground(mutedColor)
	readerFoldStyle    = lipgloss.NewStyle().Foreground(accentColor)
	readerFoldEndStyle = readerFoldStyle.Faint(true)
	// Scrollbars follow Herdr's restrained three-level hierarchy while staying
	// within reviewr's terminal ANSI roles.
	scrollbarTrackStyle          = mutedStyle.Faint(true)
	scrollbarUnfocusedThumbStyle = mutedStyle
	scrollbarFocusedThumbStyle   = lipgloss.NewStyle().Foreground(accentColor)
	errorStyle                   = lipgloss.NewStyle().Foreground(errorColor)
	addedStyle                   = lipgloss.NewStyle().Foreground(addedColor)
	specialStyle                 = lipgloss.NewStyle().Foreground(specialColor)
	warningStyle                 = lipgloss.NewStyle().Foreground(warningColor)
)
