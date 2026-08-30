## Context

See `proposal.md` for motivation and `specs/review/file-ledger/spec.md` for behavior. The completed
first file-tree slice owns hierarchy, folds, and independent navigator/reader identities. The
current Go source seam still yields raw paths and bounded worktree content; typed Git entries and
All/Changed plus comparison-scope enumeration are being added separately. Review therefore needs a
domain boundary that consumes exact comparisons without becoming a competing status enumerator or
tree.

The Bubble Tea root currently routes one semantic action and tags asynchronous list/content loads.
UI painting and mouse routing share stored pane geometry, but navigator rows do not yet describe a
right-aligned semantic field. Persistence is new application I/O and must remain outside both the
repository and pure coverage derivation.

## Goals / Non-Goals

**Goals:**

- Make exact endpoint coverage and all five states pure, deterministic, and exhaustively testable.
- Accept explicit comparison snapshots from the Files source integration and keep source/scope
  identity intact across refreshes.
- Persist user-authored deltas safely across processes while keeping an immediate valid in-memory
  result on failure.
- Add review actions, incremental reader bounds, gap navigation, roll-ups, and badge hit targets
  through localized Files and shared-UI seams.
- Keep refresh reconciliation identity-based for both tree place and comparison-reader place.

**Non-Goals:**

- Enumerate Git status, choose rename/copy similarity, implement All/Changed filtering, or own the
  separately developed typed-entry/scope models.
- Add rich filetype mappings, recolor Git status, or replace the existing file tree.
- Add approval, shared reviewer state, sessions, comment persistence, agent-turn causality, fuzzy
  lineage, or semantic dependency invalidation.
- Add review controls to Git Log, Stashes, Refs, Notes, or any other observation-only destination.

## Decisions

### Use one pure `internal/review` endpoint graph

The domain will define `FileKind`, `FileAction`, `EndpointSource`, `Endpoint`,
`ComparisonIdentity`, `FileComparison`, `Receipt`, `Ledger`, `Assessment`, and `ReviewState`.
Endpoint equality includes path, kind, numeric mode, and exact content identity; an absent endpoint
uses an exact sentinel, while an empty content identity is explicitly unavailable. A comparison
also carries its scope/provenance identity, old/new sources, action, and an optional producer-supplied
conservative basis reason.

`Ledger.Assess` is read-only. It first recognizes a direct receipt for the exact full action and
bounds, then computes endpoints reachable from the comparison's old endpoint through compatible
exact receipts. Only equal endpoint values join graph edges. The newest reachable checkpoint that
can continue to current becomes Updated when retained text exists; missing retained content becomes
Basis changed. Related coverage in another/narrow scope without reachability becomes Partial;
related same-scope identity breaks become Basis changed. An explicit direct full receipt remains
authoritative even when the producer supplied a prior basis warning.

`Mark` and `Clear` are the only mutations. Mark receives the exact bounds represented by the reader,
deduplicates an existing edge, assigns monotonic sequence, and retains only policy-eligible text.
Clear removes the newest direct or composing proof applicable to the exact current result rather
than every receipt sharing a path. Compaction keeps edge identities for exact reverts, caps history,
and sheds older retained payloads before it can exceed the total byte budget.

This follows the oracle's graph semantics without porting its Rust layout. A sticky current hash was
rejected because it cannot distinguish a narrow scope, action, mode, or hidden older gap. A mutable
per-file boolean was rejected because refresh would either lie or discard useful reviewed prefixes.
Patch-similarity transfer across a moved base is deliberately conservative: unless the comparison
producer supplies an exact proof accepted by a future domain extension, a moved or ambiguous basis
is Basis changed.

### Consume exact comparisons through an additive source interface

Add a small review-specific source contract alongside the existing application `Source`, rather
than changing raw path enumeration or importing Git concepts into the file tree. A review snapshot
contains concrete comparisons keyed by the current file path and exposes bounded endpoint content
loading for requested full or incremental bounds. Repository/worktree identity is a separate value
containing canonical common-Git-directory and canonical worktree paths.

