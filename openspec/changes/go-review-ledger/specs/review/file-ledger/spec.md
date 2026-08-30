## Purpose

Define exact, explicit, private review coverage for concrete per-file comparisons and its honest
presentation as repository contents, comparison bounds, and application lifetimes change.

## ADDED Requirements

### Requirement: Review coverage records exact displayed comparison edges
For each concrete changed file, reviewr SHALL distinguish the comparison identity, old and new
source, path, action, file kind, mode, and exact content identity at both endpoints. A receipt SHALL
cover only the exact old-to-new edge the reviewer explicitly marked, and reviewr SHALL compose
coverage only through receipts whose exact endpoints meet.

#### Scenario: Reviewer marks the active full comparison
- **WHEN** the reader represents exact bounds from `base` to `r1` and the reviewer marks it reviewed
- **THEN** reviewr records coverage for exactly `base -> r1`

#### Scenario: Adjacent reviewed updates meet
- **WHEN** receipts cover `base -> r1` and `r1 -> r2` with equal `r1` endpoint identities
- **THEN** reviewr derives complete coverage for `base -> r2`

#### Scenario: Reviewed edges do not meet
- **WHEN** related receipts leave an older or noncontiguous edge of the active comparison uncovered
- **THEN** reviewr does not derive complete coverage for the active comparison

#### Scenario: Endpoint identity is stale or unavailable
- **WHEN** the content currently displayed cannot be proven to have the active comparison's exact
  right endpoint identity
- **THEN** reviewr refuses to mark that content as the active current revision and requests or waits
  for a matching refresh

### Requirement: Only explicit review actions change coverage
Only an explicit mark/unmark action SHALL create, advance, or remove review coverage. Opening,
scrolling, reaching end of file, selecting lines, adding or editing comments, changing scopes,
refreshing, restarting, toggling a pane, and receiving Herdr or world events SHALL NOT create,
advance, or remove receipts.

#### Scenario: Reviewer observes an entire file
- **WHEN** the reviewer opens a file, scrolls through every line, and reaches end of file
- **THEN** its review coverage is unchanged

#### Scenario: Reviewer comments on an unreviewed file
- **WHEN** the reviewer creates or edits a comment without invoking the review action
- **THEN** the file remains unreviewed and the comment remains independent of review coverage

#### Scenario: Background content changes
- **WHEN** a refresh lands new exact endpoints for a file
- **THEN** reviewr only rederives its status from existing receipts and does not mutate the ledger

#### Scenario: Herdr activity occurs
- **WHEN** a turn, conversation, pane event, or agent state transition occurs
- **THEN** no review receipt or comparison bound is inferred from that activity

### Requirement: Review status is derived honestly
Reviewr SHALL derive exactly five internal per-file proof states for changed files: Unreviewed when
no receipt covers the active comparison from its left endpoint; Reviewed when exact coverage reaches
the exact current endpoint; Updated when a contiguous reviewed prefix safely supports an incremental
comparison to current; Partial when related coverage exists but an older or noncontiguous gap
remains; and Basis changed when related coverage exists but incremental proof is unsafe. The UI
SHALL expose four states by mapping them to `[ ]` Unreviewed, `[x]` Reviewed, `[~]` Updated since
review, and `[!]` Re-review required for either Partial or Basis changed.

#### Scenario: No related receipt exists
- **WHEN** a concrete changed-file comparison has no related receipt
- **THEN** reviewr derives Unreviewed

#### Scenario: Current endpoint advances after review
- **WHEN** a receipt covers the active comparison from its left endpoint through a retained exact
  checkpoint and current advances compatibly
- **THEN** reviewr derives Updated and identifies that checkpoint as the reviewed frontier

#### Scenario: Narrow comparison is reviewed
- **WHEN** a Last Turn edge is reviewed but an older Branch edge remains uncovered
- **THEN** the broader Branch comparison derives Partial rather than Reviewed

#### Scenario: Comparison basis becomes unsafe
- **WHEN** a rebase, changed base, ambiguous lineage, unavailable retained content, binary or
  oversized checkpoint edit, or other identity break prevents a trustworthy incremental proof
- **THEN** reviewr derives Basis changed with a concise reason and exposes the full active comparison

#### Scenario: Reviewer explicitly reviews a conservative full comparison
- **WHEN** the reviewer marks the exact full bounds of a Basis changed comparison
- **THEN** that direct receipt is authoritative and the exact comparison derives Reviewed

### Requirement: File identity changes remain reviewable work
Exact content and mode reversion under a compatible comparison SHALL restore Reviewed when an exact
receipt already proves that result. Rename, copy, deletion, file-type, executable-bit, symlink-target,
and submodule changes SHALL remain distinct reviewable work and SHALL NOT inherit Reviewed solely
because bytes or paths resemble a reviewed endpoint. Comparison-base movement SHALL preserve
coverage only when the complete reviewed patch is proven identical, including whitespace, path
action, and mode.

#### Scenario: File returns to an exact reviewed revision
- **WHEN** transient edits are followed by an exact byte-for-byte, kind-for-kind, and mode-for-mode
  return to a compatible reviewed endpoint
