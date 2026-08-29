## Purpose

Provide a small, useful, read-only Git workspace that lets users inspect recent current-HEAD history
before richer refs, graph, stash, and diff-review workflows are introduced.

## ADDED Requirements

### Requirement: Git workspace lists recent current-HEAD commits
The Git workspace SHALL show a bounded list of commits reachable from the repository's current
`HEAD`, ordered newest first. Each navigator row SHALL include an unambiguous abbreviated object ID
and a terminal-safe single-line subject.

#### Scenario: Repository has commits
- **WHEN** the user first opens Git in a repository with a valid `HEAD`
- **THEN** reviewr lists recent reachable commits newest first and initially selects `HEAD`

#### Scenario: History exceeds the bound
- **WHEN** the reachable history contains more commits than reviewr's bounded history limit
- **THEN** reviewr returns the newest commits up to that limit without loading the remaining history

### Requirement: Selected commit has a useful summary
The reader SHALL show the selected commit's full object ID, author identity, authored time, complete
message, and changed-file stat. Commit output SHALL be terminal-safe and bounded before rendering.

#### Scenario: Select another commit
- **WHEN** the user selects a different commit with keyboard or mouse navigation
- **THEN** the reader loads and displays the summary for that exact commit identity

#### Scenario: Summary is loading
- **WHEN** a selected commit's summary has not finished loading
- **THEN** the reader identifies the selected commit and shows an explicit loading state rather than stale content from another commit

### Requirement: Empty and failed history is explicit
The Git workspace SHALL remain navigable and display an explicit state when the repository has no
valid `HEAD` or when Git history cannot be read.

#### Scenario: Unborn repository
- **WHEN** the repository has no commits
- **THEN** the navigator reports that no commits exist and the reader reports that no commit is selected

#### Scenario: History read fails
- **WHEN** Git cannot provide commit history
- **THEN** the navigator displays a terminal-safe Git error without showing fabricated commits

### Requirement: Commit history obeys read-only and continuity guarantees
Loading or navigating commit history SHALL perform no Git write. Refresh and selection results SHALL
be latest-wins and SHALL reconcile selection and scroll by commit object ID under **No writes** and
**Continuity**.

#### Scenario: Reload preserves a surviving commit
- **WHEN** history refreshes and the selected object ID still exists in the bounded result
- **THEN** reviewr retains that commit selection and its user-controlled place state

#### Scenario: Stale summary completes
- **WHEN** a summary for a previously selected commit completes after the user selected another commit
- **THEN** reviewr discards the stale completion and leaves the current commit reader unchanged

#### Scenario: Inspect history without mutation
- **WHEN** reviewr lists commits and reads one or more commit summaries
- **THEN** the worktree, index, branches, `HEAD`, and refs remain unchanged
