package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type refsState struct {
	place    navigation.State
	sources  []repository.RefSource
	commits  []repository.RefCommit
	selected repository.RefSourceID

	sourceGeneration  uint64
	previewGeneration uint64
	loaded            bool
	entered           bool
	sourceLoading     bool
	previewLoading    bool
	sourceError       error
	previewError      error
	preferredOID      string
	previewSource     repository.RefSourceID
	preservePreview   bool
}

func newRefsState() refsState {
	return refsState{place: navigation.State{Focus: navigation.FocusNavigator}}
}

func (state *refsState) enter(preferredOID string) effect {
	if !state.entered {
		state.entered = true
		state.preferredOID = preferredOID
	}
	if !state.loaded && !state.sourceLoading {
		return state.reload()
	}
	return effect{}
}

func (state *refsState) reload() effect {
	state.sourceGeneration++
	state.sourceLoading = true
	state.sourceError = nil
	return effect{kind: effectLoadRefSources, generation: state.sourceGeneration}
}

func (state refsState) landSources(msg refSourcesLoadedMsg, visibleRows int) (refsState, effect) {
	if msg.generation != state.sourceGeneration {
		return state, effect{}
	}
	firstLoad := !state.loaded
	state.loaded = true
	state.sourceLoading = false
	if msg.err != nil {
		state.sourceError = msg.err
		return state, effect{}
	}
	state.sourceError = nil
	oldSelected := state.selected
	state.sources = append([]repository.RefSource(nil), msg.sources...)
	identities := make([]string, len(state.sources))
	for index, source := range state.sources {
		identities[index] = source.ID.Key()
	}
	state.place.Reconcile(identities)

	target := 0
	if firstLoad {
		target = initialRefSourceIndex(state.sources, state.preferredOID)
	} else if index, ok := refSourceIndex(state.sources, oldSelected); ok {
		target = index
	} else {
		target, _ = refSourceIndex(state.sources, repository.AllRefsSource().ID)
	}
	if len(state.sources) == 0 {
		state.selected = repository.RefSourceID{}
		state.commits = nil
		state.previewSource = repository.RefSourceID{}
		state.previewLoading = false
		state.previewError = nil
		state.place.ReaderOffset = 0
		return state, effect{}
	}
	target = min(max(0, target), len(state.sources)-1)
	state.place.Selected = target
	state.selected = state.sources[target].ID
	state.place.EnsureSelectionVisible(visibleRows)
	preserve := !firstLoad && state.selected == oldSelected
	return state, state.requestPreview(state.sources[target], preserve)
}

func (state refsState) landPreview(msg refCommitsLoadedMsg, visibleRows int) refsState {
	if msg.generation != state.previewGeneration || msg.sourceID != state.previewSource || msg.sourceID != state.selected {
		return state
	}
	state.previewLoading = false
	if msg.err != nil {
		state.previewError = msg.err
		return state
	}
	state.previewError = nil
	oldCommits := state.commits
	oldOffset := state.place.ReaderOffset
	state.commits = append([]repository.RefCommit(nil), msg.commits...)
	if state.preservePreview {
		state.place.ReaderOffset = reconcilePreviewOffset(oldCommits, oldOffset, state.commits)
	} else {
		state.place.ReaderOffset = 0
	}
	state.place.ClampReader(len(state.commits), visibleRows)
	return state
}

func (state *refsState) selectDelta(delta, visibleRows int) effect {
	if !state.place.SelectDelta(delta, visibleRows) {
		return effect{}
	}
	return state.selectCurrent()
}

func (state *refsState) selectIndex(index, visibleRows int) effect {
	if !state.place.SelectIndex(index, visibleRows) {
		return effect{}
	}
	return state.selectCurrent()
}

func (state *refsState) selectCurrent() effect {
	if state.place.Selected < 0 || state.place.Selected >= len(state.sources) {
		return effect{}
	}
	source := state.sources[state.place.Selected]
	state.selected = source.ID
	return state.requestPreview(source, false)
}

func (state *refsState) requestPreview(source repository.RefSource, preserve bool) effect {
	state.previewGeneration++
	state.previewSource = source.ID
	state.previewLoading = true
	state.previewError = nil
	state.preservePreview = preserve
	if !preserve {
		state.commits = nil
		state.place.ReaderOffset = 0
	}
	return effect{
		kind:       effectLoadRefCommits,
		generation: state.previewGeneration,
		refSource:  source,
	}
}

