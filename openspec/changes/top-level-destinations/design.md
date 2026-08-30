## Context

Files and Git already share workspace-neutral navigation and reader seams. Notes is now a peer
destination with independent project/worktree sessions, so the root no longer needs a layered or
secondary-mode state model.

## Decisions

### Keep one destination identity and resident domain states

The root owns one active Files/Git/Notes kind. Files and every Git subview retain their own selection,
focus, scroll, and reader state; Notes retains its editor and scope state. Direct selection of the
already-active destination is a no-op. Asynchronous completions remain destination- and
generation-tagged, and reconcile place by stable identity under **Continuity**.

### Route input before domain mutation

Outside Notes, bare `1`, `2`, and `3` select destinations. Files local controls start at `4` and Git
local controls start at `4`; eligible rich-diff treatment remains `7`. Inside Notes all printable
keys, including digits, belong to the editor. Escape performs the existing save-gated return to
Files, while `ctrl+t` and the scope labels retain Notes scope switching.

### Share exact header paint and hit geometry

The fixed logical group is `[ files | git | notes ]`. Only its three label rectangles are clickable
and highlighted. Complete right-side controls shed before partial controls are painted; the group
clips safely at very narrow widths. Structured footer entries keep key and label styling separate.

### Preserve the read-only Git and reader seams

Git list and reader generations remain independent and latest-wins. The ReaderDocument and
ReaderGeometry contracts remain the authority for text, mouse mapping, scrolling, scrollbar
geometry, and eligible key-7 diff treatment. No Git operation writes the worktree, index, branches,
`HEAD`, or refs.

## Migration

This supersedes the two-destination switcher without resetting resident state. Notes persistence and
legacy note import are specified by the Notes change; destination navigation itself adds no stored
format.
