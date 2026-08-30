package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestEditorKeyTargetsCurrentWorktreeFileAndSourceLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := &fakeSource{root: root}
	model := newTestModel(source)
	model.files.readerEntry = repository.Entry{
		Path: "current/name.go", PreviousPath: "historical/name.go", State: repository.FileRenamed,
	}
	model.files.place.ReaderCursor = 1
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Kind: ui.ReaderContext, NewLine: 40},
		{Kind: ui.ReaderInsertion, NewLine: 41},
	}}
	model.files.readerPresentation = &document

	pending := model.apply(Action{Kind: OpenEditor})
	if pending.kind != effectOpenEditor || pending.path != filepath.Join(root, "current", "name.go") || pending.line != 41 {
		t.Fatalf("OpenEditor effect = %+v", pending)
	}
	if strings.Contains(pending.path, "historical") {
		t.Fatalf("editor targeted historical rename path %q", pending.path)
	}
}

func TestResolveEditorInvocationParsesConfigurationAndUsesDialects(t *testing.T) {
	t.Parallel()
	path := "/worktree/a file.go"
	tests := []struct {
		name   string
		visual string
		editor string
		want   editorInvocation
	}{
		{
			name:   "VISUAL wins with quoted executable and arguments",
			visual: `"/opt/Neo Vim/nvim" --clean --cmd "set number"`, editor: "nano",
			want: editorInvocation{name: "/opt/Neo Vim/nvim", args: []string{"--clean", "--cmd", "set number", "+37", path}},
		},
		{
			name:   "EDITOR fallback uses plus line",
			editor: `nano -w`,
			want:   editorInvocation{name: "nano", args: []string{"-w", "+37", path}},
		},
		{
			name:   "helix uses path address",
			visual: "hx",
			want:   editorInvocation{name: "hx", args: []string{path + ":37:1"}},
		},
		{
			name:   "kakoune uses trailing address",
			visual: "kak -n",
			want:   editorInvocation{name: "kak", args: []string{"-n", path, "+37:1"}},
		},
		{
			name:   "VS Code uses goto address",
			visual: "code --wait",
			want:   editorInvocation{name: "code", args: []string{"--wait", "--goto", path + ":37:1"}},
		},
		{
			name:   "JetBrains uses line option",
			visual: "goland --wait",
			want:   editorInvocation{name: "goland", args: []string{"--wait", "--line", "37", path}},
		},
		{
			name:   "unknown editor gets conservative file only fallback",
			visual: `custom-editor "--profile=review mode"`,
			want:   editorInvocation{name: "custom-editor", args: []string{"--profile=review mode", path}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lookup := editorEnvironment(test.visual, test.editor)
			got, err := resolveEditorInvocation(lookup, path, 37)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolveEditorInvocation() = (%+v, %v), want %+v", got, err, test.want)
			}
		})
	}
}

func TestResolveEditorInvocationRejectsInvalidConfigurationWithoutShellEvaluation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		visual string
		editor string
		want   string
	}{
		{name: "unset", want: "VISUAL and EDITOR are unset"},
		{name: "unterminated quote", visual: `nvim "unfinished`, editor: "nano", want: "parse VISUAL: unterminated quote"},
		{name: "empty executable", visual: `"" --wait`, want: "editor executable is empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveEditorInvocation(editorEnvironment(test.visual, test.editor), "/worktree/file", 1)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveEditorInvocation() error = %v, want %q", err, test.want)
			}
		})
	}

	words, err := splitEditorCommand(`nvim -c 'set number' "C:\Program Files\editor.rc"`)
	want := []string{"nvim", "-c", "set number", `C:\Program Files\editor.rc`}
	if err != nil || !reflect.DeepEqual(words, want) {
		t.Fatalf("splitEditorCommand() = (%#v, %v), want %#v", words, err, want)
	}
	words, err = splitEditorCommand(`nvim; touch /tmp/not-run`)
	if err != nil || !reflect.DeepEqual(words, []string{"nvim;", "touch", "/tmp/not-run"}) {
		t.Fatalf("shell metacharacters were interpreted: (%#v, %v)", words, err)
	}
}

func TestEditorProcessErrorsDistinguishLaunchAndTerminalHandoff(t *testing.T) {
	t.Parallel()
	launch := describeEditorProcessError(&exec.Error{Name: "missing-editor", Err: exec.ErrNotFound})
	if launch == nil || !strings.Contains(launch.Error(), "could not launch") {
		t.Fatalf("launch error = %v", launch)
	}
	handoff := describeEditorProcessError(errors.New("restore terminal"))
	if handoff == nil || !strings.Contains(handoff.Error(), "terminal handoff failed") {
		t.Fatalf("handoff error = %v", handoff)
	}
	if err := describeEditorProcessError(nil); err != nil {
		t.Fatalf("successful process error = %v", err)
	}
}

