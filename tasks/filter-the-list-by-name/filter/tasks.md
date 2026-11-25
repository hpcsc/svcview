## Contents
1. Task 1: Open a filter line with / that takes each key that types text
2. Task 2: Show only the services whose name contains the filter
3. Task 3: Show no service matches when the filter hides each service
4. Task 4: Keep the filter with enter
5. Task 5: Describe the filter in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-012 "Filter the list by name" (`launchd-tui/US-012`).
Proposal: `docs/proposal-launchd-tui.md`, "The Screen" and "Keys" (`/` filters the list by name).

A developer with many services finds one service with a few keys. `/` opens a filter line under the panes. Each key
that types text goes into the filter, also `h`, `j`, `k`, `l` and the other keys of svcview and of the actions of a
group, so no key acts on a service while you type. After each key, the list shows only the services whose name
contains the filter, with no regard to case, and a group with none of them is not in the list. `enter` keeps the
filter, and `↑` and `↓` then select a service in the filtered list. `esc` clears the filter, and the list shows all
services again. When no service matches, the list shows `no service matches`. The help that `?` shows lists the new
keys, and `docs/cli.md` describes the filter.

The story criteria that this change satisfies:

- `/` opens a filter line. After each key, the list shows only the services whose name contains the text, with no
  regard to case.
- A group with no service that matches the filter is not in the list.
- `enter` keeps the filter, and `↑` and `↓` then select a service in the filtered list.
- `esc` clears the filter, and the list shows all services again.
- While the filter line is open, each letter goes into the filter, also `h`, `j`, `k`, and `l`.
- When no service matches, the list shows `no service matches`.

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
- The filter is a part of the name, with no regard to case. No regular expression, no glob, and no fuzzy match.
- The filter compares only with the name of a service. No filter by state, by group, by status summary, or by a
  field of a status.
- No highlight of the part of a row that contains the filter.
- A paste does not go into the filter. Only a key that types text does.
- No cursor that moves in the filter: `←` and `→` do nothing, and `backspace` deletes only the last character.
- While the filter line is open, `↑`, `↓`, `tab` and each other key that types no text do nothing; a service is
  selected with `↑` and `↓` only after `enter`.
- svcview keeps no filter across starts, and no history of filters.
- The header counts each service, also each service that the filter hides.
- A service that the filter hides keeps its status runs.
- While the filter line is not open, `enter` does nothing.
- The help line of a screen with no open filter line does not show `/`; it shows the keys of the actions, as today.
- `/` is already a key of svcview that no action can take, so the read of the config files does not change.
- The rules of the confirmation do not change: `y` answers yes, `ctrl+c` quits, and each other key answers no, `/`
  too.
- The rules of the message line do not change: a key press hides it, also a key that goes into the filter.
- `svcview status` has no filter.

**Deferred:**
- `enter` does not open the detail pane in a layout that shows one pane at a time.

## Codebase Context

- `internal/tui/screen.go:138-149`: `Update` gives each key press to `answer` while a confirmation shows, and to
  `press` otherwise. `press` (`screen.go:183-246`) handles `q`, `?`, `esc`, `tab`, `l`, `h`, `s`, `x`, `r`, `o`, the
  arrows with `j` and `k`, and gives each other key to `runAction` (`screen.go:248-269`), which runs the action of the
  group of the selected service whose key is the text of the key. So while the filter line is open, a key that types
  text must not reach `press`. `answer` (`screen.go:271-281`) is the precedent for a state that takes each key.
- A key that leaks past an open filter line acts on launchd: `s` starts the selected service with no confirmation
  (`screen.go:204-207`), and the word `proxy` holds `r`, `o`, `x` and then `y`, so a leak of `x` asks to stop the
  selected service and `y` answers yes. The tests that a key types text use a selected service that each of `s`, `x`
  and `r` acts on, with the start, the stop and the restart recorded (`serviceCalls`, `screen_test.go:384-400`).
