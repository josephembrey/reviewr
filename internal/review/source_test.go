package review

import (
	"errors"
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

func TestBuildDocumentIsExactLinearAndReportsMetadata(t *testing.T) {
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
