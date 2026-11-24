## Contents
1. Task 1: Make j and k do what ↓ and ↑ do
2. Task 2: Move the focus to the detail pane with l and to the list with h
3. Task 3: Describe h, j, k and l in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-016 "Move with the keys of vim" (`launchd-tui/US-016`).
Proposal: `docs/proposal-launchd-tui.md`, "Keys".

A developer who uses vim moves in the screen with `h`, `j`, `k`, and `l`, so the hand stays off the arrow keys. `j`
and `k` do what `↓` and `↑` do: with the focus on the list they select the next or the previous service, and with the
focus on the detail pane they scroll it one line. `l` moves the focus to the detail pane, and `h` moves it to the
list. The keys follow the rules of the keys that they stand beside: while the help shows they change nothing, and
while a confirmation shows they answer no. The help that `?` shows lists them, and `docs/cli.md` describes them.

The story criteria that this change satisfies:

- `j` does what `↓` does, and `k` does what `↑` does: they select the next or the previous service in the list, and
  they scroll the detail pane when it has the focus.
- `l` moves the focus to the detail pane, and `h` moves the focus to the list. `l` when the detail pane has the focus,
  and `h` when the list has the focus, do nothing.
- While the help shows, `h`, `j`, `k`, and `l` do nothing, as `↑`, `↓`, and `tab` do.
- While a confirmation shows, `h`, `j`, `k`, and `l` answer no, as each key other than `y` does.
- The help that `?` shows lists `h`, `j`, `k`, and `l` with their actions, and `docs/cli.md` describes them.

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
- No one-pane layout. The screen always shows the list and the detail pane side by side, so `l` and `h` only move the
  focus; they do not open a pane or go back from one. `enter` and `esc` get no new action.
- No filter line. The screen has no filter, so no key goes into a filter as text.
- No log key. `o` and `L` do nothing, as today.
- `←` and `→` do not move the focus. Only `h` and `l` do, and `tab` as today.
- Only the lower-case letters move. `H`, `J`, `K`, and `L` do nothing, as any other key that the screen does not know.
- No other keys of vim: no `gg`, `G`, `ctrl+d`, `ctrl+u`, and no count before a key.
- The help line at the bottom of the screen does not change. It shows `s start · x stop · ? help`, the keys of the
  actions, and no move.
- The help does not scroll. In a terminal too low for all of its lines, the help box cuts its last lines, as it does
  today.
- The rules of the confirmation do not change: `y` answers yes, `ctrl+c` quits, and each other key answers no.
- The rules of the message line do not change: a key press hides it, and the key also does what it does.

**Deferred:** None.

## Codebase Context

- `internal/tui/screen.go:124-135`: `Update` gives each key press to `answer` while a confirmation shows, and to `press`
  otherwise. `answer` (`screen.go:202-212`) answers yes only for `y` and quits only for `ctrl+c`, so `h`, `j`, `k`, and
  `l` answer no today with no code. The criterion about the confirmation is a test that fails if a later change
  handles one of these keys before the confirmation.
- `internal/tui/screen.go:165-200`: `press` switches on the text of the key. `up` and `down` (`screen.go:186-197`) do
  nothing while the help shows, scroll the detail pane when it has the focus, and select the next or the previous row
  otherwise. `tab` (`screen.go:174-177`) moves the focus to the other pane, and does nothing while the help shows. The
  focus is the private type `pane` (`screen.go:67-79`), `listPane` or `detailPane`.
- `Update` (`screen.go:157-161`) sets the scroll of the detail pane back to the first line when the selected service
  changes, and keeps the scroll and the list inside their text after each message, for each key.
- `internal/tui/screen.go:413-423`: `helpText` gives the lines of the help, with the key in a column of 10 cells. The
  help today has 7 lines. `box.render` (`internal/tui/box.go:22-33`) draws only as many lines as the box has room for.
  The golden test of the help (`internal/tui/screen_test.go:3079-3085`) opens a terminal of 80 by 13, which has room
  for exactly 7 lines, so a help of more lines needs a taller terminal in that test, or `keys.golden` cuts its last
  lines. `keys.golden` is the only golden file that shows the help; no task changes the layout of the panes, so no
  other golden file changes.
- `internal/tui/screen_test.go:100-112` declares the keys that the tests press. A letter is a key press with its code
  and its text, as `keyS` and `keyX`.
- Tests to follow, all under the umbrella `TestScreen`: the group `select` (`screen_test.go:1982-1991`, `:2071-2093`,
  `:2245-2253`), the group `focus` (`screen_test.go:2324-2405`), the group `scroll` (`screen_test.go:2408-2422`,
  `:2494-2509`, `:2646-2655`), the test that each key other than `y` answers no (`screen_test.go:2949-2965`), and the
  group `keys` (`screen_test.go:3079-3085`, `:3116-3122`).
