package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
	"github.com/josephembrey/reviewr/internal/workspace"
)

type historyState struct {
	place      navigation.State
	commits    []repository.Commit
	rows       []commitrow.Row
	summary    repository.CommitSummary
	summaryOID string
	traversal  workspace.GitTraversal

	listGeneration    uint64
	summaryGeneration uint64
	loaded            bool
	listLoading       bool
	summaryLoading    bool
	listError         error
	summaryError      error
}

func newHistoryState() historyState {
	return historyState{
		place:          navigation.State{Focus: navigation.FocusNavigator},
		listGeneration: 1,
		listLoading:    true,
	}
}

func (state *historyState) reload(traversal workspace.GitTraversal, startOID string) effect {
	state.listGeneration++
	state.listLoading = true
	state.listError = nil
	return effect{
		kind:       effectLoadCommits,
		generation: state.listGeneration,
		query:      commitQuery(traversal, startOID),
	}
}

func (state historyState) landCommits(msg commitsLoadedMsg, visibleRows int) (historyState, effect) {
	if msg.generation != state.listGeneration {
		return state, effect{}
	}
	state.loaded = true
	state.listLoading = false
	state.traversal = traversalForQuery(msg.query)
	if msg.err != nil {
		state.listError = msg.err
		return state, effect{}
	}
	state.listError = nil
	_, hadSelection := state.place.SelectedIdentity()
	state.commits = append([]repository.Commit(nil), msg.commits...)
	state.rows = buildCommitRows(state.commits, state.traversal)
	identities := make([]string, len(state.commits))
	for index, commit := range state.commits {
		identities[index] = commit.OID
	}
	state.place.Reconcile(identities)
	if !hadSelection {
		for index, commit := range state.commits {
			if commit.Head {
				state.place.SelectIndex(index, visibleRows)
				break
			}
		}
	}
	state.place.EnsureSelectionVisible(visibleRows)
	if _, ok := state.place.SelectedIdentity(); !ok {
		state.summaryGeneration++
		state.summary = repository.CommitSummary{}
		state.summaryOID = ""
		state.summaryLoading = false
		state.summaryError = nil
		state.place.ReaderOffset = 0
		return state, effect{}
	}
	return state, state.requestSelectedSummary()
}

func (state historyState) landSummary(msg commitLoadedMsg, visibleRows int) historyState {
	selectedOID, ok := state.place.SelectedIdentity()
	if msg.generation != state.summaryGeneration || !ok || msg.oid != selectedOID || msg.oid != state.summaryOID {
		return state
	}
	state.summaryLoading = false
	state.summaryError = msg.err
	if msg.err == nil {
		state.summary = msg.summary
	}
	state.place.ClampReader(len(commitSummaryLines(state.summary)), visibleRows)
	return state
}

func (state *historyState) requestSelectedSummary() effect {
	oid, ok := state.place.SelectedIdentity()
	if !ok {
		return effect{}
	}
	state.summaryGeneration++
	if state.summaryOID != oid {
		state.summary = repository.CommitSummary{}
	}
	state.summaryOID = oid
	state.summaryLoading = true
	state.summaryError = nil
	return effect{kind: effectLoadCommit, generation: state.summaryGeneration, identity: oid}
}

func (state historyState) viewModel(geometry ui.Geometry) ui.Model {
	rows := make([]ui.NavigatorRow, len(state.commits))
	for index, commit := range state.commits {
		rows[index] = ui.NavigatorRow{Identity: commit.OID, Commit: &state.rows[index]}
	}
	emptyNavigator := ui.Line{Text: "No commits", Tone: ui.ToneQuiet}
	if state.listLoading {
		emptyNavigator.Text = "Loading commits…"
	} else if state.listError != nil {
		emptyNavigator = ui.Line{Text: "Git error: " + state.listError.Error(), Tone: ui.ToneError}
	}

	readerTitle := "No selection"
	if commit, ok := state.selectedCommit(); ok {
		readerTitle = commit.ShortOID
	}
	if state.summaryLoading {
		readerTitle += "  loading…"
	}
	readerEmpty := ui.Line{Text: "Select a commit to inspect its summary.", Tone: ui.ToneQuiet}
	if state.summaryLoading {
		readerEmpty = ui.Line{Text: "Loading commit…", Tone: ui.ToneQuiet}
	} else if state.summaryError != nil {
		readerEmpty = ui.Line{Text: "Git error: " + state.summaryError.Error(), Tone: ui.ToneError}
	}

	navigatorTitle := fmt.Sprintf("commits · %d", len(rows))
	if state.listLoading {
		navigatorTitle += " · loading"
	} else if state.listError != nil {
		navigatorTitle += " · error"
	}
	return ui.Model{
		Geometry:       geometry,
		NavigatorTitle: navigatorTitle,
		NavigatorRows:  rows,
		NavigatorEmpty: emptyNavigator,
		Selected:       state.place.Selected,
		Top:            state.place.Top,
		Focus:          state.place.Focus,
		ReaderTitle:    readerTitle,
		ReaderLines:    commitSummaryLines(state.summary),
		ReaderEmpty:    readerEmpty,
		ReaderOffset:   state.place.ReaderOffset,
	}
}

func (state historyState) selectedCommit() (repository.Commit, bool) {
	oid, ok := state.place.SelectedIdentity()
	if !ok {
		return repository.Commit{}, false
	}
	for _, commit := range state.commits {
		if commit.OID == oid {
			return commit, true
		}
	}
	return repository.Commit{}, false
}

func commitSummaryLines(summary repository.CommitSummary) []ui.Line {
	if summary.OID == "" {
		return nil
	}
	lines := []ui.Line{
		{Text: "commit " + summary.OID},
		{Text: fmt.Sprintf("Author: %s <%s>", summary.AuthorName, summary.AuthorEmail)},
		{Text: "Date:   " + summary.AuthoredAt},
		{},
	}
	for _, line := range ui.SafeContentLines(summary.Message) {
		lines = append(lines, ui.Line{Text: line})
	}
	if strings.TrimSpace(summary.Stat) != "" {
		lines = append(lines, ui.Line{})
		for _, line := range ui.SafeContentLines(summary.Stat) {
			lines = append(lines, ui.Line{Text: line})
		}
	}
	return lines
}
