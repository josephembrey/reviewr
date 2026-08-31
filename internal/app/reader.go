package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// readerDocument is the narrow source boundary for Files, immutable Git
// changes, and review comparisons. build returns the shared UI document.
type readerDocument struct {
	File        repository.File
	Entry       repository.Entry
	Diff        repository.Diff
	Change      *repository.ChangeDocument
	ChangeLabel string
	Mode        workspace.ReaderMode
}

func (document readerDocument) build() ui.ReaderDocument {
	if document.Change != nil {
		if document.Mode == workspace.FileReader {
			return changedFileDocument(*document.Change)
		}
		return changeDiffDocument(*document.Change, document.ChangeLabel)
	}
	if document.Mode == workspace.DiffReader {
		return diffReaderDocument(document.Diff)
	}
	return fileReaderDocument(document.File, document.Entry)
}

func fileReaderDocument(file repository.File, entry repository.Entry) ui.ReaderDocument {
	document := ui.ReaderDocument{Kind: ui.ReaderFileDocument}
	switch file.Kind {
	case repository.FileReady:
		if file.Symlink {
			document.Rows = noticeRows("symlink → "+file.Content, ui.ToneDefault)
			return document
		}
		path := file.Path
		if path == "" {
			path = entry.Path
		}
		document.Rows = highlightedSourceRows(path, file.Content)
	case repository.FileMissing:
		if entry.State == repository.FileDeleted {
			document.Rows = noticeRows("File was deleted from the worktree.", ui.ToneError)
		} else {
			document.Rows = noticeRows("File is missing from the worktree.", ui.ToneError)
		}
	case repository.FileUnreadable:
		detail := ""
		if file.Err != nil {
			detail = ": " + file.Err.Error()
		}
		document.Rows = noticeRows("File is unreadable"+detail, ui.ToneError)
	case repository.FileBinary:
		document.Rows = noticeRows(fmt.Sprintf("Binary file (%d bytes); plain reader disabled.", file.Size), ui.ToneError)
	case repository.FileTooLarge:
		document.Rows = noticeRows(fmt.Sprintf("File is too large (%d bytes; limit %d bytes).", file.Size, repository.DefaultMaxFileBytes), ui.ToneError)
	}
	return document
}

func changedFileDocument(document repository.ChangeDocument) ui.ReaderDocument {
	if document.Change.Kind == repository.ChangeDeleted {
		return ui.ReaderDocument{Kind: ui.ReaderFileDocument, Rows: noticeRows("Deleted file; no stored result content.", ui.ToneQuiet)}
	}
	return fileReaderDocument(document.New, repository.Entry{})
}

func changeDiffDocument(document repository.ChangeDocument, label string) ui.ReaderDocument {
	if label == "" {
		label = "Stored"
	}
	result := ui.ReaderDocument{Kind: ui.ReaderDiffDocument}
	if document.Change.Binary || document.Old.Kind == repository.FileBinary || document.New.Kind == repository.FileBinary {
		result.Rows = noticeRows("Binary file changed; plain diff disabled.", ui.ToneQuiet)
		return result
	}
	rows := changeNoticeRows(document.Change)
	switch document.Patch.Kind {
	case repository.FileTooLarge:
		result.Rows = append(rows, noticeRows(
			fmt.Sprintf("%s diff is too large (%d bytes; limit %d bytes).", label, document.Patch.Size, repository.DefaultMaxFileBytes),
			ui.ToneError,
		)...)
		return result
	case repository.FileMissing, repository.FileUnreadable:
		detail := ""
		if document.Patch.Err != nil {
			detail = ": " + document.Patch.Err.Error()
		}
		result.Rows = append(rows, noticeRows(label+" diff is unavailable"+detail, ui.ToneError)...)
		return result
	}
	if document.Old.Kind == repository.FileTooLarge || document.New.Kind == repository.FileTooLarge {
		rows = append(rows,
			ui.ReaderRow{Kind: ui.ReaderNotice, Text: "Stored file content exceeds the plain-reader limit; showing its bounded diff.", Tone: ui.ToneQuiet},
			ui.ReaderRow{Kind: ui.ReaderMetadata},
		)
	}
	parsed := unifiedDiffDocument(document.Change.Path, document.Patch.Content)
	result.Rows = append(rows, parsed.Rows...)
	return result
}

func diffReaderDocument(diff repository.Diff) ui.ReaderDocument {
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument}
	switch diff.Kind {
	case repository.DiffReady:
		if diff.Content == "" {
			document.Rows = noticeRows("No uncommitted diff for this file.", ui.ToneQuiet)
			return document
		}
		return unifiedDiffDocument(diff.Entry.Path, diff.Content)
	case repository.DiffTooLarge:
		document.Rows = noticeRows(fmt.Sprintf("Diff is too large (limit %d bytes).", repository.DefaultMaxFileBytes), ui.ToneError)
	case repository.DiffUnavailable:
		detail := ""
		if diff.Err != nil {
			detail = ": " + diff.Err.Error()
		}
		document.Rows = noticeRows("Diff is unavailable"+detail, ui.ToneError)
	}
	return document
}

