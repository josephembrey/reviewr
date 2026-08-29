## Purpose

Define a compact terminal-native navigator and reader layout whose only structural chrome is a
floating divider and whose selection is communicated by a full-width row highlight.

## ADDED Requirements

### Requirement: Normal body surfaces are frameless
The normal navigator and reader body SHALL render without outer rectangles, rounded corners,
horizontal edge rules, or duplicated application-perimeter borders. The global header and footer
SHALL remain unboxed text bands.

#### Scenario: Standard two-column frame renders
- **WHEN** reviewr renders its navigator and reader at a normal terminal size
- **THEN** neither surface contains a surrounding box and their edge cells are available to content

### Requirement: One floating divider separates the columns
When navigator and reader both have usable width, reviewr SHALL reserve exactly one column between
them and SHALL paint that column as a continuous quiet `│` divider. The divider SHALL have no corner,
junction, or horizontal-rule glyphs and SHALL belong to neither content surface.

#### Scenario: Both columns are visible
- **WHEN** the body is wide enough for navigator, divider, and reader
- **THEN** one standalone `│` column separates two non-overlapping content rectangles

#### Scenario: Terminal is too narrow for a divided split
- **WHEN** the body cannot safely allocate two usable columns plus the divider
- **THEN** layout degrades with bounded non-negative rectangles and does not paint outside the screen

### Requirement: Titles and content use reclaimed cells
Each surface SHALL paint a one-row title at its own first column and SHALL begin its content exactly
one row below at that same column. Content SHALL be able to use the surface’s full width up to the
floating divider or screen edge.

#### Scenario: Navigator rows render
- **WHEN** the navigator contains visible file rows
- **THEN** the title occupies its first row and file rows span the full navigator width immediately below it

#### Scenario: Reader content renders
- **WHEN** the selected file has visible content
- **THEN** the title occupies the reader’s first row and source rows span the full reader width immediately below it

### Requirement: File selection is a full-width row highlight
The selected navigator row SHALL be highlighted across its complete visible width, including trailing
padding. Selection SHALL NOT be communicated only by changing the filename to the accent foreground.
The focused selection SHALL be stronger than the unfocused selection while both remain identifiable.

#### Scenario: Navigator owns focus
- **WHEN** a visible file is selected and the navigator is focused
- **THEN** its complete row receives the active selection treatment and its text remains readable

#### Scenario: Reader owns focus
- **WHEN** a visible file is selected and the reader is focused
- **THEN** the same complete row keeps a quieter selection treatment rather than disappearing

### Requirement: Focus is local rather than perimeter-based
The focused surface SHALL be identified by its title treatment. Focus changes SHALL NOT add a pane
border, recolor a perimeter, or change the structural divider into a focus outline.

#### Scenario: Focus moves between surfaces
- **WHEN** the user toggles focus
- **THEN** title emphasis and selection strength update without changing body geometry or divider placement

### Requirement: Rendering and input share frameless geometry
Title rows, content rows, the floating divider, pane focus targets, and navigator-row hit-testing
SHALL derive from one geometry calculation. The divider SHALL not resolve as a file row or either
surface’s focus target.

#### Scenario: User clicks a visible file row
- **WHEN** a primary click lands inside a navigator content-row rectangle
- **THEN** the row selected is the same row painted at that coordinate

#### Scenario: User clicks the divider
- **WHEN** a primary click lands on the floating divider column
- **THEN** no navigator row is selected and neither surface receives focus from that click

#### Scenario: Frame is refreshed or resized
- **WHEN** content changes or terminal geometry changes without a user navigation action
- **THEN** **Continuity** preserves selected identity, focus, and scroll while the new geometry is applied
