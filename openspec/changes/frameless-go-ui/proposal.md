## Why

The Go foundation renders its navigator and reader as two rounded cards, which duplicates the
terminal’s own frame and makes the application feel like a generic dashboard instead of reviewr.
Its selected file also turns into accent-colored text rather than reading as a stable cursor row.

## What Changes

- Remove rounded pane borders and all outer body boxes from the normal two-column screen.
- Reserve one quiet floating `│` cell between navigator and reader as the only structural body
  chrome; it stands alone without corners or horizontal rules.
- Reclaim former border cells for titles, file rows, and reader content.
- Render the selected file as a full-width row highlight: stronger while the navigator is focused,
  quieter while it is unfocused, without replacing the filename’s semantic foreground color.
- Communicate focus through local title styling rather than a perimeter that changes color.
- Keep render and mouse hit-testing on one shared frameless geometry, including tiny terminals and
  the divider cell.

## Capabilities

### New Capabilities

- `ui/frameless-split`: Defines the Go application’s floating-divider navigator/reader split, local
  focus treatment, and full-row selection presentation.

### Modified Capabilities

None.

## Impact

- Changes `internal/ui` geometry, rendering, and focused render tests.
- Adjusts mouse boundary expectations in `internal/app` without changing semantic actions or
  navigation state.
- Preserves header/footer bands, repository behavior, keyboard controls, **No writes**, and
  **Continuity**.
- Does not port the legacy application’s themes, file icons, tree hierarchy, resizable navigator,
  scrollbar, or diff rendering; those remain separate incremental changes.