- Bubble Tea v2 gives `Key.Text` only for a key that types printable characters, and an empty text for `enter`,
  `tab`, `esc`, `backspace`, the arrows and a key with `ctrl` or `alt` (module `charm.land/bubbletea/v2@v2.1.2`,
  `key.go:297-301`). The space key has the text `" "` and the string `space` (ultraviolet `key.go:391-396`). A paste
  comes as `tea.PasteMsg` (`paste.go`), which `Update` ignores.
- `internal/tui/screen.go:364-369`: `rows` gives the rows of the last read, or none after a read error, sorted by
  `row.compare`. Each method of the list uses it: `selectedAfterRead` (`:371-381`), `selectedAfterMove`
  (`:383-390`), `listTopInView` (`:392-404`), `selectedService`, `selectedActions`, `serviceForKeys` and
  `serviceForLaunchdAction` (`:411-435`), `selectedAt` (`:437-439`), `startsGroup`, `linesAbove` and `lineOf`
  (`:441-463`), `panes` (`:496-509`), `listText` (`:563-582`) and `nameWidths` (`:584-590`). `runStatuses`
  (`:313-322`) uses `rows` too, and must keep each row of the read, so that a hidden service keeps its status runs.
  `headerText` (`:548-561`) counts `lastRead.rows` and does not change.
- `selectedAfterRead` keeps the selected name when there is no row, and finds the place of a gone name with
  `row.compare`, so the same rule places the selection when the filter hides the selected service. The learning "The
  list draws and counts its lines through linesAbove and row.compare" names a filter of the list: `lineOf` and
  `listText` must count the same rows, or the scroll and the list disagree.
- `internal/tui/screen.go:465-494`: `content` draws the header, the panes or the help, the message line and the help
  line. `messageLines` is the precedent for a line that shows only while it has a text, and `paneHeight` subtracts it,
  so the panes are one line shorter while it shows. `ansi.Truncate` with `…` cuts it at the width of the terminal.
- `internal/tui/screen.go:518-546`: `helpLine` and `helpText`. The keys in the help have a column of 10 cells
  (`helpKeyWidth`, `screen.go:26`); `backspace` is 9 cells. The help has 11 lines today plus the actions of the group.
  `keys.golden` opens 80 by 17 (`screen_test.go:4212-4218`) and `keys-with-actions.golden` 80 by 19
  (`screen_test.go:3813-3818`), each with room for exactly its lines, so each line added to the help needs a taller
  terminal in these two tests.
- `internal/tui/screen.go:563-570`: while the read gives an error, `listText` shows the lines of the error in place of
  the rows. It is the precedent for a text in the list pane with no rows, and stays first: a read error shows whatever
  the filter.
- `internal/config/action.go:14-15`: `keysOfSvcview` holds `/` today, with a comment that says that `/` is kept for
  the filter of the list. The screen test `config.Read refuses only the keys of svcview, and each other printable
  ASCII key of an action reaches that action on the screen` (`screen_test.go:3872-3891`) checks the list and the screen
  together, and stays as it is.
- Tests, all under the umbrella `TestScreen` in `internal/tui/screen_test.go`: helpers `pressKeys`, `listRows`,
  `selectedName`, `bottomLines`, `detailPane`, `openWith`, `openActing`, `openActingWith`, `groups`,
  `groupsWithActions`, `fakeLess`, `readsInTurn`, `readNow`, `changingAt`, `runFor` and `requireGolden` (with
  `-update`); the key presses at `screen_test.go:140-158` (a letter is a key press with its code and its text). Groups
  to follow: `list` (`:625-793`), `refresh` (`:795-921`), `status` (`:923-990`), `select` (`:2036-2432`), `message
  line` (`:3008-3035`), `stop a service` (`:3194-3210`), `show the actions of a group` (`:3750-3843`), and `keys`
  (`:4203-4291`). A new group `filter` holds the tests of this change.
