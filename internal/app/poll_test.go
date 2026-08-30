package app

import (
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/filetree"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type pollingFakeSource struct{ *fakeSource }

func (*pollingFakeSource) PollState() (repository.StateFingerprint, error) {
	return repository.StateFingerprint{}, nil
}

func TestRepositoryPollDefersAfterUserActivity(t *testing.T) {
	t.Parallel()
	model := newTestModel(&pollingFakeSource{fakeSource: &fakeSource{}})
	model.poll.deferNext = true
	command := model.beginRepositoryPoll()
	if command == nil {
		t.Fatal("deferred poll did not schedule the next tick")
	}
	if model.poll.running || model.poll.generation != 0 || model.poll.deferNext {
		t.Fatalf("deferred poll started work: %+v", model.poll)
	}
}

func TestRepositoryPollRunsAtAResponsiveBoundedRate(t *testing.T) {
	t.Parallel()
	if repositoryPollInterval != 750*time.Millisecond {
		t.Fatalf("repository poll interval = %v", repositoryPollInterval)
	}
}

func TestRepositoryPollResultsYieldToNewerUserActivity(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.poll = repositoryPollState{
		generation: 4,
		running:    true,
		ready:      true,
		activity:   2,
		fingerprint: repository.StateFingerprint{
			Worktree: "old-worktree",
			Refs:     "old-refs",
		},
	}
	filesGeneration := model.files.listGeneration
	historyGeneration := model.history.listGeneration

	command := model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		activity:   1,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "new-refs"},
	})
	if command == nil || model.poll.running {
		t.Fatalf("stale poll was not retired and rescheduled: %+v", model.poll)
	}
	if model.files.listGeneration != filesGeneration || model.history.listGeneration != historyGeneration ||
		model.poll.fingerprint.Worktree != "old-worktree" {
		t.Fatal("stale poll result changed application state")
	}

	model.files.listGeneration++
	model.poll.activity++
	before := model.files.snapshot
	next, command := model.Update(snapshotLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true, activity: model.poll.activity - 1},
		generation:           model.files.listGeneration,
		snapshot: snapshotOf(
			repository.Entry{Path: "new.go", State: repository.FileUntracked},
		),
	})
	if command != nil || !reflect.DeepEqual(next.(Model).files.snapshot, before) {
		t.Fatal("background load that overlapped input changed the visible snapshot")
	}
}

func TestTurnBaselineSignalSurvivesAnOverlappingUserAction(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.controls.Comparison = workspace.LastTurn
	model.poll = repositoryPollState{
		generation: 4,
		running:    true,
		ready:      true,
		activity:   2,
		fingerprint: repository.StateFingerprint{
			Worktree: "same-worktree",
			Refs:     "same-refs",
		},
	}
	filesGeneration := model.files.listGeneration

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation:          4,
		activity:            1,
		state:               model.poll.fingerprint,
		turnBaselineChanged: true,
	})
	if !model.poll.turnBaselineDirty || model.files.listGeneration != filesGeneration {
		t.Fatalf("overlapped turn signal = poll %+v files generation %d", model.poll, model.files.listGeneration)
	}

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		activity:   2,
		state:      model.poll.fingerprint,
	})
	if model.poll.turnBaselineDirty || model.files.listGeneration != filesGeneration+1 {
		t.Fatalf("retained turn signal = poll %+v files generation %d", model.poll, model.files.listGeneration)
	}
}

func TestEveryBackgroundRepositoryResultUsesTheSharedActivityGate(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.poll.activity = 2
	results := []tea.Msg{
		snapshotLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		reviewDocumentLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		reviewFileLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		fileLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		diffLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		commitsLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		commitLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		refSourcesLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		refCommitsLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		stashesLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		stashFilesLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
		stashFileLoadedMsg{repositoryPollResult: repositoryPollResult{background: true, activity: 1}},
	}
	for _, result := range results {
		if model.acceptsBackgroundResult(result) {
			t.Fatalf("stale %T bypassed the shared activity gate", result)
		}
	}
}

func TestRoutedInputAdvancesRepositoryActivity(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.apply(Action{Kind: Resize, Width: 100, Height: 30})
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	got := next.(Model)
	if got.poll.activity != 1 || !got.poll.deferNext {
		t.Fatalf("repository activity after input = %+v", got.poll)
	}
}