- **THEN** reviewr derives Reviewed again without creating a new receipt

#### Scenario: Copy and rename have identical bytes
- **WHEN** a reviewed rename and an unreviewed copy share content identities
- **THEN** the rename receipt does not cover the copy action

#### Scenario: Executable mode changes without byte changes
- **WHEN** a reviewed regular file changes only its executable bit
- **THEN** the mode transition remains reviewable and is not falsely Reviewed

#### Scenario: File is deleted
- **WHEN** a present endpoint transitions to an exact absent endpoint
- **THEN** deletion is a concrete reviewable edge that becomes Reviewed only after explicit review

#### Scenario: Similarity-based lineage is ambiguous
- **WHEN** rename or copy lineage cannot be proven exactly
- **THEN** reviewr derives Basis changed and defaults to the full comparison

### Requirement: Review actions target concrete Files comparisons
While a Files navigator file or Files reader is the active target, `x` SHALL toggle the applicable
exact review checkpoint. The complete rendered badge cell SHALL perform the same semantic action.
Directory rows, unchanged All-scope rows, and non-Files observation surfaces SHALL not be directly
reviewable.

#### Scenario: Reviewer marks from navigator focus
- **WHEN** a concrete changed-file row is selected in the Files navigator and the reviewer presses
  `x`
- **THEN** reviewr records the exact active full bounds unless the file is already Reviewed, in which
  case it clears the applicable current checkpoint

#### Scenario: Reviewer marks from reader focus
- **WHEN** the Files reader displays exact current bounds and the reviewer presses `x`
- **THEN** reviewr marks exactly the bounds represented by that reader

#### Scenario: Reviewer clicks the badge separator
- **WHEN** the reviewer clicks any cell in the one-column separator plus three-column badge cell
- **THEN** reviewr performs the same mark/unmark action as `x`

#### Scenario: Directory is selected
- **WHEN** navigator focus is on a directory and the reviewer invokes the mark action
- **THEN** no receipt or directory-owned review state is created

#### Scenario: Git observation reader is active
- **WHEN** Git Log or another observation-only workspace displays the same path as a reviewed Files
  comparison and the reviewer invokes `x`, `R`, or `X`
- **THEN** it neither advertises nor mutates Files review coverage or reader bounds

### Requirement: Updated comparisons support incremental reader bounds
An Updated file SHALL open on its exact reviewed frontier to current bounds through the shared file
comparison reader. Its title SHALL clearly say `since reviewed` and report the incremental delta.
`R` SHALL toggle that reader between incremental and full active comparison bounds without changing
coverage, comments, or other place state. Partial and Basis changed files SHALL use the full active
comparison and explain the uncovered or unsafe basis.

#### Scenario: Updated file opens
- **WHEN** the reviewer opens an Updated file
- **THEN** the reader defaults to `last reviewed -> current` and labels the view `since reviewed`

#### Scenario: Reviewer toggles full bounds
- **WHEN** an Updated incremental reader is active and the reviewer presses `R`
- **THEN** the same file and current right endpoint remain active while the reader shows the full
  active comparison

#### Scenario: Reviewer toggles back to incremental bounds
- **WHEN** the full view of an Updated file is active and the reviewer presses `R` again
- **THEN** the reader returns to the exact reviewed-frontier comparison without mutating receipts or
  comments

#### Scenario: Partial comparison opens
- **WHEN** the reviewer opens a Partial file
- **THEN** the reader shows the full active comparison and states that an older gap remains

#### Scenario: Incremental edge is explicitly marked
- **WHEN** the Updated reader displays exact incremental bounds and the reviewer presses `x`
- **THEN** reviewr records that incremental edge, composes it with the reviewed prefix, and derives
  Reviewed for the active full comparison

### Requirement: Gap navigation and directory progress include the complete tree
`X` SHALL cycle through review gaps by Basis changed, Updated, Partial, then Unreviewed priority and
by tree order within each state. Reviewed files SHALL remain reachable through ordinary navigation.
Directories SHALL derive `reviewed/changed` progress from all changed descendants, including hidden
descendants, and SHALL never own receipts.

#### Scenario: Multiple gap states exist
- **WHEN** changed files include Basis changed, Updated, Partial, and Unreviewed states
- **THEN** repeated `X` actions cycle through them by state priority and tree order

#### Scenario: Highest-priority gap is collapsed
- **WHEN** the next gap is beneath a collapsed directory
- **THEN** `X` expands the necessary ancestors, selects that concrete file, and opens its reader

#### Scenario: Directory descendants change
- **WHEN** a new changed descendant appears beneath a collapsed directory
- **THEN** its directory progress denominator includes the child and cannot retain a stale complete
  roll-up

#### Scenario: No review gaps remain
- **WHEN** every concrete changed-file comparison derives Reviewed
- **THEN** `X` leaves place unchanged and reports that all changed files are reviewed

### Requirement: Review presentation keeps independent signals
Every changed-file row SHALL reserve a distinct rightmost review field using fixed-width ASCII
badges `[ ]`, `[x]`, `[~]`, and `[!]`. The review field SHALL remain semantically and visually
distinct from filetype icons, Git status, file names, and statistics. Unchanged files visible in All
scope SHALL reserve no review badge.