func (state refsState) viewModel(geometry ui.Geometry) ui.Model {
	rows := make([]ui.NavigatorRow, len(state.sources))
	for index, source := range state.sources {
		icon, tone, trail := refSourcePresentation(source)
		rows[index] = ui.NavigatorRow{
			Identity: source.ID.Key(),
			Label:    source.Label,
			Prefix:   []ui.Segment{{Text: " " + icon + " ", Tone: tone}},
			Suffix:   []ui.Segment{{Text: "  " + trail, Tone: ui.ToneQuiet}},
		}
	}
	navigatorEmpty := ui.Line{Text: "No refs or worktrees", Tone: ui.ToneQuiet}
	if state.sourceLoading && len(rows) == 0 {
		navigatorEmpty.Text = "Loading refs…"
	} else if state.sourceError != nil {
		navigatorEmpty = ui.Line{Text: "Git error: " + state.sourceError.Error(), Tone: ui.ToneError}
	}

	readerTitle := "history"
	if source, ok := state.selectedSource(); ok {
		tip := source.OID
		if tip == "" && len(state.commits) != 0 {
			tip = state.commits[0].OID
		}
		readerTitle += " · " + source.Label
		if tip != "" {
			readerTitle += " · " + abbreviateOID(tip)
		}
		readerTitle += fmt.Sprintf(" · %d commits", len(state.commits))
	}
	if state.previewLoading {
		readerTitle += " · loading…"
	}

	readerEmpty := ui.Line{Text: "Select a source to preview its history.", Tone: ui.ToneQuiet}
	if state.previewLoading {
		readerEmpty.Text = "Loading history…"
	} else if state.previewError != nil {
		readerEmpty = ui.Line{Text: "Git error: " + state.previewError.Error(), Tone: ui.ToneError}
	} else if _, ok := state.selectedSource(); ok {
		readerEmpty.Text = "No commits reachable from this source."
	}

	navigatorTitle := fmt.Sprintf("refs · %d", max(0, len(rows)-1))
	if state.sourceLoading {
		navigatorTitle += " · loading"
	} else if state.sourceError != nil {
		navigatorTitle += " · error"
	}
	if state.previewError != nil && len(state.commits) != 0 {
		readerTitle += " · error"
	}
	return ui.Model{
		Geometry:         geometry,
		NavigatorTitle:   navigatorTitle,
		NavigatorRows:    rows,
		NavigatorEmpty:   navigatorEmpty,
		Selected:         state.place.Selected,
		Top:              state.place.Top,
		Focus:            state.place.Focus,
		ReaderTitle:      readerTitle,
		ReaderCommitRows: refCommitRows(state.commits),
		ReaderEmpty:      readerEmpty,
		ReaderOffset:     state.place.ReaderOffset,
	}
}

func (state refsState) selectedSource() (repository.RefSource, bool) {
	if index, ok := refSourceIndex(state.sources, state.selected); ok {
		return state.sources[index], true
	}
	return repository.RefSource{}, false
}

func initialRefSourceIndex(sources []repository.RefSource, preferredOID string) int {
	match := -1
	if preferredOID != "" {
		for index, source := range sources {
			if source.ID.Kind == repository.RefSourceAll || source.OID != preferredOID {
				continue
			}
			if match != -1 {
				match = -1
				break
			}
			match = index
		}
	}
	if match >= 0 {
		return match
	}
	for index, source := range sources {
		if source.ID.Kind == repository.RefSourceCurrentWorktree {
			return index
		}
	}
	if index, ok := refSourceIndex(sources, repository.AllRefsSource().ID); ok {
		return index
	}
	return 0
}

func refSourceIndex(sources []repository.RefSource, id repository.RefSourceID) (int, bool) {
	for index, source := range sources {
		if source.ID == id {
			return index, true
		}
	}
	return 0, false
}

func reconcilePreviewOffset(old []repository.RefCommit, oldOffset int, current []repository.RefCommit) int {
	if len(current) == 0 {
		return 0
	}
	oldIDs := make([]string, len(old))
	for index, commit := range old {
		oldIDs[index] = commit.OID
	}
	currentIDs := make([]string, len(current))
	for index, commit := range current {
		currentIDs[index] = commit.OID
	}
	place := navigation.State{Items: oldIDs, Selected: oldOffset, Top: oldOffset}
	place.Reconcile(currentIDs)
	return place.Selected
}

func refSourcePresentation(source repository.RefSource) (icon string, tone ui.Tone, trail string) {
	switch source.ID.Kind {
	case repository.RefSourceAll:
		return "", ui.ToneAccent, "public refs"
	case repository.RefSourceCurrentWorktree:
		return "", ui.ToneAdded, "current worktree"
	case repository.RefSourceLinkedWorktree:
		return "", ui.ToneAccent, source.Path
	case repository.RefSourceLocalBranch:
		trail = "local branch"
		if source.Upstream != "" {
			trail = source.Upstream
			if source.Tracking != "" {
				trail += " " + source.Tracking
			}
		}
		return "", ui.ToneAdded, trail
	case repository.RefSourceRemoteBranch:
		return "", ui.ToneInfo, "remote"
	case repository.RefSourceTag:
		return "", ui.ToneWarning, "tag"
	default:
		return "·", ui.ToneQuiet, ""
	}
}

func refCommitRows(commits []repository.RefCommit) []commitrow.Row {
	rows := make([]commitrow.Row, len(commits))
	for index, commit := range commits {
		refs := make([]commitrow.Ref, 0, len(commit.Decorations))
		for _, decoration := range commit.Decorations {
			kind := commitrow.Branch
			switch decoration.Kind {
			case repository.RefDecorationRemote:
				kind = commitrow.Remote
			case repository.RefDecorationTag:
				kind = commitrow.Tag
			}
			refs = append(refs, commitrow.Ref{Kind: kind, Name: decoration.Label})
		}
		graph := commitgraph.Layout([]commitgraph.Commit{{
			OID:   commit.OID,
			Merge: commit.Merge,
		}})
		rows[index] = commitrow.Row{
			Graph:        graph[0],
			OID:          commit.OID,
			ShortOID:     commit.ShortOID,
			Subject:      commit.Subject,
			Author:       commit.Author,
			AuthoredUnix: commit.AuthoredUnix,
			Refs:         refs,
			Merge:        commit.Merge,
		}
	}
	return rows
}

func abbreviateOID(oid string) string {
	oid = strings.TrimSpace(oid)
	if len(oid) <= 7 {
		return oid
	}
	return oid[:7]
}
