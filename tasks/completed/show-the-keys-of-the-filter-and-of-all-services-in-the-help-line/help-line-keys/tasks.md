## Contents
1. Task 1: Show / filter and a all on the help line before ? help
2. Task 2: Show the help line with / filter and a all in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-018 "Show the keys of the filter and of all services in the help line"
(`launchd-tui/US-018`).
Proposal: `docs/proposal-launchd-tui.md`, "The Screen" (the help line under the example screen) and "Keys" (the last
paragraph: the help line shows the keys of the actions, `/`, and `a`).

A developer finds the filter and the list of all services with no look at the help. The help line at the bottom of
the screen shows `/ filter` and `a all` after the keys of the actions of the group of the selected service, and before
`? help`. With no selected service, or with a service of a group with no actions, the line shows the keys of svcview
with `/ filter` and `a all`. The text stays `a all` while all services show, because the mark `· all` in the header
says which list is on the screen. While the filter line is open, the help line stays
`enter keep the filter · esc clear the filter`. `docs/cli.md` shows the new help line.

The story criteria that this change satisfies:

- The help line shows `/ filter` and `a all` after the keys of the actions of the group, and before `? help`, for
  example `s start · x stop · r restart · o log · L worker log · / filter · a all · ? help`.
- With no selected service, or when the group of the selected service has no actions, the help line shows
  `s start · x stop · r restart · o log · / filter · a all · ? help`.
- The text stays `a all` while all services show. The mark in the header shows which list is on the screen.
- While the filter line is open, the help line stays `enter keep the filter · esc clear the filter`.
- `docs/cli.md` shows the new help line.

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
- No key changes. `/` and `a` do what they do today, `keysOfSvcview` in `internal/config/action.go` does not change,
  and no action of a group gets or loses a key.
- The help that `?` shows does not change. It lists `/` and `a` already.
- The help line shows no other new key: not `enter`, `esc`, `tab`, `h`, `j`, `k`, `l`, `backspace` or `q`.
- The text of `a` does not change with the list on the screen. It is `a all` while the services with a plist show and
  while all services show; only the mark `· all` in the header says which list shows.
- The help line keeps the rule of when it shows which keys. Under the help that `?` shows, under a confirmation, and
  with a kept filter, it shows the same keys as under the panes, `/ filter` and `a all` too. While the filter line is
  open, it shows only `enter keep the filter · esc clear the filter`.
- No color and no style on the help line.

**Deferred:**
- A help line that keeps `? help` in a narrow terminal. svcview cuts the help line at its right edge as before, so a
  terminal narrower than the line loses `? help` first, and no key leaves the line to make room.

## Codebase Context

- `internal/tui/screen.go:615-624`: `Screen.helpLine` returns ` enter keep the filter · esc clear the filter` while the
  filter line is open (`:616-618`). Otherwise it gives `s start`, `x stop`, `r restart` and `o log`, then the key and
  the name of each action that `selectedActions` (`:505-511`) gives, each through
  `terminal.ReplaceControlCharacters`, then `? help`, joined with ` · ` after one space. It reads neither `Screen.all`
  nor `lastRead.all`, and not whether the help or a confirmation shows, so the same line shows under the panes, under
  the help, and under a confirmation. `selectedActions` gives no action with no selected service: with no rows, while
  the list shows a read error, while the list shows `no service matches`, and before the first read ends.
- `internal/tui/screen.go:550-565`: `content` puts the help line last and cuts it at the width of the terminal with no
  `…` (`:564`).
- Widths on the screen, with the space that starts the line: with no action the new line has 65 columns. With the one
  action `L worker log` it has 80 columns, so it fills a terminal of 80 columns exactly (the story says 78; the line is
  79 columns after the first space). With the two actions of `groupsWithActions` (`internal/tui/screen_test.go:365-374`)
  it has 98 columns, and with `B job backlog` 81.
- Tests in `internal/tui/screen_test.go` that name a help line: the groups `start a service` (`:3166`),
  `stop a service` (`:3298`), `restart a service` (`:3493`), `read the log of a service` (`:3715`),
  `show the actions of a group` (`:3906-3995`), `run the action of a group` (`:4003`), `confirm the action of a group`
  (`:4236`), `keys` (`:4425-4439`) and `filter` (`:4450`, `:4549-4559`, `:4598`, `:4902`, `:4963`, `:4987`). The
  groups `run the action of a group` and `confirm the action of a group` draw 80 columns: with the one action of
  `run the action of a group` the new line fits exactly, and with the two actions of `confirm the action of a group`
  svcview cuts it at 80 columns. The test of the exact fit (`:3961-3967`) opens 61 and 60 columns for the line with
  `L worker log`. The test that the help line is cut at 12 columns (`:4434-4439`) keeps ` s start · x`.
