## Purpose

Define a hierarchical, foldable Files navigator whose cursor and open reader remain stable as the
repository changes, without coupling tree structure to later Git status or review metadata.

## ADDED Requirements

### Requirement: Repository paths render as a hierarchical tree
The Files navigator SHALL group repository-relative file paths beneath synthesized directory rows.
Directories SHALL sort before files at each level, with each group sorted by display name. Directory
rows SHALL expose their depth and expansion state, and file rows SHALL display their basename while
retaining the complete repository-relative path as identity.

#### Scenario: Nested paths are loaded
- **WHEN** the repository contains files at `src/app.go`, `src/ui/render.go`, and `README.md`
- **THEN** the navigator shows directory and file rows in their hierarchy rather than three flat full-path rows

#### Scenario: A directory chain has no branch
- **WHEN** a path passes through consecutive single-child directories before reaching a branch or file
- **THEN** the navigator MAY compact that chain into one slash-separated row while retaining the deepest directory or file identity

#### Scenario: File count is displayed
- **WHEN** directory rows change visibility through folding
- **THEN** the Files title continues to report the repository file count rather than the visible row count

#### Scenario: Tree loads for the first time
- **WHEN** the first repository snapshot contains at least one file
- **THEN** the navigator selects and opens the first visible file row rather than stopping on a synthesized directory

### Requirement: Directories can be expanded and collapsed
Directories SHALL start expanded in this first tree slice. While the navigator is focused, `l` and
Right SHALL expand the selected collapsed directory, and `h` and Left SHALL collapse the selected
expanded directory. These inputs SHALL be inert for file rows and directories already in the
requested state. Clicking a directory row SHALL select it and toggle its expansion.

#### Scenario: Reviewer collapses a directory with the keyboard
- **WHEN** the navigator cursor is on an expanded directory and the reviewer presses `h` or Left
- **THEN** its descendant rows become hidden and the directory row remains selected and visible

#### Scenario: Reviewer expands a directory with the mouse
- **WHEN** the reviewer clicks a collapsed directory row
- **THEN** that row becomes selected and its immediate visible descendants appear

#### Scenario: Folding input targets a file
- **WHEN** the navigator cursor is on a file and the reviewer presses a folding key
- **THEN** neither tree place nor reader place changes

### Requirement: Tree cursor and open file are distinct place state
The navigator cursor SHALL be allowed to identify either a directory or a file. Moving or clicking
onto a file SHALL open that file in the reader. Moving or clicking onto a directory SHALL leave the
open file and reader scroll unchanged.

#### Scenario: Reviewer moves from a file to a directory
- **WHEN** a file is open and the navigator cursor moves onto a directory row
- **THEN** the directory becomes selected while the same file content and reader place remain visible

#### Scenario: Reviewer moves from a directory to a file
- **WHEN** the navigator cursor moves from a directory onto a file row
- **THEN** that file becomes the open reader identity and its content begins loading

#### Scenario: Open file becomes hidden by a fold
- **WHEN** the reviewer collapses an ancestor of the currently open file
- **THEN** the reader continues showing that file and its reader place does not reset

### Requirement: Refresh preserves tree place by identity
Repository refresh SHALL reconcile the tree cursor, navigator top row, open file, reader scroll, and
directory expansion state by repository-relative identity under **Continuity**. Surviving identities
SHALL remain selected or open even when their row index changes. Missing identities SHALL fall back
to the nearest surviving target of the same role before clamping.

#### Scenario: Files are inserted before the cursor
- **WHEN** refresh adds paths that sort before the selected row and the selected identity survives
- **THEN** the same directory or file remains selected and visible

#### Scenario: Collapsed directory survives refresh
- **WHEN** a collapsed directory and its descendants survive a repository refresh
- **THEN** that directory remains collapsed and the hidden descendants remain hidden

#### Scenario: Open file survives while hidden
- **WHEN** an open file survives refresh beneath a collapsed directory
- **THEN** the reader retains that open file and its scroll even though no visible row identifies it

#### Scenario: Selected or open identity disappears
- **WHEN** refresh removes the selected tree row or open file
- **THEN** reviewr chooses the nearest surviving tree row or file respectively and clamps only when no nearer survivor exists

### Requirement: Tree interaction remains read-only and geometry-consistent
File-tree navigation and folding SHALL NOT mutate the worktree, index, branches, or filesystem under
the **No writes** invariant. Painted tree rows and mouse selection SHALL use the same navigator row
geometry, and full-width focused and unfocused selection styling SHALL remain intact.

#### Scenario: Reviewer navigates and folds the tree
- **WHEN** the reviewer selects, expands, or collapses any row
- **THEN** only in-memory place state changes and the mouse target continues to match the painted row
