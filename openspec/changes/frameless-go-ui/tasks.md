## 1. Shared frameless geometry

- [x] 1.1 Add explicit title, content, and floating-divider rectangles to the shared geometry and
  verify standard and tiny-width partition tests cover every cell without overlap or overflow.
- [x] 1.2 Make divider hits neutral and update mouse-routing tests to verify painted rows, titles,
  surfaces, and the divider resolve from the same geometry.

## 2. Floating-divider rendering

- [x] 2.1 Replace rounded pane blocks with fixed-size frameless surfaces and one continuous floating
  `│`, then verify rendered frames contain no box corners or horizontal pane rules.
- [x] 2.2 Move focused styling to local titles and verify focus changes do not alter geometry or the
  divider’s structural style.

## 3. Row selection

- [x] 3.1 Replace accent-colored selected text and the chevron-only cursor with a padded full-width
  row treatment, then verify focused selection is reversed and bold while unfocused selection remains
  reversed and readable.
- [x] 3.2 Add render coverage showing long, short, Unicode, empty, focused, and unfocused navigator
  rows retain exact frame dimensions and full-width selection padding.

## 4. Validation

- [x] 4.1 Run focused UI and app tests, `just check`, `just build`, strict OpenSpec validation, and
  `git diff --check`; visually smoke the Go app at normal and narrow terminal sizes.
