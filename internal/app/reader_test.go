package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestSharedChangeReaderMakesSpecialStashStatesExplicit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		document repository.ChangeDocument
		want     string
		tone     ui.Tone
	}{
		{
			name: "binary",
			document: repository.ChangeDocument{
				Change: repository.ChangedFile{Path: "binary.dat", Binary: true},
			},
			want: "Binary file changed", tone: ui.ToneQuiet,
		},
		{
			name: "deleted",
			document: repository.ChangeDocument{
				Change: repository.ChangedFile{Path: "gone.go", Kind: repository.ChangeDeleted},
				Patch:  repository.File{Kind: repository.FileReady, Content: "-gone"},
			},
			want: "Deleted file", tone: ui.ToneQuiet,
		},
		{
			name: "untracked",
			document: repository.ChangeDocument{
				Change: repository.ChangedFile{Path: "new.go", Kind: repository.ChangeUntracked},
				Patch:  repository.File{Kind: repository.FileReady, Content: "+new"},
			},
			want: "Untracked file stored", tone: ui.ToneQuiet,
		},
		{
			name: "too large",
			document: repository.ChangeDocument{
				Change: repository.ChangedFile{Path: "huge.go"},
				Patch:  repository.File{Kind: repository.FileTooLarge, Size: repository.DefaultMaxFileBytes + 1},
			},
			want: "Stash diff is too large", tone: ui.ToneError,
		},
		{
			name: "stale object",
			document: repository.ChangeDocument{
				Change: repository.ChangedFile{Path: "stale.go"},
				Patch:  repository.File{Kind: repository.FileUnreadable, Err: errors.New("bad object")},
			},
			want: "Stash diff is unavailable: bad object", tone: ui.ToneError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rows := (readerDocument{Change: &test.document, ChangeLabel: "Stash", Mode: workspace.DiffReader}).build().Rows
			if len(rows) == 0 || !strings.Contains(rows[0].Text, test.want) || rows[0].Tone != test.tone {
				t.Fatalf("change reader rows = %+v, want %q tone %v", rows, test.want, test.tone)
			}
		})
	}
}
