## Contents
1. Task 1: Move the focus between the list and the detail pane with tab
2. Task 2: Scroll the detail pane with ↑ and ↓ when it has the focus
3. Task 3: Show a newly selected service from the first line of its detail pane, and keep the scroll inside a text that changes
4. Task 4: Describe the focus and the scroll in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-004 "See the fields and the events of a status" (`launchd-tui/US-004`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Colors and Glyphs" (the focused pane) and
"Keys".

With the fields and the events of a status, the detail pane can be longer than the terminal. This change lets the
user read all of it. One of the two panes has the focus: the list at start. `tab` moves the focus to the other pane.
The border of the pane with the focus has the accent color, and the border of the other pane is dim. When the list
has the focus, `↑` and `↓` select a service, as before. When the detail pane has the focus, `↑` and `↓` scroll it one
line. A service that becomes selected shows its detail pane from the first line. `?` shows `tab`, and `docs/cli.md`
describes the focus and the scroll.

The story criteria that this change satisfies:

- `tab` moves the focus to the other pane. The border of the focused pane has the accent color, and the border of the
  other pane is dim.
- When the detail pane has the focus, `↑` and `↓` scroll it. When the list has the focus, they select a service.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No work in the worker repo. The worker writes its status file, and its install script adds `WORKER_STATUS_FILE`
  to the plist and writes `conf.d/worker.yaml`. That work is outside svcview.
- No other key moves the focus or scrolls: no `shift+tab`, `page up`, `page down`, `home`, `end`, and no mouse wheel.
  `esc` only hides the help, as before.
- The list does not scroll on its own. It scrolls only to keep the selected row in view, as before.
- No mark shows that the detail pane has more lines above or under the lines that it shows.
- svcview does not keep the scroll of a service after another service is selected. When that service is selected
  again, its detail pane shows from the first line.
- The header and the help have no focus. Their borders keep their look. The title in the border of a pane keeps its
  look too.
- The lines of the detail pane do not change: this change only moves them.
- No layout for a narrow terminal, no `enter` that opens the detail pane, and no change for `NO_COLOR`.
- No mark of a stale status. `stale_after` stays a setting that svcview does not know.

**Deferred:** None.

## Codebase Context

- `internal/tui/screen.go:89-129`: `Update` handles the keys. `↑` and `↓` select a service unless the help shows
  (`screen.go:102-109`), `?` shows or hides the help, and `esc` hides it (`screen.go:98-101`). After each message,
  `listTopInView` (`screen.go:212-224`) moves `listTop` so that the selected row stays in view (`screen.go:127`).
- `screen.go:273-287`: `panes` cuts the text of the list at `listTop`. The detail pane always shows its text from the
  first line. Its text is the launchd lines, the line `Status`, and under it the lines of a failure, or the fields of
  the output with their bars and its events. A failure has up to 21 lines, and an output can have any number of
  fields and events, so the length of the text changes at each run. The pane shows its height minus 2 lines.
- `internal/tui/box.go:9-44`: `box` draws each rounded box of the screen: the top border with the title
  (`box.go:33-44`), the sides and the bottom border (`box.go:21-31`), all in the default color. The header
  (`screen.go:258`), the help (`screen.go:261`), the list and the detail pane (`screen.go:274-276`) are boxes. The
  list and the detail pane are side by side, so each line of the screen is a line of the list followed by a line of
  the detail pane.
- `accent` is blue (`screen.go:22`). Dim is the SGR attribute faint in the default color, the look of the cells of a
  bar that are not done. `textStyle` (`screen.go:385-394`) builds a style from a color.
- `helpText` (`screen.go:289-296`) lists each key of the screen, and the golden file `keys.golden` shows it.
  `tasks/learnings.md`, "Each key of the screen goes into the help and docs/cli.md": the Keys table of `docs/cli.md`
  (`docs/cli.md:109-116`) must match the help. Bubble Tea names the key `tab`.
- Tests: the helpers `listRows`, `selectedName` and `detailPane` (`internal/tui/screen_test.go`) remove the SGR
  sequences and cut the lines of the screen at the `│` of the borders, so a border with a color keeps them working.
  The subtests of the scroll of the list in the group `select` are the precedent for a scroll test, for example
  "a read that removes the rows at the end of a scrolled list keeps the list pane full" and "a smaller terminal
  scrolls the list, so the selected row stays in view". The e2e test "? shows all keys, and q ends svcview while the
  help shows" waits for the text `select a service` (`e2e/tests/screen.test.ts:118-131`).
- `docs/cli.md`: the parts of the screen (`docs/cli.md:62-67`), the glyphs and colors (`docs/cli.md:72-85`), the
  Keys table (`docs/cli.md:109-116`), and the sentence that says that the detail pane does not scroll
  (`docs/cli.md:346-348`).
- `tasks/learnings.md`, "Plan the layout change of the screen as the first task": a border with a color changes every
  golden file in `internal/tui/testdata/` that shows the panes, so Task 1 makes that change, and Tasks 2 and 3 change
  only what the keys do.
- Words: the focus is the pane that `↑` and `↓` act on. The scroll is the first line of its text that the detail pane
  shows, as `listTop` is for the list. The selection stays the selected service.

## Tasks

### Task 1: Move the focus between the list and the detail pane with tab

**Behavior:** At start, the list has the focus. `tab` moves the focus to the other pane, and `tab` again moves it
back. The lines and the corners of the border of the pane with the focus are in the accent color, and the lines and
the corners of the border of the other pane are dim. `?` shows `tab` with its action.

**Acceptance Criteria:**
- [x] At start, the border of the list is blue and the border of the detail pane is dim.
- [x] `tab` moves the focus to the detail pane: its border is blue, and the border of the list is dim. `tab` again
      moves the focus back to the list.
- [x] The title in the border of each pane keeps the default color, so the full name of the selected service stays
      easy to read. The border of the header and the help line keep their look.
- [x] The focus stays where it is at a refresh, at the end of a run, and after `?` shows and hides the help.
- [x] While the help shows, `tab` changes nothing, and the border of the help keeps its look.
- [x] `tab` moves the focus also when the list has no rows, or shows an error.
- [x] `?` shows `tab` and its action, and the golden file of the help shows it.
- [x] The golden files of the screen show the border of the list in the accent color and the border of the detail
      pane dim, and a new golden file shows the screen after `tab`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the focus, `tab`, the style of the border of each pane, and `tab` in the help
- `internal/tui/box.go` — the style of the lines and the corners of a border
- `internal/tui/screen_test.go` — the tests of the focus
- `internal/tui/testdata/` — each golden file with the panes, `keys.golden`, and a new golden file after `tab`

**Patterns to Follow:**
- `internal/tui/box.go:21-44`
- `internal/tui/screen.go:98-101`
- `internal/tui/screen.go:385-394`
- `internal/tui/screen.go:289-296`

**Testable:** Yes — through the `Screen` model with key presses and golden files.

**Certainty:** medium — `box` draws each border (`box.go:21-44`) and `?` toggles the help with a key
(`screen.go:98-101`), but no border has a style, and no state of the screen decides which pane a key acts on.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** None

### Task 2: Scroll the detail pane with ↑ and ↓ when it has the focus

**Behavior:** When the detail pane has the focus, `↓` shows its text from one line further down, and `↑` from one
line further up. The scroll stops at the first line, and at the line that puts the last line of the text at the
bottom of the pane. The selected service stays the same. When the list has the focus, `↑` and `↓` select a service,
as before. `?` shows both actions of `↑` and `↓`.

**Acceptance Criteria:**
- [x] With the focus on the detail pane, `↓` moves the text of the pane up one line: its first line goes out of view,
      and the next line shows at the bottom. `↑` moves the text back down one line.
- [x] `↑` when the first line shows, and `↓` when the last line is at the bottom of the pane, change nothing. A text
      that fits in the pane does not move.
- [x] Only the lines inside the detail pane move. Its title and its border stay.
- [x] With the focus on the detail pane, `↑` and `↓` do not change the selected service, and the list does not move.
- [x] With the focus on the list, `↑` and `↓` select a service as before, and the detail pane does not scroll.
- [x] While the help shows, `↑` and `↓` change nothing, as before.
- [x] `?` shows `↑ ↓` with both actions: select a service, or scroll the detail pane when it has the focus. The text
      `select a service` stays in the help.
- [x] e2e test: a status file with 40 events, `tab`, and then `↓` until the last event shows: the first line of the
      detail pane is no longer on the screen, and the same service stays selected.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the scroll of the detail pane, and `↑ ↓` in the help
- `internal/tui/screen_test.go` — the tests of the scroll
- `internal/tui/testdata/` — `keys.golden`, and a new golden file of a scrolled detail pane
- `e2e/tests/screen.test.ts` — `tab` and the scroll of a detail pane with many events

**Patterns to Follow:**
- `internal/tui/screen.go:212-224`
- `internal/tui/screen.go:273-287`
- `internal/tui/screen_test.go`
- `e2e/tests/screen.test.ts:86-94`
- `task:1`

**Testable:** Yes — through the `Screen` model with key presses and fake runs, and in an e2e test.

**Certainty:** medium — `listTopInView` and the cut of the list at `listTop` (`screen.go:212-224`,
`screen.go:273-287`) scroll the list, but the detail pane scrolls one line for each key, with no selected row to
keep in view.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1

### Task 3: Show a newly selected service from the first line of its detail pane, and keep the scroll inside a text that changes

**Behavior:** When another service becomes selected, its detail pane shows from the first line. This happens after
`↑` or `↓` with the focus on the list, and after a read that removes the selected service. When the text of the
detail pane gets shorter, or the pane gets lower or higher, the scroll moves back so that the last line of the text
is at the bottom of the pane or above it. In each other case, a refresh or a run keeps the scroll where it is.

**Acceptance Criteria:**
- [ ] After a scroll of the detail pane, `tab` and `↓` select the next service, whose detail pane shows from its
      first line. `↑` then shows the first service from its first line too.
- [ ] A read that removes the selected service, while its detail pane is scrolled, selects the service that takes its
      place, and shows it from its first line.
- [ ] A run that gives the selected service a shorter text, for example a failure after an output with many events,
      shows the last line of the new text at the bottom of the pane, with no empty line under it, while the text is
      longer than the pane. A new text that fits in the pane shows from its first line.
- [ ] A terminal that gets higher shows more lines, with no empty line under the last line while the text is longer
      than the pane. A terminal that gets lower keeps the scroll inside the text.
- [ ] A refresh, and a run that gives a text of the same length, keep the scroll where it is. The age in the line
      `Status` grows with no change of the scroll.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the scroll after a new selection, a run, a read, and a change of the size of the terminal
- `internal/tui/screen_test.go` — the tests of the scroll when the selection or the text changes

**Patterns to Follow:**
- `internal/tui/screen.go:127`
- `internal/tui/screen.go:212-224`
- `internal/tui/screen_test.go`
- `task:2`

**Testable:** Yes — through the `Screen` model with key presses, reads, fake runs and window sizes.

**Certainty:** medium — `listTopInView` keeps the list pane full after a read that removes rows and after a change of
the size of the terminal (`screen.go:212-224`), but the list has no return to the first line when the selected
service changes.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 2

### Task 4: Describe the focus and the scroll in docs/cli.md

**Behavior:** `docs/cli.md` describes the focus of a pane, `tab`, both actions of `↑` and `↓`, and the scroll of the
detail pane. The Keys table matches the help of the screen.

**Acceptance Criteria:**
- [ ] The Keys table lists `tab`, and `↑` `↓` with both actions, as the help shows them.
- [ ] The table of the glyphs and colors lists the border of the pane with the focus, blue, and the border of the
      other pane, dim.
- [ ] The table of the parts of the screen says that the list has the focus at start, and that the detail pane
      scrolls when it has the focus.
- [ ] The two sentences that say that the detail pane does not scroll, at the end of "Failures" and at the end of
      "In the list and the detail pane", say that `↑` and `↓` scroll it when it has the focus.
- [ ] It says that a newly selected service shows from the first line of its detail pane.

**Affected Files/Modules:**
- `docs/cli.md` — the focus, `tab`, and the scroll of the detail pane

**Patterns to Follow:**
- `docs/cli.md:62-116`

**Testable:** No — a document.

**Certainty:** high — the tables of `docs/cli.md:62-116` are the precedent, and the proposal gives the keys ("Keys")
and the look of the focused pane ("Colors and Glyphs").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the help of the screen and against `task cli` with a status
command whose output has more events than the pane shows.

**Depends on:** Task 3

## Summary

- 4 tasks. Task 1 gives one pane the focus and draws its border in the accent color. It changes the look of every
  pane, so it comes first. Task 2 makes `↑` and `↓` act on the pane with the focus. Task 3 keeps the scroll correct
  when the selection, the text or the size of the terminal changes. The document comes last.
- Covers the criterion on `tab` and the borders (Task 1), and the criterion on `↑` and `↓` (Tasks 2 and 3). Task 4
  describes them.
- Certainty: Tasks 1 to 3 are medium, because the repo has a scroll and a key that changes the view, but no focus, no
  border with a style, and no scroll without a selected row. Task 4 repeats the tables of `docs/cli.md`. No task has
  a high blast radius: this change shows the same lines and adds a key. It runs no program and reads no new data.
- The planner chose these details, which the story does not give: the list has the focus at start; only the lines and
  the corners of a border take the color of the focus, and its title keeps the default color; dim is the SGR
  attribute faint; `tab` changes nothing while the help shows; `↑` and `↓` scroll one line, and no other key scrolls;
  a newly selected service shows from the first line; when the text gets shorter, the last line stays at the bottom.
