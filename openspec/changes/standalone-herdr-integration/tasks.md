## 1. Runtime detection

- [ ] 1.1 Add the immutable Herdr context and pure environment detector, with focused tests for
  hosted, standalone, partial-context, and non-authoritative marker inputs.
- [ ] 1.2 Detect context once in `cmd/reviewr`, inject it through application construction, and verify
  executable and app tests receive one stable snapshot without a Herdr CLI call.

## 2. Plugin retirement

- [x] 2.1 Delete `herdr-plugin.toml` and the `herdr/` installer and pane-lifecycle scripts, then verify
  the active tree contains no plugin manifest, plugin action, event hook, or plugin-owned launch path.
- [x] 2.2 Update AGENTS, README, security, and other active guidance to describe standalone launch and
  automatic optional-host detection, while leaving explicitly historical `legacy/` material intact.

## 3. Validation

- [ ] 3.1 Run focused Herdr detection tests with hosted and standalone environments and verify startup
  performs no Git write, Herdr mutation, pane operation, or agent send.
- [ ] 3.2 Run `just check`, `just build`, strict OpenSpec validation, active-reference auditing, and
  `git diff --check`; record a clean implementation diff with the unrelated demo stash untouched.
