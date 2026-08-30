package git

import (
	"fmt"
	"strconv"
)

// ListRefCommits returns a bounded compact history for one exact source.
func (Client) ListRefCommits(root string, source RefSource) ([]RefCommit, error) {
	revisions, err := refHistoryRevisions(root, source)
	if err != nil || len(revisions) == 0 {
		return nil, err
	}
	args := []string{
		"log",
		"-z",
		"--topo-order",
		"--no-color",
		"--no-show-signature",
		"--max-count=" + strconv.Itoa(CommitLimit),
		commitLogFormat,
	}
	args = append(args, revisions...)
	out, err := runBounded(root, DefaultMaxHistoryBytes, args...)
	if err != nil {
		return nil, err
	}
	commits, err := parseRefCommitLog(out)
	if err != nil {
		return nil, err
	}
	oids := make([]string, len(commits))
	for index, commit := range commits {
		oids[index] = commit.OID
	}
	parents, err := readCommitParents(root, oids)
	if err != nil {
		return nil, err
	}
	decorations, _, err := listCommitRefs(root)
	if err != nil {
		return nil, err
	}
	decorateRefCommits(commits, parents, decorations)
	return commits, nil
}

func refHistoryRevisions(root string, source RefSource) ([]string, error) {
	switch source.Kind() {
	case RefSourceAll:
		revisions := []string{"--branches", "--remotes", "--tags"}
		_, hasCurrentHead, err := resolveHead(root)
		if err != nil {
			return nil, err
		}
		if hasCurrentHead {
			revisions = append(revisions, "HEAD")
		}
		return revisions, nil
	case RefSourceCurrentWorktree, RefSourceLinkedWorktree:
		if source.Revision == "" {
			return nil, nil
		}
		if source.Revision != source.OID || !validObjectID(source.Revision) {
			return nil, fmt.Errorf("invalid worktree history source")
		}
		return []string{"--end-of-options", source.Revision}, nil
	case RefSourceLocalBranch:
		if !validNamedSource(source, "refs/heads/") {
			return nil, fmt.Errorf("invalid local branch history source")
		}
		return []string{"--end-of-options", source.Revision}, nil
	case RefSourceRemoteBranch:
		if !validNamedSource(source, "refs/remotes/") {
			return nil, fmt.Errorf("invalid remote branch history source")
		}
		return []string{"--end-of-options", source.Revision}, nil
	case RefSourceTag:
		if !validNamedSource(source, "refs/tags/") {
			return nil, fmt.Errorf("invalid tag history source")
		}
		return []string{"--end-of-options", source.Revision}, nil
	default:
		return nil, fmt.Errorf("invalid ref history source kind %d", source.Kind())
	}
}

func decorateRefCommits(commits []RefCommit, parents [][]string, decorations map[string][]CommitRef) {
	for index := range commits {
		commits[index].Merge = len(parents[index]) > 1
		for _, reference := range decorations[commits[index].OID] {
			kind := RefDecorationBranch
			switch reference.Kind {
			case RemoteRef:
				kind = RefDecorationRemote
			case TagRef:
				kind = RefDecorationTag
			}
			commits[index].Decorations = append(commits[index].Decorations, RefDecoration{
				Kind:  kind,
				Label: reference.Name,
			})
		}
	}
}
