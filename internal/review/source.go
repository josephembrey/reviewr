package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Snapshot is one source-pinned set of concrete changed-file comparisons.
type Snapshot struct {
	Scope       string
	Comparisons map[string]FileComparison
}

// Candidate is typed status input already enumerated by the repository
// snapshot. Providers enrich it with exact endpoint identities; they do not
// enumerate status a second time.
type Candidate struct {
	Path         string
	PreviousPath string
	Action       FileAction
}

// Provider is the additive seam implemented by authoritative typed Git entry
// sources. It deliberately does not enumerate files or infer status.
type Provider interface {
	ReviewComparisons(scope, basis string, candidates []Candidate) (Snapshot, error)
	ReadReviewContent(source EndpointSource, endpoint Endpoint) Content
	ReviewRepositoryID() (RepositoryID, error)
}

// ContentState classifies bounded endpoint content without weakening identity.
type ContentState uint8

const (
	ContentUnavailable ContentState = iota
	ContentText
	ContentBinary
	ContentTooLarge
	ContentAbsent
)

// Content is a bounded materialization of one exact endpoint. Endpoint must
// equal the requested endpoint before the result can be marked or painted current.
type Content struct {
	Endpoint Endpoint
	State    ContentState
	Text     string
	Size     int64
	Err      string
}

// RetainedText returns an independent snapshot only when safe for incremental use.
func (content Content) RetainedText() *string {
	if content.State != ContentText || len(content.Text) > MaxRetainedBytes {
		return nil
	}
	text := content.Text
	return &text
}

// AbsentContent returns the exact empty materialization of a missing endpoint.
func AbsentContent(path string) Content {
	return Content{Endpoint: AbsentEndpoint(path), State: ContentAbsent}
}

// UnavailableContent returns a non-exact result with a concise reason.
func UnavailableContent(path string, kind FileKind, mode uint32, err error) Content {
	reason := "exact content unavailable"
	if err != nil {
		reason = err.Error()
	}
	return Content{Endpoint: Endpoint{Path: path, Kind: kind, Mode: mode}, State: ContentUnavailable, Err: reason}
}

// ReadExactContent streams one complete state through SHA-256 with bounded
// retained memory. Binary and oversized inputs still receive exact identities.
func ReadExactContent(path string, kind FileKind, mode uint32, reader io.Reader, retainLimit int64) Content {
	if kind == Absent {
		return AbsentContent(path)
	}
	if reader == nil {
		return UnavailableContent(path, kind, mode, fmt.Errorf("exact content unavailable"))
	}
	if retainLimit < 0 {
		retainLimit = 0
	}
	contentID, size, binary, text, err := streamExactContent(reader, retainLimit)
	if err != nil {
		return UnavailableContent(path, kind, mode, err)
	}
	endpoint := Endpoint{Path: path, Kind: kind, Mode: mode, ContentID: contentID}
	content := Content{Endpoint: endpoint, Size: size}
	switch {
	case binary:
		content.State = ContentBinary
	case size > retainLimit:
		content.State = ContentTooLarge
	default:
		content.State = ContentText
		content.Text = text
	}
	return content
}

func streamExactContent(reader io.Reader, retainLimit int64) (string, int64, bool, string, error) {
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	retained := bytes.NewBuffer(make([]byte, 0, retainedCapacity(retainLimit)))
	var size int64
	binary := false
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			chunk := buffer[:count]
			_, _ = hash.Write(chunk)
			size += int64(count)
			binary = binary || bytes.IndexByte(chunk, 0) >= 0
			remaining := retainLimit - int64(retained.Len())
			if remaining > 0 {
				keep := int64(count)
				if keep > remaining {
					keep = remaining
				}
				_, _ = retained.Write(chunk[:keep])
			}
		}
		if err == io.EOF {
			contentID := "sha256:" + hex.EncodeToString(hash.Sum(nil))
			return contentID, size, binary, retained.String(), nil
		}
		if err != nil {
			return "", 0, false, "", err
		}
	}
}

func retainedCapacity(limit int64) int {
	const initialMaximum = 64 * 1024
	if limit <= 0 {
		return 0
	}
	if limit < initialMaximum {
		return int(limit)
	}
	return initialMaximum
}
