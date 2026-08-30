# reviewr

A fast terminal application for reviewing repository changes beside coding agents.

reviewr is being built from the ground up in Go with Bubble Tea. The current slice provides a
read-only repository navigator and file reader with keyboard and mouse navigation, stable selection
across reloads, shared render/hit-test geometry, an explicit content-addressed review ledger, and a
modeless Markdown Notes editor. Files, Git, and Notes are independent top-level destinations;
outside the editor, `1`, `2`, and `3` select them directly. Notes accepts every printable character,
autosaves project-wide and checkout-local scopes outside the repository, and uses `Esc` as its route
home to Files.

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
```

The file tree uses Nerd Font file and folder glyphs; terminals should use a Nerd Font for the
intended one-cell alignment and filetype silhouettes.

Changed files carry independent `[ ]`, `[x]`, `[+]`, `[~]`, or `[!]` review badges. Review coverage
changes only through `x` or a badge click; reading a file never marks it. See
[docs/review-ledger.md](docs/review-ledger.md) for the exact semantics, private state format, and
comparison-provider boundary.
