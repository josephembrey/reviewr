package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestStashStatePreservesCollectionFileAndDiffPlaceByIdentity(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.listGeneration = 1
	state.listLoading = true
	state, files := state.landStashes(stashesLoadedMsg{
		generation: 1,
		stashes: []repository.Stash{
			stashFixture("a", "stash@{0}"),
			stashFixture("b", "stash@{1}"),
			stashFixture("c", "stash@{2}"),
		},
	}, 5)
	files = state.selectStashIndex(1, 5)
	reader := state.landFiles(stashFilesLoadedMsg{
		generation: files.generation, oid: "b",
		files: []repository.ChangedFile{changeFixture("one.go"), changeFixture("two.go")},
	})
	state.landReader(stashFileLoadedMsg{
		generation: reader.generation, oid: "b", fileIdentity: reader.changedFile.Identity(),
		document: stashDocumentFixture(reader.changedFile, strings.Repeat("line\n", 30)),
	})
	reader = state.selectFileDelta(1, 8)
	state.landReader(stashFileLoadedMsg{
		generation: reader.generation, oid: "b", fileIdentity: reader.changedFile.Identity(),
		document: stashDocumentFixture(reader.changedFile, strings.Repeat("other\n", 30)),
	})
	state.inspection.place.Top = 1
	state.inspection.place.ReaderOffset = 5
	state.inspection.place.ReaderCursor = 6
	state.inspection.saveReaderPlace()
	beforeFocus := workspace.GitDiff
	state.focus = beforeFocus

	reload := state.poll()
	state, files = state.landStashes(stashesLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true}, generation: reload.generation,
		stashes: []repository.Stash{
			stashFixture("inserted", "stash@{0}"),
			stashFixture("a", "stash@{1}"),
			stashFixture("b", "stash@{2}"),
			stashFixture("c", "stash@{3}"),
		},
	}, 5)
	selected, _ := state.place.SelectedIdentity()
	if selected != "b" || state.place.Selected != 2 || state.focus != beforeFocus || files.kind != effectLoadStashFiles || !files.background {
		t.Fatalf("stash reconciliation = selected %q place %+v focus %v effect %+v", selected, state.place, state.focus, files)
	}
	reader = state.landFiles(stashFilesLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true}, generation: files.generation, oid: "b",
		files: []repository.ChangedFile{changeFixture("one.go"), changeFixture("two.go")},
	})
	file, _ := state.inspection.place.SelectedIdentity()
	if file != changeFixture("two.go").Identity() || state.inspection.place.Top != 1 ||
		state.inspection.place.ReaderOffset != 5 || state.inspection.place.ReaderCursor != 6 || reader.kind != effectLoadStashFile {
		t.Fatalf("stash nested place = file %q place %+v effect %+v", file, state.inspection.place, reader)
	}
}

func TestStashViewAlwaysExposesStashFilesAndDominantDiff(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.stashes = []repository.Stash{stashFixture("oid", "stash@{0}")}
	state.place = navigation.State{Items: []string{"oid"}}
	state.focus = workspace.GitFiles
	state.inspection.ownerID = "oid"
	state.inspection.files = []repository.ChangedFile{changeFixture("a.go"), changeFixture("b.go")}
	state.inspection.place = navigation.State{Items: changedFileIdentities(state.inspection.files), Selected: 1}
	state.inspection.readerOwnerID = "oid"
	state.inspection.readerFileID = state.inspection.files[1].Identity()
	state.inspection.reader = stashDocumentFixture(state.inspection.files[1], "@@ -1 +1 @@\n-old\n+new")

	wideGeometry := ui.CalculateGitGeometry(ui.Calculate(120, 18), ui.GitStashesLayout, ui.GitWidths{})
	presentation := state.viewModel(wideGeometry, time.Unix(2_000_000_000, 0))
	if wideGeometry.FilesStacked || presentation.RailTitle == "" || len(presentation.FilesRows) != 2 ||
		presentation.ReaderDocument.Kind != ui.ReaderDiffDocument || wideGeometry.Content.Width <= wideGeometry.Files.Width {
		t.Fatalf("wide stash layout/presentation = geometry %+v model %+v", wideGeometry, presentation)
	}
	if presentation.Focus != workspace.GitFiles || presentation.FilesSelected != 1 || !strings.Contains(presentation.ReaderTitle, "b.go") {
		t.Fatalf("stash visible regions lost authored focus/place: %+v", presentation)
	}

	narrow := ui.CalculateGitGeometry(ui.Calculate(60, 18), ui.GitStashesLayout, ui.GitWidths{})
	if !narrow.FilesStacked || narrow.Rail.Width <= 0 || narrow.Files.Height <= 0 || narrow.Content.Height <= narrow.Files.Height {
		t.Fatalf("responsive stash layout = %+v", narrow)
	}
	for _, point := range []struct {
		x, y  int
		focus workspace.GitFocus
	}{
		{narrow.RailRows.X, narrow.RailRows.Y, workspace.GitStash},
		{narrow.FilesRows.X, narrow.FilesRows.Y, workspace.GitFiles},
		{narrow.ContentRows.X, narrow.ContentRows.Y, workspace.GitDiff},
	} {
		hit := narrow.HitTest(point.x, point.y, workspace.Git, workspace.Controls{Git: workspace.GitStashes}, ui.GitHitState{
			RailCount: 1, FilesCount: 2, ReaderRows: len(state.readerRows()),
		})
		if hit.Region != point.focus {
			t.Fatalf("stash hit at (%d,%d) = %+v, want %v", point.x, point.y, hit, point.focus)
		}
	}
}

func TestStashBackgroundErrorDoesNotReplaceVisibleInspection(t *testing.T) {
	t.Parallel()
	state := newStashState()
	state.inspection.ownerID = "stash"
	state.inspection.filesGeneration = 3
	state.inspection.files = []repository.ChangedFile{changeFixture("keep.go")}
	state.inspection.place = navigation.State{Items: changedFileIdentities(state.inspection.files), ReaderOffset: 4}
	before := state.inspection
	state.landFiles(stashFilesLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true}, generation: 3, oid: "stash",
		err: errors.New("not found"),
	})
	if !reflect.DeepEqual(state.inspection.files, before.files) || state.inspection.place.ReaderOffset != 4 || state.inspection.filesError != nil {
		t.Fatalf("background error replaced stash inspection: before %+v after %+v", before, state.inspection)
	}
}

func stashFixture(oid, selector string) repository.Stash {
	return repository.Stash{
		OID: oid, Selector: selector, Branch: "main", Message: oid,
		Files: 2, Additions: 2, Deletions: 1,
		Source: repository.ChangeSource{OID: oid, BaseOID: "base-" + oid},
	}
}

func changeFixture(path string) repository.ChangedFile {
	return repository.ChangedFile{Path: path, Kind: repository.ChangeModified, Additions: 1, Deletions: 1}
}

func stashDocumentFixture(change repository.ChangedFile, patch string) repository.ChangeDocument {
	return repository.ChangeDocument{
		Change: change,
		Patch:  repository.File{Path: change.Path, Kind: repository.FileReady, Content: patch},
	}
}
