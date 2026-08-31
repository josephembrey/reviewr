package repository

// ListCommitFiles enumerates the first-parent changes stored by one immutable
// commit snapshot.
func (r *Repository) ListCommitFiles(oid string) ([]ChangedFile, error) {
	changes, err := r.git.ListCommitChanges(r.root, oid)
	if err != nil {
		return nil, err
	}
	files := make([]ChangedFile, len(changes))
	for index, change := range changes {
		files[index] = fromGitChangedFile(change)
	}
	return files, nil
}

// ReadCommitFile reads exact old/new blobs and a first-parent patch for one
// changed file without consulting mutable worktree state.
func (r *Repository) ReadCommitFile(oid string, change ChangedFile) ChangeDocument {
	document := ChangeDocument{Change: change}
	source, err := r.git.ResolveCommitChangeSource(r.root, oid)
	if err != nil {
		document.Patch = File{Path: change.Path, Kind: FileUnreadable, Err: err}
		return document
	}
	oldPath := change.Path
	if change.PreviousPath != "" {
		oldPath = change.PreviousPath
	}
	if change.Kind == ChangeAdded {
		document.Old = File{Path: oldPath, Kind: FileMissing}
	} else {
		document.Old = fromGitObject(oldPath, r.git.ReadObjectFile(r.root, source.BaseOID, oldPath, r.maxBytes))
	}
	if change.Kind == ChangeDeleted {
		document.New = File{Path: change.Path, Kind: FileMissing}
	} else {
		document.New = fromGitObject(change.Path, r.git.ReadObjectFile(r.root, source.OID, change.Path, r.maxBytes))
	}
	paths := []string{change.Path}
	if oldPath != change.Path {
		paths = append([]string{oldPath}, paths...)
	}
	document.Patch = fromGitObject(change.Path, r.git.DiffObjects(r.root, source.BaseOID, source.OID, paths, r.maxBytes))
	return document
}
