## 1. Typed repository snapshot

- [x] 1.1 Parse NUL-delimited porcelain v2 ordinary, rename, unmerged, untracked, and ignored records
  into typed Git entries; cover hostile names and malformed records.
- [x] 1.2 Build one repository snapshot from unchanged tracked paths plus status entries, with All and
  Changed derivations for unchanged, modified, added, deleted, renamed, untracked, and ignored cases.
- [x] 1.3 Add bounded literal-path file/diff reads for deleted, renamed, and untracked entries while
  preserving the read-only Git-state test.

## 2. Scoped Files state and Continuity

- [x] 2.1 Replace `ListFiles` and bare file-load effects at the repository/app boundary with the typed
  snapshot and entry-based effects while preserving list and content latest-wins behavior.
- [x] 2.2 Make header control 2 rebuild one active tree from All or Changed without another snapshot;
  reconcile cursor, top, reader, and surviving folds by identity, same role, then clamp.
- [x] 2.3 Make file/diff mode requests coherent for ignored, deleted, renamed, and untracked entries;
  preserve reader place when identity and mode survive.

## 3. Minimal status presentation

- [x] 3.1 Add explicit workspace-neutral navigator status metadata and a fixed concise marker column
  with restrained ANSI styling and dim ignored rows.
- [x] 3.2 Cover decorated tree keyboard/mouse selection, clipping, scrollbar coexistence, and shared
  render/hit-test geometry without adding filetype icon mappings.

## 4. Validation

- [x] 4.1 Run focused Git, repository, app, navigation, geometry, and render tests; run
  `nix develop -c just check`, `nix develop -c just build`, strict OpenSpec validation when the CLI is
  available, and `git diff --check`.
