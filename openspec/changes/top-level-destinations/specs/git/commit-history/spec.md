## Purpose

Provide bounded, useful, read-only Git history while preserving destination continuity.

## ADDED Requirements

### Requirement: Git lists recent current-HEAD commits
Git SHALL show a bounded newest-first list of commits reachable from `HEAD`, using full object IDs as
state identities and terminal-safe abbreviated IDs and subjects for display.

#### Scenario: Repository has commits
- **WHEN** Git first loads a repository with a valid `HEAD`
- **THEN** recent reachable commits appear newest first with `HEAD` initially selected

### Requirement: Selected commit has a bounded summary
The reader SHALL show the exact selected commit's identity, author, authored time, message, and
changed-file stat. Loading and failure states SHALL never show stale content for another commit.

#### Scenario: Stale summary completes
- **WHEN** an older selection's summary completes after selection changed
- **THEN** the completion is rejected and the current reader remains unchanged

### Requirement: History preserves No writes and Continuity
History reads SHALL not mutate Git or repository state. Refresh SHALL retain a surviving selection by
full object ID, then use nearest-survivor reconciliation and clamping under **Continuity**.

#### Scenario: Inspect history
- **WHEN** reviewr lists commits or reads summaries
- **THEN** the worktree, index, branches, `HEAD`, and refs remain unchanged
