package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

func TestFilePaneTitlesDescribeContentWithoutPaneNames(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state.place.Items = []string{"one.go", "path/to/file.ext"}
	state.place.Selected = 1
	state.readerPath = "path/to/file.ext"
	view := state.viewModel(ui.Calculate(80, 20))
	if view.NavigatorTitle != "2 files" || view.ReaderTitle != "path/to/file.ext" {
		t.Fatalf("pane titles = %q / %q", view.NavigatorTitle, view.ReaderTitle)
	}
}

func TestFileReaderLinesPreserveExplicitSafetyStates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file repository.File
		want string
		tone ui.Tone
	}{
		{name: "ready", file: repository.File{Kind: repository.FileReady, Content: "one\ntwo"}, want: "one\ntwo"},
		{name: "symlink", file: repository.File{Kind: repository.FileReady, Symlink: true, Content: "target"}, want: "symlink → target"},
		{name: "missing", file: repository.File{Kind: repository.FileMissing}, want: "File is missing from the worktree.", tone: ui.ToneError},
		{name: "unreadable", file: repository.File{Kind: repository.FileUnreadable, Err: errors.New("denied")}, want: "File is unreadable: denied", tone: ui.ToneError},
		{name: "binary", file: repository.File{Kind: repository.FileBinary, Size: 42}, want: "Binary file (42 bytes); plain reader disabled.", tone: ui.ToneError},
		{name: "too large", file: repository.File{Kind: repository.FileTooLarge, Size: repository.DefaultMaxFileBytes + 1}, want: "File is too large", tone: ui.ToneError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lines := fileReaderLines(test.file)
			texts := make([]string, len(lines))
			for index, line := range lines {
				texts[index] = line.Text
				if line.Tone != test.tone {
					t.Fatalf("tone = %v, want %v", line.Tone, test.tone)
				}
			}
			if got := strings.Join(texts, "\n"); !strings.Contains(got, test.want) {
				t.Fatalf("reader lines = %q, want substring %q", got, test.want)
			}
		})
	}
}
