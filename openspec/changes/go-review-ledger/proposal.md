## Why

The Go TUI can display and navigate files but cannot durably distinguish an explicitly reviewed
comparison from content that was merely opened. This slice ports the exact, content-addressed
review ledger so live edits and comparison changes never turn observation into false coverage.

## What Changes

- Add an application-private per-file ledger of explicitly reviewed comparison edges whose exact
  endpoints compose only when they meet.
- Derive Unreviewed, Reviewed, Updated, Partial, and Basis changed states, including conservative
  handling for comparison-base movement, identity ambiguity, missing retained content, binary and
  oversized content, file actions, modes, and exact reverts.
- Add semantic `x` mark/unmark, `R` full/incremental bounds, and `X` next-gap actions from Files
  navigator and reader focus, plus full-cell mouse review actions.
- Present fixed-width ASCII review badges, directory reviewed/changed progress, contextual reader
  titles and warnings, and no badge for unchanged All-scope files while keeping filetype and Git
  status channels independent.
- Persist receipts in versioned, atomically replaced, canonical repository/worktree-keyed state
  outside the repository, with bounded retained text and safe recovery from unavailable, corrupt,
  newer-version, identity-mismatched, and unwritable state.
- Preserve path, cursor, scroll, focus, folds, selected comparison bounds, and logical reader line
  while refreshes only rederive status; preserve **No writes** and **Comments survive**.
- Define review endpoints and comparisons as explicit domain inputs so the separately owned typed
  Git-entry/scope slice can supply them without duplicating status enumeration or a file tree.
- Keep Git Log and other observation-only workspaces review-inert, and do not couple receipts to
  Herdr turns, conversations, elapsed time, EOF, comments, or viewport exposure.

## Capabilities

### New Capabilities

- `review/file-ledger`: Exact review coverage, private persistence, review interactions,
  incremental bounds, gap navigation, presentation, and continuity behavior for concrete Files
  comparisons.

### Modified Capabilities

None.

## Impact

- Adds a focused pure `internal/review` domain and private state-store implementation.
- Extends localized Files state, semantic action/effect routing, file-tree metadata derivation, and
  shared UI row geometry/rendering; it does not add another tree or own Git status enumeration.
- Extends the read-only application source seam with explicit comparison endpoints supplied by the
  file-source integration and keeps the current source and UI channels independently mergeable.
- Adds persistence-format and locking dependencies only where required for safe atomic receipt
  updates; application state remains outside the repository.
- Likely reconciliation hotspots with parallel slices are `internal/app/files.go`,
  `internal/app/model.go`, `internal/ui/model.go`, `internal/ui/geometry.go`, and
  `internal/ui/render.go`; rich filetype mappings and typed Git enumeration remain out of scope.
