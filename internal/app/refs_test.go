package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

const (
	refsOIDa = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	refsOIDb = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	refsOIDc = "cccccccccccccccccccccccccccccccccccccccc"
	refsOIDd = "dddddddddddddddddddddddddddddddddddddddd"
)

func TestFirstRefsEntryMapsLogCommitOnlyWhenSourceIsUnambiguous(t *testing.T) {
	all := repository.AllRefsSource()
	current := testRefSource(repository.RefSourceCurrentWorktree, "/repo", "main", refsOIDa)
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	tag := testRefSource(repository.RefSourceTag, "refs/tags/same", "same", refsOIDb)

	for _, test := range []struct {
		name      string
		sources   []repository.RefSource
		preferred string
		want      repository.RefSourceID
	}{
		{name: "one exact source", sources: []repository.RefSource{all, current, branch}, preferred: refsOIDb, want: branch.ID},
		{name: "same tip is ambiguous", sources: []repository.RefSource{all, current, branch, tag}, preferred: refsOIDb, want: current.ID},
		{name: "unmapped commit", sources: []repository.RefSource{all, current, branch}, preferred: refsOIDc, want: current.ID},
		{name: "no worktree falls back all", sources: []repository.RefSource{all, branch}, preferred: refsOIDc, want: all.ID},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := &fakeSource{refSources: test.sources}
			model := newTestModel(source)
			model.active = workspace.Git
			model.geometry = ui.Calculate(80, 20)
			if test.preferred != "" {
				model.history.place = navigation.State{Items: []string{test.preferred}}
			}
			loadSources := model.apply(Action{Kind: ToggleSecondary})
			if model.controls.Git != workspace.GitRefs || loadSources.kind != effectLoadRefSources {
				t.Fatalf("Refs entry = controls %+v effect %+v", model.controls, loadSources)
			}
			next, previewCommand := model.Update(model.command(loadSources)())
			model = next.(Model)
			if model.refs.selected != test.want {
				t.Fatalf("selected source = %+v, want %+v; sources=%#v", model.refs.selected, test.want, test.sources)
			}
			if previewCommand == nil {
				t.Fatal("selected source did not request a preview")
			}
		})
	}
}

func TestRefsEntryAfterFirstVisitPreservesTypedSelection(t *testing.T) {
	all := repository.AllRefsSource()
	current := testRefSource(repository.RefSourceCurrentWorktree, "/repo", "main", refsOIDa)
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	state := newRefsState()
	load := state.enter(refsOIDa)
	state, _ = state.landSources(refSourcesLoadedMsg{generation: load.generation, sources: []repository.RefSource{all, current, branch}}, 8)
	state.selectIndex(2, 8)
	if state.selected != branch.ID {
		t.Fatalf("selected = %+v, want branch", state.selected)
	}

	model := newTestModel(&fakeSource{})
	model.active = workspace.Git
	model.controls.Git = workspace.GitRefs
	model.refs = state
	model.apply(Action{Kind: ToggleSecondary}) // Stashes.
	model.apply(Action{Kind: ToggleSecondary}) // Log.
	if pending := model.apply(Action{Kind: ToggleSecondary}); pending.kind != effectNone {
		t.Fatalf("return to loaded Refs reloaded: %+v", pending)
	}
	if model.refs.selected != branch.ID || model.refs.place.Selected != 2 {
		t.Fatalf("return to Refs reset typed selection: %+v", model.refs)
	}
}

func TestRefsLoadsAreLatestWinsByGenerationAndTypedSource(t *testing.T) {
	t.Parallel()
	all := repository.AllRefsSource()
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	state := newRefsState()
	staleSources := state.enter(refsOIDb)
	currentSources := state.reload()
	stale, pending := state.landSources(refSourcesLoadedMsg{
		generation: staleSources.generation,
		sources:    []repository.RefSource{all, branch},
	}, 4)
	if len(stale.sources) != 0 || pending.kind != effectNone || !stale.sourceLoading {
		t.Fatalf("stale source inventory landed: state=%+v effect=%+v", stale, pending)
	}

	state, branchPreview := state.landSources(refSourcesLoadedMsg{
		generation: currentSources.generation,
		sources:    []repository.RefSource{all, branch},
	}, 4)
	if state.selected != branch.ID {
		t.Fatalf("current source inventory selected %+v, want branch", state.selected)
	}
	allPreview := state.selectIndex(0, 4)
	state = state.landPreview(refCommitsLoadedMsg{
		generation: branchPreview.generation,
		sourceID:   branch.ID,
		commits:    testRefCommits(refsOIDb),
	}, 4)
	if len(state.commits) != 0 || state.selected != all.ID || state.previewSource != all.ID {
		t.Fatalf("stale branch preview landed over All refs: %+v", state)
	}
	state = state.landPreview(refCommitsLoadedMsg{
		generation: allPreview.generation,
		sourceID:   all.ID,
		commits:    testRefCommits(refsOIDa),
	}, 4)
	if len(state.commits) != 1 || state.commits[0].OID != refsOIDa {
		t.Fatalf("current All refs preview did not land: %+v", state)
	}
}