var unifiedHunkHeader = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?:.*)?$`)

type hunkPosition struct {
	oldLine      uint64
	newLine      uint64
	oldRemaining uint64
	newRemaining uint64
	active       bool
}

func (position *hunkPosition) takeRow(text string) (ui.ReaderRow, diffCodeKind, bool) {
	if !position.active || text == "" {
		return ui.ReaderRow{}, diffContext, false
	}
	marker, payload := text[0], text[1:]
	row := ui.ReaderRow{Text: payload}
	kind := diffContext
	valid := false
	switch marker {
	case ' ':
		if position.oldRemaining > 0 && position.newRemaining > 0 {
			row.Kind = ui.ReaderContext
			row.OldLine, row.NewLine = position.oldLine, position.newLine
			position.oldLine, position.newLine = incrementLine(position.oldLine), incrementLine(position.newLine)
			position.oldRemaining--
			position.newRemaining--
			valid = true
		}
	case '-':
		if position.oldRemaining > 0 {
			row.Kind, kind = ui.ReaderDeletion, diffRemoved
			row.OldLine = position.oldLine
			position.oldLine = incrementLine(position.oldLine)
			position.oldRemaining--
			valid = true
		}
	case '+':
		if position.newRemaining > 0 {
			row.Kind, kind = ui.ReaderInsertion, diffAdded
			row.NewLine = position.newLine
			position.newLine = incrementLine(position.newLine)
			position.newRemaining--
			valid = true
		}
	}
	if valid {
		row.Identity = diffRowIdentity(row)
	}
	return row, kind, valid
}

func parseHunkHeader(line string) (hunkPosition, bool) {
	match := unifiedHunkHeader.FindStringSubmatch(line)
	if match == nil {
		return hunkPosition{}, false
	}
	parse := func(value string, omitted uint64) (uint64, bool) {
		if value == "" {
			return omitted, true
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		return parsed, err == nil
	}
	oldLine, oldOK := parse(match[1], 0)
	oldCount, oldCountOK := parse(match[2], 1)
	newLine, newOK := parse(match[3], 0)
	newCount, newCountOK := parse(match[4], 1)
	if !oldOK || !oldCountOK || !newOK || !newCountOK {
		return hunkPosition{}, false
	}
	return hunkPosition{
		oldLine: oldLine, newLine: newLine,
		oldRemaining: oldCount, newRemaining: newCount, active: oldCount > 0 || newCount > 0,
	}, true
}

func unifiedDiffDocument(path, content string) ui.ReaderDocument {
	rawLines := ui.SafeContentLines(content)
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: make([]ui.ReaderRow, 0, len(rawLines))}
	group := make([]diffCodeRow, 0, len(rawLines))
	flush := func() {
		decorateDiffGroup(path, document.Rows, group)
		group = group[:0]
	}
	position := hunkPosition{}
	for _, text := range rawLines {
		if header, ok := parseHunkHeader(text); ok {
			flush()
			position = header
			document.Rows = append(document.Rows, ui.ReaderRow{Kind: ui.ReaderMetadata, Text: text, Tone: ui.ToneSpecial})
			continue
		}
		if row, kind, ok := position.takeRow(text); ok {
			index := len(document.Rows)
			document.Rows = append(document.Rows, row)
			group = append(group, diffCodeRow{index: index, payload: row.Text, kind: kind})
			if position.oldRemaining == 0 && position.newRemaining == 0 {
				flush()
				position.active = false
			}
			continue
		}
		flush()
		position.active = false
		kind, tone := ui.ReaderMetadata, ui.ToneQuiet
		if strings.HasPrefix(text, `\ No newline at end of file`) {
			kind = ui.ReaderNotice
		}
		if text == "" {
			tone = ui.ToneDefault
		}
		document.Rows = append(document.Rows, ui.ReaderRow{Kind: kind, Text: text, Tone: tone})
	}
	flush()
	return document
}

func incrementLine(line uint64) uint64 {
	if line == ^uint64(0) {
		return line
	}
	return line + 1
}

func diffRowIdentity(row ui.ReaderRow) string {
	return fmt.Sprintf("%d:%d:%d:%s", row.Kind, row.OldLine, row.NewLine, row.Text)
}

func readerRowIdentities(rows []ui.ReaderRow) []string {
	identities := make([]string, len(rows))
	occurrences := make(map[string]int, len(rows))
	for index, row := range rows {
		identity := row.Identity
		if identity == "" {
			identity = diffRowIdentity(row)
		}
		occurrences[identity]++
		identities[index] = fmt.Sprintf("%s\x00%d", identity, occurrences[identity])
	}
	return identities
}

func reconcileLogicalLine(old []string, oldIndex int, current []string) int {
	if len(current) == 0 {
		return 0
	}
	if len(old) == 0 {
		return clampIndex(oldIndex, len(current))
	}
	oldIndex = clampIndex(oldIndex, len(old))
	indices := make(map[string]int, len(current))
	for index, identity := range current {
		indices[identity] = index
	}
	if index, ok := indices[old[oldIndex]]; ok {
		return index
	}
	for distance := 1; distance < len(old); distance++ {
		if next := oldIndex + distance; next < len(old) {
			if index, ok := indices[old[next]]; ok {
				return index
			}
		}
		if previous := oldIndex - distance; previous >= 0 {
			if index, ok := indices[old[previous]]; ok {
				return index
			}
		}
	}
	return clampIndex(oldIndex, len(current))
}

func clampIndex(index, length int) int {
	if length <= 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func changeNoticeRows(change repository.ChangedFile) []ui.ReaderRow {
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
	return []ui.ReaderRow{
		{Kind: ui.ReaderNotice, Text: ui.SafeSingleLine(notice), Tone: ui.ToneQuiet},
		{Kind: ui.ReaderMetadata},
	}
}

func noticeRows(text string, tone ui.Tone) []ui.ReaderRow {
	lines := ui.SafeContentLines(text)
	rows := make([]ui.ReaderRow, len(lines))
	for index, line := range lines {
		rows[index] = ui.ReaderRow{Kind: ui.ReaderNotice, Text: line, Tone: tone}
	}
	return rows
}
