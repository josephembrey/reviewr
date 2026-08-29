# Go rewrite foundation: Measurements

Date: 2026-08-29
Environment: x86_64 Linux, Go 1.26.7 from `nix develop`

## Build and test

Measurements use the committed dependency graph and the same machine/load. The clean build used a
new isolated `GOCACHE` created with `mktemp -d`; it did not delete or perturb shared caches. The Go
module cache was already populated, so network download time is excluded. The warm build reused
that isolated cache. The test measurement used `-count=1` so tests executed instead of returning
cached results.

| Measurement | Result |
| --- | ---: |
| Clean `go build -o target/go/reviewr ./cmd/reviewr` | 8.060 s |
| Warm same-cache Go build | 0.154 s |
| `go test -count=1 ./...` | 2.687 s |
| `target/go/reviewr` size | 5,667,504 bytes (5.41 MiB) |

## Source size

Line counts use `wc -l` across `cmd/` and `internal/`.

| Class | Lines |
| --- | ---: |
| Production Go (`*.go`, excluding `*_test.go`) | 1,105 |
| Go tests (`*_test.go`) | 787 |
| Total Go | 1,892 |

Largest files at measurement time:

| File | Lines | Class |
| --- | ---: | --- |
| `internal/app/model.go` | 231 | production |
| `internal/ui/render.go` | 229 | production |
| `internal/repository/repository_test.go` | 226 | test |
| `internal/app/model_test.go` | 173 | test |
| `internal/repository/repository.go` | 155 | production |
| `internal/navigation/state.go` | 149 | production |
| `internal/ui/geometry_test.go` | 143 | test |
| `internal/navigation/state_test.go` | 127 | test |
| `internal/ui/geometry.go` | 111 | production |
| `internal/git/git.go` | 101 | production |

## Validation evidence

- `gofmt`, `go vet ./...`, `go test ./...`, and `go test -race ./...`: pass.
- `just go-ci`: pass, including output at `target/go/reviewr`.
- PTY startup/input/quit smoke for both `target/go/reviewr` and `target/go/reviewr .`: pass. The
  smoke was kept as an acceptance command rather than a committed harness because the semantic,
  render, and root-loop tests carry the deterministic behavior coverage.
- `nix flake check --no-build`: pass. The default derivation remains the Rust
  `reviewr-0.36.1`; `nix develop --command go version` reports Go 1.26.7.
- Unchanged Rust `just ci`: pass (format, Clippy with warnings denied, all tests, QA script tests,
  and optimized release build).

## Known gaps

This foundation intentionally stops at plain file browsing. Diffing, highlighting, Git browsing,
search, comments and the future **Comments survive** implementation, review state, configuration,
Herdr integration/export, editor handoff, polling, and packaging remain absent. Rust remains the
only packaged implementation and the behavioral oracle.
