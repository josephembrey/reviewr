## 1. Pure review domain

- [x] 1.1 Add `internal/review` endpoint, source, comparison-identity, action, receipt, assessment,
  state, and badge types with exact equality and unavailable/absent identity semantics; cover the
  five fixed-width ASCII badge values and gap priorities.
- [x] 1.2 Implement explicit mark, applicable clear, exact-endpoint reachability, adjacent receipt
  composition, reviewed-frontier selection, duplicate-edge replacement, and read-only assessment;
  test cumulative edges, direct full receipts, disjoint paths, and narrow versus broad scopes.
- [x] 1.3 Implement conservative action/kind/mode/basis derivation and exhaustive table tests for
  exact revert, rename, copy, deletion, file type, executable bit, symlink target, submodule,
  unavailable identity, ambiguous lineage, and rebased comparison behavior.
- [x] 1.4 Implement bounded retained-text and receipt compaction policies; verify binary and
  oversized exact receipts, changed missing snapshots, per-item/total limits, deduplication, and
  conservative behavior after old receipts are discarded.

## 2. Private receipt persistence

- [x] 2.1 Add canonical common-repository/worktree identity resolution, collision-guarded SHA-256
  file keys, injectable platform state roots, and private state paths outside the repository; test
  symlinked paths, linked worktrees, distinct worktrees, and stable canonical keys.
- [x] 2.2 Add version-1 JSON load and restart recovery with exact repository and comparison identity;
  test missing, corrupt, older/invalid, newer, and identity-mismatched files, including warnings and
  refusal to overwrite suspect state.
- [x] 2.3 Add private sibling locking, locked single-delta replay, compaction, same-directory
  temporary writes, file/directory sync, and atomic replacement; test file modes, replacement (not
  append), concurrent disjoint marks, and explicit clears from stale panes.
- [x] 2.4 Preserve locally applied valid receipts on directory, lock, reload, encode, write, sync, or
  rename failure and surface the restart warning; add a no-writes fixture comparing worktree status,
  index bytes, `HEAD`, refs, and Git objects before and after persistence and restart.

## 3. Comparison source and reader bounds

- [x] 3.1 Add the additive exact-comparison, endpoint-content, and canonical-identity source contracts
  without changing or duplicating file/status enumeration; add streamed content-identity helpers for
  regular, absent, symlink, submodule, binary, oversized, and unavailable endpoints.
- [x] 3.2 Land comparison snapshots under the same Files generation and derive assessments only for
  matching current path/comparison identities; verify missing capability, scope switches, refreshes,
  reorders, and stale snapshot results remain review-inert and never mutate receipts.
- [x] 3.3 Build full and reviewed-frontier-to-current views through one bounded comparison reader,
  with exact displayed-bound metadata, stable logical line identities, binary/oversized/deletion
  notices, delta counts, `since reviewed` titles, and Partial/Basis changed explanations.
- [x] 3.4 Gate reader landing and marking by generation, comparison identity, bounds, and exact current
  right endpoint; test a worktree race cannot paint or mark stale content as current.
- [x] 3.5 Reconcile same-file reader cursor, logical line, scroll, selection anchor, and explicit
  full/incremental preference by identity, nearest survivor, then clamping across edits, refreshes,
  bound toggles, and viewport changes.

## 4. Semantic review interaction

- [x] 4.1 Add semantic `x` mark/unmark routing for selected concrete Files navigator rows and the open
  Files reader, keeping directories, unchanged rows, Git, Notes, unavailable bounds, and ordinary
  observation inert; verify opening, scrolling, EOF, comments, refresh, and Herdr events never cover.
- [x] 4.2 Apply marks immediately in memory and serialize persistence effects as authored deltas,
  adopting locked merged state without losing queued local actions; test success, failure, rapid
  actions, restart, pane toggle, and scope-switch survival.
- [x] 4.3 Add `R` incremental/full bound toggling only for Updated readers and verify it changes no
  coverage, comments, focus, folds, selection, or layout; marking either displayed exact bound SHALL
  record only that edge.
- [x] 4.4 Add `X` cycling by Basis changed, Updated, Partial, Unreviewed, then complete tree order;
  extend the existing tree only with ancestor expansion/select support and test collapsed gaps,
  cycling, no-gap status, and retained ordinary access to Reviewed files.
- [x] 4.5 Derive directory `reviewed/changed` progress from every changed descendant, including
  collapsed rows, and test refresh invalidation and that directories never become independently
  reviewable.

## 5. Presentation and shared geometry

- [x] 5.1 Extend optional navigator presentation with independent review state and directory progress,
  plus contextual reader/status/footer hints, without owning or replacing Git status or filetype icon
  fields; ensure unchanged All-scope and observation-only rows advertise no review action.
- [x] 5.2 Add one navigator-row layout calculation used by both rendering and hit testing, reserving
  the rightmost separator-plus-badge cell ahead of label clipping and scrollbars; route every cell in
  that four-column rectangle to the semantic review action.
- [x] 5.3 Add render and geometry tests for all five aligned ASCII badges, independent tones, complete
  badge mouse targets, directory roll-ups, unchanged rows, tree prefixes, scrollbars, narrow widths,
  focused/unfocused selection, and no accidental row activation or folding.
- [x] 5.4 Add end-to-end app tests for `x`, `R`, `X`, mouse badges, updated/full titles, persistence
  warnings, refresh state rederivation, comments orthogonality, and full Continuity of path, cursor,
  logical line, scroll, focus, folds, bounds, selection, and layout.

## 6. Documentation, integration, and validation

- [x] 6.1 Document review semantics, badges, keyboard/mouse actions, private state behavior, recovery
  warnings, and the explicit comparison-provider integration seam; record likely reconciliation
  conflicts with the typed-entry/scope and rich-icon branches.
- [x] 6.2 Run focused domain, persistence, app, routing, file-tree, navigation, render, geometry, and
  no-writes tests; perform live nested/collapsed/updated/binary/oversized/restart smoke checks and
  record refresh plus steady-state navigation latency observations.
- [x] 6.3 Run `nix develop -c just build`, `git diff --check`, and strict OpenSpec validation, resolve
  every failure, and review the final diff for accidental status enumeration, Git writes, comment
  persistence, agent-turn coupling, placeholder packages, or duplicated trees.
- [x] 6.4 Mark only fully completed tasks, run `nix develop -c just check` as the final verification,
  and commit the focused logical changes without pushing or merging.