- e2e: `e2e/tests/screen.test.ts` and `e2e/tests/start-stop.test.ts` open the screen with `openCli`, `homeWithPlists`
  and `writeConfig`; `launchctlCalls` (`e2e/testUtils.ts:89-93`) gives each call of the fake launchctl other than
  `list`. `session.type` types text, and `session.press` takes `esc`, `backspace` and `enter`
  (`e2e/node_modules/tuistory/dist/session.d.ts:26`). No e2e test presses `esc` yet. `openCli` gives svcview 160
  columns and 40 rows.
- `docs/cli.md`: the table of parts (`:70-76`), "The focus and the scroll" (`:114-135`), "Refresh" (`:137-145`),
  "Keys" (`:147-162`), "The message line" (`:257-288`), and "The key of an action" (`:813-825`), whose line `:821`
  says that svcview keeps `/` for a filter of the list.
- `tasks/learnings.md`: "Plan the layout change of the screen as the first task" (the filter line is in Task 1); "Each
  key of the screen goes into the help and docs/cli.md" (each task adds the help line of its own key); "The list draws
  and counts its lines through linesAbove and row.compare"; "A test that keys change nothing presses each key from its
  own start state"; "A screen test of a read error must select a service before the error"; "A test of a cut at a
  width needs the exact-fit width and one cell less"; "A test that a pane does not move needs a pane that can scroll";
  "A test folded into an earlier task commit uses only the behaviour of that commit" (a test of Task 1 must not assert
  the rows of the list, which Task 2 filters); "An e2e test finds the help line by a short part of it" (the help line
  of an open filter line has no `? help`); "Fold the fixes of two tasks in one file with one clerk fixup mark for each
  task in turn"; "Test a Bubble Tea timer on the fake clock of synctest"; "Run clerk mutants and clerk audit run under
  mise exec"; "task test:e2e:local leaves e2e/package-lock.json in the tree"; "Pass --integrate to clerk land when the
  request asks to land on main".

## Tasks

### Task 1: Open a filter line with / that takes each key that types text

**Behavior:** `/` opens the filter line on the line under the panes and above the message line, and moves the focus
to the list. The line shows `/`, the filter, and a cursor after the filter, and the panes are one line shorter while
it shows. While the line is open, each key that types text adds its text to the end of the filter, so no letter, no
key of svcview and no key of an action acts. `backspace` deletes the last character of the filter. `esc` clears the
filter and closes the line. `ctrl+c` quits, and each other key does nothing. The help line shows
`esc clear the filter` while the line is open. The help that `?` shows lists `/` and `backspace`, and `esc` with both
of its jobs. This task does not change which rows the list shows.

**Acceptance Criteria:**
- [x] `/` shows the filter line ` /` with a cursor after it, under the panes, and the panes end one line higher. A
      golden file shows the screen with the filter line open and a filter typed.
- [x] With the filter line open on a selected service that `s`, `x` and `r` each act on, in a group with actions,
      each key that types text adds its text to the end of the filter: each printable ASCII character from `!` to `~`
      and the space, so `h`, `j`, `k`, `l`, `q`, `?`, `/`, `s`, `x`, `r`, `o`, `y` and the key of an action too. After
      them the filter line shows `/` and each character in the order of the keys, no key returned a command, nothing
      called a start, a stop or a restart, and the help does not show.
- [x] `backspace` deletes the last character of the filter, also a character of more than one byte such as `é`. On
      an empty filter it changes nothing, and the filter line stays open.
- [x] `esc` clears the filter and hides the filter line. From a screen with the focus on the list, the screen after
      `/`, a filter that the name of the selected service contains, and `esc` shows the same as before `/`.
- [x] After `esc`, the keys act again: `j` selects the next row, and `?` shows the help.
- [x] While the filter line is open, `↑`, `↓`, `tab`, `←` and `→` change nothing: each key pressed from its own start
      state gives the same screen as that start state. `ctrl+c` ends the screen.
- [x] While the filter line is open, the help line shows ` esc clear the filter`. After `esc`, it shows the help line
      of the selected service again.
- [x] `/` moves the focus to the list: after `tab` gives the focus to the detail pane, `/` makes the border of the list
      blue and the border of the detail pane dim.
