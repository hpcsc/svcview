## Contents
1. Task 1: Show all after the domain while the screen shows all services
2. Task 2: Change between all services and the services with a plist with a
3. Task 3: Read the services that a asks for at once
4. Task 4: Describe a in docs/cli.md and the README

## Story Reference

Story: `user-stories/launchd-tui.md`, US-017 "Show all services with one key" (`launchd-tui/US-017`).
Proposal: `docs/proposal-launchd-tui.md`, "Keys" (the row of `a`) and "Data From launchd" (the paragraph that starts
"By default, svcview shows only the services that have a plist").

A developer checks a service of macOS with no restart of svcview. `a` changes the list between each service that
`launchctl list` prints, as `--all` shows, and only the services that have a plist in `~/Library/LaunchAgents`. The
screen reads the new services at once, so they show as soon as `launchctl list` and `ps` end, with no wait for the
next refresh. `--all` still sets the services that svcview shows at start. While all services show, the header shows
`all` after the domain, for example `gui/501 · all`, so the screen always says which services the list shows. The
selected service stays selected when the new list shows it, and the row at its place is selected when it does not. A
kept filter stays and applies to the new list, and while the filter line is open `a` goes into the filter. `a` is a
key of svcview: a config file with an action on `a` stops svcview at start, the help that `?` shows lists `a`, and
`docs/cli.md` describes it.

The story criteria that this change satisfies:

- `a` shows each service that `launchctl list` prints, as `--all` does. `a` again shows only the services that have a
  plist in `~/Library/LaunchAgents`.
- `--all` sets the list that svcview shows at start. Without `--all`, svcview starts with the services that have a
  plist.
- While all services show, the header shows `all` after the domain, for example `gui/501 · all`.
- After `a`, the selected service stays selected when the new list shows it. When the new list does not show it, the
  row at its place in the list is selected.
- A kept filter stays after `a`, and applies to the new list. While the filter line is open, `a` goes into the filter,
  as each other letter does.
- `a` is a key of svcview: a config file with an action on `a` stops svcview at start. The help that `?` shows lists
  `a`, and `docs/cli.md` describes it.

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
- No setting of a config file sets the services that svcview shows at start, and svcview keeps no choice of `a`
  across starts: each start shows the services that `--all` gives.
- `--all` does not change: the same usage, no short form, and a flag of the screen only. `svcview status` finds only a
  service that has a plist, as before.
- `A` in upper case is not a key of svcview, and an action can take it.
- The help line at the bottom of the screen does not show `a`. It shows the keys of svcview and of the actions, as
  before; only the help that `?` shows lists `a`.
- No glyph, color or text in the list marks a service with no plist. Only the header says which services the list
  shows, and the header counts each service of the read, of all services too.
- The keys and the confirmations do not change. `s`, `x`, `r`, `o` and the keys of the actions act on a service with
  no plist as on any other service in its state, and `x` asks `Stop <name>? (y/N)` for each service. No key refuses a
  service with no plist.
- `a` starts no status run. A service that `a` adds gets its first run at the next `every` of its group, as a service
  that a refresh adds. svcview keeps the statuses that it has: a service that `a` shows again shows the status of its
  last run until its group runs again.
- `a` does not change the filter, the focus, or the setting `refresh`.
- No layout for a narrow terminal, and no change for `NO_COLOR`. With the mark, the header keeps the rule that cuts
  `svcview` first and does not cut the domain.

**Deferred:** None.

## Codebase Context