- `e2e/tests/screen.test.ts:94-102` checks that `↓` selects the next row, and `:126-154` that `tab` gives the focus to
  the detail pane and `↓` then scrolls it. `start-stop.test.ts` sends a letter with `session.press`, and
  `screen.test.ts:156-169` with `session.type`. `openCli` gives svcview 160 columns and 40 rows.
- `docs/cli.md`: the part of the detail pane in the table of parts (`:74`), "The focus and the scroll" (`:114-128`),
  "Keys" (`:140-150`), and the confirmation (`:166-173`). The status sections (`:521-522`, `:570-571`) say that `↑` and
  `↓` scroll the detail pane, with a link to "The focus and the scroll".
- `tasks/learnings.md`: "Each key of the screen goes into the help and docs/cli.md" (so each task that adds a key adds
  its line to the help); "A test that a pane does not move needs a pane that can scroll" (the tests that `j` and `k`
  do not move the list, and that a key changes nothing); "Fold the fixes of two tasks in one file with one clerk
  fixup mark for each task in turn" (Tasks 1 and 2 both change `press`, `helpText`, `screen_test.go` and
  `keys.golden`); "Run clerk mutants and clerk audit run under mise exec"; "task test:e2e:local leaves
  e2e/package-lock.json in the tree"; "Pass --integrate to clerk land when the request asks to land on main".

## Tasks

### Task 1: Make j and k do what ↓ and ↑ do

**Behavior:** `j` does what `↓` does, and `k` does what `↑` does. With the focus on the list, `j` selects the next
service and `k` the previous one, in the order of the list. With the focus on the detail pane, `j` scrolls its text up
one line and `k` scrolls it back. While the help shows, they change nothing, and while a confirmation shows, they
answer no. The help lists `k` and `j` on the line of `↑` and `↓`.

**Acceptance Criteria:**
- [x] With the focus on the list, `j` selects the next row in the order of the list, also in the next group, and `k`
      the row before it.
- [x] `j` at the last row keeps the last row selected, and `k` at the first row keeps the first row selected.
- [x] With no rows, `j` and `k` change nothing.
- [x] With the focus on the detail pane, `j` moves its text up one line, so the next line shows at the bottom, and `k`
      moves it back down one line. The scroll stops at the same lines as for `↓` and `↑`.
- [x] With the focus on the detail pane, `j` and `k` do not change the selected service, and the list does not move,
      in a list that has more rows than its pane.
- [x] While the help shows, `j` and `k` do not change the selected service and do not scroll the detail pane.
- [x] While a confirmation shows, `j` and `k` answer no: the confirmation hides, nothing is called, and nothing else
      changes.
- [x] The help shows `↑ ↓ k j` in the column of the keys, with the action of `↑` and `↓`. The help keeps its 7 lines,
      and `keys.golden` shows the new line.
- [x] e2e: `j` selects the next row, which then starts with `▌`, and `k` then selects the row before it.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `press` handles `j` and `k`; `helpText` names them
- `internal/tui/screen_test.go` — the tests of `j` and `k` in the groups `select`, `scroll` and `keys` of
  `TestScreen`, and `j` and `k` in the test that each key other than `y` answers no
- `internal/tui/testdata/keys.golden` — the help with `k` and `j`
- `e2e/tests/screen.test.ts` — the e2e test of `j` and `k`

**Patterns to Follow:**
- `internal/tui/screen.go:186-197`
- `internal/tui/screen.go:413-423`
- `internal/tui/screen_test.go:100-112`
- `internal/tui/screen_test.go:1982-1991`
- `internal/tui/screen_test.go:2071-2093`
- `internal/tui/screen_test.go:2245-2253`
- `internal/tui/screen_test.go:2408-2422`
- `internal/tui/screen_test.go:2494-2509`
- `internal/tui/screen_test.go:2646-2655`
- `internal/tui/screen_test.go:2949-2965`
- `internal/tui/screen_test.go:3079-3085`
- `internal/tui/screen_test.go:3116-3122`
- `e2e/tests/screen.test.ts:94-102`

**Testable:** Yes

**Certainty:** high — `↑` and `↓` do the same moves at `internal/tui/screen.go:186-197`, with their tests in the
groups `select` and `scroll` of `internal/tui/screen_test.go`.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of `j` and `k`.

**Depends on:** None

### Task 2: Move the focus to the detail pane with l and to the list with h

**Behavior:** `l` moves the focus to the detail pane, and `h` moves the focus to the list. `l` when the detail pane has
the focus, and `h` when the list has the focus, change nothing. While the help shows, they change nothing, and while a
confirmation shows, they answer no. The help lists `l` and `h`, each on a line of its own with its action.

**Acceptance Criteria:**
- [ ] `l` moves the focus to the detail pane: its border is blue, and the border of the list is dim. `h` then moves the
      focus back to the list.
- [ ] `l` with the focus on the detail pane, and `h` with the focus on the list, change nothing: the screen shows the
      same as before the key.