func TestRepositoryPollOnlyReloadsChangedDomainsWithoutLoadingState(t *testing.T) {
	t.Parallel()
	model := newTestModel(&fakeSource{})
	model.files.loaded = true
	model.files.listLoading = false
	model.history.loaded = true
	model.history.listLoading = false
	model.poll = repositoryPollState{
		generation:  4,
		ready:       true,
		fingerprint: repository.StateFingerprint{Worktree: "old-worktree", Refs: "same-refs"},
	}
	filesGeneration := model.files.listGeneration
	historyGeneration := model.history.listGeneration

	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "same-refs"},
	})
	if model.files.listGeneration != filesGeneration+1 {
		t.Fatalf("worktree poll generation = %d", model.files.listGeneration)
	}
	if model.history.listGeneration != historyGeneration {
		t.Fatal("worktree-only poll reloaded history")
	}
	if model.files.listLoading || model.history.listLoading {
		t.Fatalf("background poll exposed loading state: files=%v history=%v", model.files.listLoading, model.history.listLoading)
	}

	filesGeneration = model.files.listGeneration
	historyGeneration = model.history.listGeneration
	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "same-refs"},
	})
	if model.files.listGeneration != filesGeneration || model.history.listGeneration != historyGeneration {
		t.Fatal("unchanged fingerprint caused another reload")
	}

	model.refs.loaded = true
	model.stashes.loaded = true
	refsGeneration := model.refs.sourceGeneration
	stashesGeneration := model.stashes.listGeneration
	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "new-refs"},
	})
	if model.history.listGeneration != historyGeneration+1 || model.refs.sourceGeneration != refsGeneration+1 ||
		model.stashes.listGeneration != stashesGeneration+1 {
		t.Fatalf(
			"ref poll generations = history %d refs %d stashes %d",
			model.history.listGeneration,
			model.refs.sourceGeneration,
			model.stashes.listGeneration,
		)
	}
	if model.history.listLoading || model.refs.sourceLoading || model.stashes.listLoading {
		t.Fatal("ref poll exposed a Git-view loading state")
	}

	model.controls.Comparison = workspace.Branch
	filesGeneration = model.files.listGeneration
	historyGeneration = model.history.listGeneration
	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation: 4,
		state:      repository.StateFingerprint{Worktree: "new-worktree", Refs: "newer-refs"},
	})
	if model.files.listGeneration != filesGeneration+1 || model.history.listGeneration != historyGeneration+1 {
		t.Fatalf("branch ref poll generations = files %d history %d", model.files.listGeneration, model.history.listGeneration)
	}

	model.controls.Comparison = workspace.LastTurn
	filesGeneration = model.files.listGeneration
	historyGeneration = model.history.listGeneration
	_ = model.landRepositoryPoll(repositoryPolledMsg{
		generation:          4,
		state:               repository.StateFingerprint{Worktree: "new-worktree", Refs: "newer-refs"},
		turnBaselineChanged: true,
	})
	if model.files.listGeneration != filesGeneration+1 || model.history.listGeneration != historyGeneration {
		t.Fatalf("turn baseline poll generations = files %d history %d", model.files.listGeneration, model.history.listGeneration)
	}
}

func TestBackgroundSnapshotPreservesPlaceAndReconcilesReaderByLineIdentity(t *testing.T) {
	t.Parallel()
	state := loadedFilesState(t,
		repository.Entry{Path: "src/a.go", State: repository.FileModified},
		repository.Entry{Path: "src/b.go", State: repository.FileModified},
		repository.Entry{Path: "root.go", State: repository.FileUnchanged},
	)
	state.selectIdentity(filetree.DirectoryIdentity("src"))
	if !state.expandSelected(10) {
		t.Fatal("src did not expand")
	}
	state.selectIdentity(filetree.FileIdentity("src/a.go"))
	state.readerEntry = repository.Entry{Path: "src/a.go", State: repository.FileModified}
	state.readerMode = workspace.FileReader
	state.reader = repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "zero\nanchor\ntail"}
	presentation := fileReaderDocument(state.reader, state.readerEntry)
	state.readerPresentation = &presentation
	state.readerLoading = false
	state.place.Focus = navigation.FocusReader
	state.place.ReaderOffset = 1
	beforePlace := state.place

	poll := state.poll("uncommitted")
	if state.listLoading || state.readerLoading || !reflect.DeepEqual(state.place, beforePlace) {
		t.Fatalf("starting background poll moved place or exposed loading: before=%+v after=%+v", beforePlace, state.place)
	}
	state, reader := state.landSnapshot(snapshotLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true},
		generation:           poll.generation, reviewGeneration: poll.reviewGeneration,
		snapshot: snapshotOf(
			repository.Entry{Path: "src/a.go", State: repository.FileModified},
			repository.Entry{Path: "src/b.go", State: repository.FileModified},
			repository.Entry{Path: "src/new.go", State: repository.FileUntracked},
			repository.Entry{Path: "root.go", State: repository.FileUnchanged},
		),
	}, workspace.AllFiles, workspace.FileReader, 10)
	selected, _ := state.place.SelectedIdentity()
	src, _ := state.tree.Row(filetree.DirectoryIdentity("src"))
	if selected != filetree.FileIdentity("src/a.go") || !src.Expanded || state.place.Focus != navigation.FocusReader ||
		state.place.ReaderOffset != 1 || state.readerLoading || state.reader.Content != "zero\nanchor\ntail" {
		t.Fatalf("background snapshot changed visible place: selected=%q src=%+v state=%+v", selected, src, state)
	}
	if reader.kind != effectLoadFile || reader.entry.Path != "src/a.go" {
		t.Fatalf("background reader effect = %+v", reader)
	}

	updated := repository.File{Path: "src/a.go", Kind: repository.FileReady, Content: "inserted\nzero\nanchor\ntail"}
	state = state.landFile(fileLoadedMsg{
		generation:   reader.generation,
		entry:        reader.entry,
		file:         updated,
		presentation: fileReaderDocument(updated, reader.entry),
	})
	if state.place.ReaderOffset != 2 || state.readerRows()[state.place.ReaderOffset].Text != "anchor" || state.place.Focus != navigation.FocusReader {
		t.Fatalf("reader anchor was not reconciled: offset=%d line=%q focus=%v", state.place.ReaderOffset, state.readerRows()[state.place.ReaderOffset].Text, state.place.Focus)
	}
}
