## Context

See `proposal.md` for motivation and the two delta specs for the behavior contract. The current Go
slice has one files-specific navigation state, one files-specific asynchronous load pipeline, and a
derived UI model whose navigator rows and reader content are repository-file types. Its header is a
single fitted string, so it has no shared geometry for mouse-selectable header segments.

The change crosses app state, navigation identity, UI geometry, repository reads, and the Git adapter.
It must preserve **No writes** and **Continuity**, keep the Bubble Tea root a thin semantic router, and
avoid designing the eventual refs/stashes/graph system prematurely.

## Goals / Non-Goals

**Goals:**

- Establish Files and Git as explicit primary workspace identities with independent place state.
- Give the UI a workspace-neutral navigator/reader presentation seam.
- Make header painting and header hit-testing consume the same geometry.
- Deliver one bounded and genuinely useful read-only Git history slice.
- Keep every asynchronous list and reader completion identity-tagged and latest-wins.

**Non-Goals:**

- Define the eventual Git submodes, branch/ref/worktree navigation, graph, stashes, or commit diffs.
- Share loaded content, selection, focus, or scroll positions between primary workspaces.
- Add configurable bindings, a generic tab framework, or a generic component/card system.
- Port the legacy Git view's internal structure or compatibility surface.

## Decisions

### Give each primary workspace its own domain state

The root model will own an explicit active-workspace enum plus separate Files and Git state. Each
state owns its item identities, selection/top/focus/reader offset, loaded reader identity, generation
counters, loading flags, and errors. Semantic actions select or toggle the active workspace; active
workspace handlers interpret navigation and refresh actions.

The generic navigation seam will reconcile an ordered list of stable string identities instead of
being named around file paths. Files use raw Git paths as identities and Git uses full commit object
IDs. Labels and reader content remain outside place state. This gives both workspaces the same
**Continuity** behavior without coupling their domain payloads.

Alternatives considered:

- Swap one navigation object between workspaces: this would require fragile stash/restore copying and
  make late background results capable of disturbing the inactive workspace's place.
- Build a generic workspace interface immediately: two concrete workspaces do not yet justify a
  plugin-style abstraction, and it would hide rather than clarify their different loading behavior.
- Reuse abbreviated commit IDs as identities: abbreviations can become ambiguous and are unsuitable
  for reconciliation; only full object IDs are stable identities.

### Derive a workspace-neutral UI document before rendering

The app will derive a small presentation model containing workspace identity, navigator title and
rows, selected/top indices, focus, reader title and already-classified reader lines, loading/error
state, and repository context. File classification remains in the repository/app boundary; commit
summary formatting remains in the Git workspace boundary. The UI only sanitizes, fits, styles, and
paints derived text.

This removes file-only branching from the renderer and lets both workspaces reuse the same frameless
surface, selection, scrolling, and focus treatment. It is intentionally a concrete derived model,
not a component framework.

Alternatives considered:

- Add Git branches throughout the existing file renderer: every future workspace would multiply
  conditional rendering and make exact geometry harder to reason about.
- Make repository domain types render themselves: that would mix terminal policy into read models and
  weaken package ownership.

### Treat the switcher segments as first-class header geometry

Shared geometry will describe the clipped header switcher, Files label, Git label, and repository
context rectangles in addition to the existing body rectangles. The fixed logical switcher is
`1 [ files | git ]`; only visible intersections become hit targets. Brackets, spaces, separator, and
repository context remain neutral. The repository context begins after the complete logical switcher
and its gap, then fits only the remaining width.

The active segment uses the same terminal-native reverse treatment as body selection, with bold
emphasis. The inactive label and punctuation use the quiet header treatment. Header truncation clips
the right edge and never wraps.

Alternatives considered:

- Parse the rendered ANSI string to infer click columns: styles and wide characters would make paint
  and hit-testing disagree.
- Make the whole switcher toggle on click: direct label selection is predictable, while brackets and
  separators should not have invisible behavior.
- Put `1` inside the Files segment: it visually implies that the key selects Files rather than toggles
  the two-state control.

### Start Git with bounded current-HEAD history and summaries

The repository boundary will expose a bounded recent-commit list and one bounded commit summary. The
initial implementation will request at most 200 commits reachable from `HEAD`, newest first. Rows use
the full object ID for identity and an abbreviated ID plus subject for display. A selected summary
contains full ID, author name/email, authored timestamp, full message, and changed-file stat.

Git output will use machine-delimited formats for typed fields, disable optional locks and optional
diff/text-conversion hooks, request no color, and cap captured output at the same one-mebibyte reader
budget used for files. An unborn `HEAD` is an empty history, while other command failures remain
explicit errors. Commit stat output is informational plain text; patch content is deliberately absent.

Alternatives considered:

- Show only branch and `HEAD`: it would make Git technically nonempty but not useful enough to earn a
  primary workspace.
- Include commit patches now: patch geometry, folds, highlighting, and review anchoring are their own
  change and would turn this header work into the diff migration.
- Shell out to an external history TUI: reviewr needs its own place identity, mouse geometry, and
  eventual review integration.

### Keep loads workspace-scoped and latest-wins

Files and Git use separate list and reader generations. Switching to an unloaded workspace displays
its loading state immediately and starts its list load; switching away does not cancel or retag the
request. A completion may update its owning inactive state, but it cannot change the active workspace
or any user-controlled place. Reader completions land only when generation and full selected identity
both still match.

Refresh reloads only the active workspace. Returning to an already loaded workspace does not reload
it merely because it became visible, which makes the switch immediate and preserves place.

Alternatives considered:

- One global generation: unrelated Files activity could invalidate correct Git work and vice versa.
- Reload on every toggle: this would introduce flicker and make switching itself disturb otherwise
  valid state.

## Risks / Trade-offs

- [The root model could grow into another hero file] → Put Files and Git transitions/effects in
  focused domain files and leave `Update`, `View`, and top-level action dispatch as thin routing.
- [Git output can be large or hostile terminal text] → Use bounded capture, machine delimiters, and
  the existing terminal sanitization before rendering.
- [A 200-commit window can drop an older selected commit after refresh] → Reconcile by full object ID,
  then nearest surviving item, then clamp as required by **Continuity**.
- [Header geometry may be wrong at clipped widths] → Test every width around each switcher segment and
  assert render and hit targets use the same half-open rectangles.
- [The simple history may imply the richer Git UX is complete] → Name the navigator `Commits`, keep the
  proposal's non-goals explicit, and avoid placeholder controls for unimplemented submodes.

## Migration Plan

1. Generalize navigation identity and UI presentation seams while keeping Files behavior unchanged.
2. Add typed bounded commit reads and repository tests proving **No writes**.
3. Add independent Git workspace state and latest-wins effects.
4. Add shared header geometry, semantic switch/select actions, and switcher rendering.
5. Validate normal, narrow, and sub-switcher terminal widths plus Files/Git place restoration.

Rollback is a normal commit revert. The change adds no persisted state, Git writes, configuration, or
external dependency.
