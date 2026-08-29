# reviewr

A fast terminal application for reviewing repository changes beside coding agents.

reviewr is being built from the ground up in Go with Bubble Tea. The current slice provides a
read-only repository navigator and file reader with keyboard and mouse navigation, stable selection
across reloads, and shared render/hit-test geometry.

## Development

```bash
nix develop
just setup
just dev
```

Run `just` to see the complete task list. The primary lifecycle is:

```bash
just build
just test
just check
```

## Structure

```text
cmd/reviewr/       executable wiring
internal/          Go application packages
openspec/          active product specifications and changes
legacy/            frozen Rust behavioral oracle and historical documentation
```

The Rust oracle remains runnable with `just legacy build`, `just legacy dev`, and
`just legacy run`. It is not the default location for new work.
