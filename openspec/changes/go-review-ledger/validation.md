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
  `[+]` and opened `since reviewed`; `R` showed the full comparison; reader-focused `x` marked only
  the incremental edge and restored `[x]`.
- Quitting and restarting with the same isolated state root recovered `[x]`.
- An exact binary addition displayed the binary notice and was explicitly markable.
- An oversized text addition displayed the bounded notice and was explicitly markable. Changing it
  after review refreshed to `[!]` with `review basis changed; full comparison`, without inventing an
  interdiff.
- Collapsing a nested directory and invoking `X` expanded the required ancestor and selected the
  hidden gap.

## Final repository gates

- `nix develop -c just build`: pass.
- `git diff --check`: pass.
- `openspec validate go-review-ledger --strict`: pass.
- `nix develop -c just check`: pass, including all hooks, ordinary and `dev` tests, and the full
  repository race suite.
- Source/tests and specification/documentation were committed as focused local commits. No push or
  merge commit was performed; the branch retains the user-authorized fast-forward to `640515b`.
