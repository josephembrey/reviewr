package ui

import "charm.land/lipgloss/v2"

var (
	// Structural and semantic roles use the terminal's basic palette:
	// cyan is focus/action/info, white is readable chrome, BrightBlack is
	// secondary content, red/green are negative/positive, magenta is a special
	// identity, and yellow is warning/attention. The shared vivid palette is
	// the narrow truecolor exception for file-type icons and Git graph lanes:
	// small identity glyphs that must remain distinguishable when ANSI slots
	// collapse under a terminal theme.
	accentColor    = lipgloss.Cyan
	secondaryColor = lipgloss.White
	mutedColor     = lipgloss.BrightBlack
	errorColor     = lipgloss.Red
	addedColor     = lipgloss.Green
	specialColor   = lipgloss.Magenta
	warningColor   = lipgloss.Yellow

	vividRedColor    = lipgloss.Color("#E06C75")
	vividGreenColor  = lipgloss.Color("#98C379")
	vividYellowColor = lipgloss.Color("#E5C07B")
	vividOrangeColor = lipgloss.Color("#D19A66")
	vividPurpleColor = lipgloss.Color("#C678DD")
	vividBlueColor   = lipgloss.Color("#61AFEF")
	vividCyanColor   = lipgloss.Color("#56B6C2")

	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	focusedTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	chromeStyle         = lipgloss.NewStyle().Foreground(secondaryColor)
	mutedStyle          = lipgloss.NewStyle().Foreground(mutedColor)
	readerFoldStyle     = lipgloss.NewStyle().Foreground(accentColor)
	readerFoldEndStyle  = readerFoldStyle.Faint(true)
	commentBorderStyle  = lipgloss.NewStyle().Foreground(mutedColor).Faint(true)
	commentTitleStyle   = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	commentBodyStyle    = lipgloss.NewStyle().Foreground(secondaryColor)
	composerBorderStyle = lipgloss.NewStyle().Foreground(warningColor)
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
