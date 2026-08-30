package repository

// ResolveWorktree resolves an agent directory to its canonical checkout.
func (r *Repository) ResolveWorktree(path string) (string, bool, error) {
	return r.git.FindRoot(path)
}

// SnapshotTurnWorktree captures a private immutable tree without changing the
// worktree or real index.
func (r *Repository) SnapshotTurnWorktree() (string, error) {
	return r.git.SnapshotWorktree(r.root)
}

// ReadTurnBaseline returns this worktree's persisted private turn baseline.
func (r *Repository) ReadTurnBaseline() (string, bool, error) {
	return r.git.TurnBaseline(r.root)
}

// WriteTurnBaseline atomically advances this worktree's private turn baseline.
func (r *Repository) WriteTurnBaseline(tree string) error {
	return r.git.WriteTurnBaseline(r.root, tree)
}
