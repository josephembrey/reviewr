# Go rewrite foundation: Plan

Delivers the sibling `spec.md` as one independent vertical slice. Tickets are intentionally omitted:
the package contracts and validation stages are small enough to keep together here.

## 1. Establish the coexistence foundation

**Status:** complete

1. Add `go.mod`/`go.sum` for Go 1.25, Bubble Tea v2, and Lip Gloss v2.
2. Add Go to the existing Nix development shell without changing the Rust package or default app.
3. Add only the additive `go-build`, `go-test`, `go-run`, and `go-ci` recipes. Keep `just ci`
   byte-for-byte in behavior.
4. Document the dual-language commands, real Go package boundaries, and Rust-oracle status in
   `AGENTS.md`.

## 2. Build the read-only repository boundary

**Status:** complete

1. Implement root resolution and the NUL-safe `ls-files` adapter with optional locks disabled.
2. Implement bounded, root-relative file reads and explicit safe display classifications.
3. Cover unusual path bytes representable as Go strings, file races/failures, and the **No writes**
   real-repository assertion.

## 3. Build place state and shared geometry

**Status:** complete

1. Implement pure Navigator/Reader place state and **Continuity** reconciliation by raw path.
2. Implement half-open rectangles, one responsive layout calculation, and explicit mouse hit
   results in `internal/ui`.
3. Test nearest-survivor fallback, clamping, resize, focus, scrolling, mouse precedence, and the
   agreement between visible rows and click targets.

## 4. Compose the Bubble Tea vertical slice

**Status:** complete

1. Route Bubble Tea key, resize, and mouse messages into semantic actions.
2. Keep root update thin and model repository work as explicit commands with latest-wins list and
   content generations.
3. Render the Navigator, Reader, loading/error/file states, focus, and compact help with Lip Gloss.
4. Run the application against temporary and real worktrees, adding a PTY smoke only if reliable.

## 5. Validate, measure, and commit

**Status:** complete

1. Run formatting, unit/integration/race/vet, `just go-ci`, Nix evaluation, and unchanged Rust
   coexistence checks.
2. Measure cold and warm Go build times under distinct empty caches, warm test time, binary size,
   production/test line counts, and largest Go files without deleting shared caches.
3. Review the full diff for scope, invariant, and packaging violations.
4. Mark the spec and plan implemented, commit coherent changes on `go-rewrite-foundation`, and
   confirm the worktree is clean. Do not push, merge, QA-install, or operate Herdr panes.

## Acceptance checklist

- [x] The minimal TUI behavior in `spec.md` works from the current directory and an explicit path.
- [x] **No writes**, **Continuity**, **Single geometry**, and **Rust oracle** are enforced by code and
  evidence.
- [x] All requested focused tests exist and pass, including race and real-Git no-write coverage.
- [x] Go output is `target/go/reviewr`; Rust packaging and `just ci` are unchanged.
- [x] Nix exposes Go alongside Rust while the default package remains Rust reviewr.
- [x] Measurements and known gaps are recorded without expanding scope for artificial parity.
