package app

import (
	"fmt"
	"strings"

	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

func (state refsState) viewModel(geometry ui.Geometry) ui.Model {
	rows := state.navigatorRows()
	return ui.Model{
		Geometry:         geometry,
		NavigatorTitle:   state.navigatorTitle(len(rows)),
		NavigatorRows:    rows,
		NavigatorEmpty:   state.navigatorEmpty(len(rows)),
		Selected:         state.place.Selected,
		Top:              state.place.Top,
		Focus:            state.place.Focus,
		ReaderTitle:      state.readerTitle(),
		ReaderCommitRows: refCommitRows(state.commits),
		ReaderEmpty:      state.readerEmpty(),
		ReaderOffset:     state.place.ReaderOffset,
	}
}

func (state refsState) navigatorRows() []ui.NavigatorRow {
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
	return rows
}

func (state refsState) navigatorEmpty(rowCount int) ui.Line {
	empty := ui.Line{Text: "No refs or worktrees", Tone: ui.ToneQuiet}
	if state.sourceLoading && rowCount == 0 {
		empty.Text = "Loading refs…"
	} else if state.sourceError != nil {
		empty = ui.Line{Text: "Git error: " + state.sourceError.Error(), Tone: ui.ToneError}
	}
	return empty
}

func (state refsState) navigatorTitle(rowCount int) string {
	title := fmt.Sprintf("refs · %d", max(0, rowCount-1))
	if state.sourceLoading {
		title += " · loading"
	} else if state.sourceError != nil {
		title += " · error"
	}
	return title
}

func (state refsState) readerTitle() string {
	title := "history"
	if source, ok := state.selectedSource(); ok {
		tip := source.OID
		if tip == "" && len(state.commits) != 0 {
			tip = state.commits[0].OID
		}
		title += " · " + source.Label
		if tip != "" {
			title += " · " + abbreviateOID(tip)
		}
		title += fmt.Sprintf(" · %d commits", len(state.commits))
	}
	if state.previewLoading {
		title += " · loading…"
	}
	if state.previewError != nil && len(state.commits) != 0 {
		title += " · error"
	}
	return title
}

func (state refsState) readerEmpty() ui.Line {
	empty := ui.Line{Text: "Select a source to preview its history.", Tone: ui.ToneQuiet}
	if state.previewLoading {
		empty.Text = "Loading history…"
	} else if state.previewError != nil {
		empty = ui.Line{Text: "Git error: " + state.previewError.Error(), Tone: ui.ToneError}
	} else if _, ok := state.selectedSource(); ok {
		empty.Text = "No commits reachable from this source."
	}
	return empty
}

func refSourcePresentation(source repository.RefSource) (icon string, tone ui.Tone, trail string) {
	switch source.ID.Kind {
	case repository.RefSourceAll:
		return "", ui.ToneSpecial, "public refs"
	case repository.RefSourceCurrentWorktree:
		return "", ui.ToneAdded, "current worktree"
	case repository.RefSourceLinkedWorktree:
		return "", ui.ToneSpecial, source.Path
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
