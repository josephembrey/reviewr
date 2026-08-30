## Context

The completed hierarchical tree consumes bare paths from `Source.ListFiles`. All/Changed is only a
header value, and file/diff is likewise presentation-only. Git status must now enter the application
without creating per-scope loads or per-scope trees.

## Goals / Non-Goals

**Goals:**

- Establish one immutable typed snapshot at the repository/app boundary.
- Parse hostile Git names only from NUL-delimited machine output.
- Rebuild one tree projection from snapshot-derived scope entries.
- Preserve independent tree and reader place through scope and world changes.
- Keep status presentation explicit, minimal, and available for later styling.

**Non-Goals:**

- Add broad icon mappings or rich filetype truecolor styling.
- Add comparison-basis implementations beyond the existing uncommitted snapshot.
- Add review tracking, persistence, mutation, or a second cached tree per scope.

## Decisions

### One repository snapshot owns scope derivation

`repository.Snapshot` contains typed entries with current path, optional previous rename path, and a
single Git state. `All` returns tracked, untracked, and ignored entries; `Changed` filters the same
entries to changed, untracked, deleted, and renamed states. The app stores one snapshot and rebuilds
one `filetree.Tree` for the active scope.

Git needs two machine-readable reads for a complete snapshot: `ls-files -z --cached` supplies
unchanged tracked identities, while `status --porcelain=v2 -z --untracked-files=all --ignored`
supplies changes, untracked paths, ignored paths, deletion, and rename pairs. No human-formatted Git
output is parsed, and switching scope performs no Git command.

### Scope rebuilding reconciles each place by role

Before rebuilding, Files captures visible rows, complete file order, the cursor identity, reader
entry, and scroll state. It restores exact surviving identities first. A missing file cursor falls
through the old file order to the nearest current file; a missing directory does the same among
directories. Only when that role has no survivor does normal index clamping select another row.
Reader reconciliation is file-only and also recognizes Git's explicit old-to-new rename relation.
`filetree.Tree.Rebuild` remains the sole fold owner and intersects collapse state with surviving
directory identities.

### Reader effects carry typed entries and mode

File and diff requests carry the selected repository entry, not a raw boundary path, and share one
content generation. File mode reads the current path, showing an explicit deleted state when absent.
Diff mode renders a bounded no-color Git patch against HEAD; untracked paths use a bounded
`--no-index` patch. Rename requests include both old and current literal pathspecs. Mode or scope
changes reuse loaded content when identity and mode are unchanged and otherwise issue one tagged
latest-wins request.

### UI receives semantic status, not filetype policy

Navigator rows gain a small semantic status enum and dim flag. Rendering reserves one marker cell
for file rows and uses restrained ANSI palette colors; ignored rows are dim. The same row rectangles
continue to govern painting and mouse hits. Rich filetype icons and truecolor policy remain outside
this change.

## Risks / Trade-offs

- [Snapshot needs two Git reads] -> Keep them inside one repository snapshot operation; never reload
  on scope switch and test call counts.
- [Worktree changes between the two reads] -> Treat the typed result as one tagged refresh and let
  the next latest-wins refresh reconcile it; no lock or write is introduced.
- [Rename changes path identity] -> Carry the prior path and prefer that explicit relation before
  deterministic same-role fallback.
- [Status marker competes with narrow labels] -> Reserve its fixed column before clipping the label
  and keep mouse geometry independent from presentation width.

## Migration Plan

1. Add typed Git and repository snapshot parsing/derivation.
2. Replace Files raw path loads with the snapshot and one scoped tree projection.
3. Activate controls 2 and 3 with typed, tagged reader effects.
4. Add minimal status rendering and focused continuity/geometry tests.
5. Run full checks, build, strict OpenSpec validation when available, and diff checks.
