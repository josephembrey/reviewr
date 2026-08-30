package git

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type worktreeRecord struct {
	path   string
	oid    string
	branch string
}

type refRecord struct {
	name     string
	oid      string
	peeled   string
	upstream string
	tracking string
	unixTime int64
}

func parseWorktreeList(data []byte) ([]worktreeRecord, error) {
	data = bytes.TrimSuffix(data, []byte{0})
	if len(data) == 0 {
		return nil, nil
	}
	records := bytes.Split(data, []byte{0, 0})
	result := make([]worktreeRecord, 0, len(records))
	for _, raw := range records {
		var record worktreeRecord
		for _, field := range bytes.Split(raw, []byte{0}) {
			switch {
			case bytes.HasPrefix(field, []byte("worktree ")):
				record.path = string(bytes.TrimPrefix(field, []byte("worktree ")))
			case bytes.HasPrefix(field, []byte("HEAD ")):
				oid := string(bytes.TrimPrefix(field, []byte("HEAD ")))
				if validObjectID(oid) && strings.Trim(oid, "0") != "" {
					record.oid = oid
				}
			case bytes.HasPrefix(field, []byte("branch refs/heads/")):
				record.branch = string(bytes.TrimPrefix(field, []byte("branch refs/heads/")))
			}
		}
		if record.path == "" {
			return nil, fmt.Errorf("parse git worktree list: record has no path")
		}
		record.path = filepath.Clean(record.path)
		result = append(result, record)
	}
	return result, nil
}

func parseRefList(data []byte) ([]refRecord, error) {
	lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil, nil
	}
	result := make([]refRecord, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte{0})
		fields := bytes.Split(line, []byte{0})
		if len(fields) != 6 {
			return nil, fmt.Errorf("parse git for-each-ref: record has %d fields", len(fields))
		}
		unixTime, _ := strconv.ParseInt(string(fields[5]), 10, 64)
		result = append(result, refRecord{
			name:     string(fields[0]),
			oid:      string(fields[1]),
			peeled:   string(fields[2]),
			upstream: string(fields[3]),
			tracking: string(fields[4]),
			unixTime: unixTime,
		})
	}
	return result, nil
}
