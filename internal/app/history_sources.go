package app

import (
	"fmt"

	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/ui"
)

type historySourceGroup uint8

const (
	historyAllRefs historySourceGroup = iota
	historyWorktrees
	historyLocalBranches
	historyRemotes
	historyTags
)

var historySourceGroups = [...]historySourceGroup{
	historyAllRefs,
	historyWorktrees,
	historyLocalBranches,
	historyRemotes,
	historyTags,
}

func (group historySourceGroup) identity() string {
	return fmt.Sprintf("history-source-group:%d", group)
}

func (group historySourceGroup) label() string {
	switch group {
	case historyWorktrees:
		return "Worktrees"
	case historyLocalBranches:
		return "Local branches"
	case historyRemotes:
		return "Remotes"
	case historyTags:
		return "Tags"
	default:
		return "All refs"
	}
}

type historySourceRow struct {
	identity string
	group    historySourceGroup
	source   *repository.RefSource
	count    int
}

func (state *historyState) rebuildSourceRows() {
	buckets := make(map[historySourceGroup][]repository.RefSource, len(historySourceGroups))
	for _, source := range state.sources {
		group := sourceGroup(source.ID.Kind)
		buckets[group] = append(buckets[group], source)
	}
	rows := make([]historySourceRow, 0, len(state.sources)+len(historySourceGroups))
	for _, group := range historySourceGroups {
		sources := buckets[group]
		if len(sources) == 0 && group != historyAllRefs {
			continue
		}
		rows = append(rows, historySourceRow{identity: group.identity(), group: group, count: len(sources)})
		if state.sourceFolds[group] {
			continue
		}
		for index := range sources {
			source := sources[index]
			rows = append(rows, historySourceRow{identity: source.ID.Key(), group: group, source: &source})
		}
	}
	state.sourceRows = rows
}

func sourceGroup(kind repository.RefSourceKind) historySourceGroup {
	switch kind {
	case repository.RefSourceCurrentWorktree, repository.RefSourceLinkedWorktree:
		return historyWorktrees
	case repository.RefSourceLocalBranch:
		return historyLocalBranches
	case repository.RefSourceRemoteBranch:
		return historyRemotes
	case repository.RefSourceTag:
		return historyTags
	default:
		return historyAllRefs
	}
}

func historySourceRowIdentities(rows []historySourceRow) []string {
	identities := make([]string, len(rows))
	for index, row := range rows {
		identities[index] = row.identity
	}
	return identities
}

func historySourceRowExists(rows []historySourceRow, identity string) bool {
	for _, row := range rows {
		if row.identity == identity {
			return true
		}
	}
	return false
}

func (state historyState) sourceNavigatorRows() []ui.NavigatorRow {
	rows := make([]ui.NavigatorRow, len(state.sourceRows))
	for index, row := range state.sourceRows {
		if row.source == nil {
			fold := "▾"
			if state.sourceFolds[row.group] {
				fold = "▸"
			}
			rows[index] = ui.NavigatorRow{
				Identity: row.identity,
				Prefix:   []ui.Segment{{Text: fold + " ", Tone: ui.ToneQuiet}},
				Label:    row.group.label(),
				Suffix:   []ui.Segment{{Text: fmt.Sprintf("  %d", row.count), Tone: ui.ToneQuiet}},
			}
			continue
		}
		icon, tone, trail := refSourcePresentation(*row.source)
		active := "  "
		if row.source.ID.Key() == state.selectedSource {
			active = "● "
		}
		rows[index] = ui.NavigatorRow{
			Identity: row.identity,
			Prefix: []ui.Segment{
				{Text: active, Tone: ui.ToneSpecial},
				{Text: icon + " ", Tone: tone},
			},
			Label:  row.source.Label,
			Suffix: []ui.Segment{{Text: "  " + trail, Tone: ui.ToneQuiet}},
		}
	}
	return rows
}

func refSourcePresentation(source repository.RefSource) (icon string, tone ui.Tone, trail string) {
	switch source.ID.Kind {
	case repository.RefSourceAll:
		return "", ui.ToneSpecial, "public refs"
	case repository.RefSourceCurrentWorktree:
		return "", ui.ToneAdded, "current"
	case repository.RefSourceLinkedWorktree:
		return "", ui.ToneSpecial, source.Path
	case repository.RefSourceLocalBranch:
		trail = "local"
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
