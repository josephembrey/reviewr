package repository

import gitadapter "github.com/josephembrey/reviewr/internal/git"

// RefSourceKind classifies one source in the read-only refs browser.
type RefSourceKind uint8

const (
	RefSourceAll RefSourceKind = iota + 1
	RefSourceCurrentWorktree
	RefSourceLinkedWorktree
	RefSourceLocalBranch
	RefSourceRemoteBranch
	RefSourceTag
)

// RefSourceID is the typed stable identity of a source. OID and display label
// are intentionally excluded so equal tips and renamed labels cannot alias.
type RefSourceID struct {
	Kind RefSourceKind
	Name string
}

// Key adapts a typed source identity to generic navigation storage.
func (id RefSourceID) Key() string {
	return gitadapter.RefSourceID{Kind: gitadapter.RefSourceKind(id.Kind), Name: id.Name}.Key()
}

// RefSource is a branch, remote, tag, worktree, or the synthetic All refs row.
type RefSource struct {
	ID       RefSourceID
	Label    string
	Revision string
	OID      string
	Path     string
	Branch   string
	Upstream string
	Tracking string
	Remote   string
	UnixTime int64
}

// RefDecorationKind classifies a public ref decorating a commit preview row.
type RefDecorationKind uint8

const (
	RefDecorationBranch RefDecorationKind = iota + 1
	RefDecorationRemote
	RefDecorationTag
)

// RefDecoration is a typed public ref pointing at a commit.
type RefDecoration struct {
	Kind  RefDecorationKind
	Label string
}

// RefCommit is one compact log-style preview row.
type RefCommit struct {
	OID          string
	ShortOID     string
	Subject      string
	Author       string
	AuthoredUnix int64
	Decorations  []RefDecoration
	Merge        bool
}

// AllRefsSource returns the synthetic complete public-ref source.
func AllRefsSource() RefSource {
	return RefSource{ID: RefSourceID{Kind: RefSourceAll}, Label: "All refs"}
}

// ListRefSources discovers every public refs-browser source without mutation.
func (r *Repository) ListRefSources() ([]RefSource, error) {
	sources, err := r.git.ListRefSources(r.root)
	if err != nil {
		return nil, err
	}
	result := make([]RefSource, len(sources))
	for index, source := range sources {
		result[index] = refSourceFromGit(source)
	}
	return result, nil
}

// ListRefCommits returns a bounded history for one exact typed source.
func (r *Repository) ListRefCommits(source RefSource) ([]RefCommit, error) {
	commits, err := r.git.ListRefCommits(r.root, refSourceToGit(source))
	if err != nil {
		return nil, err
	}
	result := make([]RefCommit, len(commits))
	for index, commit := range commits {
		decorations := make([]RefDecoration, len(commit.Decorations))
		for decorationIndex, decoration := range commit.Decorations {
			decorations[decorationIndex] = RefDecoration{
				Kind:  RefDecorationKind(decoration.Kind),
				Label: decoration.Label,
			}
		}
		result[index] = RefCommit{
			OID:          commit.OID,
			ShortOID:     commit.ShortOID,
			Subject:      commit.Subject,
			Author:       commit.Author,
			AuthoredUnix: commit.AuthoredUnix,
			Decorations:  decorations,
			Merge:        commit.Merge,
		}
	}
	return result, nil
}

func refSourceFromGit(source gitadapter.RefSource) RefSource {
	return RefSource{
		ID: RefSourceID{
			Kind: RefSourceKind(source.ID.Kind),
			Name: source.ID.Name,
		},
		Label:    source.Label,
		Revision: source.Revision,
		OID:      source.OID,
		Path:     source.Path,
		Branch:   source.Branch,
		Upstream: source.Upstream,
		Tracking: source.Tracking,
		Remote:   source.Remote,
		UnixTime: source.UnixTime,
	}
}

func refSourceToGit(source RefSource) gitadapter.RefSource {
	return gitadapter.RefSource{
		ID: gitadapter.RefSourceID{
			Kind: gitadapter.RefSourceKind(source.ID.Kind),
			Name: source.ID.Name,
		},
		Label:    source.Label,
		Revision: source.Revision,
		OID:      source.OID,
		Path:     source.Path,
		Branch:   source.Branch,
		Upstream: source.Upstream,
		Tracking: source.Tracking,
		Remote:   source.Remote,
		UnixTime: source.UnixTime,
	}
}
