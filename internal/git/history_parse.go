package git

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type historyRow struct {
	oid          string
	shortOID     string
	author       string
	authoredUnix int64
	subject      string
}

func parseHistoryRows(data []byte, label string) ([]historyRow, error) {
	rows := make([]historyRow, 0, min(bytes.Count(data, []byte{0, 0})+1, CommitLimit))
	reader := nulReader{data: data}
	for oid, ok := reader.next(); ok; oid, ok = reader.next() {
		if len(oid) == 0 {
			continue
		}
		shortOID, author, authored, subject, err := readHistoryFields(&reader, label)
		if err != nil {
			return nil, err
		}
		if !validObjectID(string(oid)) || len(shortOID) == 0 {
			return nil, fmt.Errorf("parse %s: invalid object identity", label)
		}
		timestamp, err := strconv.ParseInt(string(authored), 10, 64)
		if err != nil || timestamp < 0 {
			return nil, fmt.Errorf("parse %s: invalid authored timestamp", label)
		}
		rows = append(rows, historyRow{
			oid:          string(oid),
			shortOID:     string(shortOID),
			author:       string(author),
			authoredUnix: timestamp,
			subject:      string(subject),
		})
		if len(rows) == CommitLimit {
			break
		}
		delimiter, hasDelimiter := reader.next()
		if hasDelimiter && len(delimiter) != 0 {
			return nil, fmt.Errorf("parse %s: record delimiter is missing", label)
		}
	}
	return rows, nil
}

func readHistoryFields(reader *nulReader, label string) ([]byte, []byte, []byte, []byte, error) {
	fields := [4][]byte{}
	for index := range fields {
		field, ok := reader.next()
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("parse %s: truncated record", label)
		}
		fields[index] = field
	}
	return fields[0], fields[1], fields[2], fields[3], nil
}

func parseCommitLog(data []byte) ([]Commit, error) {
	rows, err := parseHistoryRows(data, "git log")
	if err != nil {
		return nil, err
	}
	commits := make([]Commit, len(rows))
	for index, row := range rows {
		commits[index] = Commit{
			OID:          row.oid,
			ShortOID:     row.shortOID,
			Author:       row.author,
			AuthoredUnix: row.authoredUnix,
			Subject:      row.subject,
		}
	}
	return commits, nil
}

func parseRefCommitLog(data []byte) ([]RefCommit, error) {
	rows, err := parseHistoryRows(data, "git ref log")
	if err != nil {
		return nil, err
	}
	commits := make([]RefCommit, len(rows))
	for index, row := range rows {
		commits[index] = RefCommit{
			OID:          row.oid,
			ShortOID:     row.shortOID,
			Subject:      row.subject,
			Author:       row.author,
			AuthoredUnix: row.authoredUnix,
		}
	}
	return commits, nil
}

func parseCommitParents(data []byte, oids []string) ([][]string, error) {
	cursor := 0
	result := make([][]string, 0, len(oids))
	for _, oid := range oids {
		parents, next, err := parseCommitParentRecord(data, cursor, oid)
		if err != nil {
			return nil, err
		}
		result = append(result, parents)
		cursor = next
	}
	return result, nil
}

func parseCommitParentRecord(data []byte, cursor int, oid string) ([]string, int, error) {
	if cursor >= len(data) {
		return nil, cursor, fmt.Errorf("parse git cat-file: missing header for %s", oid)
	}
	relativeHeaderEnd := bytes.IndexByte(data[cursor:], '\n')
	if relativeHeaderEnd < 0 {
		return nil, cursor, fmt.Errorf("parse git cat-file: truncated header for %s", oid)
	}
	headerEnd := cursor + relativeHeaderEnd
	header := bytes.Fields(data[cursor:headerEnd])
	if len(header) != 3 || string(header[0]) != oid || !bytes.Equal(header[1], []byte("commit")) {
		return nil, cursor, fmt.Errorf("parse git cat-file: invalid header for %s", oid)
	}
	size, sizeErr := strconv.Atoi(string(header[2]))
	bodyStart := headerEnd + 1
	if sizeErr != nil || size < 0 || size >= len(data)-bodyStart {
		return nil, cursor, fmt.Errorf("parse git cat-file: invalid body for %s", oid)
	}
	bodyEnd := bodyStart + size
	if data[bodyEnd] != '\n' {
		return nil, cursor, fmt.Errorf("parse git cat-file: invalid body for %s", oid)
	}
	parents, err := parseParentHeaders(data[bodyStart:bodyEnd], oid)
	return parents, bodyEnd + 1, err
}

func parseParentHeaders(body []byte, oid string) ([]string, error) {
	parents := make([]string, 0, 2)
	for _, line := range bytes.Split(body, []byte{'\n'}) {
		if len(line) == 0 {
			break
		}
		if parent, ok := bytes.CutPrefix(line, []byte("parent ")); ok {
			if !validObjectID(string(parent)) {
				return nil, fmt.Errorf("parse git cat-file: invalid parent for %s", oid)
			}
			parents = append(parents, string(parent))
		}
	}
	return parents, nil
}

func parseCommitRefs(data []byte) (map[string][]CommitRef, error) {
	result := make(map[string][]CommitRef)
	for _, record := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse git refs: record has %d fields", len(fields))
		}
		oid := string(fields[0])
		if len(fields[1]) != 0 {
			oid = string(fields[1])
		}
		if !validObjectID(oid) {
			// A public ref can legally point at a non-commit object. It cannot
			// decorate a commit row, so leave it out without failing history.
			continue
		}
		name := string(fields[2])
		var reference CommitRef
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			reference = CommitRef{Kind: BranchRef, Name: strings.TrimPrefix(name, "refs/heads/")}
		case strings.HasPrefix(name, "refs/remotes/"):
			reference = CommitRef{Kind: RemoteRef, Name: strings.TrimPrefix(name, "refs/remotes/")}
		case strings.HasPrefix(name, "refs/tags/"):
			reference = CommitRef{Kind: TagRef, Name: strings.TrimPrefix(name, "refs/tags/")}
		default:
			continue
		}
		if reference.Name != "" {
			result[oid] = append(result[oid], reference)
		}
	}
	return result, nil
}

func parseCommitSummary(data []byte) (CommitSummary, int64, int64, error) {
	fields := bytes.SplitN(data, []byte{0}, 6)
	if len(fields) != 6 {
		return CommitSummary{}, 0, 0, fmt.Errorf("parse git show: metadata has %d fields", len(fields))
	}
	stat := bytes.TrimPrefix(fields[5], []byte{'\n'})
	metadataBytes := int64(4)
	for _, field := range fields[:5] {
		metadataBytes += int64(len(field))
	}
	return CommitSummary{
		OID:         string(fields[0]),
		AuthorName:  string(fields[1]),
		AuthorEmail: string(fields[2]),
		AuthoredAt:  string(fields[3]),
		Message:     string(bytes.TrimSuffix(fields[4], []byte{'\n'})),
		Stat:        string(bytes.TrimSuffix(stat, []byte{'\n'})),
	}, metadataBytes, int64(len(stat)), nil
}

func validObjectID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}
