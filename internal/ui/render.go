package ui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
)

var (
	accentColor = lipgloss.Color("#7AA2F7")
	dimColor    = lipgloss.Color("#777777")
	errorColor  = lipgloss.Color("#F7768E")

	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	titleStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(dimColor)
	errorStyle    = lipgloss.NewStyle().Foreground(errorColor)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
)

// Model contains only the derived state needed to paint a frame.
type Model struct {
	Geometry      Geometry
	Root          string
	Files         []string
	Selected      int
	Top           int
	Focus         navigation.Focus
	Reader        repository.File
	ReaderPath    string
	ReaderOffset  int
	ReaderLoading bool
	ListLoading   bool
	ListError     string
}

// Render paints one fixed-size frame from the shared Geometry.
func Render(model Model) string {
	g := model.Geometry
	blocks := make([]string, 0, 3)
	if g.Header.Height > 0 {
		status := ""
		if model.ListLoading {
			status = "  refreshing…"
		}
		blocks = append(blocks, fit(headerStyle.Render("reviewr")+"  "+SafeSingleLine(model.Root)+dimStyle.Render(status), g.Header.Width))
	}
	if g.Navigator.Height > 0 || g.Reader.Height > 0 {
		navigator := renderNavigator(model)
		reader := renderReader(model)
		blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, navigator, reader))
	}
	if g.Footer.Height > 0 {
		blocks = append(blocks, fit(dimStyle.Render("j/k or ↑/↓ navigate  •  tab focus  •  r refresh  •  q quit"), g.Footer.Width))
	}
	if len(blocks) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

// ReaderLines returns terminal-safe source rows or an explicit file state.
func ReaderLines(file repository.File) []string {
	switch file.Kind {
	case repository.FileReady:
		if file.Symlink {
			return []string{"symlink → " + SafeSingleLine(file.Content)}
		}
		return SafeContentLines(file.Content)
	case repository.FileMissing:
		return []string{"File is missing from the worktree."}
	case repository.FileUnreadable:
		detail := ""
		if file.Err != nil {
			detail = ": " + SafeSingleLine(file.Err.Error())
		}
		return []string{"File is unreadable" + detail}
	case repository.FileBinary:
		return []string{fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", file.Size)}
	case repository.FileTooLarge:
		return []string{fmt.Sprintf("File is too large (%d bytes; limit %d bytes).", file.Size, repository.DefaultMaxFileBytes)}
	default:
		return nil
	}
}

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
	return strings.Join(SafeContentLines(value), "↵")
}

func renderNavigator(model Model) string {
	g := model.Geometry
	rows := make([]string, 0, max(0, g.Navigator.Height-2))
	title := fmt.Sprintf("Navigator  %d files", len(model.Files))
	rows = append(rows, titleStyle.Render(title))
	visibleRows := g.NavigatorRows.Height
	for row := 0; row < visibleRows; row++ {
		index := model.Top + row
		if index >= len(model.Files) {
			if row == 0 && len(model.Files) == 0 {
				message := "No files"
				if model.ListLoading {
					message = "Loading files…"
				} else if model.ListError != "" {
					message = "Git error: " + SafeSingleLine(model.ListError)
				}
				rows = append(rows, dimStyle.Render(message))
			} else {
				rows = append(rows, "")
			}
			continue
		}
		prefix := "  "
		style := lipgloss.NewStyle()
		if index == model.Selected {
			prefix = "› "
			style = selectedStyle
		}
		rows = append(rows, style.Render(prefix+SafeSingleLine(model.Files[index])))
	}
	return renderPane(g.Navigator, rows, model.Focus == navigation.FocusNavigator)
}

func renderReader(model Model) string {
	g := model.Geometry
	path := SafeSingleLine(model.ReaderPath)
	if path == "" {
		path = "No selection"
	}
	title := "Reader  " + path
	if model.ReaderLoading && model.Reader.Kind != 0 {
		title += "  refreshing…"
	}
	rows := []string{titleStyle.Render(title)}
	content := ReaderLines(model.Reader)
	if model.ReaderLoading && len(content) == 0 {
		content = []string{"Loading file…"}
	} else if !model.ReaderLoading && len(content) == 0 {
		content = []string{"Select a file to read its current content."}
	}
	for row := 0; row < g.ReaderRows.Height; row++ {
		index := model.ReaderOffset + row
		if index < len(content) {
			line := content[index]
			if model.Reader.Kind != repository.FileReady {
				line = errorStyle.Render(line)
			}
			rows = append(rows, line)
		} else {
			rows = append(rows, "")
		}
	}
	return renderPane(g.Reader, rows, model.Focus == navigation.FocusReader)
}

func renderPane(rect Rect, rows []string, focused bool) string {
	if rect.Width <= 0 || rect.Height <= 0 {
		return ""
	}
	if rect.Width < 2 || rect.Height < 2 {
		return blankBlock(rect.Width, rect.Height)
	}
	innerWidth := max(0, rect.Width-2)
	innerHeight := max(0, rect.Height-2)
	fitted := make([]string, innerHeight)
	for index := range fitted {
		if index < len(rows) {
			fitted[index] = fit(rows[index], innerWidth)
		} else {
			fitted[index] = strings.Repeat(" ", innerWidth)
		}
	}
	borderColor := dimColor
	if focused {
		borderColor = accentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(rect.Width).
		Height(rect.Height).
		Render(strings.Join(fitted, "\n"))
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = lipgloss.NewStyle().MaxWidth(width).Render(value)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func blankBlock(width, height int) string {
	line := strings.Repeat(" ", max(0, width))
	return strings.Repeat(line+"\n", max(0, height-1)) + line
}
