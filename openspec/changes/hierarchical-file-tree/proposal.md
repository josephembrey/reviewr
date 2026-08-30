## Why

The Files navigator currently renders repository-relative paths as one flat list, so directory
structure is difficult to scan and cannot be folded. A real tree is also the necessary state seam
for later file scopes, Git status decoration, and review tracking without duplicating navigator state.

## What Changes

- Replace flat full-path rows in the Files navigator with a hierarchical directory and file tree.
- Add persistent directory expansion state and keyboard and mouse folding controls.
- Separate the tree cursor from the open-file identity so visiting a directory never blanks or
  replaces the reader.
- Reconcile cursor, open file, scroll, and expanded directories by repository-relative identity
  when files refresh, preserving **Continuity**.
- Render depth, disclosure state, and restrained directory/file glyphs while retaining the shared
  frameless geometry and full-width selection treatment.
- Keep All/Changed filtering, Git status and ignored metadata, rich filetype icon coloring, search,
  and filesystem mutation outside this first tree slice.

## Capabilities

### New Capabilities

- `ui/file-tree`: Hierarchical file navigation, folding, open-file behavior, and refresh continuity.

### Modified Capabilities

None.

## Impact

- Adds a focused Go file-tree domain model between repository paths and the workspace-neutral UI.
- Changes Files workspace state, semantic actions, mouse hit handling, and navigator row presentation.
- Extends focused model, application-flow, routing, geometry, and rendering tests.
- Does not change Git reads, repository writes, dependencies, the Git workspace, or reader content.
