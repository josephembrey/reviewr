package repository

import gitadapter "github.com/josephembrey/reviewr/internal/git"

// ChangeKind describes how one immutable comparison path changed.
type ChangeKind uint8

const (
	ChangeModified ChangeKind = iota + 1
	ChangeAdded
	ChangeDeleted
	ChangeRenamed
	ChangeCopied
	ChangeUntracked
)

// ChangedFile is one selectable file in an immutable comparison.
type ChangedFile struct {
	Path         string
	PreviousPath string
	Kind         ChangeKind
	Additions    uint64
	Deletions    uint64
	Binary       bool
}

// Identity is stable across refreshes of the same immutable comparison.
func (file ChangedFile) Identity() string {
	return file.PreviousPath + "\x00" + file.Path
}

// ChangeSource contains only immutable object identities needed to inspect a stash.
type ChangeSource struct {
	OID          string
	BaseOID      string
	UntrackedOID string
}

// Stash is one read-only stash reflog row. OID, not Selector, is its identity.
type Stash struct {
	OID       string
	Selector  string
	Branch    string
	Message   string
	Timestamp int64
	Files     uint64
	Additions uint64
	Deletions uint64
	Source    ChangeSource
}

// ChangeDocument is the narrow immutable file/diff reader input shared with Files.
type ChangeDocument struct {
	Change ChangedFile
	Old    File
	New    File
	Patch  File
}

// ListStashes returns every refs/stash reflog entry with OID-backed aggregate stats.
func (r *Repository) ListStashes() ([]Stash, error) {
	entries, err := r.git.ListStashes(r.root)
	if err != nil {
		return nil, err
	}
	stashes := make([]Stash, len(entries))
	for index, entry := range entries {
		stashes[index] = Stash{
			OID: entry.OID, Selector: entry.Selector, Branch: entry.Branch,
			Message: entry.Message, Timestamp: entry.Timestamp, Files: entry.FileCount,
			Additions: entry.Additions, Deletions: entry.Deletions,
			Source: ChangeSource{OID: entry.OID, BaseOID: entry.BaseOID, UntrackedOID: entry.UntrackedOID},
		}
	}
	return stashes, nil
}

// ListStashFiles enumerates the combined tracked and untracked paths stored by a stash.
func (r *Repository) ListStashFiles(source ChangeSource) ([]ChangedFile, error) {
	changes, err := r.git.ListStashChanges(r.root, gitStashSource(source))
	if err != nil {
		return nil, err
	}
	files := make([]ChangedFile, len(changes))
	for index, change := range changes {
		files[index] = fromGitChangedFile(change)
	}
	return files, nil
}

// ReadStashFile reads exact old/new blobs and their patch without consulting
// the index, worktree, HEAD, selectors, or mutable refs.
func (r *Repository) ReadStashFile(source ChangeSource, change ChangedFile) ChangeDocument {
	document := ChangeDocument{Change: change}
	oldOID := source.BaseOID
	newOID := source.OID
	oldPath := change.Path
	if change.PreviousPath != "" {
		oldPath = change.PreviousPath
	}
	if change.Kind == ChangeUntracked {
		empty, err := r.git.EmptyTree(r.root)
		if err != nil {
			document.Patch = File{Path: change.Path, Kind: FileUnreadable, Err: err}
			return document
		}
		oldOID = empty
		newOID = source.UntrackedOID
		document.Old = File{Path: oldPath, Kind: FileMissing}
	} else {
		document.Old = fromGitObject(oldPath, r.git.ReadObjectFile(r.root, oldOID, oldPath, r.maxBytes))
	}
	if change.Kind == ChangeDeleted {
		document.New = File{Path: change.Path, Kind: FileMissing}
	} else {
		document.New = fromGitObject(change.Path, r.git.ReadObjectFile(r.root, newOID, change.Path, r.maxBytes))
	}
	paths := []string{change.Path}
	if oldPath != change.Path {
		paths = append([]string{oldPath}, paths...)
	}
	document.Patch = fromGitObject(change.Path, r.git.DiffObjects(r.root, oldOID, newOID, paths, r.maxBytes))
	return document
}

func gitStashSource(source ChangeSource) gitadapter.StashSource {
	return gitadapter.StashSource{
		OID: source.OID, BaseOID: source.BaseOID, UntrackedOID: source.UntrackedOID,
	}
}

func fromGitChangedFile(change gitadapter.ChangedFile) ChangedFile {
	return ChangedFile{
		Path: change.Path, PreviousPath: change.PreviousPath,
		Kind: fromGitChangeKind(change.Kind), Additions: change.Additions,
		Deletions: change.Deletions, Binary: change.Binary,
	}
}

func fromGitChangeKind(kind gitadapter.ChangeKind) ChangeKind {
	switch kind {
	case gitadapter.ChangeAdded:
		return ChangeAdded
	case gitadapter.ChangeDeleted:
		return ChangeDeleted
	case gitadapter.ChangeRenamed:
		return ChangeRenamed
	case gitadapter.ChangeCopied:
		return ChangeCopied
	case gitadapter.ChangeUntracked:
		return ChangeUntracked
	default:
		return ChangeModified
	}
}

func fromGitObject(path string, object gitadapter.ObjectFile) File {
	file := File{Path: path, Content: string(object.Data), Size: object.Size, Err: object.Err}
	switch object.Kind {
	case gitadapter.ObjectReady:
		file.Kind = FileReady
	case gitadapter.ObjectMissing:
		file.Kind = FileMissing
	case gitadapter.ObjectBinary:
		file.Kind = FileBinary
	case gitadapter.ObjectTooLarge:
		file.Kind = FileTooLarge
	default:
		file.Kind = FileUnreadable
	}
	return file
}
