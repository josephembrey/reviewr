## Context

See `proposal.md` for motivation and `specs/ui/frameless-split/spec.md` for the visual contract. The Go
UI currently gives each body surface a Lip Gloss rounded border. Geometry therefore removes two
columns and three rows per pane, and focus is painted by recoloring the complete border. Navigator
selection uses a blue bold foreground plus a `›` prefix, so the cursor does not occupy a stable visual
row.

The existing `Geometry` object already feeds both rendering and mouse routing. This change should
strengthen that seam rather than deriving frameless offsets inside render helpers. The legacy
frameless-split implementation is a behavioral reference, not code to port mechanically.

## Goals / Non-Goals

**Goals:**

- Make the current two-column Go slice visually match reviewr’s terminal-native frameless direction.
- Encode title, content, and divider rectangles explicitly in shared geometry.
- Make the selected file a consistent full-row cursor with focused and unfocused states.
- Reclaim the cells currently consumed by rounded boxes without changing navigation state.

**Non-Goals:**

- Add navigator positioning, hiding, or resizing in this change.
- Add themes, file icons, Git status colors, tree hierarchy, scrolling chrome, or diff presentation.
- Change keyboard bindings, repository effects, or Bubble Tea action routing beyond new hit targets.
- Introduce a generic component/card framework.

## Decisions

### Model the divider and title rows as first-class geometry

`Geometry` will gain a one-cell divider rectangle plus explicit navigator-title and reader-title
rectangles. Navigator and reader rectangles describe actual frameless surfaces; their content-row
rectangles begin one row below their title and retain the full surface width.

At usable widths, the horizontal partition is navigator + one divider cell + reader. The existing
navigator sizing policy is applied to the width left after reserving the divider. At tiny widths,
saturating calculations omit the divider when it cannot separate two positive-width surfaces.

Alternatives considered:

- Keep the old pane rectangles and paint over their borders: rendering and hit-testing would retain
  misleading insets and waste the reclaimed cells.
- Derive the divider only while rendering: mouse boundary behavior could disagree with the screen.
- Give the divider to one pane: clicks and width calculations would make it act like content.

### Render plain fixed-size surfaces instead of styled border blocks

The renderer will replace the border-producing `renderPane` helper with a frameless surface helper
that fits exactly one title row and the geometry-defined content rows. The body is composed as
navigator, divider, and reader blocks. The divider repeats `│` for the body height in a subdued style
and never gains corners or horizontal lines.

The focused title is bold and accented; the unfocused title is quiet. Focus styling is local and does
not alter geometry.

Alternatives considered:

- Use square borders: still duplicates terminal chrome and consumes the same cells.
- Draw separate right/left borders: creates a two-cell seam or assigns structural chrome to a pane.

### Apply selection after padding the complete row

Navigator rows will first be fitted and padded to the full content width, then receive a selection
style over the entire row. The selected style will not set the filename to the blue accent. A
terminal-native reverse treatment gives a visible background without assuming the user’s terminal
base color; bold distinguishes focused selection, while the unfocused row remains reversed but
quieter.

This follows the legacy terminal palette’s cursor behavior and remains legible under dynamic terminal
themes. It also leaves future icon and Git-status foreground spans free to keep their semantic role
before the row-level selection treatment is applied.

Alternatives considered:

- A fixed RGB background: may clash with the user’s terminal theme before the Go theme system exists.
- Accent-colored filename text: the current behavior is too weak and destroys semantic foreground
  distinctions.
- A leading chevron alone: does not form the requested selection bar/highlight and is easy to lose in
  a dense tree.

## Risks / Trade-offs

- [Reverse video varies with terminal palettes] → It is intentionally terminal-native and guarantees
  contrast; focused bold supplies the only additional emphasis.
- [One divider column slightly changes the current width ratio] → Compute pane shares after reserving
  the divider and lock the partition with geometry tests.
- [ANSI styling can make string snapshots brittle] → Test geometry and semantic style properties in
  focused helpers, with a small rendered-frame assertion for box-glyph absence and divider continuity.
- [Tiny terminals cannot preserve the normal split] → Use saturating rectangles and test every width
  around the divider threshold.

## Migration Plan

1. Change and test shared geometry, including divider and title/content rectangles.
2. Replace boxed render helpers with frameless surfaces and the floating divider.
3. Replace selected-text styling with full-width focused/unfocused row treatment.
4. Update mouse boundary tests and run the complete Go and repository checks.

Rollback is a normal commit revert; the change contains no persisted state or repository mutation.
