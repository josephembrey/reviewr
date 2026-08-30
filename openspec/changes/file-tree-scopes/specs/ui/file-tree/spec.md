## MODIFIED Requirements

### Requirement: Repository paths render as a hierarchical scoped tree
The Files navigator SHALL derive one active hierarchical tree from one typed repository snapshot.
All SHALL include tracked, untracked, and ignored file entries. Changed SHALL include modified,
added, deleted, renamed, and untracked entries and SHALL exclude unchanged and ignored-only entries.
Header control 2 SHALL switch these projections without loading another snapshot or retaining a
second hidden tree.

#### Scenario: Scope changes
- **WHEN** the reviewer activates Files header control 2
- **THEN** the existing snapshot is projected into the other scope and no Git refresh occurs

#### Scenario: Ignored file is visible
- **WHEN** an ignored file exists in the snapshot
- **THEN** it appears dimmed and readable in All and does not appear in Changed

### Requirement: Tree cursor and open file are distinct typed place state
The tree cursor and open reader entry SHALL retain repository-relative path identity independently.
Scope changes and refreshes SHALL preserve an identity that remains available, then choose the
nearest surviving identity of the same role, and only then clamp. Explicit Git rename metadata MAY
reconcile an old file path to its new path. Deleted files SHALL remain selectable in Changed and
SHALL show a coherent deleted state in file mode and their deletion patch in diff mode. Renamed
files SHALL read the current path in file mode and include old and new paths in diff mode.

#### Scenario: Open file survives scope switch
- **WHEN** the current reader path exists in the target scope
- **THEN** the reader identity, content, and scroll remain unchanged

#### Scenario: Selected role is absent
- **WHEN** the selected identity disappears from the target projection
- **THEN** the nearest surviving row of the same file-or-directory role is selected before clamping

#### Scenario: Fold survives scope switch
- **WHEN** a collapsed directory identity exists in both scope projections
- **THEN** it remains collapsed after switching scopes

### Requirement: Repository status is explicit and safely sourced
Repository/app entries SHALL carry typed Git state and optional prior rename path. Status SHALL be
parsed from NUL-delimited machine-oriented Git output so spaces, newlines, non-ASCII bytes, and
pathspec-like names remain identities rather than syntax. File rows SHALL reserve a concise status
marker for modified, added, deleted, renamed, untracked, and ignored states using restrained,
terminal-aware styling.

#### Scenario: Hostile rename is parsed
- **WHEN** porcelain v2 reports a rename whose paths contain whitespace or control bytes
- **THEN** both NUL-delimited paths are retained exactly without parsing human-oriented output

#### Scenario: Mouse selects a decorated row
- **WHEN** a status marker changes the painted row prefix or suffix
- **THEN** mouse hit testing still selects the same visible row through shared geometry

### Requirement: Scoped refresh remains read-only and latest wins
Snapshot, file, and diff loads SHALL be generation tagged so stale results cannot replace newer
state. Scope switching SHALL reuse the loaded snapshot and SHALL NOT introduce duplicate Git list or
status subprocesses. All operations SHALL preserve No writes for the worktree, index, branches,
refs, object database, and filesystem.

#### Scenario: Two snapshots finish out of order
- **WHEN** an older repository snapshot lands after a newer refresh was requested
- **THEN** the older snapshot is ignored

#### Scenario: Reviewer switches scope repeatedly
- **WHEN** control 2 changes All and Changed without an explicit refresh
- **THEN** the repository snapshot operation count does not change