- `internal/cli/root.go:72-92`: `showScreen` picks one read, `domain.AllServices` when `--all` is set and
  `domain.Services` otherwise (`:77-80`), and gives it to `tui.NewScreen` (`:82`). `internal/launchd/domain.go:12-14`
  says that callers must not call `Services` or `AllServices` of one `Domain` from two goroutines at the same time:
  both read the plists through one `plistFolder` that keeps what it read (`internal/launchd/plist_folder.go:10-45`).
  The Go tests have no terminal, so only the e2e tests reach `showScreen` (tasks/learnings.md, "Only e2e tests reach
  the terminal branch of the root command").
- `internal/tui/screen.go:125-139`: `NewScreen` takes the domain, one read, the status run, the start, the stop, the
  restart and the settings. `screen_test.go` calls it 134 times; tasks/learnings.md, "Add a parameter to
  tui.NewScreen with gofmt -r", gives the rewrite.
- `internal/tui/screen.go:141-191`: `Init` returns `readServices`; `Update` stores each `servicesRead` in `lastRead`,
  asks for one `refresh` tick after it (`:163`), starts the status runs at the first read (`:161-166`), and starts a
  read at each `refresh` (`:181-182`). So today at most one read runs, and one tick or one read is waiting at any
  time. The test `does not start a read while the previous read runs` (`screen_test.go:918-937`) checks the first.
  `readServices` (`:325-333`) calls `s.read` and sorts the rows with `row.compare`.
- `internal/tui/screen.go:193-264`: `press` handles the keys; `/` (`:218-221`) is the latest key of svcview, and does
  nothing while the help shows. `answer` (`:289-299`) makes each key other than `y` and `ctrl+c` answer no, and
  `pressInFilterLine` (`:301-317`) adds the text of each other key to the filter, so `a` reaches neither `press` nor
  an action while a confirmation shows or the filter line is open. Each other key goes to `runAction` (`:260-261`).
- `internal/tui/screen.go:407-427`: `rowsInList` applies the filter to the rows of the read, and `selectedInList`, which
  `Update` calls after each message (`:184-189`), keeps the selected name or selects the row at its place by
  `row.compare`. A read of other services therefore keeps or places the selection, and keeps the filter, by the rules
  that a refresh uses today (`screen_test.go:2161-2172`, `:4880-4891`).
- `internal/tui/screen.go:511-526` and `internal/tui/box.go:35-46`: the header is a `box` whose `right` is the domain.
  `box.top` cuts the title `svcview` with `…` and never cuts `right`. `screen_test.go:619-631` checks the top border
  at each width from 13 to 30 with `gui/501`; ` gui/501 · all ` needs 6 columns more, so the same rule fits it from 19
  columns. `headerText` (`:612-625`) shows no counts before the first read ends or on a read error (tasks/learnings.md,
  "Show no data of a read before the first read ends").
- `internal/tui/screen.go:587-610`: `helpText`; the keys have a column of 10 cells. `keys.golden` opens 80 by 20
  (`screen_test.go:4229-4235`) and `keys-with-actions.golden` 80 by 22 (`screen_test.go:3830-3836`), each with room for
  exactly its lines, so the line of `a` needs one more row in each.
- `internal/config/action.go:14-15`: `keysOfSvcview` holds the keys that no action can take, and the error lists them.
  The screen test `config.Read refuses only the keys of svcview, and each other printable ASCII key of an action
  reaches that action on the screen` (`screen_test.go:3889-3907`) fails when the list and the keys of the screen
  differ, so `a` goes into both in one task. `internal/config/config_test.go:850-860` lists each key of svcview, and
  `config_test.go:872-897` reads an action on `a` and one on `A` as two keys that differ only in case.
- No e2e test and no fixture has an action on `a`. `screen_test.go:4331-4355` types each printable ASCII key into the
  open filter line, `a` too. The tests that each key other than `y` and `ctrl+c` answers no are at
  `screen_test.go:3211-3227` (stop), `:3419` (restart) and `:4157` (the action of a group).
- Test helpers in `internal/tui/screen_test.go`: `open`, `openWith`, `openIn` (`:92-110`), `servicesNamed`,
  `readGiving`, `changingAt`, `readsInTurn`, `readNow` (`:76-90`, `:238-274`), `runFor` and `runSized` (`:277-303`),
  `readTimes` (`:366-389`), `serviceCalls`, `openActing`, `bottomLines`, `listRows`, `selectedName`, `requireGolden`.
  `readNow` sends the command of `Init`, which reads the services that the screen asks for.
- e2e: `e2e/tests/all-services.test.ts:14-45` builds a home with two plists and a launchctl list that also prints
  `com.apple.a` and `com.apple.b`; `openCli` gives 160 columns and 40 rows; `launchctlCalls` gives each call other
  than `list`. `e2e/tests/screen.test.ts:15-27` finds `gui/<uid>` in the header. The setting `refresh` takes a Go
  duration, for example `1m` (`docs/cli.md:441`).
- `docs/cli.md`: "The screen" (`:35-50`) says today that the screen shows no mark that `--all` is on; the table of
  parts (`:78-85`); "A service with no plist" (`:92-101`); "The filter" (`:159-228`), whose `:175-176` lists the keys
  of svcview that go into the filter; "Refresh" (`:229-239`); "Keys" (`:241-262`); "The key of an action"
  (`:934-946`), whose `:941` lists the keys of svcview. `README.md:3-5` names `--all`. The status runs of a service
  that a read adds (`docs/cli.md:553-560`).
- `tasks/learnings.md`: "Test a Bubble Tea timer on the fake clock of synctest"; "Plan each Bubble Tea message and
  private type in design.yaml"; "Each key of the screen goes into the help and docs/cli.md"; "A test that keys change
  nothing presses each key from its own start state"; "A test of a cut at a width needs the exact-fit width and one
  cell less"; "A test folded into an earlier task commit uses only the behaviour of that commit" (the tests of Task 2
  check the screen after the next read, which Task 3 does not change); "A boundary that turns off a check must name
  the code that has the check today" (`docs/cli.md:48` and the boundary of US-013 say that the screen shows no mark);
  "An e2e test finds the help line by a short part of it"; "Run clerk mutants and clerk audit run under mise exec";
  "task test:e2e:local leaves e2e/package-lock.json in the tree"; "Pass --integrate to clerk land when the request
  asks to land on main".