The Files load effect requests the ordinary path/entry snapshot and review comparison snapshot from
the same source generation. A review comparison is shown only when its path and comparison identity
land together; missing review capability leaves the existing row unchanged and review-inert. The
typed-entry/scope branch can implement this contract from its already enumerated status records,
preserving copy/rename action and comparison pins without duplicate Git commands or a second tree.
Tests use a fake source with exact endpoints so this slice can exercise every app transition before
branch reconciliation.

The alternative—derive HEAD/worktree comparisons from `ListFiles` here—was rejected because it
would miss scopes and reliable rename/copy/deletion identity, duplicate slice 2, and create false
checks. Making the tree own endpoint metadata was rejected because comparison provenance changes
independently of path hierarchy.

### Keep review orchestration in Files state and effects

Files state will own the loaded comparison map, ledger, derived reader bounds, current comparison
identity, full/incremental preference, reader-build generation, persistence warning, and a serialized
queue of review deltas. The root adds only semantic `MarkReviewed`, `ToggleReviewBounds`,
`NextReviewGap`, and `ActivateReviewBadge` cases plus tagged load/build/persist effects. No world or
content landing path calls a coverage mutation.

From navigator focus, mark targets only the selected concrete file row. From reader focus it targets
the independently open file. An exact endpoint gate compares displayed bounds and current
comparison before producing a delta; a raced build becomes visibly stale and schedules refresh.
Mark applies locally first, then one persistence effect replays the delta under lock. Later queued
deltas remain layered over any merged ledger returned by that transaction, preventing quick input
or another pane from overwriting newer authored actions.

`R` changes only the reader bound preference for Updated. `X` derives all gaps from the complete
tree file order (including collapsed descendants), sorts state priority then tree index, cycles from
the current target, expands the chosen file's ancestors through a focused file-tree method, and
selects it through the existing Files transition.

Putting persistence and coverage mutation directly in `Model.Update` was rejected because it would
mix blocking I/O, domain rules, and place changes into the Bubble Tea root. Treating refresh as a
ledger transaction was rejected by REVIEW-EXPLICIT.

### Reuse one comparison reader with explicit endpoint bounds

The Files reader build request carries file path, comparison identity, exact old/new endpoints, and
their sources. Updated defaults to the assessment frontier and current endpoint; full mode uses the
active comparison endpoints. Partial, Basis changed, and Unreviewed use full bounds. The same
bounded content result and line-diff presentation serve both modes, including absent, symlink,
submodule, binary, and oversized notices.

Each rendered comparison line carries a stable logical identity made from side, old/new logical
line numbers, and occurrence-qualified content identity. Before rebuilding the same file, Files
captures cursor, top-line, and selection anchors; landing reconciles each by exact identity, nearest
survivor, then clamping. A generation plus comparison/bounds equality gate prevents stale content
from painting as current or becoming markable. Switching `R` preserves the same file and right
endpoint and never touches the comment integration seam.

A separate incremental-only widget was rejected because it would split rendering, commenting, and
continuity behavior. Treating retained text as current without rechecking endpoint identity was
rejected because a refresh race could mark stale content.

### Persist version 1 JSON with locked delta replay and atomic replacement

Production state resolves beneath the platform application-state root, using
`$XDG_STATE_HOME/reviewr` or `$HOME/.local/state/reviewr` on Unix and the platform user config/state
directory as fallback. A canonical repository identity is the real path of the common Git directory
plus the real path of the worktree. SHA-256 of `common-git-dir NUL worktree` names
`reviews/<hex>.json`; the unhashed identity is also stored inside the file as a collision and
misrouting guard. Receipts retain comparison identity in the schema.

Schema version 1 contains `version`, `repository`, and `ledger` with receipts and next sequence.
Files and lock files are private (`0600`) and their directory is private (`0700`) where supported.
Each mutation opens a sibling lock, takes an exclusive OS lock, reloads current state, applies the
single authored delta, compacts it, writes a same-directory temporary file, flushes file content,
renames over the destination, and best-effort flushes the directory. The app adopts the committed
ledger on success. This avoids stale whole-ledger replacement across panes.

