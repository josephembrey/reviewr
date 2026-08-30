## 1. Pure editor domain

- [x] 1.1 Implement sanitized grapheme-buffer insertion, deletion, selection, word movement, bounded undo/redo, and paste.
- [x] 1.2 Implement the shared soft-wrap document, cursor/preferred-column movement, viewport continuity, clipping, and geometry-derived point mapping.
- [x] 1.3 Add focused editor and wrap tests covering text, Unicode width, tabs, navigation, selection, history, paste, clipping, and resize.

## 2. Private persistence

- [x] 2.1 Resolve and canonicalize the absolute Git common directory with a machine-safe read-only Git invocation.
- [x] 2.2 Implement versioned XDG private-state identity, restrictive permissions, atomic durable replacement, and a nonblocking OS-backed edit lock behind a narrow store interface.
- [x] 2.3 Add persistence and repository fixtures for linked-worktree sharing, clone isolation, permissions, replacement/failures, contention, and repository No writes.

## 3. App and UI integration

- [x] 3.1 Extend shared UI geometry and rendering for the full-width Scratch title, wrapped editor, visible selection, cursor, status, and draggable scrollbar.
- [x] 3.2 Add Scratch semantic keyboard, paste, click, drag, wheel, and scrollbar actions while preserving Escape and literal `1` overlay behavior.
- [x] 3.3 Add generation-tagged load/debounced save effects, synchronous close/quit saves, read-only/error states, and startup/shutdown store ownership.
- [x] 3.4 Add app, routing, render, geometry, background-isolation, persistence-state, and close/quit tests.

## 4. End-to-end validation

- [x] 4.1 Add and run PTY smoke coverage for typing, paste, pointer editing, scrolling, resize, reopen, and two-process locking.
- [x] 4.2 Run focused race tests, `nix develop -c just check`, `nix develop -c just build`, `git diff --check`, and strict OpenSpec validation.
