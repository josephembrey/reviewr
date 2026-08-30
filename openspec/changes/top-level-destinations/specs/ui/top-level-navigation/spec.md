## Purpose

Expose Files, Git, and Notes as continuously visible peer destinations without losing place.

## ADDED Requirements

### Requirement: Header exposes all destinations
The header SHALL begin with `[ files | git | notes ]`. All three labels SHALL remain readable and
clickable when visible, and only the active label SHALL receive the selected-tab highlight.

#### Scenario: Destination is active
- **WHEN** Files, Git, or Notes is active
- **THEN** all labels are shown and only that destination's label is highlighted

### Requirement: Destinations are directly selectable
Reviewr SHALL start in Files. Outside Notes, bare `1`, `2`, and `3` SHALL directly select Files, Git,
and Notes and selecting the active destination SHALL do nothing. Clicking a label SHALL directly
select it; punctuation and other header cells SHALL be inert.

#### Scenario: Direct keyboard selection
- **WHEN** the user presses a destination digit outside Notes
- **THEN** that destination becomes active without visiting another destination

#### Scenario: Notes owns printable digits
- **WHEN** Notes is active and the user types `1`, `2`, or `3`
- **THEN** the digit is inserted normally and the destination does not change

### Requirement: Escape returns to Files at the appropriate priority
Escape from Git SHALL return to Files. Escape from Notes SHALL run its save-gated transition to
Files. Escape in Files SHALL remain available to higher-priority local dismissal or cancel behavior.

#### Scenario: Notes save fails
- **WHEN** leaving Notes requires a save and that save fails
- **THEN** Notes remains active with the authored text and error visible

### Requirement: Destinations retain independent place
Files, every Git subview, and Notes editor/scope sessions SHALL retain independent user-controlled
place. Background results SHALL reconcile their owning state by stable identity under **Continuity**.

#### Scenario: Return to a destination
- **WHEN** the user changes place, visits another destination, and returns
- **THEN** the surviving selection, focus, viewport, reader offset, or editor place is restored

### Requirement: Responsive chrome preserves exact geometry
Header paint and hit testing SHALL consume one geometry calculation. The destination group SHALL
clip without wrapping at tiny widths, repository context SHALL yield to it, and optional controls
SHALL shed only as complete controls.

#### Scenario: Narrow width
- **WHEN** the width cannot fit every header element
- **THEN** complete lower-priority elements disappear and all remaining paint and hits stay bounded
