## 1. Workspace-neutral seams

- [x] 1.1 Generalize navigation place state from file paths to stable item identities, and verify
  focused tests preserve selection/top identities, focus, and reader offset across reconciliation.
- [x] 1.2 Replace the file-specific UI input with a concrete workspace-neutral navigator/reader
  presentation model, and verify the existing Files frame, safety states, dimensions, and mouse rows
  retain their behavior.

## 2. Read-only commit history

- [x] 2.1 Add bounded, machine-delimited Git operations for current-HEAD commit listing and selected
  commit metadata/stat, and verify parsing, a 200-commit limit, unborn `HEAD`, hostile text, and output
  bounds with focused adapter tests.
- [x] 2.2 Expose typed commit list and summary reads through the repository boundary, and verify fixture
  history order, root and merge commits, summary fields, explicit failures, and **No writes**.

## 3. Independent primary workspace state

- [x] 3.1 Add explicit Files/Git workspace identity and separate place/load/reader state with
  workspace-scoped generations, and verify initial Files state, first Git activation, active-only
  refresh, and immediate switching back to loaded state.
- [x] 3.2 Add Git list and summary transitions and derive both workspace presentation models, and verify
  latest-wins completions, full-object-ID reconciliation, empty/error/loading states, and independent
  Files/Git selection, focus, and scroll under **Continuity**.

## 4. Header switcher

- [x] 4.1 Add clipped switcher, Files-label, Git-label, and repository-context rectangles to shared
  geometry, and verify every width around the switcher boundaries stays bounded and produces exact
  neutral versus selectable hit targets.
- [x] 4.2 Add semantic toggle/select actions for `1` and direct label clicks, and verify punctuation,
  repository context, and unused header cells remain neutral while body mouse routing is unchanged.
- [x] 4.3 Render `1 [ files | git ]` at the left edge with one full-segment terminal-native active
  treatment and lower-priority repository context, and verify Files/Git frames, normal/narrow/tiny
  dimensions, truncation priority, active styling, and same-frame body switching.

## 5. Validation

- [x] 5.1 Run focused navigation, Git, repository, app, geometry, and render tests; run `just check`,
  `just build`, strict OpenSpec validation, and `git diff --check`; then visually smoke Files and Git
  at normal, narrow, and sub-switcher terminal widths.