- The group `all services` (`internal/tui/screen_test.go:5036-5066`) opens a screen with both reads through
  `openWithReads` (`:117-124`), presses `keyA`, reads with `readNow`, and reads the header with `headerTop`. The test
  of `no service matches` is at `:4802-4810`; the test of a read error after a read that selected a service of a group
  with actions is at `:3950-3956` (tasks/learnings.md, "A screen test of a read error must select a service before the
  error").
- Golden files: 29 of the 32 files in `internal/tui/testdata` end with the help line under the panes or under the help,
  and each changes in its last line only. `keys-with-actions.golden` draws 80 columns with the two actions of
  `groupsWithActions`, so its last line is the new line cut at 80 columns. `filter-line.golden`,
  `no-service-matches.golden` and `filtered-groups.golden` end with the open filter line and do not change.
  tasks/learnings.md, "go test -run splits its pattern at / before it matches": an `-update` run whose `-run` pattern
  has an alternation across a `/` writes no golden file and gives no error, so check
  `git diff --stat internal/tui/testdata` after the run.
- e2e: `e2e/tests/actions.test.ts:44-60` is the one e2e test that checks the content of the help line: it expects
  `o log · L worker log · ? help` on a worker and `o log · ? help` on a service of `Other`. The other e2e tests find
  the help line by `? help` (tasks/learnings.md, "An e2e test finds the help line by a short part of it") or by the
  open filter line (`e2e/tests/filter.test.ts:21`, `:82`), and pass as they are. `openCli` gives 160 columns.
- `docs/cli.md`: the example screen under "The screen" (`:52-73`) ends with the help line; the table of parts gives the
  help line in its last row (`:86`); "The filter" (`:160-190`) gives the help line of the open filter line (`:188`);
  "All services" says that the help line does not show `a` (`:264-268`); "The help line and the help" of the actions
  (`:1045-1056`) shows an example with two actions. `docs/proposal-launchd-tui.md:90` has the line with
  `L worker log`, and `:149-150` says what the help line shows.
- tasks/learnings.md: "Plan the layout change of the screen as the first task" (Task 1 rewrites the golden files, and
  Task 2 touches none); "A test of a cut at a width needs the exact-fit width and one cell less"; "Run clerk mutants
  and clerk audit run under mise exec"; "task test:e2e:local leaves e2e/package-lock.json in the tree"; "Pass
  --integrate to clerk land when the request asks to land on main".

## Tasks

### Task 1: Show / filter and a all on the help line before ? help

**Behavior:** The help line under the panes and under the help shows `/ filter` and `a all` after the keys of the
actions of the group of the selected service, and before `? help`. With no selected service, or with a service of a
group with no actions or of `Other`, it shows ` s start · x stop · r restart · o log · / filter · a all · ? help`. The
text stays `a all` while all services show. While the filter line is open, the help line stays
` enter keep the filter · esc clear the filter`. svcview cuts the line at the right edge of the terminal, as before.

**Acceptance Criteria:**
- [x] With a service of `Workers` of `groupsWithActions` selected, the help line is
      ` s start · x stop · r restart · o log · L worker log · E worker errors · / filter · a all · ? help` at 100
      columns. The line follows the selection: ` s start · x stop · r restart · o log · B job backlog · / filter · a all · ? help`
      on a service of `Jobs`, and the line with no action on a service of a group with no actions and of `Other`.
- [x] With no selected service, the help line is ` s start · x stop · r restart · o log · / filter · a all · ? help`:
      with no rows, while the list shows a read error after a read that selected a service of a group with actions,
      with a kept filter that the name of no service contains, and before the first read ends.
- [x] On a screen that starts with all services, the header shows `gui/501 · all` and the help line is
      ` s start · x stop · r restart · o log · / filter · a all · ? help`. On a screen that starts with the services
      with a plist, `a` and the next read show the mark, and the help line is the same line as before `a`.
- [x] While the filter line is open, the help line is ` enter keep the filter · esc clear the filter`, also with a
      service of a group with actions selected. After `enter` with a kept filter, and after `esc`, the help line of the
      selected service shows again, with `/ filter` and `a all`.
- [x] Under the help that `?` shows, and under a confirmation of `x` or `r`, the help line shows the same keys as under
      the panes, with `/ filter` and `a all`.
- [x] The help line with the one action `L worker log` has 80 columns: it shows whole in a terminal of 80 columns, and
      a terminal of 79 columns cuts its last cell, so `? help` loses its end first. In a terminal of 12 columns, the
      help line is still ` s start · x`.
- [x] Each golden file in `internal/tui/testdata` that ends with the help line under the panes or under the help shows
      the new line in its last line and changes in no other line. `keys-with-actions.golden` shows the line with two
      actions cut at 80 columns. The golden files of the open filter line do not change.
- [x] Each other test in `internal/tui/screen_test.go` that names a help line expects the new line, cut at the width of
      its terminal when the line is wider.
- [x] e2e: on a worker of a group with the action `L worker log`, the help line contains
      `o log · L worker log · / filter · a all · ? help`. After `↓` selects a service of `Other`, it contains
      `o log · / filter · a all · ? help` and no `worker log`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `Screen.helpLine` gives `/ filter` and `a all` before `? help`
- `internal/tui/screen_test.go` — the expected help lines; the help line with all services, with no selected service,
  and at the exact fit of 80 columns
- `internal/tui/testdata/` — the last line of 29 golden files
- `e2e/tests/actions.test.ts` — the help line on a worker and on a service of `Other`

**Patterns to Follow:**
- `internal/tui/screen.go:615-624`
- `internal/tui/screen.go:550-565`
- `internal/tui/screen_test.go:3906-3967`
- `internal/tui/screen_test.go:4425-4439`
- `internal/tui/screen_test.go:4549-4559`
- `internal/tui/screen_test.go:4802-4810`
- `internal/tui/screen_test.go:5036-5066`
- `e2e/tests/actions.test.ts:44-60`

**Testable:** Yes

**Certainty:** high — the keys of the actions came onto the help line in the same method, with the same tests of the
selection, the exact fit and the e2e help line (`internal/tui/screen.go:615-624`, `internal/tui/screen_test.go:3906-3967`,
`e2e/tests/actions.test.ts:44-60`).

**Blast radius:** low — the change is text on the screen: no key, no read, no launchctl call and no config file
changes.

**Verification:** `mise exec -- task test:unit` passes; `git diff --stat internal/tui/testdata` names 29 golden files,
each with one changed line; `mise exec -- task test:e2e` passes `e2e/tests/actions.test.ts`.

**Depends on:** None

### Task 2: Show the help line with / filter and a all in docs/cli.md

**Behavior:** `docs/cli.md` shows the help line that svcview draws: the example screen, the row of the help line in the
table of the parts of the screen, the filter, all services, and the help line of the actions of a group.

**Acceptance Criteria:**
- [x] The example screen under "The screen" ends with ` s start · x stop · r restart · o log · / filter · a all · ? help`.
- [x] The row "The help line" of the table of parts gives `s start · x stop · r restart · o log · / filter · a all · ? help`,
      says that the key and the name of each action show after `o log` and before `/ filter`, with the example
      `s start · x stop · r restart · o log · L worker log · / filter · a all · ? help`, and keeps the help line of the
      open filter line.
- [x] "The filter" says that the help line shows `/ filter` while the filter line is not open.
- [x] "All services" says that the help line shows `a all`, also while all services show, and that the mark `· all` in
      the header says which list shows. No sentence of `docs/cli.md` says that the help line does not show `a`.
- [x] "The help line and the help" of the actions shows its example with `/ filter · a all` after the actions and
      before `? help`.
- [x] Each help line in `docs/cli.md` is a line that the tests of Task 1 expect.

**Affected Files/Modules:**
- `docs/cli.md` — the example screen, the table of parts, "The filter", "All services", and "The help line and the help"

**Patterns to Follow:**
- `docs/cli.md:52-86`
- `docs/cli.md:160-190`
- `docs/cli.md:264-268`
- `docs/cli.md:1045-1056`
- `docs/proposal-launchd-tui.md:68-90`
- `task:1`

**Testable:** No

**Certainty:** high — `docs/cli.md:86` and `docs/cli.md:1045-1056` describe the keys of the actions on the help line,
and this adds two keys to the same text.

**Blast radius:** low — documentation only.

**Verification:** Manual check: `rg -n "\? help" docs/cli.md` shows `/ filter · a all` before `? help` on each help
line under the panes, and each such line is a line that `internal/tui/screen_test.go` expects.

**Depends on:** Task 1

## Summary

- 2 tasks. Task 1 changes the text that `Screen.helpLine` gives, with its tests, the golden files and the e2e test,
  first, because it rewrites the last line of 29 golden files (tasks/learnings.md, "Plan the layout change of the
  screen as the first task"). Task 2 describes the new line in `docs/cli.md`.
- Criteria: the first four story criteria in Task 1, and `docs/cli.md` in Task 2. None deferred.
- The story is routine: each task repeats a change that the keys of the actions made to the same method, the same
  tests and the same docs, so both tasks are high certainty and low blast radius.