- [x] `/` while the help shows changes nothing, and shows no filter line.
- [x] While a confirmation shows, `/` answers no: the confirmation hides, nothing is called, and no filter line shows.
- [x] `/` hides the message line, as each key press does. An error of a start that ends while the filter line is open
      shows on the message line, under the filter line and above the help line, and the next key goes into the filter
      and hides it.
- [x] The filter line shows each control character of the filter as `�`.
- [x] svcview cuts a filter line that is wider than the terminal with `…`: a test at the width that fits the line
      exactly, and a test at one column less.
- [x] The help that `?` shows lists `/` with `filter the list by name` on the line after `h`, then `backspace` with
      `delete the last character of the filter` before `s`, and `esc` with `hide this help, or clear the filter`. The
      golden tests of the help open terminals tall enough for each line, and `keys.golden` and
      `keys-with-actions.golden` show them.
- [x] The comment of `keysOfSvcview` in `internal/config/action.go` says that the screen handles each of these keys,
      `/` too. The test that `config.Read` refuses only the keys of svcview passes with no change.
- [x] e2e: on a running service, `/` and then the text `proxy` show the filter line `/proxy`, the screen stays open,
      and the fake launchctl gets no call. `esc` then hides the filter line.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `Update` gives each key to the filter while the filter line is open; `press` opens the
  filter line on `/`; `content` and `paneHeight` draw the filter line; `helpLine` and `helpText` name the keys
- `internal/tui/screen_test.go` — a new group `filter` in `TestScreen`, `/` in the test that each key other than `y`
  answers no, and taller terminals in the golden tests of the help
- `internal/tui/testdata/` — a new golden file of the open filter line; `keys.golden` and `keys-with-actions.golden`
- `internal/config/action.go` — the comment of `keysOfSvcview`
- `e2e/tests/filter.test.ts` — new: the e2e test of the open filter line

**Patterns to Follow:**
- `internal/tui/screen.go:138-149`
- `internal/tui/screen.go:183-246`
- `internal/tui/screen.go:271-281`
- `internal/tui/screen.go:465-494`
- `internal/tui/screen.go:518-546`
- `internal/tui/screen_test.go:384-400`
- `internal/tui/screen_test.go:3008-3035`
- `internal/tui/screen_test.go:3194-3210`
- `internal/tui/screen_test.go:3872-3891`
- `internal/tui/screen_test.go:4203-4291`
- `e2e/tests/start-stop.test.ts:8-17`
- `e2e/tests/actions.test.ts:44-60`

**Testable:** Yes

**Certainty:** medium — the confirmation is a state that takes each key (`screen.go:144-149`, `:271-281`) and the
message line is a line that shows only while it has a text (`screen.go:477-494`), but no instance takes text that the
user types, or changes the help line with the state of the screen.

**Blast radius:** high — the open filter line is the guard in front of `s`, `x` with `y`, `r` with `y` and the keys of
actions: a key that leaks past it starts, stops or restarts the selected service, or runs the command of an action, on
a typed word.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of the open filter line.

**Depends on:** None

### Task 2: Show only the services whose name contains the filter

**Behavior:** After each key while the filter line is open, the list shows only the services whose name contains the
filter, with no regard to case. The filter compares with the full name, not with the short name that a row shows. A
group with no such service shows no group line. The selected service stays selected while the list shows it; when the
filter hides it, the row at its place in the order of the list is selected. `esc` clears the filter, and the list
shows each service again with the same service selected. Each read applies the filter. The header counts and the
status runs keep each service of the read.

**Acceptance Criteria:**
- [ ] After `/` and `WORKER`, the list shows only the services whose name contains `worker` in any case, each under
      the line of its group, and no line `Other` when no service of `Other` contains it.
- [ ] The list follows each key: after `/`, `w` and `o`, the list shows the services whose name contains `wo`; after
      `backspace`, the services whose name contains `w`.
- [ ] The filter compares with the full name of the service: `example` shows each row of a group whose match is
      `^com\.example\.`, although the short names that the rows show do not contain it.