## Tasks

### Task 1: Show all after the domain while the screen shows all services

**Behavior:** The screen gets both reads of the domain, the read of the services with a plist and the read of all
services, and which of the two it asks for at start. `svcview --all` starts it with all services, and `svcview` with
the services with a plist. The screen calls only the read that it asks for. While the rows on the screen come from a
read of all services, the header shows `· all` after the domain, for example `gui/501 · all`. No key changes.

**Acceptance Criteria:**
- [ ] A screen that starts with all services calls only the read of all services, at start and at each refresh, and
      shows its rows. A screen that starts with the services with a plist calls only the read of the services with a
      plist, as before. The test records the calls of each read.
- [ ] While the rows on the screen come from a read of all services, the top border of the header shows
      `gui/501 · all` at its right, where it shows `gui/501` with the services with a plist. A new golden file shows
      the header with the mark and the counts of a read of all services.
- [ ] With the services with a plist, the header shows the domain alone, and each golden file in
      `internal/tui/testdata` stays as it is.
- [ ] Before the first read ends, the header shows the domain alone, also on a screen that starts with all services.
- [ ] A read of all services that gives an error shows the error in the list, no counts, and the mark in the header.
- [ ] With the mark, svcview cuts `svcview` with `…` first and cuts neither the domain nor `all`: the top border of
      the header is as wide as the terminal at each width from 19 to 30 columns, and at 25 columns it is
      `╭ svcvi…  gui/501 · all ╮`.
- [ ] `showScreen` gives the screen `Domain.Services`, `Domain.AllServices`, and whether `--all` is set, in place of
      one read. `svcview status` does not change.
- [ ] e2e: `svcview --all` shows `gui/<uid> · all` in the header. `svcview` with no flag, on the same plists and the
      same launchctl list, shows `gui/<uid>` and no `· all`. The other tests of `e2e/tests/all-services.test.ts` pass
      as they are.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `NewScreen` takes both reads and which one the screen asks for; the message of a read
  says which services it read; `readServices` calls the read that the screen asks for; the header shows the mark
- `internal/tui/screen_test.go` — each call of `tui.NewScreen` gets the new arguments; new tests in the groups `start`
  and `header`
- `internal/tui/testdata/` — a new golden file of the header with the mark
- `internal/cli/root.go` — `showScreen` gives the screen both reads and `--all`
- `e2e/tests/all-services.test.ts` — the mark with `--all` and with no flag

**Patterns to Follow:**
- `internal/tui/screen.go:125-143`
- `internal/tui/screen.go:325-333`
- `internal/tui/screen.go:511-526`
- `internal/tui/screen.go:612-625`
- `internal/tui/box.go:35-46`
- `internal/cli/root.go:72-92`
- `internal/tui/screen_test.go:366-389`
- `internal/tui/screen_test.go:589-640`
- `e2e/tests/all-services.test.ts:24-45`
- `e2e/tests/screen.test.ts:15-27`

**Testable:** Yes

