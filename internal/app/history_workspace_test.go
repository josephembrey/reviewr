package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestHistorySourcesGroupFoldAndChooseUniquePreferredCommit(t *testing.T) {
	t.Parallel()
	const preferred = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	branch := testHistorySource(repository.RefSourceLocalBranch, "refs/heads/topic", preferred)
	sources := []repository.RefSource{
		testHistorySource(repository.RefSourceCurrentWorktree, "/repo", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		branch,
		testHistorySource(repository.RefSourceRemoteBranch, "refs/remotes/origin/topic", "cccccccccccccccccccccccccccccccccccccccc"),
		testHistorySource(repository.RefSourceTag, "refs/tags/v1", "dddddddddddddddddddddddddddddddddddddddd"),
	}
	state := newHistoryState()
	state.preferredOID = preferred
	state, pending := state.landSources(historySourcesLoadedMsg{
		generation: state.sourceGeneration, sources: sources,
	}, 20)
	if state.selectedSource != branch.ID.Key() || pending.kind != effectLoadCommits || pending.query.SourceOID != preferred {
		t.Fatalf("initial source = %q, effect %+v", state.selectedSource, pending)
	}
	selected, _ := state.sourcePlace.SelectedIdentity()
	if selected != branch.ID.Key() {
		t.Fatalf("initial source cursor = %q, want branch identity", selected)
	}
	wantGroups := []historySourceGroup{historyAllRefs, historyWorktrees, historyLocalBranches, historyRemotes, historyTags}
	gotGroups := make([]historySourceGroup, 0, len(wantGroups))
	for _, row := range state.sourceRows {
		if row.source == nil {
			gotGroups = append(gotGroups, row.group)
		}
	}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Fatalf("source groups = %v, want %v", gotGroups, wantGroups)
	}
	state.selectSourceCursor(historyRemotes.identity(), 20)
	state.setSourceGroupExpanded(false, 20)
	if !state.sourceFolds[historyRemotes] {
		t.Fatal("remote group did not collapse")
	}
	for _, row := range state.sourceRows {
		if row.source != nil && row.group == historyRemotes {
			t.Fatalf("collapsed remote survived rows: %+v", row)
		}
	}
	if selected, _ := state.sourcePlace.SelectedIdentity(); selected != historyRemotes.identity() {
		t.Fatalf("fold moved source cursor to %q", selected)
	}

	ambiguous := newHistoryState()
	ambiguous.preferredOID = preferred
	ambiguous, _ = ambiguous.landSources(historySourcesLoadedMsg{
		generation: ambiguous.sourceGeneration,
		sources:    append(sources, testHistorySource(repository.RefSourceTag, "refs/tags/same", preferred)),
	}, 20)
	if ambiguous.selectedSource != repository.AllRefsSource().ID.Key() {
		t.Fatalf("ambiguous preferred commit chose %q instead of All refs", ambiguous.selectedSource)
	}
}

func TestCommitInspectionBackPreservesOverviewAndNestedPlaceExactly(t *testing.T) {
	t.Parallel()
	const oid = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	model := newTestModel(&fakeSource{})
	model.active = workspace.Git
	model.apply(Action{Kind: Resize, Width: 100, Height: 18})
	model.history.sources = []repository.RefSource{repository.AllRefsSource()}
	for index := 0; index < 20; index++ {
		model.history.sources = append(model.history.sources, testHistorySource(
			repository.RefSourceTag, string(rune('a'+index)), string(rune('a'+index))+"-oid",
		))
	}
	model.history.selectedSource = repository.AllRefsSource().ID.Key()
	model.history.rebuildSourceRows()
	model.history.sourcePlace = navigation.State{
		Items: historySourceRowIdentities(model.history.sourceRows), Selected: 1, Top: 1,
	}
	model.history.commits = make([]repository.Commit, 30)
	for index := range model.history.commits {
		commitOID := string(rune('a'+index)) + "-commit"
		model.history.commits[index] = repository.Commit{OID: commitOID, ShortOID: commitOID, Subject: commitOID}
	}
	model.history.commits[17] = repository.Commit{OID: oid, ShortOID: "bbbbbbb", Subject: "selected"}
	model.history.rows = buildCommitRows(model.history.commits, workspace.GitGraph)
	model.history.timelinePlace = navigation.State{Items: commitIdentities(model.history.commits), Selected: 17, Top: 5}
	model.history.focus = workspace.GitTimeline
	overviewSource := model.history.sourcePlace
	overviewTimeline := model.history.timelinePlace

	filesEffect := model.apply(Action{Kind: EnterGit})
	if filesEffect.kind != effectLoadCommitFiles || filesEffect.identity != oid || !model.history.inspecting {
		t.Fatalf("enter inspection = effect %+v state %+v", filesEffect, model.history)
	}
	files := make([]repository.ChangedFile, 30)
	for index := range files {
		files[index] = changeFixture(string(rune('a'+index)) + ".go")
	}
	readerEffect := model.history.landInspectionFiles(commitFilesLoadedMsg{
		generation: filesEffect.generation, oid: oid, files: files,
	})
	model.history.inspection.selectIndex(17, model.gitGeometry.FilesRows.Height)
	readerEffect = model.history.requestSelectedInspectionFile()
	document := stashDocumentFixture(readerEffect.changedFile, "@@ -1,30 +1,30 @@\n"+strings.Repeat(" stable line\n", 30))
	model.history.inspection.landReader(
		readerEffect.generation, oid, readerEffect.changedFile.Identity(), document, changeDiffDocument(document, "Commit"),
	)
	model.history.inspection.place.Top = 5
	model.history.inspection.place.ReaderOffset = 2
	model.history.inspection.place.ReaderColumn = 0
	model.history.inspection.place.ReaderCursor = 2
	model.history.focus = workspace.GitDiff
	inspectionPlace := model.history.inspection.place

	model.apply(Action{Kind: BackGit})
	if model.history.inspecting || model.history.focus != workspace.GitTimeline ||
		!reflect.DeepEqual(model.history.sourcePlace, overviewSource) || !reflect.DeepEqual(model.history.timelinePlace, overviewTimeline) {
		t.Fatalf("back changed overview place: source %+v timeline %+v focus %v", model.history.sourcePlace, model.history.timelinePlace, model.history.focus)
	}
	reenter := model.apply(Action{Kind: EnterGit})
	if reenter.kind != effectLoadCommitFiles || !reflect.DeepEqual(model.history.inspection.place, inspectionPlace) {
		t.Fatalf("re-enter changed nested place: effect %+v place %+v want %+v", reenter, model.history.inspection.place, inspectionPlace)
	}
}

