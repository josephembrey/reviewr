## Context

See `proposal.md` for motivation and
`specs/runtime/herdr-detection/spec.md` for the behavior contract. The active Go executable currently
opens a repository and constructs the Bubble Tea model directly. It has no Herdr package or runtime
dependency. Root `herdr-plugin.toml` and `herdr/` still implement installation, pane actions, and an
event hook for the frozen Rust application.

The current Herdr runtime injects `HERDR_ENV=1` plus optional `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`,
`HERDR_PANE_ID`, `HERDR_SOCKET_PATH`, and `HERDR_BIN_PATH` values into managed panes. That inherited
environment is sufficient to identify the host without a subprocess or plugin context payload.

## Goals / Non-Goals

**Goals:**

- Establish one tested startup seam for optional Herdr context.
- Remove all active plugin distribution and pane-lifecycle artifacts.
- Keep standalone startup independent of Herdr availability and latency.
- Give later agent/comment features a narrow host context instead of direct global environment reads.

**Non-Goals:**

- Reimplement plugin toggle, open, close, placement, or worktree auto-open behavior.
- Port comment export, turn tracking, or other Rust Herdr features in this change.
- Switch the default Nix package from the Rust oracle to Go.
- Add a generalized plugin/host framework before a second host exists.

## Decisions

### Use the inherited marker as the detection authority

A focused `internal/herdr` package will expose an immutable `Context` and a pure detector accepting an
environment lookup function. `HERDR_ENV=1` selects hosted mode; the remaining values are optional
capability inputs captured verbatim.

This avoids a startup subprocess, socket probing, and false positives from merely finding a `herdr`
binary on `PATH`. CLI or socket validation belongs to the specific future action that needs it.

Alternatives considered:

- `exec.LookPath("herdr")`: installation does not prove that this process is hosted.
- Run `herdr pane current`: adds startup latency and failure modes and performs work before any
  feature needs it.
- Read environment values in each feature: creates inconsistent snapshots and scattered host policy.

### Inject context at the executable composition root

`cmd/reviewr` will detect context once and pass it through explicit application construction. The
Bubble Tea update loop will not read process environment or invoke Herdr during initialization. The
context remains data, not a global singleton, which keeps unit tests deterministic and leaves effect
ownership explicit when host features arrive.

Alternatives considered:

- Package-global detection: simple initially, but hidden state makes tests and alternate runtimes
  harder to reason about.
- Defer the context until the first Herdr feature: would remove the plugin but fail to establish the
  requested automatic host boundary now.

### Delete plugin lifecycle assets instead of preserving compatibility wrappers

The root manifest and `herdr/` scripts will be deleted. Active docs will describe the executable as
standalone and AGENTS guidance will identify Herdr as an optional detected host. Historical Rust code
under `legacy/` remains untouched, but the plugin manifest, installer, and pane scripts will not be
moved there because they are distribution machinery rather than part of the oracle executable.

Keeping deprecated aliases or wrapper scripts would make the removed plugin path appear supported and
continue carrying hundreds of lines of shell lifecycle logic.

## Risks / Trade-offs

- [Users lose plugin-managed installation and automatic pane creation] → Call this out as a breaking
  migration and document standalone launch through a shell, layout, or Herdr user configuration.
- [A hosted process can have partial context] → Preserve hosted mode and represent individual values
  as optional; features validate only the fields they consume.
- [The first Go slice does not yet visibly use Herdr context] → Keep the seam minimal and tested; do
  not add placeholder agent or pane services.
- [Legacy documentation still mentions the former plugin] → Keep `legacy/` explicitly historical and
  exclude it from active-reference audits.

## Migration Plan

1. Add and test the pure context detector and composition-root injection.
2. Delete the root plugin manifest and `herdr/` plugin scripts.
3. Remove active plugin references and document standalone optional-host behavior.
4. Run repository checks, ensure no active plugin identifiers remain, and smoke both Herdr and
   non-Herdr detection inputs.

Rollback is a normal commit revert: the removed manifest and scripts remain recoverable from Git
history, while the new detector is isolated from repository and layout state.
