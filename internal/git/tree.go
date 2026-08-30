package git

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

const maxTreePathspecBytes = 128 << 10

// TreeEntry is exact immutable metadata for one path at a revision.
type TreeEntry struct {
	Mode uint32
	Type string
	OID  string
	Path string
}

// ReadTreeEntry reads one literal path from an immutable tree.
func (client Client) ReadTreeEntry(root, revision, path string) (TreeEntry, bool, error) {
	entries, err := client.ReadTreeEntries(root, revision, []string{path})
	if err != nil {
		return TreeEntry{}, false, err
	}
	entry, exists := entries[path]
	return entry, exists, nil
}

// ReadTreeEntries reads literal paths from one immutable tree. Path arguments
// are chunked below platform command-line limits, while captured output keeps
// one aggregate memory budget across every chunk.
func (Client) ReadTreeEntries(root, revision string, paths []string) (map[string]TreeEntry, error) {
	entries := make(map[string]TreeEntry, len(paths))
	remaining := defaultMaxCommandBytes
	for start := 0; start < len(paths); {
		end := treePathspecChunk(paths, start)
		args := []string{"ls-tree", "-z", "--full-tree", revision, "--"}
		args = append(args, paths[start:end]...)
		out, err := runBounded(root, remaining, args...)
		if err != nil {
			return nil, err
		}
		if err := parseTreeEntries(out, entries); err != nil {
			return nil, err
		}
		remaining -= int64(len(out))
		start = end
	}
	return entries, nil
}

func treePathspecChunk(paths []string, start int) int {
	bytesUsed := 0
	end := start
	for end < len(paths) {
		next := len(paths[end]) + 1
		if end > start && bytesUsed+next > maxTreePathspecBytes {
			break
		}
		bytesUsed += next
		end++
	}
	return end
}

func parseTreeEntries(data []byte, entries map[string]TreeEntry) error {
	reader := nulReader{data: data}
	for record, ok := reader.next(); ok; record, ok = reader.next() {
		if len(record) == 0 {
			return errors.New("malformed git ls-tree record")
		}
		entry, err := parseTreeEntry(record)
		if err != nil {
			return err
		}
		entries[entry.Path] = entry
	}
	return nil
}

func parseTreeEntry(record []byte) (TreeEntry, error) {
	tab := bytes.IndexByte(record, '\t')
	if tab < 0 {
		return TreeEntry{}, errors.New("malformed git ls-tree record")
	}
	fields := bytes.Fields(record[:tab])
	if len(fields) != 3 {
		return TreeEntry{}, errors.New("malformed git ls-tree metadata")
	}
	mode, err := strconv.ParseUint(string(fields[0]), 8, 32)
	if err != nil {
		return TreeEntry{}, fmt.Errorf("parse git tree mode: %w", err)
	}
	return TreeEntry{
		Mode: uint32(mode), Type: string(fields[1]), OID: string(fields[2]), Path: string(record[tab+1:]),
	}, nil
}