**Certainty:** medium — `showScreen` already picks the read (`internal/cli/root.go:77-80`) and `NewScreen` took a new
parameter before (tasks/learnings.md, "Add a parameter to tui.NewScreen with gofmt -r"), but no instance gives the
screen two reads, or ties a part of the header to which read the rows on the screen came from.

**Blast radius:** high — it decides which read the screen calls at start: a mistake shows each service of macOS with
no `--all`, and `x` on a service with no plist runs `launchctl bootout`, which svcview cannot undo until the next
login.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the tests of `e2e/tests/all-services.test.ts`.

**Depends on:** None

### Task 2: Change between all services and the services with a plist with a

**Behavior:** `a` changes the services that the screen asks for, from the services with a plist to all services and
back, and the next read reads them: the list then shows each service that `launchctl list` prints, with the mark, or
only the services with a plist, with no mark. The selected service stays selected when the new rows show it, and the
row at its place in the list is selected when they do not. A kept filter stays and applies to the new rows. While the
filter line is open, `a` goes into the filter. While the help shows, `a` does nothing, and while a confirmation shows,
`a` answers no. `a` is a key of svcview: `config.Read` refuses an action on `a`, so a config file with one stops
svcview at start, and the help that `?` shows lists `a`.

**Acceptance Criteria:**
- [ ] On a screen that starts with the services with a plist, `a` and the next read show each service of the read of
      all services, and the mark. `a` again and the next read show only the services with a plist, and no mark. A
      screen that starts with all services goes the other way. The tests check the screen after the next read, not
      between `a` and that read.
- [ ] After `a` and the next read, the selected service stays selected when the new rows show it, also when new rows
      come before it. When they do not show it, the row that takes its place in the order of the list is selected:
      the row after it, also in the next group, or the last row when it came after each row. The detail pane of a
      newly selected service shows from its first line.
- [ ] After `a` from a long list of all services that has scrolled, the selected row of the services with a plist is
      in view, and a list of them that fits in the pane shows from its first line.
- [ ] A kept filter stays after `a` and the next read: the filter line shows the same filter, the list shows only the
      new rows whose name contains it, and `no service matches` shows when no new row does.
- [ ] While the filter line is open, `a` adds `a` to the end of the filter, and the screen never calls the read of the
      other services. The test that each key that types text goes into the filter checks that no read of the other
      services happens.
- [ ] While the help shows, `a` changes nothing: after the next read and `?`, the list shows the services that it
      showed before `a`, with no mark.
- [ ] While a confirmation shows, `a` answers no: it hides the confirmation, calls nothing, and changes nothing else.
      `a` is in each test that each key other than `y` and `ctrl+c` answers no.
- [ ] `keysOfSvcview` holds `a`. `config.Read` refuses an action on `a` with the error that names the file, the group,
      the action and the key, and lists `a` among the keys that an action cannot take. An action can take `A`. The
      screen test that `config.Read` refuses only the keys of svcview refuses `a` too, and the config test of two keys
      that differ only in case uses a letter that is not a key of svcview.
- [ ] The help that `?` shows lists `a` with `show all services, or only the services with a plist`, on the line after
      `backspace` and before `s`. The golden tests of the help open terminals one line taller, and `keys.golden` and
      `keys-with-actions.golden` show the line.
- [ ] The help line at the bottom of the screen does not change.
- [ ] e2e: with the plists of `com.example.worker` and `com.example.not-loaded` and a launchctl list that also prints
      `com.apple.a`, `a` shows `com.apple.a` and `gui/<uid> · all`, and `a` again hides both. The fake launchctl gets
      no call other than `list`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `press` changes the services that the screen asks for on `a`; `helpText` lists `a`
- `internal/tui/screen_test.go` — a new group `all services` in `TestScreen`; `a` in the tests that each key answers
  no and in the test of the keys that type text into the filter; the refused keys; taller terminals in the golden
  tests of the help
- `internal/tui/testdata/` — `keys.golden` and `keys-with-actions.golden`
- `internal/config/action.go` — `keysOfSvcview` holds `a`
- `internal/config/config_test.go` — the keys of svcview in the test of the refusal; another letter in the test of two
  keys that differ only in case
- `e2e/tests/all-services.test.ts` — `a` and `a` again

