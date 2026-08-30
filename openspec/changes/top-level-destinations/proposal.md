## Why

Files, Git, and Notes are peer destinations. A stable, directly selectable header makes that shape
visible while preserving the user's place inside each destination.

## What Changes

- Render one persistent `[ files | git | notes ]` group with only the active label highlighted.
- Select Files, Git, and Notes directly with `1`, `2`, and `3` outside the Notes editor; support the
  same direct selection through the three header labels.
- Preserve independent Files and Git place state and the resident Notes editor/scope sessions.
- Retain bounded, read-only Git history and the shared navigator/reader presentation seam.
- Keep header paint and hit testing on one responsive geometry contract.

## Capabilities

### New Capabilities

- `ui/top-level-navigation`: Stable destination tabs, direct keyboard and mouse selection, responsive
  priority, and independent destination continuity.
- `git/commit-history`: Bounded read-only current-HEAD commit history and selected-commit summary.

## Impact

The Bubble Tea root owns one active destination plus resident Files, Git, and Notes state. The change
adds no Git write and preserves the existing repository, reader, and Continuity boundaries.
