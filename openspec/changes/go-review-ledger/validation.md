# Validation evidence

Date: 2026-08-29
Environment: x86_64 Linux, Go 1.26.7 in `nix develop`

## Focused automated checks

- `go test -race ./internal/review ./internal/repository ./internal/app ./internal/ui
  ./internal/filetree ./internal/navigation`: pass.
- Focused pure-domain coverage includes exact endpoint composition, direct and cumulative edges,
  narrow versus broad scopes, all five states, exact revert, action/kind/mode changes, rename, copy,
  deletion, symlink, submodule, binary, oversized and unavailable snapshots, rebases, compaction, and
  bounded retained content.
- Persistence coverage includes canonical linked-worktree keys, private paths/modes, restart,
  corrupt/older/newer/identity-mismatched recovery, locked concurrent deltas, stale clears, every
  injected atomic-write failure stage, and a repository no-writes audit including Git objects.
- App/render coverage includes `x`, `R`, `X`, the four-cell mouse target, latest-wins review
  generations, stale endpoint rejection, directory rollups over hidden descendants, unchanged rows,
  reader titles/notices, queued persistence, write warnings, scope and refresh rederivation, and
  path/focus/fold/bounds/logical-line/scroll/selection-anchor Continuity.

## Latency observations

Command:

```text
go test ./internal/app -run '^$' \
  -bench 'BenchmarkReview(RefreshDerivation|NavigatorSteadyState)1000$' \
  -benchtime=500ms -count=3
```

The fixture contains 1,000 changed-file comparisons; refresh derivation also contains 500 exact
receipts. Results on this host:

| Operation | Three observations |
| --- | --- |
| Review refresh derivation | 2.736 ms, 2.632 ms, 2.895 ms |
| Cached steady navigator presentation | 4.338 ms, 4.785 ms, 4.748 ms |

Steady navigation performs no Git calls or content hashing. Worktree hashing occurs only in tagged
comparison/document/verification effects. Endpoint bodies and retained receipts remain bounded.

## Live PTY smoke

The built `dist/reviewr` was exercised in an 80x24 PTY against both this worktree and an isolated Git
fixture:

- All/Changed and File/Diff switched over the single typed snapshot. Nested Bontree folders,
  independent status prefixes, icons, right badges, rollups, scrollbar geometry, folding, and `X`
  expansion remained aligned.
- Marking a new text file changed `[ ]` to `[x]`; editing it externally and refreshing changed it to
  `[~]` and opened `since reviewed`; `R` showed the full comparison; reader-focused `x` marked only
  the incremental edge and restored `[x]`.
- Quitting and restarting with the same isolated state root recovered `[x]`.
- An exact binary addition displayed the binary notice and was explicitly markable.
- An oversized text addition displayed the bounded notice and was explicitly markable. Changing it
  after review refreshed to `[!]` with `re-review required; full comparison`, without inventing an
  interdiff.
- Collapsing a nested directory and invoking `X` expanded the required ancestor and selected the
  hidden gap.

## Final repository gates

- `nix develop -c just build`: pass.
- `git diff --check`: pass.
- `openspec validate go-review-ledger --strict`: pass.
- `nix develop -c just check`: pass, including all hooks, ordinary and `dev` tests, and the full
  repository race suite.
- Source/tests and specification/documentation were committed as focused local commits without a
  push. The later main-tip reconciliation is recorded below.

## Main-tip reconciliation

The completed ledger was reconciled with exact main tip
`c4b0574f1cea2f299a8912be260c2d888e04c7a4` after main added Git Log/Refs/Stashes, the shared
Files/Stashes reader document, Notes, compact rows and headers, and initial recursive Files
collapse.

- Main remains authoritative for the typed repository snapshot and reads, Git and Notes state,
  Bontree icons/status styling, first-snapshot `CollapseAll`, and shared render/hit-test geometry.
  Review remains an additive Files-only comparison provider and optional right-side row field.
- The canonical Git common-directory resolver is shared as the path authority. Notes remains
  clone-scoped under its own private note store; review receipt keys additionally include the
  canonical worktree and remain under the independent `reviews/` store.
- `TestInitialNestedCollapseCoexistsWithReviewBadgesAndRollups` proves hidden nested descendants
  remain initially collapsed while their parent derives complete review progress and visible changed
  files retain independent badges.
- `TestReviewActivityLeavesGitAndNotesPlaceUntouched` performs exact Files review activity and
  proves Log, Refs, Stashes, and Notes place state remain unchanged; semantic review actions are
  rejected whenever the current destination is not Files.
- `TestPTYReviewLedgerReconciliation` passes against a real Git fixture at 80x24 and 60x12. It
  covers initial collapsed progress, binary Basis changed, `x`, both `R` bounds, priority `X`
  ancestor expansion, a painted separator-cell badge click, text Updated and `since reviewed`,
  refresh, restart recovery, and Git/Notes switching.

### Reconciliation benchmark

The same three-run, 500 ms benchmark command was captured immediately before and after the merge.
The post-merge steady-state fixture explicitly expands its initially collapsed directory so both
sides measure 1,000 visible changed rows.

| Operation | Pre-merge observations | Post-merge observations | Mean change |
| --- | --- | --- | --- |
| Review refresh derivation | 2.598 ms, 2.814 ms, 2.733 ms | 2.791 ms, 2.666 ms, 2.680 ms | 2.715 ms → 2.712 ms (-0.1%) |
| Cached steady navigator presentation | 4.778 ms, 4.195 ms, 4.833 ms | 5.858 ms, 4.619 ms, 5.515 ms | 4.602 ms → 5.331 ms (+15.8%) |

The presentation samples have visible run-to-run variance. They perform no Git calls or endpoint
hashing; the merge adds main's commit-row scan and time-aware shared row dispatch while retaining
one tree renderer and one review geometry calculation.

### Reconciliation gates

- Focused ordinary and race tests for review, repository, app, UI, tree/navigation, Git,
  Notes, commit graph/rows, and the executable: pass.
- `nix develop -c just check`: pass, including hooks, ordinary tests, `dev` tests, the full race
  suite, and both real PTY tests.
- `nix develop -c just build`: pass.
- `git diff --check`: pass.
- `openspec validate go-review-ledger --strict`: pass; all 26 tasks remain complete.
