package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

func TestFilePaneTitlesDescribeContentWithoutPaneNames(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state.tree.Rebuild([]string{"one.go", "path/to/file.ext"})
	state.place.Items = state.tree.Identities()
	state.place.Selected = 1
	state.readerPath = "path/to/file.ext"
	view := state.viewModel(ui.Calculate(80, 20))
	if view.NavigatorTitle != "2 files" || view.ReaderTitle != "path/to/file.ext" {
		t.Fatalf("pane titles = %q / %q", view.NavigatorTitle, view.ReaderTitle)
	}
}

func TestFileSelectionSeparatesTreeCursorFromOpenReader(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, pending := state.landFiles(filesLoadedMsg{
		generation: state.listGeneration,
		files:      []string{"src/a.go", "src/b.go", "root.go"},
	}, 10)
	if pending.identity != "src/a.go" || state.readerPath != "src/a.go" {
		t.Fatalf("initial file = %q effect=%+v", state.readerPath, pending)
	}
	state.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "content"}
	state.readerLoading = false
	state.place.ReaderOffset = 7

	directoryIndex := -1
	for index, identity := range state.place.Items {
		if identity == filetree.DirectoryIdentity("src") {
			directoryIndex = index
		}
	}
	if directoryIndex < 0 {
		t.Fatal("src directory row not found")
	}
	if effect := state.selectIndex(directoryIndex, 10); effect.kind != effectNone {
		t.Fatalf("directory selection effect = %+v", effect)
	}
	if state.readerPath != "src/a.go" || state.reader.Content != "content" || state.place.ReaderOffset != 7 {
		t.Fatalf("directory selection changed reader: %+v", state)
	}

	fileIndex := -1
	for index, identity := range state.place.Items {
		if identity == filetree.FileIdentity("src/b.go") {
			fileIndex = index
		}
	}
	if effect := state.selectIndex(fileIndex, 10); effect.identity != "src/b.go" {
		t.Fatalf("file selection effect = %+v", effect)
	}
	if state.readerPath != "src/b.go" || state.reader.Kind != 0 || state.place.ReaderOffset != 0 {
		t.Fatalf("file selection state = %+v", state)
	}
}

func TestFileRefreshPreservesHiddenOpenFileAndCollapsedDirectory(t *testing.T) {
	t.Parallel()
	state := newFilesState()
	state, _ = state.landFiles(filesLoadedMsg{
		generation: state.listGeneration,
		files:      []string{"src/a.go", "src/b.go", "root.go"},
	}, 10)
	state.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "open"}
	state.readerLoading = false
	state.place.ReaderOffset = 4
	if !state.tree.Collapse(filetree.DirectoryIdentity("src")) {
		t.Fatal("src did not collapse")
	}
	state.place.Reconcile(state.tree.Identities())
	state.place.EnsureSelectionVisible(10)

	refresh := state.reload()
	state, pending := state.landFiles(filesLoadedMsg{
		generation: refresh.generation,
		files:      []string{"src/a.go", "src/b.go", "src/c.go", "root.go"},
	}, 10)

	row, ok := state.tree.Row(filetree.DirectoryIdentity("src"))
	if !ok || row.Expanded {
		t.Fatalf("src after refresh = %#v, %v; want collapsed", row, ok)
	}
	if pending.identity != "src/a.go" || state.readerPath != "src/a.go" || state.reader.Content != "open" || state.place.ReaderOffset != 4 {
		t.Fatalf("hidden open file refresh = effect %+v state %+v", pending, state)
	}
	for _, identity := range state.place.Items {
		if identity == filetree.FileIdentity("src/a.go") {
			t.Fatal("collapsed descendant became visible during refresh")
		}
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
