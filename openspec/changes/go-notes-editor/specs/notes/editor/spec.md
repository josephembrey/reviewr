## Purpose

Provides a durable modeless Markdown editor as the third top-level review destination while
preserving read-only repository behavior and independent Continuity state.

## ADDED Requirements

### Requirement: Files, Git, and Notes are ordinary destinations
Reviewr SHALL start in Files and show one stable tab group containing all three destinations. Outside
Notes, `1`, `2`, and `3` SHALL select Files, Git, and Notes directly and idempotently. Mouse labels
SHALL use the same shared geometry. Escape from Git and Notes SHALL return to Files; Files Escape
SHALL remain available to local cancellation.

### Requirement: Notes owns printable input
Every printable character, including `1`, `2`, and `3`, SHALL edit the note. Notes SHALL support
Unicode graphemes, tabs, wrapping, cursor movement, selection, undo/redo, paste, mouse editing, and a
proportional scrollbar. Escape SHALL be the only current keyboard route home.

### Requirement: Destination and scope places survive
Files, every Git subview, and both Notes scopes SHALL retain independent cursor, selection, focus,
scroll, fold, layout, and editor state. Background results SHALL reconcile by Continuity without
arbitrarily moving user-controlled place.

### Requirement: Project and worktree Notes persist privately
Project Notes SHALL use canonical Git common-directory identity. Every checkout SHALL have a
separate worktree scope keyed additionally by canonical worktree root. Each scope SHALL preserve
independent sessions, locks, generation tags, and editor places. Notes SHALL perform no Git writes.

### Requirement: Legacy data imports without replacement
When and only when a Notes target is absent, Reviewr SHALL copy the corresponding prior private note
to that target atomically. Project SHALL map only to project and worktree only to that worktree. Any
Notes target object SHALL win. The source SHALL remain. Invalid UTF-8, lock contention, permissions,
and filesystem errors SHALL remain recoverable without overwriting authored data.

### Requirement: Saves gate transitions
Autosaves and explicit destination, scope, quit, and shutdown flushes SHALL use generation- and
scope-tagged completions. A stale completion SHALL not clear newer text. A failed transition save
SHALL keep Notes visible, modified, and retryable; authored memory SHALL not be discarded.

### Requirement: Markdown ink is geometry-neutral
Notes SHALL tokenize Markdown using the existing Chroma pipeline and render terminal-theme-aware
styles through Lip Gloss. The plain grapheme document SHALL remain authoritative for wrapping,
cursor, selection, pointer mapping, and scrollbars. Cursor and selection styles SHALL override syntax
ink. Unchanged text generations SHALL not be retokenized by cursor movement, scrolling, or unrelated
frames.

### Requirement: Header, controls, and footer are truthful
The active tab label alone SHALL receive the selected background treatment. Browser-local controls
SHALL begin at `4`; Files SHALL use `4`/`5`/`6`, Git SHALL use `4` and eligible `5`, and eligible rich
diffs MAY use reserved `7`. Browser footers SHALL advertise destinations and current controls. Notes
SHALL advertise Escape Files and applicable scope help without digit shortcuts.
