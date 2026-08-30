## Context

Files and Git already retain independent navigation places and warm repository state. Notes adds a
third resident destination with two possible editor places, while input remains semantic and
`ui.Geometry` stays the sole paint/hit-test authority.

## Goals / Non-Goals

**Goals:**

- Collapse destination selection to one root state without a secondary active/primary concept.
- Keep editing deterministic, Unicode-aware, and subordinate to review navigation.
- Preserve authored text across destination, scope, error, quit, and restart boundaries.
- Keep Markdown styling presentation-only and cached by document generation.

**Non-Goals:**

- General-purpose editor architecture, configurable bindings, keyboard navigation out of Notes,
  cross-process merge, lock waiting, or repository-local persistence.

## Decisions

### Use one resident top-level destination state

The root owns a single `workspace.Kind`: Files, Git, or Notes. Direct destination actions are
idempotent. Files is startup and home; Git and Files retain their existing places, and Notes retains
its current project/worktree scope and each editor place. Leaving Notes is save-gated.

### Keep the editor document authoritative

The grapheme editor derives wrapped rows from one width, expanding tabs at four-column stops.
Movement, cursor and selection painting, click/drag mapping, scrollbars, and resize reconciliation
all consume those rows. Markdown ink is indexed back to the same graphemes and never changes width.
Cursor and selection presentation overrides syntax ink.

### Reuse the terminal-aware syntax pipeline

The Markdown lexer runs through the existing bounded Chroma highlighter. Its styles use ANSI palette
roles and are rendered with Lip Gloss, so output follows the terminal theme. App state records the
generation represented by cached styles; cursor motion, scrolling, resizing, and unrelated frames
do not tokenize again.

### Persist independent scopes with target-wins import

Project Notes are keyed by canonical Git common directory. Linked-worktree Notes add the canonical
worktree root below that project identity. Each scope owns its session and nonblocking lock. Notes
targets use 0700 directories and 0600 note/lock files with same-directory atomic saves.

On load, an absent Notes target may import only its corresponding prior project or worktree source.
The source is validated, copied atomically without replacement, and retained. Any Notes target
object wins, including empty, unreadable, or invalid content. Contending processes never overwrite
the winner and read-only sessions can still display the legacy source during a concurrent import.

### Tag asynchronous work by scope and generation

Loads, debounces, and saves carry scope and generation. Stale completions cannot replace another
scope or mark newer text saved. Destination, scope, and quit transitions wait for their save; a
failed save keeps Notes visible and authored memory intact. Shutdown attempts every scope and joins
independent errors.

## Risks / Trade-offs

- Whole-buffer undo snapshots are bounded to 100 because Notes is intentionally one small note.
- There is intentionally no printable keyboard route out of Notes besides Escape; future safe
  navigation bindings remain deferred.
- Advisory locks are tied to descriptors, so a stale lock pathname after a crash is harmless.
