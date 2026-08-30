package ui

const settingsPopupWidth = 54

func renderSettingsOverlay(frame string, screen Rect, settings Settings) string {
	width := min(settingsPopupWidth, screen.Width)
	return renderPopupOverlay(frame, screen, renderSettingsPopup(width, settings))
}

func renderSettingsPopup(width int, settings Settings) string {
	rows := make([]string, len(settings.Entries))
	for index, entry := range settings.Entries {
		rows[index] = renderSettingEntry(entry)
	}
	return renderPopupCard(width, "Settings · ,/esc close", rows)
}

func renderSettingEntry(entry SettingEntry) string {
	checkbox := "[ ]"
	if entry.Enabled {
		checkbox = "[x]"
	}
	line := checkbox + " " + SafeSingleLine(entry.Label)
	if entry.Selected {
		return selectionStyle(true).Render(line)
	}
	return chromeStyle.Render(line)
}
