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
	Entry  repository.Entry
	Diff   repository.Diff
	Change *repository.ChangeDocument
	Mode   workspace.ReaderMode
}

func (document readerDocument) lines() []ui.Line {
	if document.Change != nil {
		if document.Mode == workspace.FileReader {
			return changedFileLines(*document.Change)
		}
		return changeDiffLines(*document.Change)
	}
	if document.Mode == workspace.DiffReader {
		return diffReaderLines(document.Diff)
	}
	return fileReaderLines(document.File, document.Entry)
}

func fileReaderLines(file repository.File, entry repository.Entry) []ui.Line {
	switch file.Kind {
	case repository.FileReady:
		if file.Symlink {
			return []ui.Line{{Text: "symlink → " + file.Content}}
		}
		path := file.Path
		if path == "" {
			path = entry.Path
		}
		return highlightedSourceLines(path, file.Content)
	case repository.FileMissing:
		if entry.State == repository.FileDeleted {
			return []ui.Line{{Text: "File was deleted from the worktree.", Tone: ui.ToneError}}
		}
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
	return fileReaderLines(document.New, repository.Entry{})
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
	return append(lines, unifiedDiffLines(document.Change.Path, document.Patch.Content)...)
}

func diffReaderLines(diff repository.Diff) []ui.Line {
	switch diff.Kind {
	case repository.DiffReady:
		if diff.Content == "" {
			return []ui.Line{{Text: "No uncommitted diff for this file.", Tone: ui.ToneQuiet}}
		}
		return unifiedDiffLines(diff.Entry.Path, diff.Content)
	case repository.DiffTooLarge:
		return []ui.Line{{Text: fmt.Sprintf("Diff is too large (limit %d bytes).", repository.DefaultMaxFileBytes), Tone: ui.ToneError}}
	case repository.DiffUnavailable:
		detail := ""
		if diff.Err != nil {
			detail = ": " + diff.Err.Error()
		}
		return []ui.Line{{Text: "Diff is unavailable" + detail, Tone: ui.ToneError}}
	default:
		return nil
	}
}

func unifiedDiffLines(path, content string) []ui.Line {
	rawLines := ui.SafeContentLines(content)
	lines := make([]ui.Line, len(rawLines))
	group := make([]diffCodeRow, 0, len(rawLines))
	flush := func() {
		decorateDiffGroup(path, lines, group)
		group = group[:0]
	}
	for index, text := range rawLines {
		lines[index].Text = text
		switch {
		case strings.HasPrefix(text, "@@"):
			flush()
			lines[index].Tone = ui.ToneAccent
		case strings.HasPrefix(text, "+") && !strings.HasPrefix(text, "+++"):
			lines[index].Tone = ui.ToneAdded
			group = append(group, diffCodeRow{index: index, marker: "+", payload: text[1:], kind: diffAdded})
		case strings.HasPrefix(text, "-") && !strings.HasPrefix(text, "---"):
			lines[index].Tone = ui.ToneRemoved
			group = append(group, diffCodeRow{index: index, marker: "-", payload: text[1:], kind: diffRemoved})
		case strings.HasPrefix(text, " "):
			group = append(group, diffCodeRow{index: index, marker: " ", payload: text[1:], kind: diffContext})
		case strings.HasPrefix(text, "\\ No newline at end of file"):
			lines[index].Tone = ui.ToneQuiet
		default:
			flush()
			if text != "" {
				lines[index].Tone = ui.ToneQuiet
			}
		}
	}
	flush()
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
