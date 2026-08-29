## Why

The repository still presents Rust-first commands, CI, source layout, and specifications even
though the Go rewrite is now the chosen implementation. That makes the fast path harder to discover
and leaves every change paying for tooling that is no longer part of active development.

## What Changes

- **BREAKING** Give the short `build`, `check`, `dev`, `setup`, and `test` recipes to Go.
- Add Prek-managed repository hooks and make `just check` the single finite local and CI gate.
- **BREAKING** Move the Rust Cargo project and old documentation under `legacy/`, exposed only as
  `just legacy build|dev|run`.
- Remove the Rust PTY A/B harness and its committed baseline.
- Replace the audit, release, and Rust CI workflows with one push workflow running `just check`.
- Initialize a fresh root OpenSpec workspace for future Go changes.

## Capabilities

### New Capabilities

- `developer-workflow`: Clone setup, primary Go development commands, validation, the legacy oracle
  boundary, and continuous integration.

### Modified Capabilities

None.

## Impact

This changes the root command surface, development-shell tools, Git hooks, GitHub Actions, source
layout, Nix paths, contributor instructions, and specification workflow. Runtime behavior is not
changed, and the default Nix/plugin package remains the legacy Rust binary for now.