- [ ] After `l`, `↓` and `j` scroll the detail pane, and after `h`, they select the next service.
- [ ] `l` and `h` move the focus also when the list has no rows, or shows a read error, as `tab` does.
- [ ] While the help shows, `l` and `h` change nothing, and after the help hides, the focus is where it was before the
      help showed.
- [ ] While a confirmation shows, `l` and `h` answer no: the confirmation hides, nothing is called, and nothing else
      changes.
- [ ] The help shows `l` with `move the focus to the detail pane` and `h` with `move the focus to the list`, in this
      order, on the lines after `tab`. The golden test of the help opens a terminal tall enough to show each of its 9
      lines, and `keys.golden` shows them all.
- [ ] e2e: `l` gives the focus to the detail pane, and `j` then scrolls it with the same service selected; `h` gives
      the focus back to the list, and `j` then selects the next row.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `press` handles `l` and `h`; `helpText` names them
- `internal/tui/screen_test.go` — the tests of `l` and `h` in the groups `focus` and `keys` of `TestScreen`, and `l`
  and `h` in the test that each key other than `y` answers no
- `internal/tui/testdata/keys.golden` — the help with `l` and `h`, in a taller terminal
- `e2e/tests/screen.test.ts` — the e2e test of `l` and `h`

**Patterns to Follow:**
- `internal/tui/screen.go:67-79`
- `internal/tui/screen.go:174-177`
- `internal/tui/screen.go:413-423`
- `internal/tui/screen_test.go:2324-2405`
- `internal/tui/screen_test.go:2408-2422`
- `internal/tui/screen_test.go:2949-2965`
- `internal/tui/screen_test.go:3079-3085`
- `e2e/tests/screen.test.ts:126-154`
- `task:1`

**Testable:** Yes

**Certainty:** high — `tab` moves the focus at `internal/tui/screen.go:174-177` with the same rule for the help, and the
group `focus` at `internal/tui/screen_test.go:2324-2405` tests it through the borders.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of `l` and `h`.

**Depends on:** Task 1

### Task 3: Describe h, j, k and l in docs/cli.md

**Behavior:** `docs/cli.md` tells a user that `j` and `k` select a service or scroll the detail pane as `↓` and `↑`
do, that `l` and `h` move the focus to the detail pane and to the list, and what the four keys do while the help or a
confirmation shows.

**Acceptance Criteria:**
- [ ] The table of "Keys" names `k` and `j` in the row of `↑` and `↓`, and has a row for `l` and a row for `h`, each
      with its action and with the pane where it changes nothing.
- [ ] "The focus and the scroll" says that `l` moves the focus to the detail pane and `h` to the list, that `j` and
      `k` select and scroll as `↓` and `↑` do, and that while the help shows, `h`, `j`, `k`, and `l` change nothing,
      as `tab`, `↑`, and `↓` do.
- [ ] Each place in `docs/cli.md` that says which keys select a service or scroll the detail pane names `k` and `j`
      beside `↑` and `↓`, or links to "The focus and the scroll".
- [ ] The confirmation section still says that each key other than `y` answers no, which covers `h`, `j`, `k`, and
      `l`.
- [ ] The help line at the bottom of the screen stays `s start · x stop · ? help` in the docs.
- [ ] Each statement matches what Tasks 1 and 2 built.

**Affected Files/Modules:**
- `docs/cli.md` — the table of parts, "The focus and the scroll", and "Keys"

**Patterns to Follow:**
- `docs/cli.md:74`
- `docs/cli.md:114-128`
- `docs/cli.md:140-150`
- `docs/cli.md:166-173`

**Testable:** No

**Certainty:** high — the keys `tab`, `↑` and `↓` are described the same way in "The focus and the scroll" and "Keys"
at `docs/cli.md:114-150`.

**Blast radius:** low

**Verification:** Manual check of `docs/cli.md` against the tests of Tasks 1 and 2.

**Depends on:** Task 2

## Summary

- 3 tasks.
- Order: the pair of keys that repeats an existing move first (Task 1, `j` and `k` beside `↑` and `↓`); then the pair
  that moves the focus (Task 2, `l` and `h` beside `tab`), whose e2e test uses `j`; then the docs (Task 3). Each key
  goes into the help in the task that adds it, so the help lists each key that the screen handles after each task.
- No task changes the layout of the panes, so `keys.golden` is the only golden file that changes. Tasks 1 and 2 both
  change it, and Task 2 makes its test terminal taller, because the help grows from 7 lines to 9.
- Criteria: the first (`j` and `k`) in Task 1; the second (`l` and `h`) in Task 2; the third (the help shows) in
  Tasks 1 and 2, each for its own keys; the fourth (the confirmation) in Tasks 1 and 2, as tests of a rule that holds
  today with no code; the fifth (the help and `docs/cli.md`) in Tasks 1 and 2 for the help, and Task 3 for the docs.
  None is deferred.
- The story is routine: every task is high certainty, because each key copies the rules of a key that the screen has
  (`↑` and `↓`, or `tab`), and each task is low blast radius, because no task touches launchd, a config file, a
  command that svcview runs, or text from outside.
