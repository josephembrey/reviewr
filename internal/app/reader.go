package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// readerDocument is the narrow shared presentation boundary for raw Files
// content and immutable commit-like changes.
type readerDocument struct {
	File   repository.File
	Change *repository.ChangeDocument
	Mode   workspace.ReaderMode
}

func (document readerDocument) lines() []ui.Line {
	if document.Change == nil {
		return fileReaderLines(document.File)
	}
	if document.Mode == workspace.FileReader {
		return changedFileLines(*document.Change)
	}
	return changeDiffLines(*document.Change)
}

func fileReaderLines(file repository.File) []ui.Line {
	switch file.Kind {
	case repository.FileReady:
		if file.Symlink {
			return []ui.Line{{Text: "symlink → " + file.Content}}
		}
		rawLines := ui.SafeContentLines(file.Content)
		lines := make([]ui.Line, len(rawLines))
		for index, line := range rawLines {
			lines[index] = ui.Line{Text: line}
		}
		return lines
	case repository.FileMissing:
		return []ui.Line{{Text: "File is missing from the worktree.", Tone: ui.ToneError}}
	case repository.FileUnreadable:
		detail := ""
		if file.Err != nil {
			detail = ": " + file.Err.Error()
		}
		return []ui.Line{{Text: "File is unreadable" + detail, Tone: ui.ToneError}}
	case repository.FileBinary:
		return []ui.Line{{Text: fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", file.Size), Tone: ui.ToneError}}
	case repository.FileTooLarge:
		return []ui.Line{{Text: fmt.Sprintf("File is too large (%d bytes; limit %d bytes).", file.Size, repository.DefaultMaxFileBytes), Tone: ui.ToneError}}
	default:
		return nil
	}
}

func changedFileLines(document repository.ChangeDocument) []ui.Line {
	if document.Change.Kind == repository.ChangeDeleted {
		return []ui.Line{{Text: "Deleted file; no stored result content.", Tone: ui.ToneQuiet}}
	}
	return fileReaderLines(document.New)
}

func changeDiffLines(document repository.ChangeDocument) []ui.Line {
	if document.Change.Binary || document.Old.Kind == repository.FileBinary || document.New.Kind == repository.FileBinary {
		return []ui.Line{{Text: "Binary file changed; plain diff disabled.", Tone: ui.ToneQuiet}}
	}
	lines := changeNotice(document.Change)
	switch document.Patch.Kind {
	case repository.FileTooLarge:
		return append(lines, ui.Line{
			Text: fmt.Sprintf("Stash diff is too large (%d bytes; limit %d bytes).", document.Patch.Size, repository.DefaultMaxFileBytes),
			Tone: ui.ToneError,
		})
	case repository.FileMissing, repository.FileUnreadable:
		detail := ""
		if document.Patch.Err != nil {
			detail = ": " + document.Patch.Err.Error()
		}
		return append(lines, ui.Line{Text: "Stash diff is unavailable" + detail, Tone: ui.ToneError})
	}
	if document.Old.Kind == repository.FileTooLarge || document.New.Kind == repository.FileTooLarge {
		lines = append(lines, ui.Line{Text: "Stored file content exceeds the plain-reader limit; showing its bounded diff.", Tone: ui.ToneQuiet}, ui.Line{})
	}
	patch := fileReaderLines(document.Patch)
	for _, line := range patch {
		switch {
		case strings.HasPrefix(line.Text, "@@"):
			line.Tone = ui.ToneAccent
		case strings.HasPrefix(line.Text, "+") && !strings.HasPrefix(line.Text, "+++"):
			line.Tone = ui.ToneAdded
		case strings.HasPrefix(line.Text, "-") && !strings.HasPrefix(line.Text, "---"):
			line.Tone = ui.ToneRemoved
		case strings.HasPrefix(line.Text, "diff "), strings.HasPrefix(line.Text, "index "),
			strings.HasPrefix(line.Text, "---"), strings.HasPrefix(line.Text, "+++"):
			line.Tone = ui.ToneQuiet
		}
		lines = append(lines, line)
	}
	return lines
}

func changeNotice(change repository.ChangedFile) []ui.Line {
	var notice string
	switch change.Kind {
	case repository.ChangeDeleted:
		notice = "Deleted file — stored result is empty."
	case repository.ChangeRenamed:
		notice = "Renamed: " + change.PreviousPath + " → " + change.Path
	case repository.ChangeCopied:
		notice = "Copied: " + change.PreviousPath + " → " + change.Path
	case repository.ChangeUntracked:
		notice = "Untracked file stored in this stash."
	}
	if notice == "" {
		return nil
	}
	return []ui.Line{{Text: notice, Tone: ui.ToneQuiet}, {}}
}