**Patterns to Follow:**
- `internal/tui/screen.go:193-264`
- `internal/tui/screen.go:301-317`
- `internal/tui/screen.go:407-427`
- `internal/tui/screen.go:587-610`
- `internal/config/action.go:14-15`
- `internal/config/config_test.go:850-860`
- `internal/config/config_test.go:872-897`
- `internal/tui/screen_test.go:2161-2172`
- `internal/tui/screen_test.go:3211-3227`
- `internal/tui/screen_test.go:3889-3907`
- `internal/tui/screen_test.go:4229-4235`
- `internal/tui/screen_test.go:4331-4355`
- `internal/tui/screen_test.go:4880-4891`
- `e2e/tests/filter.test.ts:8-27`
- `task:1`

**Testable:** Yes

**Certainty:** high — `/` became a key of svcview the same way: a case of `press` (`internal/tui/screen.go:218-221`), a
line of `helpText` (`:593`), `keysOfSvcview` (`internal/config/action.go:15`) and the test that ties them
(`screen_test.go:3889-3907`); the selection and the filter after a read follow `selectedInList` and `rowsInList`
(`screen.go:407-427`).

**Blast radius:** high — a config file with an action on `a`, also a file of `conf.d` that another program wrote, now
stops svcview at start; and `a` puts each service of macOS under `s`, `x` and `r` with one key.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the tests of `a` in
`e2e/tests/all-services.test.ts`.

**Depends on:** Task 1

### Task 3: Read the services that a asks for at once

**Behavior:** `a` reads the services that the screen asks for at once, so the new rows and the mark show as soon as
the read ends, with no wait for the refresh. One read runs at a time: when a read runs at `a`, the screen reads the
services that it asks for as soon as that read ends, and the rows of that read do not show. One `refresh` after each
read ends, also after `a`, so each `a` adds at most one read and the screen keeps one chain of refreshes.

**Acceptance Criteria:**
- [ ] With no read running, `a` starts the read of the services that the screen asks for at once: with a refresh of
      2 s, the rows of all services and the mark show 100 ms after `a`, and with a refresh of 1 m too. `a` again shows
      the services with a plist at once in the same way.
- [ ] While a read runs (a read that takes 3 s), `a` starts no second read. When the read that runs ends, its rows do
      not show, the header keeps the mark of the rows on the screen, and the read of the services that `a` asks for
      starts at once.
- [ ] `a` twice while a read runs: the rows of the read that runs show when it ends, and no other read starts before
      the next refresh.
- [ ] No two reads run at the same time, whatever the time of `a`: with reads that take 3 s and `a` pressed at several
      times over 20 s, at most one read runs at any time, and the screen reads more than three times.
- [ ] Each `a` adds at most one read: over 10 s with a refresh of 2 s, a screen with one `a` reads at most one time
      more than the same screen with no `a`. After `a`, a service that a later read adds shows no later than 2 s after
      the read before it.
- [ ] `a` before the first read ends: the rows of the first read do not show, the list shows the services that `a`
      asks for, and each group with a status command runs its command once for each service of that read and then
      once in each `every`, with no second chain of status runs.
- [ ] A read after `a` that gives an error shows the error in the list and the mark of that read, and the next read
      comes one `refresh` after it.
- [ ] e2e: with `refresh: 1m` in the config, `a` shows `com.apple.a` and `gui/<uid> · all` long before the next
      refresh, and `a` again hides both.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `Update` starts the read at `a`, keeps one read at a time and one chain of refreshes, and
  drops the rows of a read of the services that the screen no longer asks for
- `internal/tui/screen_test.go` — the tests of the read at once in the group `all services`, on the fake clock of
  synctest
- `e2e/tests/all-services.test.ts` — `a` with `refresh: 1m`

**Patterns to Follow:**
- `internal/tui/screen.go:145-191`
- `internal/tui/screen.go:335-343`
- `internal/tui/screen.go:367-375`
- `internal/tui/screen_test.go:238-255`
- `internal/tui/screen_test.go:277-303`
- `internal/tui/screen_test.go:366-389`
- `internal/tui/screen_test.go:918-937`
- `task:2`

**Testable:** Yes

