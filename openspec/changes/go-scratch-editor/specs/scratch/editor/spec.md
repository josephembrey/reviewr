## Purpose

Provides a durable, minimal note editor that stays subordinate to review navigation and preserves
review place state while offering familiar modeless text and mouse interaction.

## ADDED Requirements

### Requirement: Scratch remains a global overlay
Reviewr SHALL present Scratch over the remembered Files or Git workspace rather than as a third
primary workspace. Opening or closing it MUST NOT mutate the underlying focus, cursor, scroll,
controls, folds, layout, or selection.

#### Scenario: Escape opens and closes Scratch
- **WHEN** the user presses Escape in Files or Git and later presses Escape in Scratch
- **THEN** Scratch opens and closes while the exact underlying primary workspace place state is restored

#### Scenario: One returns to the remembered workspace
- **WHEN** the user presses `1` while Scratch is open
- **THEN** Scratch closes to the remembered Files or Git workspace without toggling that workspace

#### Scenario: Background results land behind Scratch
- **WHEN** repository refresh results arrive while Scratch is open
- **THEN** world state may update but Scratch text and place state remain unchanged

### Requirement: Scratch provides minimal modeless editing
Printable UTF-8 text, including `h`, `j`, `k`, and `l`, SHALL insert directly. The editor SHALL
provide arrows, Home, End, Page Up, Page Down, Ctrl+Left, Ctrl+Right, newline, tab, backspace,
forward delete, shift selection, Ctrl+A, bounded undo and redo, and bracketed paste.

#### Scenario: Familiar keys edit and navigate
- **WHEN** the user types, moves, selects, inserts, deletes, undoes, or redoes with a supported input
- **THEN** the single note and cursor update according to familiar modeless editor semantics

#### Scenario: Paste is one safe edit
- **WHEN** bracketed paste contains multiline Unicode text or terminal control bytes
- **THEN** safe text is inserted as one undoable edit without allowing terminal control sequences to render

### Requirement: Text geometry is Unicode-correct and shared
The editor SHALL treat Unicode grapheme clusters as editing units, measure displayed cell width for
tabs, wide glyphs, and combining characters, and soft-wrap to the available body width. Painting,
cursor placement, selection, mouse mapping, scrolling, and resize reconciliation SHALL consume the
same wrapped document.

#### Scenario: Unicode and tabs wrap consistently
- **WHEN** a note contains tabs, wide glyphs, combining sequences, and long lines
- **THEN** every painted cell, cursor position, selection, and pointer target agrees with the same wrap rows

#### Scenario: Resize preserves place
- **WHEN** the terminal changes size
- **THEN** text, cursor, selection, preferred visual column, and the nearest surviving scroll anchor remain stable and valid

#### Scenario: Narrow geometry remains safe
- **WHEN** the editor is laid out with zero or very few terminal cells
- **THEN** wrapping, clipping, painting, and hit testing remain bounded and do not panic

### Requirement: Mouse editing is first class
Scratch SHALL support click placement, drag selection, wheel scrolling, and a draggable proportional
vertical scrollbar whenever the wrapped note exceeds the viewport.

#### Scenario: Pointer actions use painted geometry
- **WHEN** the user clicks, drags, wheels, or drags the scrollbar over Scratch
- **THEN** the action resolves against the exact rows and cells currently painted

### Requirement: Scratch communicates its small state
Scratch SHALL show a calm title, a visibly highlighted selection, and a compact status containing the
cursor line and column plus loading, modified, saving, saved, read-only, or recoverable error state.

#### Scenario: Persistence status changes
- **WHEN** a load or generation-tagged save starts, succeeds, is superseded, or fails
- **THEN** the status describes the current buffer without a stale completion marking newer text saved

### Requirement: Notes are private per Git common directory
Reviewr SHALL identify a note by the canonical absolute Git common directory, so linked worktrees of
one clone share a note and separate clones do not. It MUST NOT key identity from remote URLs or a
worktree path and MUST NOT write project files, Git refs, the index, branches, HEAD, or worktree data.

#### Scenario: Linked worktrees share identity
- **WHEN** two worktrees resolve to the same canonical Git common directory
- **THEN** they resolve to the same private Scratch note and lock

#### Scenario: Separate clones stay isolated
- **WHEN** two clones have different canonical Git common directories despite matching remotes
- **THEN** they resolve to different private Scratch notes and locks

### Requirement: Persistence is atomic, private, and recoverable
Reviewr SHALL store Scratch outside the repository in a versioned platform-private state directory,
respecting `XDG_STATE_HOME` and its standard Linux fallback. Directories and files MUST have
restrictive permissions. Saves SHALL use a same-directory temporary file, flush it, atomically rename
it, and flush the directory; a failed save MUST preserve the previous valid file and edited memory.

#### Scenario: Missing or unreadable state does not block review
- **WHEN** state is absent, corrupt, unreadable, or unwritable
- **THEN** Reviewr remains usable with an empty or loaded in-memory buffer and a concise recoverable error

#### Scenario: Edits autosave and close flushes
- **WHEN** writable Scratch text changes, then the debounce expires, Scratch closes, or Reviewr quits normally
- **THEN** the latest text is saved without polling, leaked goroutines, or per-frame disk or Git work

### Requirement: A nonblocking OS lock enforces one writer
Reviewr SHALL attempt a nonblocking OS-backed lock for the clone note. One process SHALL edit; a
contending process SHALL load the latest note read-only, clearly report that state, and never merge or
overwrite the writer's changes.

#### Scenario: Lock contention opens read-only
- **WHEN** another Reviewr process already holds the note's edit lock
- **THEN** Scratch loads the latest disk text read-only without waiting and editing inputs do not mutate it
