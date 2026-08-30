package git

import (
	"bytes"
	"testing"
)

var benchmarkEntries []FileEntry
var benchmarkPaths []string

func BenchmarkParseNUL(b *testing.B) {
	data := bytes.Repeat([]byte("directory/hostile name.txt\x00"), 10_000)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		benchmarkPaths = ParseNUL(data)
	}
}

func BenchmarkParsePorcelainV2(b *testing.B) {
	data := bytes.Repeat([]byte("1 .M N... 100644 100644 100644 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb path with spaces.txt\x00"), 10_000)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		var err error
		benchmarkEntries, err = ParsePorcelainV2(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
