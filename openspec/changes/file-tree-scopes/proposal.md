## Why

The Files header exposes All and Changed controls, but both currently render the same nonignored path
list and carry no Git state. Building separate lists or trees would split navigator place and make
scope changes violate Continuity.

## What Changes

- Replace the repository/app raw-path list with one typed repository snapshot carrying stable
  repository-relative path identity, prior rename path, and Git state.
- Derive All and Changed from that snapshot and make Files control 2 switch the one active tree
  projection without another repository load.
- Preserve cursor, open reader file, viewport, and surviving folds across scope changes and refresh.
- Show restrained status metadata for modified, added, deleted, renamed, untracked, and ignored
  files; ignored files remain readable only in All.
- Make deleted and renamed entries meaningful in both file and diff reader modes while preserving
  latest-wins effects and read-only Git behavior.

## Capabilities

### Modified Capabilities

- `ui/file-tree`: Add snapshot-derived scopes, Git status, and scope-switch Continuity to the
  hierarchical Files navigator.

## Impact

- Changes `internal/git`, `internal/repository`, `internal/app`, and a small navigator presentation
  seam in `internal/ui`.
- Extends focused Git parsing, snapshot derivation, app continuity, reader, input, geometry, and
  rendering tests.
- Does not add rich filetype icons/colors, persistence, comments, filesystem watching, or any Git,
  index, worktree, branch, or filesystem write.
