## Context

The existing app has a boolean Scratch overlay above independently retained Files and Git models, but
renders only a placeholder. Input is routed to semantic actions and `ui.Geometry` is the sole
render/hit-test authority. Repository access is read-only and background loads already use tagged
results. See `proposal.md` and `specs/scratch/editor/spec.md` for the behavior contract.

The Fresh README and public interaction documentation reinforce the desired familiar, modeless
typing and mouse vocabulary. No Fresh source, algorithms, fixtures, or assets are implementation
inputs.

## Goals / Non-Goals

**Goals:**

- Keep editing logic pure, deterministic, Unicode-aware, and independently testable.
- Keep persistence and clone identity narrow enough to reuse or replace beside later private state.
- Keep the Bubble Tea root responsible only for message routing, generations, debounce commands,
  and presentation assembly.

**Non-Goals:**

- General-purpose editor architecture, file semantics, configurable bindings, or durable undo.
- Cross-process merge, lock waiting, background polling, or repository-local persistence.

## Decisions

### Use a focused grapheme buffer rather than Bubbles textarea

The Bubbles v2 textarea provides a general input component, but does not naturally expose the shared
selection, mouse mapping, draggable scrollbar, and wrap-document state required here. Adapting around
those boundaries would duplicate geometry and bypass component state. A small buffer stores sanitized
grapheme clusters, cursor/anchor indices, bounded snapshots, preferred visual column, and viewport
scroll. Existing `rivo/uniseg` and runewidth support provide grapheme segmentation and terminal width;
no Bubbles dependency is added.

### Build one immutable wrapped document per editor transition or frame

The editor derives visual rows and cell spans from one width, expanding tabs at four-column stops and
soft-wrapping by display cells. Movement, selection painting, cursor display, click/drag mapping, and
scroll clamping all use this structure. Resize continuity maps the old top row's grapheme boundary to
the new document before clamping.

### Extend shared UI geometry with a full-width Scratch surface

`ui.Geometry` owns explicit Scratch title, text viewport, and scrollbar rectangles. The scrollbar lane
is reserved consistently, preventing content reflow when it appears. UI rendering consumes a pure
Scratch presentation; the app passes the same rectangles to semantic pointer routing.

### Treat persistence as a session interface

The app depends on a narrow `Load`, `Save`, and `Close` contract. The production private store hashes
the canonical absolute common-directory identity into
`$XDG_STATE_HOME/reviewr/v1/scratch/<sha256>/`, falling back to
`$HOME/.local/state`. It creates 0700 directories and 0600 note/lock files.

`Load` creates the private directory, takes an advisory exclusive nonblocking `flock`, and reads the
note. Lock contention returns read-only state immediately. `Save` writes a 0600 same-directory temp,
flushes it, closes it, renames over `note.txt`, then best-effort flushes the containing directory.
Failed staging removes only the temporary file; failed rename leaves the prior note intact. Once the
atomic rename succeeds the save is considered successful, avoiding a post-replacement error that
would incorrectly promise the prior file is still current.

### Tag debounce and save completion with edit generations

Every mutation increments the Scratch generation and schedules a short `tea.Tick`. The tick carries
that generation; only a matching current generation begins a save. Save completions also carry their
generation, so an older success cannot clear a newer modified state. Close and normal quit bypass the
debounce and save synchronously through a command before changing the overlay or terminating.

### Resolve common-directory identity once at startup

The read-only Git adapter runs `git rev-parse --path-format=absolute --git-common-dir` with optional
locks disabled, validates the single-line result, then canonicalizes symlinks. No Git or disk identity
lookup occurs in Update or View.

## Risks / Trade-offs

- [Snapshot undo copies the note] → Bound history to 100 entries; Scratch is intentionally one small
  note rather than a huge-file editor.
- [A process crash can leave an unlocked lock file] → `flock` is tied to the open descriptor, so the
  stale path is harmless and the kernel releases ownership.
- [Directory flush support varies by platform] → Flush the staged file before rename and attempt a
  directory flush, but treat the already-valid atomic replacement as success.
- [The global `1` binding consumes a printable digit] → Preserve the explicitly requested behavior
  and document the literal-`1` editing limitation rather than introducing an unrequested escape.
- [Value-style Bubble Tea models can copy state] → Keep the store behind a pointer/session interface;
  close it once from executable ownership after the final model is returned.

## Migration Plan

There is no prior Scratch data. Deploy the versioned private state path with the editor. Rolling back
leaves that private note untouched for a later return to this version.
