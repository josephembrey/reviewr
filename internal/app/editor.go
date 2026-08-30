package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type editorFinishedMsg struct {
	err error
}

type editorInvocation struct {
	name string
	args []string
}

func (m *Model) openEditor() effect {
	if m.active != workspace.Files {
		return effect{}
	}
	m.files.editorError = ""
	if m.files.readerEntry.Path == "" {
		m.files.editorError = "Editor: select a file first"
		return effect{}
	}
	path, err := currentWorktreePath(m.source.Root(), m.files.readerEntry.Path)
	if err != nil {
		m.files.editorError = "Editor: " + err.Error()
		return effect{}
	}
	return effect{
		kind: effectOpenEditor,
		path: path,
		line: currentWorktreeLine(m.files.readerDocument(), m.files.place.ReaderCursor),
	}
}

func (m Model) editorCommand(pending effect) tea.Cmd {
	invocation, err := resolveEditorInvocation(os.LookupEnv, pending.path, pending.line)
	if err == nil {
		err = validateEditorTarget(pending.path)
	}
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}

	command := exec.Command(invocation.name, invocation.args...)
	command.Dir = m.source.Root()
	return tea.ExecProcess(command, func(err error) tea.Msg {
		return editorFinishedMsg{err: describeEditorProcessError(err)}
	})
}

func describeEditorProcessError(err error) error {
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return fmt.Errorf("exited unsuccessfully: %w", err)
	}
	var executableError *exec.Error
	if errors.As(err, &executableError) {
		return fmt.Errorf("could not launch: %w", err)
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) && strings.Contains(pathError.Op, "exec") {
		return fmt.Errorf("could not launch: %w", err)
	}
	return fmt.Errorf("terminal handoff failed: %w", err)
}

func currentWorktreePath(root, path string) (string, error) {
	if root == "" {
		return "", errors.New("worktree root is unavailable")
	}
	relative := filepath.FromSlash(path)
	if !filepath.IsLocal(relative) || relative == "." {
		return "", errors.New("selected path is not worktree-relative")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve worktree root: %w", err)
	}
	return filepath.Join(absoluteRoot, relative), nil
}

func validateEditorTarget(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("current worktree file is unavailable: %w", err)
	}
	if info.IsDir() {
		return errors.New("selected worktree path is a directory")
	}
	return nil
}

func currentWorktreeLine(document ui.ReaderDocument, cursor int) uint64 {
	if len(document.Rows) == 0 {
		return 1
	}
	cursor = max(0, min(cursor, len(document.Rows)-1))
	if line, ok := currentLine(document.Rows[cursor]); ok {
		return line
	}
	for distance := 1; distance < len(document.Rows); distance++ {
		if next := cursor + distance; next < len(document.Rows) {
			if line, ok := currentLine(document.Rows[next]); ok {
				return line
			}
		}
		if previous := cursor - distance; previous >= 0 {
			if line, ok := currentLine(document.Rows[previous]); ok {
				return line
			}
		}
	}
	return 1
}

func currentLine(row ui.ReaderRow) (uint64, bool) {
	switch row.Kind {
	case ui.ReaderFile, ui.ReaderContext, ui.ReaderInsertion:
		return row.NewLine, row.NewLine > 0
	default:
		return 0, false
	}
}

func resolveEditorInvocation(lookupEnv func(string) (string, bool), path string, line uint64) (editorInvocation, error) {
	var source, configured string
	for _, name := range []string{"VISUAL", "EDITOR"} {
		value, ok := lookupEnv(name)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		source, configured = name, value
		break
	}
	if configured == "" {
		return editorInvocation{}, errors.New("VISUAL and EDITOR are unset")
	}
	words, err := splitEditorCommand(configured)
	if err != nil {
		return editorInvocation{}, fmt.Errorf("parse %s: %w", source, err)
	}
	if len(words) == 0 || words[0] == "" {
		return editorInvocation{}, fmt.Errorf("parse %s: editor executable is empty", source)
	}
	for _, word := range words {
		if strings.IndexByte(word, 0) >= 0 {
			return editorInvocation{}, fmt.Errorf("parse %s: NUL byte is not allowed", source)
		}
	}
	if line == 0 {
		line = 1
	}
	return editorInvocation{name: words[0], args: editorArguments(words[0], words[1:], path, line)}, nil
}

func editorArguments(executable string, configured []string, path string, line uint64) []string {
	args := append([]string(nil), configured...)
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(executable), ".exe"))
	lineText := strconv.FormatUint(line, 10)
	switch name {
	case "vi", "view", "vim", "vimdiff", "gvim", "gvimdiff", "nvim", "nvr", "nano", "pico", "emacs", "emacsclient", "jed", "joe":
		return append(args, "+"+lineText, path)
	case "kak", "kakoune":
		return append(args, path, "+"+lineText+":1")
	case "hx", "helix", "micro", "subl", "sublime_text", "zed":
		return append(args, path+":"+lineText+":1")
	case "code", "code-insiders", "codium", "vscodium":
		return append(args, "--goto", path+":"+lineText+":1")
	case "clion", "goland", "idea", "phpstorm", "pycharm", "rider", "rubymine", "rustrover", "webstorm":
		return append(args, "--line", lineText, path)
	case "kate":
		return append(args, "--line", lineText, path)
	default:
		// An unknown editor still receives the file, but no guessed flag that
		// could be interpreted as content, a command, or another filename.
		return append(args, path)
	}
}

func splitEditorCommand(command string) ([]string, error) {
	const (
		unquoted = iota
		singleQuoted
		doubleQuoted
	)
	state := unquoted
	escaped := false
	started := false
	var word strings.Builder
	var words []string
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}

	for _, value := range command {
		if state != singleQuoted && escaped {
			if state == doubleQuoted && value != '"' && value != '\\' && value != '$' && value != '`' && value != '\n' {
				word.WriteByte('\\')
			}
			word.WriteRune(value)
			started = true
			escaped = false
			continue
		}
		switch state {
		case singleQuoted:
			if value == '\'' {
				state = unquoted
			} else {
				word.WriteRune(value)
			}
		case doubleQuoted:
			switch value {
			case '"':
				state = unquoted
			case '\\':
				escaped = true
			default:
				word.WriteRune(value)
			}
		default:
			switch {
			case unicode.IsSpace(value):
				flush()
			case value == '\'':
				state = singleQuoted
				started = true
			case value == '"':
				state = doubleQuoted
				started = true
			case value == '\\':
				escaped = true
				started = true
			default:
				word.WriteRune(value)
				started = true
			}
		}
	}
	if escaped {
		return nil, errors.New("trailing escape")
	}
	if state != unquoted {
		return nil, errors.New("unterminated quote")
	}
	flush()
	return words, nil
}
