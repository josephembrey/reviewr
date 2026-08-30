package review

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestReadExactContentStreamsEveryEndpointState(t *testing.T) {
	cases := []struct {
		name  string
		kind  FileKind
		text  string
		limit int64
		state ContentState
		exact bool
	}{
		{"regular", Regular, "hello\n", 100, ContentText, true},
		{"symlink", Symlink, "../target", 100, ContentText, true},
		{"submodule", Submodule, "0123456789abcdef", 100, ContentText, true},
		{"binary", Regular, "a\x00b", 100, ContentBinary, true},
		{"oversized", Regular, "abcdef", 3, ContentTooLarge, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			content := ReadExactContent("path", test.kind, 0o100644, strings.NewReader(test.text), test.limit)
			if content.State != test.state || content.Endpoint.Exact() != test.exact || content.Size != int64(len(test.text)) {
				t.Fatalf("content = %+v", content)
			}
			if content.Endpoint.ContentID != ContentIdentity([]byte(test.text)) {
				t.Fatalf("identity = %q", content.Endpoint.ContentID)
			}
			if test.state != ContentText && content.Text != "" {
				t.Fatalf("bounded content retained %d bytes", len(content.Text))
			}
		})
	}
	absent := ReadExactContent("gone", Absent, 0, nil, 0)
	if absent.State != ContentAbsent || absent.Endpoint != AbsentEndpoint("gone") {
		t.Fatalf("absent = %+v", absent)
	}
	unavailable := UnavailableContent("bad", Regular, 0o100644, errors.New("denied"))
	if unavailable.State != ContentUnavailable || unavailable.Endpoint.Exact() || unavailable.Err != "denied" {
		t.Fatalf("unavailable = %+v", unavailable)
	}
}

func TestBuildDocumentIsExactAndReportsMetadata(t *testing.T) {
	old := ReadExactContent("a", Regular, 0o100644, strings.NewReader("same\nold\ntail\n"), 100)
	new := ReadExactContent("a", Regular, 0o100644, strings.NewReader("same\nnew\ntail\n"), 100)
	bounds := Bounds{Old: old.Endpoint, New: new.Endpoint}
	document := BuildDocument(bounds, old, new)
	if !document.Exact || document.Added != 1 || document.Removed != 1 || len(document.Lines) != 4 {
		t.Fatalf("document = %+v", document)
	}
	if document.Lines[1].Text != "- old" || document.Lines[2].Text != "+ new" {
		t.Fatalf("diff lines = %+v", document.Lines)
	}
	for _, identity := range document.LineIdentities() {
		if identity == "" {
			t.Fatal("empty line identity")
		}
	}

	modeNew := new
	modeNew.Endpoint.Mode = 0o100755
	modeDocument := BuildDocument(Bounds{Old: old.Endpoint, New: modeNew.Endpoint}, old, modeNew)
	if !modeDocument.Exact || !strings.Contains(modeDocument.Lines[0].Text, "mode") {
		t.Fatalf("mode document = %+v", modeDocument)
	}
}

func TestBuildDocumentKeepsDistantEditsAsSeparateHunks(t *testing.T) {
	oldText, newText := distantReviewContents(600)
	old := ReadExactContent("a", Regular, 0o100644, strings.NewReader(oldText), int64(len(oldText)+1))
	new := ReadExactContent("a", Regular, 0o100644, strings.NewReader(newText), int64(len(newText)+1))
	document := BuildDocument(Bounds{Old: old.Endpoint, New: new.Endpoint}, old, new)
	if !document.Exact || document.Added != 2 || document.Removed != 2 || len(document.Lines) != 602 {
		t.Fatalf("distant edit summary = exact:%v +%d -%d rows:%d", document.Exact, document.Added, document.Removed, len(document.Lines))
	}
	contexts := 0
	for _, line := range document.Lines {
		if line.Kind == ContextLine {
			contexts++
		}
		if line.Text == "  line 300" && (line.Kind != ContextLine || line.OldLine != 301 || line.NewLine != 301) {
			t.Fatalf("middle context was misclassified: %+v", line)
		}
	}
	if contexts != 598 {
		t.Fatalf("unchanged middle collapsed into a replacement: %d context rows", contexts)
	}
}

