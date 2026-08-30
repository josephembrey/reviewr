package git

import (
	"bytes"
	"fmt"
	"strconv"
)

type changeStat struct {
	additions uint64
	deletions uint64
	binary    bool
}

func parseCount(value []byte) uint64 {
	count, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		return 0
	}
	return count
}

func parseNumstatDetails(data []byte) (map[string]changeStat, error) {
	fields := bytes.Split(data, []byte{0})
	stats := make(map[string]changeStat)
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if len(field) == 0 {
			continue
		}
		parts := bytes.SplitN(field, []byte{'\t'}, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse Git numstat record %q", field)
		}
		path := string(parts[2])
		if path == "" {
			if index+2 >= len(fields) {
				return nil, fmt.Errorf("parse truncated Git numstat rename")
			}
			index += 2
			path = string(fields[index])
		}
		if path == "" {
			continue
		}
		stats[path] = changeStat{
			additions: parseCount(parts[0]),
			deletions: parseCount(parts[1]),
			binary:    bytes.Equal(parts[0], []byte{'-'}) || bytes.Equal(parts[1], []byte{'-'}),
		}
	}
	return stats, nil
}
