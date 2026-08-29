## Purpose

Provide a small, repeatable command lifecycle for setting up, developing, validating, and comparing
the active Go application while the previous Rust implementation remains available as an oracle.

## ADDED Requirements

### Requirement: Repository setup
The repository SHALL provide one setup command that installs locked Go dependencies and the
pre-commit hook required by the project.

#### Scenario: New clone is prepared
- **WHEN** a developer enters the development shell and runs `just setup`
- **THEN** Go dependencies are downloaded without changing the module definition and the pre-commit
  hook is installed

### Requirement: Primary Go lifecycle
The repository SHALL expose short, implementation-neutral commands for building, developing, and
testing the Go application.

#### Scenario: Application is built
- **WHEN** a developer runs `just build`
- **THEN** the Go application is built at `target/go/reviewr`

#### Scenario: Application is run from source
- **WHEN** a developer runs `just dev` with an optional repository path
- **THEN** the current Go source runs against the selected repository

#### Scenario: Tests are run
- **WHEN** a developer runs `just test`
- **THEN** the Go tests and race detector complete successfully or the command fails

### Requirement: Finite validation entrypoint
The repository SHALL provide `just check` as the complete finite validation entrypoint for active
development.

#### Scenario: Repository is checked
- **WHEN** a developer or continuous integration runs `just check`
- **THEN** every configured repository hook and the Go test suite complete successfully or the
  command fails

### Requirement: Legacy oracle boundary
The previous Rust Cargo project SHALL live under `legacy/` and expose only build, source-run, and
built-run tasks through the root command surface.

#### Scenario: Legacy application is built
- **WHEN** a developer runs `just legacy build`
- **THEN** the Rust oracle is built without treating it as the primary application

#### Scenario: Legacy application is run from source
- **WHEN** a developer runs `just legacy dev` with an optional repository path
- **THEN** the Rust oracle runs from source against the selected repository

#### Scenario: Built legacy application is run
- **WHEN** a developer has built the oracle and runs `just legacy run` with an optional repository
  path
- **THEN** the existing built Rust binary runs without invoking Cargo

### Requirement: Push validation
The repository SHALL have one GitHub Actions workflow that runs the repository setup and check
commands on pushes.

#### Scenario: Commit is pushed
- **WHEN** a commit is pushed to the repository
- **THEN** GitHub Actions enters the Nix development environment and runs `just setup` followed by
  `just check`