Missing state starts empty with the required warning but remains writable. Corrupt, newer-version,
and identity-mismatched files start empty and make that store handle read-only so unknown evidence is
never overwritten. Any commit failure leaves the already-applied local delta active and warns that
it will not survive restart.

The default retention policy follows the oracle: at most 2,000,000 bytes per retained text,
16,000,000 retained bytes total, and 4,096 exact receipts. Binary and oversized endpoints still
carry exact streamed content identities but no retained body. JSON was chosen for debuggable
versioning and parity with the oracle; project files and Git refs were rejected as storage locations
under REVIEW-PRIVATE and No writes.

### Derive badge and progress layout once for paint and hit testing

Navigator presentation gains optional review badge/state and directory progress fields, leaving
zero values compatible with Git and unchanged All-scope rows. Changed file rows reserve a rightmost
four-column cell: one separator and one of the three-column badges. Directories reserve their compact
progress field but never a review action. Label clipping occurs only after structural prefixes,
scrollbar, progress, and review columns are reserved.

One UI row-layout calculation returns the label rectangle, progress rectangle, and optional review
cell rectangle. Both navigator rendering and hit testing consume it. A badge hit becomes a distinct
semantic target before generic row activation, so clicking its separator does not accidentally fold
or merely select the row. State color is chosen independently of filetype icon and Git-status
presentation.

Hard-coding an `x` range in application mouse routing was rejected because scrollbar presence,
terminal width, and parallel icon/status columns would make paint and clicks drift.

## Risks / Trade-offs

- [Parallel scope and icon branches touch the same Files and UI structures] → Keep comparison input
  additive, avoid status/icon ownership, isolate review fields, and expect line-level reconciliation
  in `internal/app/files.go`, `internal/app/model.go`, `internal/ui/model.go`,
  `internal/ui/geometry.go`, and `internal/ui/render.go`.
- [The current branch cannot authoritatively enumerate all comparison scopes] → Keep runtime rows
  review-inert unless a source supplies exact comparisons; validate full app behavior through the
  contract and reconcile its implementation with the typed-entry/scope provider rather than adding
  temporary Git enumeration.
- [Hashing very large worktree content can add refresh latency] → Stream identity calculation with
  bounded memory off the frame loop, retain no body, tag generations, and measure refresh plus
  steady-state navigation in the live smoke fixture.
- [Cross-process locking is platform-specific] → Isolate lock primitives behind small build-tagged
  files and test transaction semantics plus atomic replacement on the Nix/Linux target.
- [Receipt caps can discard old exact-revert history] → Drop oldest receipts only at the explicit
  hard bound, preserve newest unique edge identities, and never convert loss into Reviewed.
- [A narrow terminal may have no room for tree label plus progress and badge] → Reserve semantic
  cells first and clip the label; minimum-size behavior remains the outer guard.
- [Persistence warnings could become noisy] → Keep one concise current warning/status and clear it
  only after an explicit successful replacement, without blocking review.

## Migration Plan

1. Add and exhaustively test the pure endpoint graph, assessment, compaction, and badge semantics.
2. Add canonical identity, version-1 store, private locking, atomic replacement, recovery, and
   cross-pane delta tests without wiring UI behavior.
3. Add the explicit comparison/content source seam and tagged Files loads, then integrate review
   state and exact reader-bound builds behind source capability detection.
4. Add semantic keyboard and mouse actions, gap navigation, directory progress, shared row layout,
   titles, warnings, and full Continuity tests.
5. Reconcile the additive source seam with the typed-entry/scope branch and the optional review
   columns with the rich-icon branch; do not copy either branch's models.
6. Run focused tests, strict OpenSpec validation, the repository CI/build commands, no-writes audit,
   live interaction smoke, and comparative latency checks.

Rollback is a normal commit revert. Version-1 state is outside the repository; a rolled-back binary
ignores it, and a future incompatible schema increments the version rather than rewriting unknown
state.
