package git

import "fmt"

const DefaultMaxStashBytes int64 = 8 << 20

// ChangeKind describes one path stored in an immutable Git comparison.
type ChangeKind uint8

const (
	ChangeModified ChangeKind = iota + 1
	ChangeAdded
	ChangeDeleted
	ChangeRenamed
	ChangeCopied
	ChangeUntracked
)

// ChangedFile is one machine-parsed path and its line statistics.
type ChangedFile struct {
	Path         string
	PreviousPath string
	Kind         ChangeKind
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// StashEntry is one refs/stash reflog entry. OID, not Selector, is its identity.
type StashEntry struct {
	OID          string
	Selector     string
	Branch       string
	Message      string
	Timestamp    int64
	FileCount    uint64
	Additions    uint64
	Deletions    uint64
	BaseOID      string
	UntrackedOID string
}

// StashSource contains only immutable object identities needed to inspect a stash.
type StashSource struct {
	OID          string
	BaseOID      string
	UntrackedOID string
}

// ListStashes returns the complete stash reflog, newest first, with aggregate stats.
// The reflog selector is presentation only; all comparisons use full object IDs.
func (client Client) ListStashes(root string) ([]StashEntry, error) {
	hasStashes, err := hasStashRef(root)
	if err != nil || !hasStashes {
		return nil, err
	}
	out, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"log",
		"-g",
		"-z",
		"--format=%H%x00%gD%x00%gs%x00%ct%x00%P",
		"--end-of-options",
		"refs/stash",
	)
	if err != nil {
		return nil, err
	}
	entries, err := parseStashLog(out)
	if err != nil {
		return nil, err
	}
	if err := client.populateStashStats(root, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (client Client) populateStashStats(root string, entries []StashEntry) error {
	emptyTree := ""
	for index := range entries {
		changes, err := client.stashChangesForStats(root, entries[index], &emptyTree)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", entries[index].Selector, err)
		}
		entries[index].FileCount = uint64(len(changes))
		for _, change := range changes {
			entries[index].Additions = saturatingAdd(entries[index].Additions, change.Additions)
			entries[index].Deletions = saturatingAdd(entries[index].Deletions, change.Deletions)
		}
	}
	return nil
}

func (client Client) stashChangesForStats(root string, entry StashEntry, emptyTree *string) ([]ChangedFile, error) {
	if entry.UntrackedOID != "" && *emptyTree == "" {
		var err error
		*emptyTree, err = client.EmptyTreeOID(root)
		if err != nil {
			return nil, err
		}
	}
	return client.listStashChanges(root, StashSource{
		OID: entry.OID, BaseOID: entry.BaseOID, UntrackedOID: entry.UntrackedOID,
	}, *emptyTree)
}

// ListStashChanges combines the tracked stash tree with its optional untracked parent.
func (client Client) ListStashChanges(root string, source StashSource) ([]ChangedFile, error) {
	return client.listStashChanges(root, source, "")
}

func (client Client) listStashChanges(root string, source StashSource, emptyTree string) ([]ChangedFile, error) {
	if err := validateStashSource(source); err != nil {
		return nil, err
	}
	changes, err := client.changedBetween(root, source.BaseOID, source.OID)
	if err != nil {
		return nil, err
	}
	if source.UntrackedOID != "" {
		if emptyTree == "" {
			var emptyErr error
			emptyTree, emptyErr = client.EmptyTreeOID(root)
			if emptyErr != nil {
				return nil, emptyErr
			}
		}
		extras, extrasErr := client.changedBetween(root, emptyTree, source.UntrackedOID)
		if extrasErr != nil {
			return nil, extrasErr
		}
		for index := range extras {
			extras[index].Kind = ChangeUntracked
		}
		changes = mergeChanges(changes, extras)
	}
	return changes, nil
}

func (client Client) changedBetween(root, oldOID, newOID string) ([]ChangedFile, error) {
	if !validObjectID(oldOID) || !validObjectID(newOID) {
		return nil, fmt.Errorf("invalid comparison object identity")
	}
	numstat, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"diff",
		"-z",
		"--numstat",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		oldOID,
		newOID,
		"--",
	)
	if err != nil {
		return nil, err
	}
	stats, err := parseNumstatDetails(numstat)
	if err != nil {
		return nil, err
	}
	status, err := runBounded(
		root,
		DefaultMaxStashBytes,
		"diff",
		"-z",
		"--name-status",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		oldOID,
		newOID,
		"--",
	)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(status, stats)
}

func hasStashRef(root string) (bool, error) {
	_, exists, err := resolveCommitOID(root, "refs/stash")
	return exists, err
}
