package git

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func parseStashLog(data []byte) ([]StashEntry, error) {
	entries := make([]StashEntry, 0, bytes.Count(data, []byte{0})/5)
	reader := nulReader{data: data}
	for oidField, ok := reader.next(); ok; oidField, ok = reader.next() {
		selectorField, subject, timestampField, parentsField, err := readStashFields(&reader)
		if err != nil {
			return nil, err
		}
		oid := string(oidField)
		selector := strings.TrimPrefix(string(selectorField), "refs/")
		if !validObjectID(oid) || !strings.HasPrefix(selector, "stash@{") {
			return nil, fmt.Errorf("parse stash reflog: invalid identity")
		}
		timestamp, err := strconv.ParseInt(string(timestampField), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse stash reflog timestamp: %w", err)
		}
		parents := strings.Fields(string(parentsField))
		if len(parents) < 1 || !validObjectID(parents[0]) {
			return nil, fmt.Errorf("parse stash reflog: missing base object")
		}
		untracked := ""
		if len(parents) >= 3 {
			if !validObjectID(parents[2]) {
				return nil, fmt.Errorf("parse stash reflog: invalid untracked object")
			}
			untracked = parents[2]
		}
		branch, message := parseStashSubject(string(subject))
		entries = append(entries, StashEntry{
			OID: oid, Selector: selector, Branch: branch, Message: message,
			Timestamp: timestamp, BaseOID: parents[0], UntrackedOID: untracked,
		})
	}
	return entries, nil
}

func readStashFields(reader *nulReader) ([]byte, []byte, []byte, []byte, error) {
	fields := [4][]byte{}
	for index := range fields {
		field, ok := reader.next()
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("parse stash reflog: truncated record")
		}
		fields[index] = field
	}
	return fields[0], fields[1], fields[2], fields[3], nil
}

func parseStashSubject(subject string) (string, string) {
	rest := subject
	for _, prefix := range []string{"WIP on ", "On ", "index on "} {
		if strings.HasPrefix(subject, prefix) {
			rest = strings.TrimPrefix(subject, prefix)
			break
		}
	}
	if rest == subject {
		return "", subject
	}
	branch, message, ok := strings.Cut(rest, ": ")
	if !ok {
		return "", subject
	}
	return branch, message
}

func parseNameStatus(data []byte, stats map[string]changeStat) ([]ChangedFile, error) {
	changes := make([]ChangedFile, 0, bytes.Count(data, []byte{0})/2)
	reader := nulReader{data: data}
	for status, ok := reader.next(); ok; status, ok = reader.next() {
		if len(status) == 0 {
			continue
		}
		change, err := readNameStatus(&reader, status, stats)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func readNameStatus(reader *nulReader, status []byte, stats map[string]changeStat) (ChangedFile, error) {
	kind := ChangeModified
	previous := ""
	switch status[0] {
	case 'A':
		kind = ChangeAdded
	case 'D':
		kind = ChangeDeleted
	case 'R', 'C':
		oldPath, ok := reader.next()
		if !ok {
			return ChangedFile{}, fmt.Errorf("parse truncated Git rename record")
		}
		previous = string(oldPath)
		if status[0] == 'R' {
			kind = ChangeRenamed
		} else {
			kind = ChangeCopied
		}
	}
	pathField, ok := reader.next()
	if !ok {
		return ChangedFile{}, fmt.Errorf("parse truncated Git name-status record")
	}
	path := string(pathField)
	if path == "" {
		return ChangedFile{}, fmt.Errorf("parse Git name-status: empty path")
	}
	stat := stats[path]
	return ChangedFile{
		Path: path, PreviousPath: previous, Kind: kind,
		Additions: stat.additions, Deletions: stat.deletions, Binary: stat.binary,
	}, nil
}

func mergeChanges(tracked, extras []ChangedFile) []ChangedFile {
	combined := make(map[string]ChangedFile, len(tracked)+len(extras))
	for _, change := range tracked {
		combined[change.Path] = change
	}
	for _, change := range extras {
		if _, exists := combined[change.Path]; !exists {
			combined[change.Path] = change
		}
	}
	paths := make([]string, 0, len(combined))
	for path := range combined {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]ChangedFile, 0, len(paths))
	for _, path := range paths {
		result = append(result, combined[path])
	}
	return result
}

func validateStashSource(source StashSource) error {
	if !validObjectID(source.OID) || !validObjectID(source.BaseOID) ||
		(source.UntrackedOID != "" && !validObjectID(source.UntrackedOID)) {
		return fmt.Errorf("invalid stash object identity")
	}
	return nil
}

func validGitPath(path string) bool {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return false
	}
	return filepath.IsLocal(filepath.FromSlash(path))
}
