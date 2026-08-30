## 1. Pure file-tree domain

- [x] 1.1 Add a pure `internal/filetree` model that builds stable directory and file identities from
  repository paths, sorts directories before files, compacts unary chains, and derives initially
  expanded visible rows; cover nested, root-only, deep, empty, hostile-name, and ordering cases.
- [x] 1.2 Add collapsed-directory state, expand/collapse/toggle operations, complete file order, and
  rebuild reconciliation that retains only surviving directory identities; cover hidden descendants,
  stable identities, and refreshed structure with focused tests.

## 2. Files place and Continuity

- [x] 2.1 Move `filesState` from raw path items to file-tree rows, select and open the first visible
  file on initial load, and route file versus directory selection so directories preserve the open
  reader identity and offset.
- [x] 2.2 Reconcile visible cursor/top and independent open-file place across refreshes, including a
  surviving open file hidden by a collapsed ancestor and nearest same-role fallback when selected or
  open identities disappear; verify latest-wins loading remains intact.

## 3. Semantic interaction and presentation

- [x] 3.1 Add Files-navigator semantic expand, collapse, and toggle actions for `h`/Left, `l`/Right,
  and directory-row clicks; verify file rows, reader focus, Git workspace, and already-requested
  directory states remain inert.
- [x] 3.2 Extend workspace-neutral navigator rows with optional structural metadata and render depth,
  disclosure, restrained folder/file glyphs, clipping, scrollbar coexistence, and unchanged
  full-width selection styling; keep paint and mouse row targeting on shared geometry.

## 4. Validation

- [x] 4.1 Run focused file-tree, navigation, app, routing, geometry, and render tests; run `just check`,
  `just build`, strict OpenSpec validation, and `git diff --check`; visually smoke nested, root-only,
  deep, empty, collapsed-open-file, normal-width, and narrow-width repositories.
