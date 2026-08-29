## Purpose

Make Files and Git continuously visible as reviewr's two primary workspaces and let users switch
between them without losing their place inside either workspace.

## ADDED Requirements

### Requirement: Header exposes both primary workspaces
The header SHALL begin with a `1 [ files | git ]` primary-workspace switcher that displays both
workspace labels at the same time and visually distinguishes the active label. The active label
SHALL use a terminal-native full-segment highlight with stronger emphasis, while the inactive label
and switcher punctuation remain visible and quieter.

#### Scenario: Files is active
- **WHEN** the Files workspace is active and the header has room for the complete switcher
- **THEN** the header shows both `files` and `git` with only the `files` segment highlighted

#### Scenario: Git is active
- **WHEN** the Git workspace is active and the header has room for the complete switcher
- **THEN** the header shows both `files` and `git` with only the `git` segment highlighted

### Requirement: Primary workspace is directly selectable
Reviewr SHALL start in Files, SHALL toggle between Files and Git when the user presses `1`, and SHALL
select a workspace directly when the user clicks its header label. Clicking switcher punctuation or
other header cells SHALL NOT change the primary workspace.

#### Scenario: Keyboard toggle
- **WHEN** the user presses `1` while either primary workspace is active
- **THEN** reviewr activates the other primary workspace

#### Scenario: Direct mouse selection
- **WHEN** the user clicks the inactive `files` or `git` label
- **THEN** reviewr activates the clicked workspace without first activating the other workspace

#### Scenario: Neutral header cell
- **WHEN** the user clicks a bracket, separator, repository-context cell, or unused header cell
- **THEN** reviewr leaves the active primary workspace unchanged

### Requirement: Workspaces retain independent place
Files and Git SHALL retain independent selection identity, list viewport, pane focus, and reader
scroll position. Switching workspaces SHALL restore the retained place for the destination workspace;
background results SHALL reconcile each retained place by identity under **Continuity**.

#### Scenario: Return to Files
- **WHEN** the user changes Files selection, focus, or scroll, visits Git, and returns to Files
- **THEN** the Files workspace restores the same surviving path selection, focus, and viewport state

#### Scenario: Return to Git
- **WHEN** the user changes Git selection, focus, or scroll, visits Files, and returns to Git
- **THEN** the Git workspace restores the same surviving commit selection, focus, and viewport state

### Requirement: Header prioritizes the primary switcher
Repository context SHALL follow the switcher only when cells remain. The switcher SHALL begin at the
left edge; repository context SHALL truncate before any complete switcher cell is displaced, and the
entire header SHALL remain within its geometry at every terminal width.

#### Scenario: Normal width
- **WHEN** the header can contain the switcher and repository context
- **THEN** the complete switcher appears first and repository context follows it

#### Scenario: Narrow width
- **WHEN** the header can contain the complete switcher but not all repository context
- **THEN** both workspace labels remain visible and repository context is truncated to the remaining cells

#### Scenario: Smaller than the switcher
- **WHEN** the terminal is narrower than the complete switcher
- **THEN** reviewr safely clips the header at the right edge without wrapping or overflowing

### Requirement: Active workspace owns the body
The navigator and reader SHALL display the active workspace's content while preserving the shared
frameless split and its existing pane-focus behavior.

#### Scenario: Switch body content
- **WHEN** the user changes the active primary workspace
- **THEN** both body surfaces change to that workspace's navigator and reader content in the same frame