func TestRefsRefreshPreservesIdentityAndPreviewAnchorThenFallsBackToAll(t *testing.T) {
	all := repository.AllRefsSource()
	current := testRefSource(repository.RefSourceCurrentWorktree, "/repo", "main", refsOIDa)
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	tag := testRefSource(repository.RefSourceTag, "refs/tags/topic", "topic-tag", refsOIDb)
	state := newRefsState()
	load := state.enter(refsOIDb)
	var preview effect
	state, preview = state.landSources(refSourcesLoadedMsg{generation: load.generation, sources: []repository.RefSource{all, current, branch, tag}}, 3)
	if state.selected != current.ID {
		t.Fatalf("ambiguous initial tip did not choose current worktree: %+v", state.selected)
	}
	preview = state.selectIndex(2, 3)
	state = state.landPreview(refCommitsLoadedMsg{
		generation: preview.generation,
		sourceID:   branch.ID,
		commits:    testRefCommits(refsOIDa, refsOIDb, refsOIDc, refsOIDd),
	}, 2)
	state.place.ReaderOffset = 1
	state.place.Focus = navigation.FocusReader

	refresh := state.reload()
	state, preview = state.landSources(refSourcesLoadedMsg{
		generation: refresh.generation,
		sources:    []repository.RefSource{all, current, tag, branch},
	}, 3)
	if state.selected != branch.ID || state.place.Focus != navigation.FocusReader {
		t.Fatalf("refresh reset source/focus: selected=%+v place=%+v", state.selected, state.place)
	}
	state = state.landPreview(refCommitsLoadedMsg{
		generation: preview.generation,
		sourceID:   branch.ID,
		commits:    testRefCommits("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", refsOIDa, refsOIDb, refsOIDc, refsOIDd),
	}, 2)
	if state.place.ReaderOffset != 2 {
		t.Fatalf("preview top did not follow commit identity: %+v", state.place)
	}

	refresh = state.reload()
	state, preview = state.landSources(refSourcesLoadedMsg{
		generation: refresh.generation,
		sources:    []repository.RefSource{all, current, tag},
	}, 3)
	if state.selected != all.ID || preview.refSource.ID != all.ID || state.place.Selected != 0 {
		t.Fatalf("removed branch did not fall back to All refs: selected=%+v effect=%+v place=%+v", state.selected, preview, state.place)
	}
	if state.selected == tag.ID {
		t.Fatal("same-tip tag substituted for removed branch")
	}
}

func TestRefsSourceSelectionAndPreviewScrollAreIndependent(t *testing.T) {
	all := repository.AllRefsSource()
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	model := newTestModel(&fakeSource{})
	model.active = workspace.Git
	model.controls.Git = workspace.GitRefs
	model.geometry = ui.Calculate(80, 12)
	model.refs = refsState{
		place: navigation.State{
			Items:    []string{all.ID.Key(), branch.ID.Key()},
			Selected: 0,
			Focus:    navigation.FocusNavigator,
		},
		sources: []repository.RefSource{all, branch},
		commits: testRefCommits(
			refsOIDa, refsOIDb, refsOIDc, refsOIDd,
			strings.Repeat("e", 40), strings.Repeat("f", 40), strings.Repeat("1", 40), strings.Repeat("2", 40),
			strings.Repeat("3", 40), strings.Repeat("4", 40), strings.Repeat("5", 40), strings.Repeat("6", 40),
		),
		selected: all.ID,
		loaded:   true,
		entered:  true,
	}

	readerWheel := tea.MouseWheelMsg(tea.Mouse{
		X:      model.geometry.ReaderRows.X,
		Y:      model.geometry.ReaderRows.Y,
		Button: tea.MouseWheelDown,
	})
	next, command := model.Update(readerWheel)
	model = next.(Model)
	if command != nil || model.refs.place.ReaderOffset == 0 || model.refs.selected != all.ID {
		t.Fatalf("reader wheel crossed source place: state=%+v command=%v", model.refs, command != nil)
	}

	rowY := model.geometry.NavigatorRows.Y + 1
	next, command = model.Update(tea.MouseClickMsg(tea.Mouse{X: model.geometry.NavigatorRows.X, Y: rowY, Button: tea.MouseLeft}))
	model = next.(Model)
	if command == nil || model.refs.selected != branch.ID || model.refs.place.ReaderOffset != 0 || model.refs.place.Focus != navigation.FocusNavigator {
		t.Fatalf("source click = refs %+v command=%v", model.refs, command != nil)
	}
}

