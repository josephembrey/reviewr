## Context

See `proposal.md` for motivation and `specs/ui/file-tree/spec.md` for behavior. The repository
boundary currently returns sorted raw paths. `filesState` copies those paths directly into
`navigation.State.Items`, and every selection change requests that identity as reader content. This
works only while every row is a file.

The shared UI model already separates navigator row identity from its display label, and Files and
Git own independent place state. The tree must preserve that workspace-neutral presentation seam,
the one shared render/hit-test geometry authority, and **Continuity** without pushing file-specific
behavior into the Git workspace.

## Goals / Non-Goals

**Goals:**

- Add one pure owner for path hierarchy, expansion state, and visible row derivation.
- Let Files own a directory-or-file cursor independently from the open reader path.
- Keep app routing semantic and keep the Bubble Tea root a thin dispatcher.
- Make refresh reconciliation deterministic and identity-based for both visible and hidden state.

**Non-Goals:**

- Add file status, ignored state, review progress, or scope filtering to tree entries.
- Port the Rust tree data structures mechanically or expose source-entry indices as identities.
- Add filesystem watching, mutation, persistence, search, or a generic third-party tree widget.
- Change Git workspace navigation or reader content rendering.

## Decisions

### Add a pure `internal/filetree` domain

`internal/filetree` will build a hierarchy from repository-relative file paths and derive a flat
visible row sequence for rendering and navigation. Rows carry kind, full stable path, display name,
depth, and directory expansion state. Directories sort before files; single-child directory chains
may compress using the same deterministic path identity as their deepest node.

The tree owns a set of collapsed directory paths. Directories default expanded, matching the current
flat navigator's visibility while adding structure. Rebuilding intersects collapse state with
surviving directory identities so stale paths do not accumulate.

Alternatives considered:

- Build directory strings directly in `filesState`: initially smaller, but mixes hierarchy,
  expansion, reconciliation, and effects in the application state machine.
- Use a generic widget tree: Bubble Tea's standard components do not provide the repository-path and
  identity semantics needed here; the domain is small and pure.
- Store expanded paths with directories initially collapsed: saves visible rows in large trees but
  changes first-load visibility and makes the first Go slice less continuous with current behavior.

### Keep visible cursor identity separate from open file identity

`navigation.State.Items` remains the visible cursor sequence, populated with kind-qualified tree row
identities. `filesState.readerPath` remains the open-file identity and no longer follows every cursor
move. A Files-specific selection transition asks the tree whether the selected row is a file; only a
file requests content. Directory selection and folding preserve reader identity and offset.

On refresh, the cursor/top reconcile through the existing navigation identity algorithm. The tree
separately reconciles the open file against the complete file order, including files hidden by
collapsed ancestors. This prevents folding from manufacturing a file removal.

Alternatives considered:

- Make directories non-selectable: keyboard folding becomes indirect and mouse/full-row selection
  semantics diverge.
- Clear the reader on directory selection: simple, but couples browsing place to reading place and
  creates needless blank frames.
- Expand ancestors whenever a hidden open file survives: violates **Continuity** by changing folding
  in response to a world event rather than reviewer input.

### Add Files-only semantic fold actions

Input routing will emit semantic expand, collapse, and toggle actions. Keyboard actions are available
only while Files navigator focus owns the keys; Git and reader focus remain unaffected. A directory
row click first selects that identity and then toggles it in the same update. Collapsing rebuilds
visible rows around the selected directory, so it remains the anchor and is revealed in the viewport.

The application delegates those actions to `filesState`; it does not teach generic navigation or the
Git history state about directories.

### Extend navigator presentation with structural row metadata

The workspace-neutral UI row will gain optional depth, disclosure, and icon presentation fields.
Zero values preserve Git rows. Files derives those fields from pure tree rows; UI rendering owns only
column width, clipping, and style. Row hit-testing continues to use the same geometry and visible-row
index as painting, so introducing prefixes cannot shift mouse selection.

Restrained directory and file glyphs provide the first visual hierarchy. Rich per-file icon lookup
and Git-status colors remain a later presentation change.

## Risks / Trade-offs

- [Deep paths consume navigator width] → Compress unary directory chains and clip labels only after
  reserving indentation, disclosure, icon, and scrollbar columns.
- [A fold can hide the open file] → Preserve the independent reader path and test hidden-file refresh
  explicitly.
- [Generic navigation assumes every identity opens a reader] → Route Files selection through a
  Files-owned transition while leaving History on the existing generic path.
- [Refresh can reorder both directories and files] → Reconcile visible place and complete file place
  independently by stable path, never row index.
- [Unicode glyph widths vary by terminal] → Use the existing Lip Gloss width functions and fixed
  one-cell Nerd Font glyph assumptions already required by the application's target environment.

## Migration Plan

1. Add the pure tree builder and identity/continuity tests without changing repository reads.
2. Move Files state onto derived tree rows and separate reader selection from tree selection.
3. Add semantic folding inputs and mouse transitions.
4. Add structural row rendering and shared geometry coverage.
5. Run full checks and visually smoke nested, root-only, deep, empty, and refreshed repositories.

Rollback is a normal commit revert. No persisted state, Git data, or filesystem contents are migrated.
