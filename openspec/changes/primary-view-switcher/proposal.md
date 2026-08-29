## Why

Files and Git are reviewr's two primary workspaces, but the Go application currently exposes only
the Files workspace and spends the most prominent header cells on a redundant application label.
The primary mode should be continuously visible and directly switchable so the application's core
shape is obvious before the richer Git workflows are migrated.

## What Changes

- Replace the header's left application label with a persistent `1 [ files | git ]` switcher followed
  by repository context when space permits.
- Let `1` toggle the primary workspace and let mouse users select either labeled segment directly.
- Preserve independent selection, focus, and scroll place for Files and Git when switching.
- Add a small read-only Git workspace: recent commits reachable from the current `HEAD` appear in the
  navigator, while the reader shows the selected commit's metadata and changed-file stat.
- Keep the existing frameless split, terminal-native active treatment, semantic actions, and shared
  render/hit-test geometry.
- Leave branch/ref selection, worktrees, stashes, graph rendering, commit patches, and diff review out
  of this change.

## Capabilities

### New Capabilities

- `ui/primary-navigation`: Persistent header switcher, keyboard and mouse selection, responsive
  priority, and independent workspace place state.
- `git/commit-history`: Bounded read-only current-HEAD commit history and selected-commit summary.

### Modified Capabilities

None.

## Impact

- Extends the Bubble Tea model and semantic action set with an explicit primary workspace.
- Extends shared UI geometry with header switcher hit targets and renders workspace-specific body
  models through the existing frameless split.
- Extends the repository/Git read boundary with commit-list and commit-summary operations; it adds no
  dependency and performs no Git write.
- Adds focused navigation, repository, app, rendering, and mouse-routing tests.