- [ ] A group with no service whose name contains the filter shows no group line, and no empty line before the next
      group; `Other` too.
- [ ] An empty filter, after `/` alone or after `backspace` deletes each character, shows each service.
- [ ] The selected service stays selected while the list shows it. When the filter hides it, the row that takes its
      place in the order of the list is selected: the row after it, also in the next group, or the last row when it
      came after each row that the list shows. The detail pane shows the newly selected service from its first line.
- [ ] After a filter hides rows of a list that had scrolled, the list shows from its first line when the rows that it
      shows fit in the pane, and the selected row stays in view in a filtered list that is longer than the pane.
- [ ] `esc` clears the filter: the list shows each service again, and the service that was selected in the filtered
      list stays selected.
- [ ] A refresh applies the filter to the new read: a service that is added and whose name contains the filter shows
      no later than 2 s after the change, and a service whose name does not contain it stays out of the list.
- [ ] The header counts each service of the read, also each service that the filter hides.
- [ ] A service of a group with a status command that the filter hides keeps its status runs, and its row shows its
      newest summary as soon as `esc` shows it again.
- [ ] e2e: with three plists and a config with the group `Workers`, `/` and `WORKER` show the rows of `Workers` and no
      line `Other`; `esc` shows each service again.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the rows that the list shows; the selection after each key that changes the filter; the
  status runs keep each row of the read
- `internal/tui/screen_test.go` — the tests of the filtered list in the group `filter`
- `internal/tui/testdata/` — a new golden file of a filtered list in groups
- `e2e/tests/filter.test.ts` — the e2e test of the filtered list

**Patterns to Follow:**
- `internal/tui/screen.go:313-322`
- `internal/tui/screen.go:364-381`
- `internal/tui/screen.go:392-404`
- `internal/tui/screen.go:441-463`
- `internal/tui/screen.go:548-561`
- `internal/tui/screen_test.go:581-590`
- `internal/tui/screen_test.go:632-667`
- `internal/tui/screen_test.go:796-804`
- `internal/tui/screen_test.go:924-990`
- `internal/tui/screen_test.go:2093-2110`
- `internal/tui/screen_test.go:2144-2155`
- `internal/tui/screen_test.go:2390-2412`
- `task:1`

**Testable:** Yes

**Certainty:** medium — `selectedAfterRead` places the selection at the place of a gone name with `row.compare`
(`screen.go:371-381`) and `linesAbove` draws a group line only above a row of the group (`screen.go:446-455`), but no
instance shows rows other than the rows of the read, while the status runs and the header counts keep each row.

**Blast radius:** low — it changes which rows the list shows and which row is selected while the filter line is open,
when no key acts on the selected service.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of the filtered list.

**Depends on:** Task 1

### Task 3: Show no service matches when the filter hides each service

**Behavior:** When the filter has text and the name of no service of the read contains it, the list pane shows the
one line `no service matches`, and the detail pane shows no service. A read error shows the error whatever the filter,
and with no filter, a read with no services shows an empty list, as today.

**Acceptance Criteria:**
- [ ] With a filter that the name of no service of the read contains, the list pane shows the one line
      `no service matches`, the detail pane shows no title and no text, and the help line shows no key of an action. A
      golden file shows it.
- [ ] With no services and a filter, the list shows `no service matches`; with no services and no filter, the list
      shows no line, as `no-services.golden` shows.
- [ ] While the list shows a read error, after a read that selected a service, it shows the error and not
      `no service matches`, whatever the filter.
- [ ] `backspace` that makes the filter contained in a name again shows the rows, and a row is selected.
- [ ] A read that adds a service whose name contains the filter shows its row in place of `no service matches`.
- [ ] e2e: `/` and `zzz` show `no service matches`; `backspace` three times shows the rows again.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `listText` shows `no service matches`
- `internal/tui/screen_test.go` — the tests of `no service matches` in the group `filter`
- `internal/tui/testdata/` — a new golden file of `no service matches`
- `e2e/tests/filter.test.ts` — the e2e test of `no service matches`

