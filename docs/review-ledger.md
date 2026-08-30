# Review ledger

Reviewr records explicit, content-addressed per-file comparison edges. It is not a sticky viewed
flag and has no relationship to a review session, conversation, agent turn, reader scroll, EOF, or
comment lifecycle. Opening or observing content never changes coverage.

## States and actions

Only changed files with an exact comparison have a badge:

- `[ ]` Unreviewed: no receipt covers the active comparison from its left endpoint.
- `[x]` Reviewed: exact reviewed edges reach the current endpoint.
- `[+]` Updated: a safe retained reviewed frontier can be compared incrementally to current.
- `[~]` Partial: related coverage exists, but an older or noncontiguous gap remains.
- `[!]` Basis changed: related coverage exists, but a rebase, ambiguous identity, unavailable
  checkpoint, binary/oversized edit, or another proof break makes an incremental view unsafe.

The badge is independent of the Bontree filetype icon and the Git status marker. Directories show
derived `reviewed/changed` progress and never own receipts. Unchanged rows in the All projection have
no badge or review mouse target.

- `x` toggles the selected concrete changed file when the navigator is focused, or the exact bounds
  currently shown by the Files reader when the reader is focused.
- Clicking any of the four cells in the separator-plus-badge field performs the same semantic action.
- `R` switches an Updated reader between `since reviewed` and full active bounds without changing
  coverage or other place state.
- `X` selects the next gap by Basis changed, Updated, Partial, Unreviewed priority and then full tree
  order, expanding only the required ancestors.

The `since reviewed` comparison is the authoritative view of only the new work. In the full
comparison and current-file readers, Updated files also reserve a narrow cyan rail beside the
red/green change rail. It marks current lines introduced since the exact reviewed frontier and
anchors newly removed lines at the nearest surviving boundary. Reversions remain visible even when
they are ordinary context relative to the original Git base. The rail is derived, never stored, and
is omitted for Unreviewed, Reviewed, Partial, and Basis changed files where that claim would be
redundant or unsafe.

Every mark is re-read and checked against the displayed comparison's exact right endpoint before a
receipt is accepted. Rename, copy, deletion, kind, mode, executable-bit, symlink-target, and
submodule transitions remain explicit work. Exact byte/kind/mode reversion can reuse an existing
exact proof. Binary and oversized endpoints retain identities, not bodies; a later edit therefore
becomes Basis changed instead of receiving an invented interdiff.

## Private state and recovery

Schema version 1 is JSON with this envelope:

```json
{
  "version": 1,
  "repository": {
    "common_git_dir": "/canonical/common/git/dir",
    "worktree": "/canonical/worktree"
  },
  "ledger": {
    "receipts": [],
    "next_sequence": 0
  }
}
```

On Unix, the default filename is
`$XDG_STATE_HOME/reviewr/reviews/<sha256(common-git-dir,NUL,worktree)>.json`, falling back to
`$HOME/.local/state/reviewr/reviews/...`. Other platforms use the application config directory when
there is no platform state directory. The directory is mode `0700`; state and sibling lock files are
mode `0600` where supported.

Mutations are authored deltas. Reviewr takes the sibling lock, reloads guarded current state, replays
one delta, compacts bounded receipts and retained text, writes and syncs a same-directory temporary
file, atomically renames it, and syncs the directory. Missing, corrupt, newer-schema, unreadable, or
identity-mismatched state starts Unreviewed with a concise warning. Suspect existing state is left
untouched. A write failure keeps the valid in-memory action and warns that it will not survive a
restart.

This state is outside the repository. Review tracking never writes project files, comments, the
index, branches, `HEAD`, Git objects, or public refs.

## Exact comparison provider seam

`internal/review.Provider` is additive to the ordinary application source. It receives candidates
from the already-enumerated typed `repository.Snapshot`, enriches them with exact old/new endpoints,
provides bounded endpoint content, and supplies the canonical repository/worktree identity. It does
not enumerate status or build a second tree.

The production repository adapter currently supplies exact Uncommitted `HEAD -> worktree`
comparisons, including unborn repositories, deletions, renames, modes, symlinks, submodules, binary
files, and oversized files. Immutable content is read by resolved object identity; worktree content
is classified and Git-hashed from one captured stream. Branch and Last Turn controls remain
review-inert until their separate authoritative basis sources are implemented; they never borrow
Uncommitted endpoints.

The typed-entry/All-or-Changed and Bontree visual slices were integrated before this implementation.
Future reconciliation is most likely in `internal/app/model.go`, `internal/app/files.go`, and
`internal/ui/render.go`: preserve the single typed snapshot, prefix status marker, Bontree icon,
right-side review/progress fields, and `ui.LayoutNavigatorRow` as the shared paint/hit-test geometry.

Validation measurements and live-smoke observations are recorded with the active OpenSpec change.
