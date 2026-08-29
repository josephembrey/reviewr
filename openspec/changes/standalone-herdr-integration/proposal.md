## Why

reviewr is now a standalone Go application, but the repository still presents it as a Herdr plugin
with plugin-owned installation, pane lifecycle, and event scripts tied to the frozen Rust runtime.
Herdr should instead be an optional host that reviewr recognizes at runtime, without controlling how
the application is installed or launched.

## What Changes

- **BREAKING**: remove `herdr-plugin.toml`, the `herdr/` plugin installer and pane-action scripts,
  plugin event hooks, plugin-owned launch paths, and active documentation for plugin installation.
- Add a small Go host-detection boundary that inspects the process environment once at startup and
  reports whether reviewr is running inside Herdr.
- Automatically make detected Herdr context available to application services; standalone launches
  continue with the same behavior when no Herdr context is present.
- Add a reusable Herdr runtime module that can own hosted capabilities. Its first capability labels an
  otherwise-unlabeled current pane `reviewr` for the process lifetime without replacing custom labels.
- Treat the standalone `reviewr` binary as the sole application entry point. Herdr layouts or user
  configuration may launch it, but reviewr no longer opens, closes, toggles, or auto-opens panes.
- Keep the frozen plugin implementation only as historical material under `legacy/` where it is
  useful as a behavioral oracle; do not retain active compatibility shims.

## Capabilities

### New Capabilities

- `runtime/herdr-detection`: Detect optional Herdr runtime context and expose it to the standalone
  application without plugin lifecycle or packaging.

### Modified Capabilities

None.

## Impact

- Removes the root plugin manifest and `herdr/` scripts.
- Adds a focused Go package for immutable runtime-host context and wires it through executable
  startup without coupling the Bubble Tea root to environment reads. The package also owns the
  bounded, asynchronous lifecycle of Herdr-specific capabilities such as the current pane label.
- Updates active repository guidance, security language, and packaging boundaries.
- Existing `herdr plugin install`, plugin actions, and worktree-created auto-open behavior stop being
  supported. Launching reviewr remains the responsibility of the user, shell, layout, or future
  first-class Herdr configuration.
- The Nix package migration from the Rust oracle to Go and feature-level agent/comment integration
  remain separate changes.
