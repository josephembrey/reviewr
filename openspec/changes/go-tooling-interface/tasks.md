## 1. Active workflow

- [x] 1.1 Add `build`, `check`, `dev`, `setup`, and `test` root recipes and verify the task list and
  dry runs expose the intended Go commands.
- [x] 1.2 Add Prek and its hook tools to the Nix shell, install the hook with `just setup`, and verify
  `prek run --all-files` succeeds without leaving edits.

## 2. Legacy boundary

- [x] 2.1 Move the Rust Cargo project, tests, syntax assets, package derivation, and historical docs
  under `legacy/`, then verify Cargo metadata and the Nix package still resolve.
- [x] 2.2 Add the legacy Just submodule and verify `just legacy build`, `just legacy dev`, and
  `just legacy run` target the relocated oracle.

## 3. Retired tooling and CI

- [x] 3.1 Remove the Rust A/B harness, tests, baseline, and component example, then verify no active
  guidance or command references them.
- [x] 3.2 Replace all GitHub workflows with one push check invoking setup and check, and verify it
  passes Actionlint.

## 4. Specifications and validation

- [x] 4.1 Initialize root OpenSpec for Codex, record this developer workflow change, and verify it
  with strict OpenSpec validation.
- [x] 4.2 Run `just setup`, `just test`, `just build`, `just check`, the legacy build/run smoke, and
  Nix flake evaluation; finish with a clean tooling diff and restored demo fixture.
