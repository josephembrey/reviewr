# Contributing

## Setup

Enter the Nix development shell, then prepare the clone once:

```bash
nix develop
just setup
```

The setup task downloads the locked Go modules and installs the Prek pre-commit hook.

## Development

```bash
just dev                  # run against this repository
just dev ~/some/repo      # run against another repository
just test                 # tests plus the race detector
just build                # target/go/reviewr
just check                # every hook plus tests; same command CI runs
```

The old Rust application is a frozen behavioral oracle under `legacy/`:

```bash
just legacy build
just legacy dev ~/some/repo
just legacy run ~/some/repo
```

Use the legacy implementation for comparison, not as the default location for new work.

## Changes

New user-visible behavior starts with `$openspec-propose`. Review the proposal, specification,
design, and tasks under `openspec/changes/` before applying it. Port behavior and fixtures
deliberately; do not translate the Rust module structure wholesale.

Before committing, run `just check`. Pull requests should keep one concern, update the governing
OpenSpec change, and add a changelog entry when the result is visible to users.
