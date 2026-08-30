# reviewr

A fast terminal application for reviewing repository changes beside coding agents.

reviewr is being built from the ground up in Go with Bubble Tea. The current slice provides a
read-only repository navigator and file reader with keyboard and mouse navigation, stable selection
across reloads, shared render/hit-test geometry, an explicit content-addressed review ledger, and a
global minimal Scratch note editor. Scratch opens with `Esc`, autosaves outside the repository per
Git clone, and returns to the exact remembered Files or Git state when closed. While Scratch is open,
`1` is reserved to return to that remembered workspace, so a literal `1` cannot be entered in the
note.

When launched inside Herdr, reviewr automatically uses the injected host context and labels an
otherwise-unlabeled current pane `reviewr`. Standalone launches require no Herdr installation.

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

The file tree uses Nerd Font file and folder glyphs; terminals should use a Nerd Font for the
intended one-cell alignment and filetype silhouettes.

Changed files carry independent `[ ]`, `[x]`, `[+]`, `[~]`, or `[!]` review badges. Review coverage
changes only through `x` or a badge click; reading a file never marks it. See
[docs/review-ledger.md](docs/review-ledger.md) for the exact semantics, private state format, and
comparison-provider boundary.
