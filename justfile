# reviewr dev tasks — run `just <task>` (https://github.com/casey/just)

# default: list tasks
default:
    @just --list

# format the code
fmt:
    cargo fmt --all

# check formatting (CI parity)
fmt-check:
    cargo fmt --all --check

# lint with clippy, warnings as errors (CI parity)
lint:
    cargo clippy --all-targets --all-features -- -D warnings

# run the test suite
test:
    cargo test --all-features

# build (debug)
build:
    cargo build

# run reviewr in the current repo
run:
    cargo run

# build the standalone Go foundation without colliding with Rust artifacts
go-build:
    mkdir -p target/go
    go build -o target/go/reviewr ./cmd/reviewr

# run the Go foundation test suite
go-test:
    go test ./...

# run the standalone Go foundation, optionally with a repository path
go-run *args:
    #!/usr/bin/env bash
    set -euo pipefail
    exec go run ./cmd/reviewr "$@"

# format gate, vet, tests (including race), and production Go build
go-ci:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l cmd internal)"
    if [[ -n "$unformatted" ]]; then
        printf 'gofmt required:\n%s\n' "$unformatted" >&2
        exit 1
    fi
    go vet ./...
    go test ./...
    go test -race ./...
    mkdir -p target/go
    go build -o target/go/reviewr ./cmd/reviewr

# benchmark the real global-search worker and painted results with the fast QA binary
bench-search:
    cargo build --profile qa
    python3 scripts/bench_tui.py --binary target/qa/reviewr --fixture --search-only

# build release and install the binary into bin/ for `herdr plugin link`
install:
    cargo build --release
    mkdir -p bin
    ./scripts/swap-binary.sh target/release/reviewr bin/reviewr

# build the fast non-LTO QA profile and swap it into the GitHub-installed plugin
qa-install:
    cargo build --profile qa
    ./scripts/qa-install.sh

# restore the released binary the last `just qa-install` replaced
qa-restore:
    ./scripts/qa-restore.sh

# exercise local QA target selection, swap, rollback, and restore in temporary plugin roots
test-qa-scripts:
    ./scripts/test_qa_scripts.sh

# PTY smoke test of the editor path against the release-like, non-LTO QA binary
smoke-edit:
    cargo build --profile qa
    python3 scripts/smoke_edit_file.py --binary target/qa/reviewr

# everything CI runs, locally
ci: fmt-check lint test test-qa-scripts
    cargo build --release
