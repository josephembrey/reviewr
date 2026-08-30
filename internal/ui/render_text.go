package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func renderSegments(segments []Segment) string {
	var value strings.Builder
	for _, segment := range segments {
		value.WriteString(renderToneText(SafeSingleLine(segment.Text), segment.Tone))
	}
	return value.String()
}

func renderLine(line Line) string {
	if len(line.Spans) != 0 {
		var rendered strings.Builder
		for _, span := range line.Spans {
			text := SafeSingleLine(span.Text)
			tone := span.Tone
			if tone == ToneDefault {
				tone = line.Tone
			}
			if tone != ToneDefault {
				rendered.WriteString(renderToneText(text, tone))
				continue
			}
			rendered.WriteString(renderTextStyle(text, span.Style))
		}
		return rendered.String()
	}
	text := SafeSingleLine(line.Text)
	return renderToneText(text, line.Tone)
}

func renderTextStyle(text string, value TextStyle) string {
	style := lipgloss.NewStyle().
		Bold(value.Bold).
		Italic(value.Italic).
		Underline(value.Underline)
	if value.Foreground != "" {
		style = style.Foreground(lipgloss.Color(value.Foreground))
	}
	return style.Render(text)
}

func renderToneText(text string, tone Tone) string {
	switch tone {
	case ToneQuiet:
		return mutedStyle.Render(text)
	case ToneError:
		return errorStyle.Render(text)
	case ToneAccent:
		return purpleStyle.Render(text)
	case ToneAdded:
		return addedStyle.Render(text)
	case ToneRemoved:
		return errorStyle.Render(text)
	case ToneInfo:
		return headerStyle.Render(text)
	case ToneWarning:
		return yellowStyle.Render(text)
	default:
		return text
	}
}