func TestHistoryPollingReconcilesByIdentityWithoutMovingHiddenAuthoredState(t *testing.T) {
	t.Parallel()
	const (
		oidA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		oidB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		oidC = "cccccccccccccccccccccccccccccccccccccccc"
	)
	branch := testHistorySource(repository.RefSourceLocalBranch, "refs/heads/topic", oidC)
	model := newTestModel(&fakeSource{})
	model.active = workspace.Git
	model.controls.Git = workspace.GitHistory
	model.apply(Action{Kind: Resize, Width: 80, Height: 18})
	model.history.sourcesLoaded = true
	model.history.sourceLoading = false
	model.history.sources = []repository.RefSource{repository.AllRefsSource(), branch}
	model.history.selectedSource = branch.ID.Key()
	model.history.rebuildSourceRows()
	model.history.sourcePlace = navigation.State{Items: historySourceRowIdentities(model.history.sourceRows), Selected: 3, Top: 2}
	model.history.commits = []repository.Commit{{OID: oidA}, {OID: oidB}, {OID: oidC}}
	model.history.timelinePlace = navigation.State{Items: []string{oidA, oidB, oidC}, Selected: 1, Top: 1}
	model.history.inspecting = true
	model.history.inspectionOID = oidB
	model.history.inspection.ownerID = oidB
	model.history.inspection.place = navigation.State{Items: []string{"\x00keep.go"}, ReaderOffset: 4, ReaderCursor: 5}
	model.history.focus = workspace.GitDiff
	model.updateGitGeometry()
	beforeInspection := model.history.inspection.place

	model.history.sourceGeneration++
	_, pending := model.landGitResult(historySourcesLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true},
		generation:           model.history.sourceGeneration,
		sources: []repository.RefSource{
			repository.AllRefsSource(),
			testHistorySource(repository.RefSourceLocalBranch, "refs/heads/topic", oidB),
			testHistorySource(repository.RefSourceTag, "refs/tags/new", oidC),
		},
	})
	if pending.kind != effectLoadCommits || !pending.background || model.history.focus != workspace.GitDiff || !model.history.inspecting {
		t.Fatalf("source poll changed focus/drilldown: effect %+v state %+v", pending, model.history)
	}
	model.landGitResult(commitsLoadedMsg{
		repositoryPollResult: repositoryPollResult{background: true}, generation: pending.generation, query: pending.query,
		commits: []repository.Commit{{OID: "new"}, {OID: oidA}, {OID: oidB}, {OID: oidC}},
	})
	selected, _ := model.history.timelinePlace.SelectedIdentity()
	top := model.history.timelinePlace.Items[model.history.timelinePlace.Top]
	if selected != oidB || top != oidB || model.history.focus != workspace.GitDiff || !model.history.inspecting ||
		!reflect.DeepEqual(model.history.inspection.place, beforeInspection) {
		t.Fatalf("poll continuity = selected %q top %q focus %v inspect %v place %+v", selected, top, model.history.focus, model.history.inspecting, model.history.inspection.place)
	}
}

func TestGitDiffClickFocusesReaderBeforeSelectingLine(t *testing.T) {
	t.Parallel()
	document := ui.ReaderDocument{Kind: ui.ReaderDiffDocument, Rows: []ui.ReaderRow{
		{Identity: "line", Kind: ui.ReaderInsertion, Text: "new", NewLine: 1},
	}}
	model := newTestModel(&fakeSource{})
	model.active = workspace.Git
	model.controls.Git = workspace.GitHistory
	model.history.inspecting = true
	model.history.focus = workspace.GitFiles
	model.history.inspection.ownerID = "commit"
	model.history.inspection.readerOwnerID = "commit"
	model.history.inspection.readerFileID = "\x00a.go"
	model.history.inspection.readerPresentation = &document
	model.history.inspection.place.Items = []string{"\x00a.go"}
	model.apply(Action{Kind: Resize, Width: 80, Height: 18})
	layout, ok := model.activeReaderLayout()
	if !ok {
		t.Fatal("commit reader layout unavailable")
	}
	action, handled := model.routeGitClick(tea.MouseClickMsg(tea.Mouse{
		X: layout.Geometry.Code.X, Y: layout.Geometry.Rows.Y, Button: tea.MouseLeft,
	}))
	if !handled || action.Kind != SelectReaderLine || action.GitFocus != workspace.GitDiff {
		t.Fatalf("diff click routed as (%+v, %v)", action, handled)
	}
	model.apply(action)
	if model.history.focus != workspace.GitDiff {
		t.Fatalf("diff click left focus on %v", model.history.focus)
	}
}
