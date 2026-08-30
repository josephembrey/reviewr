## Why

Reviewers need a fast, durable place to write Markdown notes without leaving the read-only review
flow. Notes belongs beside Files and Git as an ordinary destination, with independent Continuity
state and text-first input.

## What Changes

- Present Files, Git, and Notes as one stable top-level tab group with direct `1`/`2`/`3`
  selection outside the editor.
- Provide a modeless Unicode editor in Notes; printable digits remain text and Escape saves before
  returning home to Files.
- Retain independent project-wide and checkout-local note sessions with generation-tagged asynchronous
  loads and saves.
- Store Notes in private platform state with atomic replacement, nonblocking locks, and a safe,
  source-preserving import from the prior private path only when the Notes target is absent.
- Render Markdown syntax through Chroma using terminal palette roles while keeping the plain editor
  document authoritative for wrapping and pointer geometry.

## Capabilities

### New Capabilities

- `notes/editor`: Top-level Notes editing, dual scopes, Continuity, Markdown syntax ink, private
  persistence, safe legacy import, and single-writer behavior.

### Modified Capabilities

None.

## Impact

- Uses focused `internal/notes` editor and private-state persistence packages.
- Extends `internal/app` semantic routing/effects and `internal/ui` shared geometry/presentation.
- Reuses the existing Chroma, Lip Gloss, grapheme, and terminal-width pipelines.