func TestRefsPresentationCoversLoadingErrorsAndTypedRowMetadata(t *testing.T) {
	t.Parallel()
	geometry := ui.Calculate(80, 14)
	state := newRefsState()
	state.sourceLoading = true
	view := state.viewModel(geometry)
	if view.NavigatorEmpty.Text != "Loading refs…" {
		t.Fatalf("loading view = %+v", view.NavigatorEmpty)
	}
	state.sourceLoading = false
	state.sourceError = errors.New("broken\x1b[31m")
	view = state.viewModel(geometry)
	if view.NavigatorEmpty.Tone != ui.ToneError || !strings.Contains(view.NavigatorEmpty.Text, "broken") {
		t.Fatalf("source error view = %+v", view.NavigatorEmpty)
	}

	all := repository.AllRefsSource()
	branch := testRefSource(repository.RefSourceLocalBranch, "refs/heads/topic", "topic", refsOIDb)
	branch.Upstream = "origin/topic"
	branch.Tracking = ">"
	state = refsState{
		place:        navigation.State{Items: []string{all.ID.Key(), branch.ID.Key()}, Selected: 1},
		sources:      []repository.RefSource{all, branch},
		selected:     branch.ID,
		previewError: errors.New("preview failed\x1b[31m"),
	}
	view = state.viewModel(geometry)
	if len(view.NavigatorRows) != 2 || view.NavigatorRows[0].Prefix[0].Tone != ui.ToneAccent || view.NavigatorRows[1].Suffix[0].Text != "  origin/topic >" {
		t.Fatalf("typed navigator metadata = %#v", view.NavigatorRows)
	}
	if view.ReaderEmpty.Tone != ui.ToneError || !strings.Contains(view.ReaderTitle, "topic") || !strings.Contains(view.ReaderTitle, refsOIDb[:7]) {
		t.Fatalf("preview error/title = %+v / %q", view.ReaderEmpty, view.ReaderTitle)
	}
}

func TestRefCommitRowsProjectIntoSharedCommitPresentation(t *testing.T) {
	t.Parallel()
	rows := refCommitRows([]repository.RefCommit{{
		OID:          refsOIDa,
		ShortOID:     refsOIDa[:7],
		Subject:      "merge subject",
		Author:       "Ada",
		AuthoredUnix: 123,
		Decorations: []repository.RefDecoration{
			{Kind: repository.RefDecorationBranch, Label: "main"},
			{Kind: repository.RefDecorationRemote, Label: "origin/main"},
			{Kind: repository.RefDecorationTag, Label: "v1"},
		},
		Merge: true,
	}})
	if len(rows) != 1 {
		t.Fatalf("shared rows = %#v", rows)
	}
	row := rows[0]
	if row.OID != refsOIDa || row.ShortOID != refsOIDa[:7] || row.Subject != "merge subject" || row.Author != "Ada" || row.AuthoredUnix != 123 || !row.Merge {
		t.Fatalf("shared row facts = %+v", row)
	}
	wantRefs := []commitrow.Ref{
		{Kind: commitrow.Branch, Name: "main"},
		{Kind: commitrow.Remote, Name: "origin/main"},
		{Kind: commitrow.Tag, Name: "v1"},
	}
	if !reflect.DeepEqual(row.Refs, wantRefs) || strings.TrimSpace(row.Graph.Text()) != "◎" {
		t.Fatalf("shared row presentation = graph %q refs %#v", row.Graph.Text(), row.Refs)
	}
}

func testRefSource(kind repository.RefSourceKind, name, label, oid string) repository.RefSource {
	source := repository.RefSource{
		ID:       repository.RefSourceID{Kind: kind, Name: name},
		Label:    label,
		Revision: name,
		OID:      oid,
	}
	if kind == repository.RefSourceCurrentWorktree || kind == repository.RefSourceLinkedWorktree {
		source.Path = name
		source.Revision = oid
	}
	return source
}

func testRefCommits(oids ...string) []repository.RefCommit {
	commits := make([]repository.RefCommit, len(oids))
	for index, oid := range oids {
		commits[index] = repository.RefCommit{
			OID:      oid,
			ShortOID: oid[:7],
			Subject:  "commit " + oid[:7],
			Author:   "Author",
		}
	}
	return commits
}