func TestBuildDocumentReconstructsBothLineSequences(t *testing.T) {
	for _, test := range []struct {
		name string
		old  string
		new  string
	}{
		{name: "identical", old: "same\ntail\n", new: "same\ntail\n"},
		{name: "insert boundaries", old: "middle\n", new: "first\nmiddle\nlast\n"},
		{name: "delete", old: "first\ngone\nlast\n", new: "first\nlast\n"},
		{name: "repeated lines", old: "repeat\nanchor\nrepeat\ntail\n", new: "repeat\ninserted\nanchor\nrepeat\nchanged\n"},
		{name: "no final newline", old: "same\nold", new: "same\nnew"},
		{name: "normalized line endings", old: "same\r\ntail\r\n", new: "same\ntail\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			limit := int64(max(len(test.old), len(test.new)) + 1)
			old := ReadExactContent("a", Regular, 0o100644, strings.NewReader(test.old), limit)
			new := ReadExactContent("a", Regular, 0o100644, strings.NewReader(test.new), limit)
			document := BuildDocument(Bounds{Old: old.Endpoint, New: new.Endpoint}, old, new)
			var oldLines, newLines []string
			oldNo, newNo := 1, 1
			for _, line := range document.Lines {
				if line.Kind == NoticeLine {
					continue
				}
				payload := line.Text[2:]
				switch line.Kind {
				case ContextLine:
					if line.OldLine != oldNo || line.NewLine != newNo {
						t.Fatalf("context numbering = %+v, want %d/%d", line, oldNo, newNo)
					}
					oldLines, newLines = append(oldLines, payload), append(newLines, payload)
					oldNo++
					newNo++
				case RemovedLine:
					if line.OldLine != oldNo || line.NewLine != 0 {
						t.Fatalf("removed numbering = %+v, want %d/0", line, oldNo)
					}
					oldLines = append(oldLines, payload)
					oldNo++
				case AddedLine:
					if line.OldLine != 0 || line.NewLine != newNo {
						t.Fatalf("added numbering = %+v, want 0/%d", line, newNo)
					}
					newLines = append(newLines, payload)
					newNo++
				}
			}
			if want := splitLines(test.old); !slices.Equal(oldLines, want) {
				t.Fatalf("old reconstruction = %#v, want %#v", oldLines, want)
			}
			if want := splitLines(test.new); !slices.Equal(newLines, want) {
				t.Fatalf("new reconstruction = %#v, want %#v", newLines, want)
			}
		})
	}
}

func BenchmarkBuildDocumentDistantEdits(b *testing.B) {
	oldText, newText := distantReviewContents(10_000)
	old := ReadExactContent("a", Regular, 0o100644, strings.NewReader(oldText), int64(len(oldText)+1))
	new := ReadExactContent("a", Regular, 0o100644, strings.NewReader(newText), int64(len(newText)+1))
	bounds := Bounds{Old: old.Endpoint, New: new.Endpoint}
	b.ReportAllocs()
	for b.Loop() {
		document := BuildDocument(bounds, old, new)
		if document.Added != 2 || document.Removed != 2 {
			b.Fatalf("summary = +%d -%d", document.Added, document.Removed)
		}
	}
}

func distantReviewContents(lineCount int) (string, string) {
	oldLines := make([]string, lineCount)
	for index := range oldLines {
		oldLines[index] = fmt.Sprintf("line %d", index)
	}
	newLines := append([]string(nil), oldLines...)
	oldLines[10], newLines[10] = "old near start", "new near start"
	oldLines[lineCount-10], newLines[lineCount-10] = "old near end", "new near end"
	return strings.Join(oldLines, "\n") + "\n", strings.Join(newLines, "\n") + "\n"
}

func TestBuildDocumentHandlesStaleBinaryOversizedDeletionAndUnavailable(t *testing.T) {
	text := ReadExactContent("a", Regular, 0o100644, strings.NewReader("a"), 10)
	newText := ReadExactContent("a", Regular, 0o100644, strings.NewReader("b"), 10)
	stale := BuildDocument(Bounds{Old: text.Endpoint, New: newText.Endpoint}, text, text)
	if stale.Exact || !strings.Contains(stale.Reason, "refresh") {
		t.Fatalf("stale = %+v", stale)
	}

	binary := ReadExactContent("a", Regular, 0o100644, strings.NewReader("b\x00"), 10)
	if document := BuildDocument(Bounds{Old: text.Endpoint, New: binary.Endpoint}, text, binary); !document.Exact || !strings.Contains(document.Lines[0].Text, "Binary") {
		t.Fatalf("binary = %+v", document)
	}
	large := ReadExactContent("a", Regular, 0o100644, strings.NewReader("large"), 2)
	if document := BuildDocument(Bounds{Old: text.Endpoint, New: large.Endpoint}, text, large); !document.Exact || !strings.Contains(document.Lines[0].Text, "too large") {
		t.Fatalf("large = %+v", document)
	}
	absent := AbsentContent("a")
	if document := BuildDocument(Bounds{Old: text.Endpoint, New: absent.Endpoint}, text, absent); !document.Exact || document.Removed != 1 {
		t.Fatalf("deletion = %+v", document)
	}
	unavailable := UnavailableContent("a", Regular, 0o100644, errors.New("gone"))
	if document := BuildDocument(Bounds{Old: text.Endpoint, New: unavailable.Endpoint}, text, unavailable); document.Exact || !strings.Contains(document.Reason, "refresh") {
		t.Fatalf("unavailable = %+v", document)
	}
}
