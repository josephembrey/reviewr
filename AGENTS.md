# AGENTS.md

reviewr is being rebuilt in Go as a Bubble Tea TUI. The Go application is the primary development
target. The previous Rust application is frozen under `legacy/` as the behavioral oracle and remains
the packaged implementation until packaging migrates separately.

## Commands

- `just` — list tasks.
- `just setup` — download Go modules and install the Prek pre-commit hook. Run this after cloning.
- `just dev [repository-path]` — run the Go application directly from source.
- `just build` — build the Go application to `dist/reviewr`.
- `just test` — run the Go tests and race detector.
- `just check` — run all repository hooks, then the tests. This is the only CI command.
- `just legacy build|dev|run [repository-path]` — build, develop, or run the frozen Rust oracle.

Run a single Go test with `go test ./internal/<package> -run <name>`. The Nix development shell
contains every command used by the Justfile and hooks.

## OpenSpec

Active behavior is specified through OpenSpec. Start product work with `$openspec-propose`, apply an
approved change with `$openspec-apply-change`, and archive completed work with
`$openspec-archive-change`. Read the applicable artifacts under `openspec/` before implementation.

Load-bearing invariants:

- **No writes**: reviewr never mutates the worktree, index, or branches. Git writes, once ported,
  remain limited to private refs under `refs/worktree/reviewr/`.
- **Comments survive**: comments leave only through explicit successful export. Their store remains
  in memory by design; do not propose persistence.
- **Continuity**: background changes reconcile place state by identity, nearest survivor, then
  clamping. World events never arbitrarily move user-controlled cursor, selection, scroll, focus,
  folds, or layout.

## Go architecture

- `cmd/reviewr/` — executable wiring only.
- `internal/app/` — thin Bubble Tea root, semantic actions, and effect routing.
- `internal/repository/` — repository resolution and bounded file reads.
- `internal/git/` — read-only Git CLI adapter.
- `internal/navigation/` — file identity, selection, scroll, and Continuity reconciliation.
- `internal/ui/` — shared geometry, hit testing, and rendering.

Keyboard and mouse input become semantic actions before they mutate state. Rendering and mouse hit
testing consume one shared geometry calculation. Do not add placeholder packages for unimplemented
features or recreate the Rust structure mechanically.

## Legacy oracle

`legacy/` is a self-contained Cargo project containing the Rust source, tests, syntax assets,
toolchain, historical documentation, and Nix package derivation. It is read-only product history:
integrate only important correctness fixes or behavior needed as a migration oracle. Do not
reorganize or extend it.

The legacy runtime remains useful for parity checks. Its architecture and detailed behavioral
contracts are recorded in the existing change specs and tests. Preserve **No writes**, **Comments
survive**, and **Continuity** whenever a legacy correctness fix is unavoidable.

The default Nix package still launches the legacy binary until the Go implementation reaches
packaging parity. Herdr is an optional runtime host for the standalone application; do not
reintroduce plugin manifests, plugin-owned installation, or pane-lifecycle scripts.
