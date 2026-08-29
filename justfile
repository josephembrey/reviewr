# reviewr tasks — run `just <task>` (https://github.com/casey/just)

set positional-arguments

# list tasks
default:
    @just --list

# build the application
build:
    mkdir -p target/go
    go build -mod=readonly -o target/go/reviewr ./cmd/reviewr

# run hooks and tests
check:
    prek run --all-files
    just test

# run the application from source, optionally against another repository
dev *args:
    #!/usr/bin/env bash
    set -euo pipefail
    exec go run -mod=readonly ./cmd/reviewr "$@"

# prepare a new clone
setup:
    go mod download
    prek install --hook-type pre-commit

# run tests
test:
    go test -mod=readonly ./...
    go test -mod=readonly -race ./...

# frozen Rust oracle: `just legacy build|dev|run`
mod legacy 'legacy/justfile'
