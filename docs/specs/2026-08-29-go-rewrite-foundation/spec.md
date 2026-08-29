# Go rewrite foundation

Status: Implemented
Date: 2026-08-29

## Decision

reviewr will be rewritten in Go to shorten implementation and validation cycles. The existing Rust
application remains runnable, packaged, and unchanged as the behavioral oracle while the Go
application grows in independent vertical slices. This is not a Rust/Go hybrid: there is no FFI,
shared runtime, mechanical translation, or packaging switch in this change.

This foundation proves one useful standalone workflow: open a Git worktree, browse its tracked and
untracked nonignored files, and read the selected file's current plain content.

## Runtime contract

`go run ./cmd/reviewr [repository-path]` starts a full-window TUI. With no path it starts from the
current directory. In either form, startup resolves the containing Git worktree root and reports a
clear error when no worktree can be resolved.

The screen has two panes:

- Navigator lists `git ls-files -z --cached --others --exclude-standard` results in Git's order.
- Reader displays the selected file's current worktree content without syntax highlighting.

Input is translated into semantic actions before state changes:

- `j`, `k`, Down, and Up act on the focused pane: Navigator changes the selected file and Reader
  scrolls by one source line.
- Tab moves focus intent between Navigator and Reader.
- `r` explicitly reloads the file list and selected content. There is no background polling.
- `q` and Ctrl-C quit.
- A left click focuses a pane; a click on a visible Navigator row selects that path.
- Mouse wheel events over Navigator move selection, and wheel events over Reader scroll content.
- Terminal resize recomputes layout without resetting selected-file identity or reader scroll.

The Navigator displays control characters in paths as visible escaped text while retaining the raw
path as identity. Reader content is terminal-safe: control bytes cannot inject terminal escape
sequences, invalid UTF-8 is replaced for display, carriage returns are normalized, and tabs are
expanded predictably.

File reads are bounded at 1 MiB and have explicit states for loading, ready, missing, unreadable,
binary (NUL-containing), and too large. Symlinks are shown as link targets without following them.
Empty repositories and transient Git/read failures remain navigable and render an explicit state.

## Architecture and package contracts

Only packages exercised by this slice are introduced:

- `cmd/reviewr` parses the optional path, opens the repository, and runs Bubble Tea.
- `internal/git` is the only Git subprocess boundary. It sets `GIT_OPTIONAL_LOCKS=0`, resolves the
  worktree root, and parses the NUL-delimited file listing. Its commands are read-only.
- `internal/repository` owns root-relative, traversal-safe file reads and their bounded result
  classification. It composes the Git adapter into the data source consumed by the app.
- `internal/navigation` owns place state: file identities, selection, visible-list offset, focus,
  and reader scroll. Its pure reconciliation preserves identity and falls back through the nearest
  surviving old neighbor before clamping.
- `internal/ui` owns the one layout/geometry calculation, half-open rectangles, hit testing,
  terminal-safe display helpers, and Lip Gloss rendering. Render and mouse routing consume the same
  geometry value; neither reconstructs pane bounds.
- `internal/app` owns the thin Bubble Tea root, semantic actions, explicit effects, tagged load
  results, and composition of navigation plus UI state. Repository loads run as Bubble Tea commands.

The root has two monotonic generations: file-list requests and content requests. Only the latest
generation may land. A content completion must also match the selected raw path. A reload that keeps
the same selected identity leaves its older content visible until replacement content lands. If
reconciliation changes identity, the old reader is blanked and an explicit loading state is shown.

Bubble Tea `v2.0.6` and Lip Gloss `v2.0.5` are the stable releases verified for this foundation on
2026-08-29. Both use their `charm.land/.../v2` import paths and require Go 1.25. Bubbles is omitted
because this slice does not need a component from it.

## Invariants

- **No writes:** the Go process never changes the worktree, index, HEAD, branches, or refs. This
  slice does not write even private `refs/worktree/reviewr/` refs. Git access is through the
  read-only adapter and file access never requests write permissions.
- **Continuity:** selection, focus, visible-list offset, and reader scroll change only under user
  input. Reload results reconcile selection by raw path identity, then the nearest surviving old
  neighbor, then a clamped index. Derived reader content may remain stale for the same identity but
  may never display content belonging to another identity.
- **Comments survive (future):** comments are absent from this slice. The eventual Go comment model
  must preserve authored comments across refresh and edits and consume them only on explicit
  export. This change deliberately creates no comment store, persistence, or placeholder API.
- **Single geometry:** render and hit testing use the same calculated half-open rectangles. For
  rectangle `[x, x+w) x [y, y+h)`, right and bottom boundary cells are outside.
- **Rust oracle:** Rust source, Cargo behavior, default Nix package, plugin actions, packaging, and
  release binary paths remain unchanged. Go builds only to `target/go/reviewr`.

## Acceptance evidence

Focused table-driven tests cover:

- key/mouse message routing into semantic actions and the root's effect decisions;
- shared geometry, half-open boundaries, mouse precedence, and render/hit-test agreement;
- path-identity reconciliation, nearest-survivor fallback, selection, focus, scroll, and resize;
- repository root resolution and startup failures;
- NUL parsing with spaces, Unicode, embedded newlines, and a missing final delimiter;
- bounded reads and ready, empty, symlink, missing, unreadable, binary, and too-large results;
- a temporary real Git repository whose status, index bytes, HEAD, and refs are identical before
  and after all repository operations.

Acceptance runs `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `just go-ci`, a
relevant Nix flake evaluation, and unchanged Rust checks sufficient to prove coexistence. A PTY
smoke test is added only if it is deterministic and carries more confidence than maintenance cost.

The handoff records clean and warm Go build time, Go test time, binary size, production/test line
counts, largest Go files, exact validation, and commit hashes.

## Deliberately out of scope

This slice has no diff, syntax highlighting or Chroma, Git status/history/refs/stashes view, search,
comments, review ledger, scratchpad, folding, config recovery, Herdr integration/export, editor
handoff, background polling, or packaging switch. It adds no empty placeholder package or interface
for any of them. It does not delete or refactor Rust.
