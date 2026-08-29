## Context

See `proposal.md` for motivation. The root currently contains two language implementations, Rust-
specific automation, and a historical specification system. The Go rewrite is now the active
product, but the Rust binary must remain runnable and Nix-packaged until parity.

## Goals / Non-Goals

**Goals:**

- Make the root command surface describe one obvious Go lifecycle.
- Make the Rust boundary spatially and operationally explicit.
- Use the same finite check locally, in hooks, and on pushes.
- Start future product specifications from an empty OpenSpec workspace.

**Non-Goals:**

- Port more runtime behavior.
- Switch the Herdr plugin or Nix default package to Go.
- Modernize, reformat, or reorganize Rust internals.
- Preserve the retired Rust benchmark and release automation.

## Decisions

### Use a Just submodule for the Rust oracle

`legacy/justfile` exposes `build`, `dev`, and `run`, while the root imports it as `legacy`. This
produces the requested `just legacy <verb>` namespace without a case statement or a long list of
prefixed root recipes. The legacy target directory stays under root `target/legacy/` so generated
artifacts remain outside source.

### Use Prek as the hook runner

Prek matches the sibling template and gives local hooks and CI one configuration. Hooks own common
file hygiene, Go formatting and vetting, Nix checks, workflow linting, and secret scanning. Tests
remain a Just recipe so they are equally useful alone and from `check`.

### Keep packaging pointed at the legacy Cargo project

The Rust toolchain and package derivation move with Cargo under `legacy/`; `flake.nix` changes only
their paths. The Go package migration remains a separate acceptance gate.

### Preserve old documentation as legacy material

The previous `docs/` and Rust responsiveness policy move under `legacy/`. A fresh `openspec/`
becomes the only active specification root. Historical documents are not rewritten to pretend they
were authored under the new layout.

## Risks / Trade-offs

- **Rust release automation is removed before Go packaging exists** → Keep the legacy Nix package
  evaluable and runnable; design the new release workflow when Go packaging is ready.
- **Repository-wide formatter hooks may expose old formatting drift** → Treat hook edits as part of
  this one-time tooling migration and keep generated or licensed assets excluded.
- **Moving Cargo can break relative paths** → Move syntax assets with the crate and validate Cargo,
  Just, and Nix entrypoints after the relocation.

## Migration Plan

1. Move Cargo, Rust source/tests/assets, and historical docs under `legacy/`.
2. Install the new root Just and Prek surfaces and initialize OpenSpec.
3. Remove retired benchmarks and workflows.
4. Validate the Go lifecycle, legacy build/run paths, OpenSpec artifacts, and Nix evaluation.
5. Restore the temporary review fixture at its new legacy paths after committing.