#### Scenario: All five internal states render together
- **WHEN** visible changed rows represent all five internal review states
- **THEN** each row renders an aligned three-column badge, with Updated rendered as `[~]` and both
  Partial and Basis changed rendered as `[!]`

#### Scenario: Unchanged All-scope file renders
- **WHEN** an unchanged file is visible in All scope
- **THEN** its row has no review badge or clickable review cell

#### Scenario: Review state changes
- **WHEN** a file moves from Reviewed to Updated after an edit
- **THEN** only its derived review presentation and applicable reader bounds change; its filetype and
  Git status channels remain independent

### Requirement: Receipts persist privately and recover safely
Review receipts SHALL survive refresh, scope switches, process restart, and pane toggles in a
versioned application-private state file outside the repository. State SHALL be keyed and guarded by
canonical common-repository identity, canonical worktree identity, and comparison identity. Each
explicit mutation SHALL be atomically replaced after replay against locked current state so multiple
processes or panes do not lose disjoint marks or resurrect cleared marks.

#### Scenario: Process restarts
- **WHEN** reviewr records a receipt successfully and later starts a new process for the same
  canonical repository and worktree
- **THEN** the new process reloads the receipt and derives status against current exact comparisons

#### Scenario: Same repository has multiple worktrees
- **WHEN** two worktrees share a common Git directory
- **THEN** their canonical worktree identities select distinct guarded receipt state

#### Scenario: Concurrent panes mark different files
- **WHEN** panes that loaded the same old ledger explicitly mark different exact comparisons
- **THEN** serialized delta replay preserves both marks rather than replacing one with stale state

#### Scenario: State is missing, corrupt, newer, or identity-mismatched
- **WHEN** persisted state cannot be safely loaded or understood
- **THEN** reviewr starts Unreviewed, shows a concise warning, leaves the suspect file untouched, and
  keeps normal in-memory review available

#### Scenario: Atomic persistence fails
- **WHEN** an explicit review mutation succeeds in memory but its locked atomic replacement fails
- **THEN** the valid in-memory receipt remains active and reviewr warns that it will not survive
  restart

#### Scenario: Repository safety is audited
- **WHEN** review state is marked, cleared, recovered, and reloaded
- **THEN** project files, comments, the index, branches, `HEAD`, Git objects, and public Git refs are
  unchanged by review tracking

### Requirement: Retained review content is bounded
Reviewr SHALL retain exact endpoint identities for binary, oversized, and text states while bounding
per-snapshot, total retained-content, and receipt-history resources. It SHALL retain text only when
safe and sufficient to render a later incremental comparison, compact redundant coverage without
inventing edges, and derive Basis changed after an edit when the needed snapshot is unavailable.

#### Scenario: Reviewed text is within the snapshot bound
- **WHEN** exact reviewed text is within configured retention budgets
- **THEN** reviewr may retain the content needed to render a later `since reviewed` comparison

#### Scenario: Reviewed text exceeds the snapshot bound
- **WHEN** an exact oversized file is explicitly reviewed and then changes
- **THEN** its original exact receipt remains valid for exact reversion but the changed comparison
  derives Basis changed instead of an invented incremental diff

#### Scenario: Reviewed binary changes
- **WHEN** an exact binary endpoint is explicitly reviewed and later has a different exact identity
- **THEN** reviewr derives Basis changed because no retained text can prove an incremental diff

#### Scenario: Receipt history is compacted
- **WHEN** duplicate or redundant receipts and retained payloads exceed configured budgets
- **THEN** reviewr preserves the newest exact edge identities needed for honest coverage and drops
  only data whose removal cannot create false Reviewed coverage

### Requirement: Refresh preserves authored place and comments
World refresh SHALL reconcile review derivation by comparison and file identity, never by row index.
It SHALL preserve the selected path, navigator cursor and top, reader cursor, logical line and scroll,
focus, directory folds, layout, and explicit incremental/full bounds wherever their identities
survive, using nearest-survivor then clamping fallback under **Continuity**. Review actions,
invalidation, persistence recovery, and bounds changes SHALL NOT delete, publish, or retarget
comments.

#### Scenario: File changes while incremental bounds are open
- **WHEN** the current right endpoint advances and the reviewed frontier remains safe
- **THEN** reviewr keeps the same file and logical reader line where possible, expands the right
  endpoint to current, and preserves the reviewed left endpoint

#### Scenario: Stale comparison build is visible
- **WHEN** old content remains painted while a matching comparison refresh is in flight
- **THEN** reviewr does not label that content current or permit it to be marked against newer
  endpoints

#### Scenario: Review state rederives on refresh
- **WHEN** a selected file changes from Reviewed to Updated, Partial, or Basis changed
- **THEN** its status changes without opportunistically moving selection, scroll, focus, folds,
  layout, reader-bound preference, or comments

#### Scenario: Selected identity disappears
- **WHEN** a refresh removes the selected changed-file identity
- **THEN** reviewr applies identity, nearest-survivor, then clamping reconciliation without using a
  receipt to choose an arbitrary place
