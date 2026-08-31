package git

import "fmt"

// CommitChangeSource pins one immutable commit comparison to its first parent,
// or to the repository's empty tree for a root commit.
type CommitChangeSource struct {
	OID     string
	BaseOID string
}

// ResolveCommitChangeSource resolves only immutable object identities. It
// never consults the index or worktree after the commit OID has been supplied.
func (client Client) ResolveCommitChangeSource(root, oid string) (CommitChangeSource, error) {
	if !validObjectID(oid) {
		return CommitChangeSource{}, fmt.Errorf("invalid commit object identity")
	}
	parents, err := readCommitParents(root, []string{oid})
	if err != nil {
		return CommitChangeSource{}, err
	}
	base := ""
	if len(parents) != 0 && len(parents[0]) != 0 {
		base = parents[0][0]
	} else {
		base, err = client.EmptyTreeOID(root)
		if err != nil {
			return CommitChangeSource{}, err
		}
	}
	return CommitChangeSource{OID: oid, BaseOID: base}, nil
}

// ListCommitChanges returns the first-parent changed paths for one exact
// commit. The result is bounded and parsed from read-only Git output.
func (client Client) ListCommitChanges(root, oid string) ([]ChangedFile, error) {
	source, err := client.ResolveCommitChangeSource(root, oid)
	if err != nil {
		return nil, err
	}
	return client.changedBetween(root, source.BaseOID, source.OID)
}
