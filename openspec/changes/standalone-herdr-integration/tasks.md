## 1. Runtime detection

- [x] 1.1 Add the immutable Herdr context and pure environment detector, with focused tests for
  hosted, standalone, partial-context, and non-authoritative marker inputs.
- [x] 1.2 Detect context once in `cmd/reviewr`, inject it through application construction, and verify
  executable and app tests receive one stable snapshot without a Herdr CLI call.
- [x] 1.3 Add a reusable Herdr runtime owner with bounded, asynchronous, ownership-safe current-pane
  labeling and focused tests for unlabeled, custom-labeled, replaced, partial, and standalone cases.

## 2. Plugin retirement

- [x] 2.1 Delete `herdr-plugin.toml` and the `herdr/` installer and pane-lifecycle scripts, then verify
  the active tree contains no plugin manifest, plugin action, event hook, or plugin-owned launch path.
- [x] 2.2 Update AGENTS, README, security, and other active guidance to describe standalone launch and
  automatic optional-host detection, while leaving explicitly historical `legacy/` material intact.

## 3. Validation

- [x] 3.1 Run focused Herdr detection and runtime tests with hosted and standalone environments and
  verify detection performs no Git write, CLI call, pane operation, or agent send while the separate
  title capability performs only its ownership-safe current-pane operations.
- [x] 3.2 Run `just check`, `just build`, strict OpenSpec validation, active-reference auditing, and
  `git diff --check`; record a clean implementation diff with the unrelated demo stash untouched.