**Patterns to Follow:**
- `internal/tui/screen.go:563-570`
- `internal/tui/screen_test.go:766-779`
- `internal/tui/screen_test.go:2955-2967`
- `internal/tui/screen_test.go:3791-3801`
- `task:2`

**Testable:** Yes

**Certainty:** high — the list shows the text of a read error in place of the rows at `screen.go:563-570`, with its
golden test at `screen_test.go:772-779`.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of `no service matches`.

**Depends on:** Task 2

### Task 4: Keep the filter with enter

**Behavior:** `enter` keeps the filter: the filter line stays with `/` and the filter and no cursor, and the keys act
again on the filtered list. `↑`, `↓`, `j` and `k` select a service among the rows that the list shows, and `s`, `x`,
`r`, `o` and the keys of actions act on the selected service, which is always a row that the list shows. The help
line shows the keys of svcview and of the actions of the selected service again. `/` opens the line again with the
kept filter. `esc` clears a kept filter; while the help shows, `esc` hides only the help. `enter` on an empty filter
closes the line. While the line is open, the help line shows `enter keep the filter` before `esc clear the filter`,
and the help that `?` shows lists `enter`.

**Acceptance Criteria:**
- [ ] `enter` keeps the filter: the filter line shows `/` and the filter with no cursor, and the list shows the same
      rows as before `enter`. A golden file shows the kept filter.
- [ ] After `enter`, `↓` and `j` select the next row that the list shows, also in the next group, and `↑` and `k` the
      row before it. `↓` at the last row keeps it selected.
- [ ] After `enter`, the help line shows the keys of svcview and the keys of the actions of the group of the selected
      service, as with no filter.
- [ ] After `enter`, `s`, `x` with `y`, `r` with `y`, `o` and the key of an action act on the selected service, which
      is a row that the list shows, and not on the service that was selected before `/` when the filter hides it.
- [ ] With a kept filter that the name of no service contains, `s`, `x`, `r`, `o` and the key of an action call
      nothing, show no confirmation and run nothing, also after a read that selected a service before the filter hid
      it.
- [ ] `/` with a kept filter opens the filter line with the kept filter and the cursor after it, and the next key adds
      its text to the end of it.
- [ ] `esc` with a kept filter clears it: the filter line hides, the panes are one line taller, the list shows each
      service, and the selected service stays selected.
- [ ] While the help shows over a kept filter, `esc` hides the help and keeps the filter, and the next `esc` clears
      it.
- [ ] `enter` on an empty filter line closes it: no filter line shows, and the list shows each service.
- [ ] `enter` while the filter line is not open changes nothing.
- [ ] A kept filter applies to each read: a service that is added and whose name contains the filter shows at the
      next read, and a service whose name does not contain it stays out of the list.
- [ ] While the filter line is open, the help line shows ` enter keep the filter · esc clear the filter`.
- [ ] The help that `?` shows lists `enter` with `keep the filter` on the line between `/` and `backspace`, and
      `keys.golden` and `keys-with-actions.golden` show it.
- [ ] e2e: with three not loaded services, `/`, a filter that two of them contain, `enter` and `↓` select the second
      of them, and `s` runs launchctl bootstrap with the plist of that service; `esc` then shows each service again.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `enter` keeps the filter; `/` opens a kept filter again; `esc` clears a kept filter;
  `helpLine` and `helpText` name `enter`
- `internal/tui/screen_test.go` — the tests of a kept filter in the group `filter`, and a taller terminal in the golden
  tests of the help
- `internal/tui/testdata/` — a new golden file of a kept filter; `keys.golden` and `keys-with-actions.golden`
- `e2e/tests/filter.test.ts` — the e2e test of a kept filter

**Patterns to Follow:**
- `internal/tui/screen.go:188-191`
- `internal/tui/screen.go:204-229`
- `internal/tui/screen.go:230-241`
- `internal/tui/screen.go:427-435`
- `internal/tui/screen_test.go:2051-2061`
- `internal/tui/screen_test.go:2955-2967`
- `internal/tui/screen_test.go:3750-3800`
- `internal/tui/screen_test.go:4220-4246`
- `e2e/tests/start-stop.test.ts:8-17`
- `task:3`