func TestCurrentWorktreeLineMapsNonCurrentRowsToNearestSurvivor(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Kind: ui.ReaderMetadata, Text: "@@"},
		{Kind: ui.ReaderContext, NewLine: 10},
		{Kind: ui.ReaderDeletion, OldLine: 11},
		{Kind: ui.ReaderFold, Text: "20 unchanged lines"},
		{Kind: ui.ReaderFoldEnd, Text: "change resumes"},
		{Kind: ui.ReaderInsertion, NewLine: 31},
		{Kind: ui.ReaderNotice, Text: "notice"},
	}}
	for _, test := range []struct {
		name   string
		cursor int
		want   uint64
	}{
		{name: "metadata chooses next current row", cursor: 0, want: 10},
		{name: "deletion chooses nearest surviving row", cursor: 2, want: 10},
		{name: "fold chooses next survivor on an equal distance", cursor: 3, want: 31},
		{name: "fold end chooses next survivor", cursor: 4, want: 31},
		{name: "notice chooses previous survivor", cursor: 6, want: 31},
		{name: "cursor clamps", cursor: 99, want: 31},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := currentWorktreeLine(document, test.cursor); got != test.want {
				t.Fatalf("currentWorktreeLine(cursor %d) = %d, want %d", test.cursor, got, test.want)
			}
		})
	}
	if got := currentWorktreeLine(ui.ReaderDocument{}, 0); got != 1 {
		t.Fatalf("empty currentWorktreeLine() = %d, want 1", got)
	}
}

func TestEditorCompletionRefreshesAndPreservesFilesContinuity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := &fakeSource{
		root: root,
		snapshot: snapshotOf(
			repository.Entry{Path: "root.go", State: repository.FileModified},
			repository.Entry{Path: "src/a.go"},
			repository.Entry{Path: "src/b.go"},
		),
		diffs: map[string]repository.Diff{
			"root.go": {Entry: repository.Entry{Path: "root.go", State: repository.FileModified}, Kind: repository.DiffReady},
		},
	}
	model := newTestModel(source)
	model.geometry = ui.Calculate(100, 20)
	model.controls.Reader = workspace.DiffReader
	model.files = loadedFilesState(t, source.snapshot.All()...)
	model.files.readerMode = workspace.DiffReader
	model.files.readerEntry = repository.Entry{Path: "root.go", State: repository.FileModified}
	document := foldableDiffDocument()
	model.files.readerPresentation = &document
	fold := document.ContextFoldIdentities()[0]
	model.files.readerContext.restore(false, map[string]bool{fold: true})
	model.files.readerContext.reconcile(document)
	presented := model.files.readerDocument()
	cursor := readerIdentityIndex(presented.Rows, "added")
	if cursor < 0 {
		t.Fatal("fixture added row is not visible")
	}
	model.files.place.Focus = navigation.FocusReader
	model.files.place.ReaderCursor = cursor
	model.files.place.ReaderOffset = cursor
	model.files.comparisonCache[repository.ComparisonBranch] = comparisonCacheEntry{}
	model.files.readerCache[readerCacheSlot{scope: repository.ComparisonBranch}] = readerCacheEntry{}
	selected, _ := model.files.place.SelectedIdentity()
	beforeGeneration := model.files.listGeneration

	next, refresh := model.Update(editorFinishedMsg{err: errors.New("exit status 7")})
	model = next.(Model)
	if refresh == nil || !model.files.listLoading || model.files.listGeneration != beforeGeneration+1 {
		t.Fatalf("completion refresh = command %v files %+v", refresh != nil, model.files)
	}
	if model.files.editorError != "Editor: exit status 7" {
		t.Fatalf("editor error = %q", model.files.editorError)
	}
	if len(model.files.comparisonCache) != 0 || len(model.files.readerCache) != 0 {
		t.Fatalf("completion retained stale comparison caches: %d / %d", len(model.files.comparisonCache), len(model.files.readerCache))
	}
	assertEditorContinuity(t, model, selected, cursor, fold)

	next, reader := model.Update(refresh())
	model = next.(Model)
	if reader == nil {
		t.Fatal("snapshot refresh did not reload the visible diff")
	}
	assertEditorContinuity(t, model, selected, cursor, fold)
	readerResult := reader()
	message, ok := readerResult.(diffLoadedMsg)
	if !ok {
		t.Fatalf("reader refresh message = %T, want diffLoadedMsg", readerResult)
	}
	message.presentation = document
	next, _ = model.Update(message)
	model = next.(Model)
	assertEditorContinuity(t, model, selected, cursor, fold)

	next, refresh = model.Update(editorFinishedMsg{})
	model = next.(Model)
	if refresh == nil || model.files.editorError != "" {
		t.Fatalf("successful completion = command %v editor error %q", refresh != nil, model.files.editorError)
	}
}

func assertEditorContinuity(t *testing.T, model Model, selected string, cursor int, fold string) {
	t.Helper()
	identity, _ := model.files.place.SelectedIdentity()
	row, _ := model.files.tree.Row(filetree.DirectoryIdentity("src"))
	if identity != selected || model.files.place.Focus != navigation.FocusReader ||
		model.files.place.ReaderCursor != cursor || model.files.place.ReaderOffset != cursor ||
		row.Expanded || !model.files.readerContext.target(fold) {
		t.Fatalf("editor refresh lost continuity: place=%+v directory=%+v fold=%v", model.files.place, row, model.files.readerContext.target(fold))
	}
}

func editorEnvironment(visual, editor string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		switch name {
		case "VISUAL":
			return visual, visual != ""
		case "EDITOR":
			return editor, editor != ""
		default:
			return "", false
		}
	}
}
