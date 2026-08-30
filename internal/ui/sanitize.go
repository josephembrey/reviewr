package ui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafeContentLines makes arbitrary worktree bytes inert before terminal output.
func SafeContentLines(content string) []string {
	content = strings.ToValidUTF8(content, "�")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var safe strings.Builder
	for _, char := range content {
		switch char {
		case '\n':
			safe.WriteRune(char)
		case '\t':
			safe.WriteString("    ")
		case '\r':
			safe.WriteRune('␍')
		case 0x7f:
			safe.WriteRune('␡')
		default:
			if char < 0x20 {
				safe.WriteRune(0x2400 + char)
			} else if unicode.IsControl(char) {
				fmt.Fprintf(&safe, "\\u%04X", char)
			} else {
				safe.WriteRune(char)
			}
		}
	}
	return strings.Split(safe.String(), "\n")
}

// SafeSingleLine renders a raw path or error without introducing screen rows.
func SafeSingleLine(value string) string {
	if safeSingleLine(value) {
		return value
	}
	return strings.Join(SafeContentLines(value), "↵")
}

func safeSingleLine(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		switch char {
		case '\n', '\t', '\r', 0x7f:
			return false
		default:
			if char < 0x20 || unicode.IsControl(char) {
				return false
			}
		}
	}
	return true
}