**Testable:** Yes

**Certainty:** high — `?` and `esc` show and hide the help with the same kind of state at `screen.go:188-191`, `↑`
and `↓` select through `rows` at `screen.go:230-241`, and the keys that act take the selected service through
`serviceForKeys` at `screen.go:427-435`, each with its tests in the groups `keys`, `select` and `start a service`.

**Blast radius:** high — after `enter`, `s`, `x`, `r` and the keys of actions act on the selection that the filter
decides: a mistake starts, stops or restarts a service that the list does not show.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of a kept filter.

**Depends on:** Task 3

### Task 5: Describe the filter in docs/cli.md

**Behavior:** `docs/cli.md` tells a user how to filter the list by name: what `/` does, which keys go into the filter,
what `backspace`, `enter` and `esc` do, how the list, the group lines and the selected service follow the filter,
`no service matches`, where the filter line shows, the help line while the line is open, and that the header counts
and the status runs keep each service.

**Acceptance Criteria:**
- [ ] A section "The filter" under "The screen" describes `/`, the filter line and where it shows, the keys that go
      into the filter, `backspace`, `enter`, `esc`, the help line while the line is open, the list and the selected
      service after each key, `no service matches`, the header counts, and the status runs of a hidden service.
- [ ] The table of parts has a row for the filter line, the row of the list says that a filter hides rows, and the row
      of the help line gives the help line of an open filter line.
- [ ] The table of "Keys" has rows for `/`, `enter` and `backspace`, and the row of `esc` says that it hides the help
      or clears the filter, as the help that `?` shows does.
- [ ] "The key of an action" says that `/` opens the filter line, in place of a key that svcview keeps for a filter.
- [ ] "Refresh" says that a kept filter applies to each read.
- [ ] Each statement matches what Tasks 1 to 4 built.

**Affected Files/Modules:**
- `docs/cli.md` — a section on the filter, the table of parts, "Keys", "Refresh" and "The key of an action"

**Patterns to Follow:**
- `docs/cli.md:70-76`
- `docs/cli.md:114-135`
- `docs/cli.md:137-145`
- `docs/cli.md:147-162`
- `docs/cli.md:813-825`

**Testable:** No

**Certainty:** high — "The focus and the scroll" and "Keys" at `docs/cli.md:114-162` describe the keys of the screen in
the same way.

**Blast radius:** low

**Verification:** Manual check of `docs/cli.md` against the tests of Tasks 1 to 4.

**Depends on:** Task 4

## Summary

- 5 tasks.
- Order: the filter line and the keys that go into it first (Task 1), because it is the layout change of the screen
  and the guard in front of each key that acts; then the list that follows the filter (Task 2); then the text of a
  filter that hides each service (Task 3); then `enter`, which lets the keys act again on the filtered list (Task 4);
  then the docs (Task 5). Each key goes into the help in the task that adds it, so the help lists each key that the
  screen handles after each task.
- Golden files: a new one for the open filter line (Task 1), a filtered list in groups (Task 2), `no service matches`
  (Task 3) and a kept filter (Task 4). `keys.golden` and `keys-with-actions.golden` change in Tasks 1 and 4. No other
  golden file changes, because the filter line shows only while a filter is open or kept.
- Criteria: the first (`/` and the list after each key) in Tasks 1 and 2; the second (a group with no service) in
  Task 2; the third (`enter`, `↑` and `↓`) in Task 4; the fourth (`esc` clears) in Task 2 for an open filter line and
  Task 4 for a kept filter; the fifth (each letter goes into the filter) in Task 1; the sixth (`no service matches`)
  in Task 3. The docs of all six are in Task 5. None is deferred.
- Tasks 1 and 4 are high blast radius: the first decides whether a key types text or acts on launchd, and the second
  lets the keys act on the selection that the filter decides. The tests of both record each start, stop and restart.