**Certainty:** low — the design is what this task decides: no instance starts a read outside the chain of refreshes,
and a tick of Bubble Tea cannot be stopped, so the task decides how one read at a time and one chain of refreshes hold
when a key starts a read, and how `Update` tells a read that ended after `a` from the read that `a` asked for.

**Blast radius:** low — it changes when a read runs, not which services the screen asks for or which service a key
acts on; the mark follows the rows on the screen, so the header and the list cannot disagree.

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of `a` with `refresh: 1m`.

**Depends on:** Task 2

### Task 4: Describe a in docs/cli.md and the README

**Behavior:** `docs/cli.md` tells a user what `a` does: it changes the list between all services and the services
with a plist, the screen reads them at once, the header shows `· all` after the domain while all services show,
`--all` sets the services at start, the selection and a kept filter follow the rules of a refresh, `a` goes into an
open filter line, and an action cannot take `a`. `README.md` names `a` beside `--all`.

**Acceptance Criteria:**
- [ ] A section "All services" under "The screen" describes `a`, the mark with the example `gui/501 · all`, the read at
      once, `--all` at start, the selection and a kept filter after `a`, `a` while the help, a confirmation or the
      open filter line shows, and the services that `a` adds to a group with a status command.
- [ ] "The screen" says that `--all` sets the services that svcview shows at start and that `a` changes them while it
      runs, in place of the sentence that the screen shows no mark that `--all` is on.
- [ ] The row of the header in the table of parts names the mark.
- [ ] "Keys" has a row for `a`, as the help that `?` shows does.
- [ ] "Refresh" says that `a` reads the services at once, and that the next read comes one `refresh` after that read
      ends.
- [ ] "The filter" names `a` among the keys of svcview that go into the filter.
- [ ] "The key of an action" lists `a` among the keys of svcview.
- [ ] "A service with no plist" says that `a` shows these services too.
- [ ] `README.md` says that `a` changes the list between the two while the screen is open.
- [ ] Each statement matches what Tasks 1 to 3 built.

**Affected Files/Modules:**
- `docs/cli.md` — a section on `a`; "The screen", the table of parts, "A service with no plist", "The filter",
  "Refresh", "Keys" and "The key of an action"
- `README.md` — the first paragraph

**Patterns to Follow:**
- `docs/cli.md:35-50`
- `docs/cli.md:78-85`
- `docs/cli.md:92-101`
- `docs/cli.md:159-228`
- `docs/cli.md:229-239`
- `docs/cli.md:241-262`
- `docs/cli.md:934-946`
- `README.md:1-5`

**Testable:** No

**Certainty:** high — "The filter" and "Keys" at `docs/cli.md:159-262` describe a key of the screen in the same way.

**Blast radius:** low

**Verification:** Manual check of `docs/cli.md` and `README.md` against the tests of Tasks 1 to 3.

**Depends on:** Task 3

## Summary

- 4 tasks.
- Order: Task 1 first, because it changes the parameters of `tui.NewScreen` and so each of its 134 calls in
  `screen_test.go`; a later fix then does not conflict with that rewrite. It also gives the header the mark that the
  rest of the story shows. Task 2 adds the key, which changes the services that the next read reads, with each rule
  that the key must keep: the selection, the filter, the help, the confirmation, the config and the help that `?`
  shows. Task 3 then makes the read happen at once, the only part with no precedent, on its own so that its rules
  have their own tests and review. Task 4 describes it all.
- The screen reads the new services at once after `a` (Task 3), not at the next refresh. Task 2 alone reads them at
  the next refresh, and its tests check only the screen after that read, so Task 3 does not rewrite them.
- Golden files: one new golden file of the header with the mark (Task 1); `keys.golden` and `keys-with-actions.golden`
  change in Task 2. No other golden file changes, because the mark shows only with all services.
- Criteria: the first (`a` and `a` again) in Task 2, at once in Task 3; the second (`--all` at start) in Task 1; the
  third (the mark) in Task 1; the fourth (the selection) in Task 2; the fifth (the filter) in Task 2; the sixth (a key
  of svcview, the help and `docs/cli.md`) in Task 2 and Task 4. None is deferred.
- Tasks 1 and 2 are high blast radius: the first decides which read the screen starts with, and the second puts each
  service of macOS under `x` with one key and refuses a config file that worked before. Task 3 is low certainty: how
  the read at once keeps one read at a time and one chain of refreshes is its own design.
