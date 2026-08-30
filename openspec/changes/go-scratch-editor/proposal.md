## Why

Reviewers need a fast, durable place to jot temporary notes without leaving the read-only review
flow or turning the application into a general-purpose editor. The existing global Scratch overlay
already establishes the navigation shape, but its body is only a stub and cannot retain text.

## What Changes

- Replace the Scratch stub with one modeless UTF-8 multiline editor whose keyboard, selection,
  wrapping, mouse, scrollbar, undo, and paste behavior is derived from one document geometry.
- Preserve Scratch as a global overlay over the exact remembered Files or Git place state, including
  the explicit `1`-closes-to-remembered-primary behavior.
- Load and atomically autosave one note per canonical Git common directory in private platform state.
- Use a nonblocking OS lock so a second process for the same clone opens the latest note read-only.
- Keep repository data read-only and surface persistence failures as recoverable Scratch status.
- Exclude file browsing, Markdown preview, syntax features, multiple buffers, search, configuration,
  plugins, and other general-editor scope.

## Capabilities

### New Capabilities

- `scratch/editor`: Global overlay editing, shared wrap geometry, Continuity, persistence identity,
  atomic autosave, and single-writer behavior for the minimal Scratch note.

### Modified Capabilities

None.

## Impact

- Adds focused `internal/scratch` editor and private-state persistence packages.
- Extends `internal/app` semantic routing/effects and `internal/ui` shared geometry/presentation.
- Extends executable wiring so repository identity and Scratch storage are established once at
  startup and closed normally on exit.
- Reuses the existing `rivo/uniseg` and terminal-width dependency already present transitively;
  does not add the Bubbles textarea dependency.
