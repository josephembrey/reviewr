package git

import (
	"bytes"
	"fmt"
	"sort"
)

// FileState is the one status assigned to a repository-relative file identity.
type FileState uint8

const (
	FileUnchanged FileState = iota
	FileModified
	FileAdded
	FileDeleted
	FileRenamed
	FileUntracked
	FileIgnored
)

// FileEntry carries a current path identity and its machine-readable Git state.
// PreviousPath is set only when porcelain reports a rename relation.
type FileEntry struct {
	Path         string
	PreviousPath string
	State        FileState
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// Snapshot reads all tracked, untracked, and ignored file identities, then
// overlays porcelain-v2 state and per-file line statistics into one result.
func (client Client) Snapshot(root string) ([]FileEntry, error) {
	trackedOutput, err := run(root, "ls-files", "-z", "--cached")
	if err != nil {
		return nil, err
	}
	statusOutput, err := run(
		root,
		"status",
		"--porcelain=v2",
		"-z",
		"--untracked-files=all",
		"--ignored=traditional",
		"--renames",
	)
	if err != nil {
		return nil, err
	}
	status, err := ParsePorcelainV2(statusOutput)
	if err != nil {
		return nil, err
	}
	entries := MergeFileEntries(ParseNUL(trackedOutput), status)
	untracked := make([]string, 0)
	for _, entry := range entries {
		if entry.State == FileUntracked {
			untracked = append(untracked, entry.Path)
		}
	}
	stats, err := client.worktreeStats(root, untracked)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		stat, changed := stats[entries[index].Path]
		if !changed {
			continue
		}
		entries[index].Additions = stat.additions
		entries[index].Deletions = stat.deletions
		entries[index].Binary = stat.binary
	}
	return entries, nil
}

// ParsePorcelainV2 parses only NUL-delimited porcelain-v2 records. Paths remain
// opaque byte strings; in particular, whitespace and newlines are not syntax.
func ParsePorcelainV2(data []byte) ([]FileEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	records := bytes.Split(data, []byte{0})
	if len(records[len(records)-1]) == 0 {
		records = records[:len(records)-1]
	}
	entries := make([]FileEntry, 0, len(records))
	for index := 0; index < len(records); index++ {
		record := records[index]
		if len(record) < 2 || record[1] != ' ' {
			return nil, fmt.Errorf("parse git status record %q", record)
		}
		switch record[0] {
		case '?':
			path, err := statusPath(record[2:])
			if err != nil {
				return nil, err
			}
			entries = append(entries, FileEntry{Path: path, State: FileUntracked})
		case '!':
			path, err := statusPath(record[2:])
			if err != nil {
				return nil, err
			}
			entries = append(entries, FileEntry{Path: path, State: FileIgnored})
		case '1':
			fields, err := statusFields(record, 9)
			if err != nil {
				return nil, err
			}
			entries = append(entries, FileEntry{Path: string(fields[8]), State: stateFromXY(fields[1])})
		case '2':
			fields, err := statusFields(record, 10)
			if err != nil {
				return nil, err
			}
			if index+1 >= len(records) || len(records[index+1]) == 0 {
				return nil, fmt.Errorf("parse truncated git status rename %q", record)
			}
			index++
			state := FileRenamed
			previousPath := string(records[index])
			if len(fields[8]) > 0 && fields[8][0] == 'C' {
				state = FileAdded
				previousPath = ""
			}
			entries = append(entries, FileEntry{
				Path:         string(fields[9]),
				PreviousPath: previousPath,
				State:        state,
			})
		case 'u':
			fields, err := statusFields(record, 11)
			if err != nil {
				return nil, err
			}
			entries = append(entries, FileEntry{Path: string(fields[10]), State: FileModified})
		default:
			return nil, fmt.Errorf("parse unsupported git status record %q", record)
		}
	}
	return entries, nil
}

func statusFields(record []byte, count int) ([][]byte, error) {
	fields := bytes.SplitN(record, []byte{' '}, count)
	if len(fields) != count || len(fields[count-1]) == 0 {
		return nil, fmt.Errorf("parse git status record %q", record)
	}
	return fields, nil
}

func statusPath(path []byte) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("parse empty git status path")
	}
	return string(path), nil
}

func stateFromXY(xy []byte) FileState {
	if bytes.IndexByte(xy, 'R') >= 0 {
		return FileRenamed
	}
	if bytes.IndexByte(xy, 'D') >= 0 {
		return FileDeleted
	}
	if bytes.IndexByte(xy, 'A') >= 0 {
		return FileAdded
	}
	if len(xy) == 2 && xy[0] == '.' && xy[1] == '.' {
		return FileUnchanged
	}
	return FileModified
}

// MergeFileEntries overlays status on the complete tracked identity set.
func MergeFileEntries(tracked []string, status []FileEntry) []FileEntry {
	byPath := make(map[string]FileEntry, len(tracked)+len(status))
	for _, path := range tracked {
		if path != "" {
			byPath[path] = FileEntry{Path: path, State: FileUnchanged}
		}
	}
	for _, entry := range status {
		if entry.Path == "" {
			continue
		}
		if entry.State == FileRenamed && entry.PreviousPath != "" && entry.PreviousPath != entry.Path {
			delete(byPath, entry.PreviousPath)
		}
		byPath[entry.Path] = entry
	}
	entries := make([]FileEntry, 0, len(byPath))
	for _, entry := range byPath {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	return entries
}
