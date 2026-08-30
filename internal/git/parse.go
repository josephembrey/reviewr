package git

import "bytes"

// nulReader walks NUL-delimited machine output without allocating an
// intermediate slice of every field. It retains a final unterminated field so
// parsers cannot silently discard malformed trailing data.
type nulReader struct {
	data   []byte
	offset int
}

func (reader *nulReader) next() ([]byte, bool) {
	if reader.offset >= len(reader.data) {
		return nil, false
	}
	rest := reader.data[reader.offset:]
	if end := bytes.IndexByte(rest, 0); end >= 0 {
		reader.offset += end + 1
		return rest[:end], true
	}
	reader.offset = len(reader.data)
	return rest, true
}

// ParseNUL splits NUL-delimited Git output. Empty fields are ignored and a
// final unterminated field is kept.
func ParseNUL(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	paths := make([]string, 0, bytes.Count(data, []byte{0})+1)
	reader := nulReader{data: data}
	for part, ok := reader.next(); ok; part, ok = reader.next() {
		if len(part) != 0 {
			paths = append(paths, string(part))
		}
	}
	return paths
}
