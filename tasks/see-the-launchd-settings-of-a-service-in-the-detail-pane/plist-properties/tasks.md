## Contents
1. Task 1: Read the program, the work folder, the log paths and the names of the environment as properties of a service
2. Task 2: Show the plist path and the properties of a service in the detail pane
3. Task 3: Continue a long part of a value on the next lines of the detail pane
4. Task 4: Show run at load and keep alive with each of its conditions
5. Task 5: Show the start interval, the start times, the watch paths and the throttle
6. Task 6: Describe the properties of the plist in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-019 "See the launchd settings of a service in the detail pane"
(`launchd-tui/US-019`).
Proposal: `docs/proposal-launchd-tui.md`, "The Screen" (the detail pane holds the data from launchd, the settings of the
plist, the fields of the status and its events) and "Data From launchd" (the properties `ProgramArguments`,
`EnvironmentVariables`, `StandardOutPath`, `StandardErrorPath`, `RunAtLoad`, `KeepAlive`, `StartInterval`,
`StartCalendarInterval`, `WatchPaths` and `ThrottleInterval` of the plist; svcview reads a plist again only when the
modification time of the file changes; svcview does not parse `launchctl print`).

A developer sees in the detail pane what the plist of a service tells launchd, so that they know why launchd starts the
service or leaves it stopped, with no read of the plist. For a service with a plist, the detail pane shows, after the
last line of the status (or after the data from launchd when no line of the status shows), an empty line, `Plist` with
the plist path, and then one line for each property that the plist has: its name in plain words in the first column,
and its value in the column of `PID` and `last exit`, with each part of a list on a line of its own. The conditions of
`KeepAlive` show as plain phrases in the words of the screen, for example `keep alive    when the last exit is 0`. The
names of `EnvironmentVariables` show, and never their values. A service with no plist shows none of these lines.
svcview reads these properties from the plist that it reads already, so a change to the plist shows at the next
refresh, and svcview runs no new command.

At 80 columns, a worker with a status of one field looks like this (the lines under `Status` are the status; the lines
from `Plist` on are new):

```
● running     PID 4122
up 3h12m      last exit 1
Status 1s ago
mode  idle

Plist         /Users/me/Library/LaunchAgents
              /com.example.worker.plist
program       /usr/local/bin/worker
              --queue
              jobs
work folder   /Users/me/work
run at load   yes
keep alive    when the last exit is 0
start every   300s
stdout        /Users/me/Library/Logs/worker.
              log
environment   WORKER_STATUS_FILE
              WORKER_TOKEN
```

The story criteria that this change satisfies:

- The detail pane shows the program and its arguments from `ProgramArguments`, or from `Program` when the plist has no
  `ProgramArguments`, and the `WorkingDirectory`.
- The detail pane shows when launchd starts the service: `RunAtLoad`, `KeepAlive` with each of its conditions (for
  example `SuccessfulExit`, `Crashed`, `OtherJobEnabled`, `PathState`, and `NetworkState`), `StartInterval`,
  `StartCalendarInterval`, `WatchPaths`, and `ThrottleInterval`. A setting that the plist does not have does not show.
- The detail pane shows the plist path, the `StandardOutPath`, the `StandardErrorPath`, and the names of the
  `EnvironmentVariables`, with no values.
- A service with no plist shows none of these settings.
- svcview reads these settings from the plist that it reads already, and runs no new command. A change to the plist
  shows at the next refresh.
- `docs/cli.md` describes the settings in the detail pane.

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
- The detail pane shows no value of a variable of `EnvironmentVariables`, only its name. `svcview status` still prints
  each value as a template value, as it does today.
- svcview runs no new command for the properties: no `launchctl print`, no `plutil`, and no call of `launchctl` other
  than `list`. It reads no plist outside `~/Library/LaunchAgents`, so a service with no plist there shows no property,
  also when a plist in another folder defines it.
- The pane shows what the plist says, and not what launchd infers from it: a `KeepAlive` does not add `run at load`, and
  a default of launchd that the plist does not state (a `ThrottleInterval` of 10 s, a `RunAtLoad` of false) shows no
  line.
- Only the properties of the story show. Other properties, also those that can make launchd start a service
  (`Sockets`, `MachServices`, `QueueDirectories`, `LaunchEvents`), and properties such as `UserName`, `Umask`,
  `ExitTimeOut` or `ProcessType`, show no line.
- No check of a value beyond its type: svcview does not check that a path exists, that a program can run, or that a
  number of `StartCalendarInterval` is in its range (an `Hour` of 25 shows as `25:00`).
- The properties are not template values. A status command or an action of a group cannot use them, and
  `internal/status` does not change.
- The rows of the list and the header do not change. No key, no help line and no help text changes.
- The lines of the data from launchd and of the status keep the cut with `…` at the edge of the pane. Only the parts of
  the values of the plist continue on the next lines.

**Deferred:**
- A terminal that is too narrow for two panes. The lines of the plist follow the width of the detail pane beside the
  list, and the screen shows no single pane.

## Codebase Context

- `internal/launchd/plist.go:10-45`: the private type `plist` keeps the modification time, the path, the `Label`, the
  environment and the two log paths of one file. `readPlist` decodes the file with `howett.net/plist` into an anonymous
  struct whose properties are typed `any`, and keeps a value only when it has the type that launchd takes (a variable
  whose value is not a text is left out, and so is a log path that is not a text). A file that does not decode, or
  that has no `Label`, gives no service. Its local variable `properties` holds the decoded plist; the planned design
  gives the word property to `service.Property`, so the run names one thing with it in this function.
  `readIfChanged` (`:47-52`) reads a file again only when its modification time changes, and `plistFolder.read`
  (`internal/launchd/plist_folder.go:15-45`) keeps the read plists between two reads.
- A probe in the scratchpad decoded the same plist as XML and as binary (made with `plutil -convert binary1`) into
  `any` with `howett.net/plist` v1.0.1: a boolean gives `bool`; an integer gives `uint64`, or `int64` when it is
  negative; a real gives `float64`; an array gives `[]any`; a dictionary gives `map[string]any`, with no order of its
  keys. XML and binary give the same values. A `KeepAlive` of `<true/>` gives `bool`, and a dictionary of conditions
  gives `map[string]any` whose `PathState` and `OtherJobEnabled` are `map[string]any` of `bool`. A
  `StartCalendarInterval` gives `map[string]any` for one dictionary and `[]any` of them for an array.
- `internal/launchd/domain.go:84-92`: `servicesOfPlists` copies the environment, the log paths and the path of each plist
  into its `service.Service`. `internal/launchd/launchctl_list.go:81-93` gives each service with no plist from `launchctl list` alone,
  with no plist path. `Domain.Services` and `Domain.AllServices` run `launchctl list` and `ps` and nothing else.
- `internal/service/service.go:35-50`: `Service` has `Env`, `StdoutPath`, `StderrPath` and `PlistPath`; `LogPath` gives the
  log of `o`. `internal/status/values.go:14-24` builds the template values from these fields by name, so a new field of
  `Service` does not become a template value.
- `internal/launchd/domain_test.go`: `homeWith` and `homeWithPlists` (`:23-46`) write fixture plists from `testdata/`
  or inline XML; the tests compare whole `[]service.Service` values with one `require.Equal` (`:175-188` XML and binary,
  `:216-233` environment and log paths, `:248-265` values of the wrong type, `:664-679` a service with no plist with
  `AllServices`, `:873-891` a plist whose environment changes with its modification time). The fixtures `xml.plist`,
  `binary.plist` and `environment*.plist` have `ProgramArguments`, and `xml.plist` and `binary.plist` have
  `RunAtLoad`, so the services of each test that reads them gain properties; `withoutPlistPath` (`:152-157`) is how a
  test that is not about a field leaves it out. `fakeLaunchctlCalls` (`:118-124`) records each call of `launchctl`
  other than `list`. A binary fixture is made on the Mac with `plutil -convert binary1`; the e2e image has no `plutil`.
- `internal/tui/screen.go:775-809`: `selectedDetailText` and `detailText` build the lines of the detail pane: the two lines
  of the data from launchd through `detailLine` (`:848-850`, the column `detailLeftWidth` of 14 cells at `:26`), then,
  when the service has a status result, `Status` with its age, the lines of a failure, the fields (`fieldLines`,
  `:811-826`) and the events (`eventLines`, `:828-838`, which start with an empty line and `Events`). Each text from
  outside goes through `terminal.ReplaceControlCharacters`.
- `internal/tui/screen.go:593-607`: `panes` gives the detail pane the width of the terminal less the list, and
  `box.render` (`internal/tui/box.go:22-33`) cuts each line at `textWidth` (the width less 4) with `…`. At 80 columns
  the text of the detail pane has 44 cells, so the room after the column of 14 cells is 30 cells; at 120 columns it is
  54 cells, and the 160 columns of `openCli` give 78 cells. `detailTopInView` (`:492-495`) clamps the scroll with the
  number of lines of `selectedDetailText`. `listText` (`:674-681`) already wraps each line of a read error at the width
  of the list, so it takes that width as a parameter.
- `internal/tui/screen_test.go`: the group `detail pane` (`:1579-2190`) tests the lines with `detailPane` (`:489-499`)
  and golden files with `requireGolden` (`:154-165`); the tests of the fields (`:1774-1799`, `:1854-1870`) and of the
  fields and events golden (`:2117-2142`) are the precedent for a block of lines. `servicesNamed` (`:76-84`) gives
  services with no plist path, so the tests and golden files that use it do not change. The stop tests select
  `com.example.c`, whose plist path is `/Users/me/Library/LaunchAgents/com.example.c.plist`, so `confirmation.golden`
  (`:3300-3312`) gains the line `Plist`. The group `scroll` (`:2749-3057`) tests the scroll of the detail pane, also
  after a change of the size of the terminal (`:2992-3006`) and in a terminal with no room (`:3025-3033`).
- e2e: `addPlist` (`e2e/testUtils.ts:34-58`) writes only `Label`, `EnvironmentVariables` and the log paths, so a test
  of this story writes its own plist. Each e2e service has a plist, so each detail pane in the e2e tests gains the line
  `Plist`; no e2e test asserts a text that the new lines hold. `setLaunchctlOutput` (`:72-77`) writes a file under
  another name and renames it, so the screen never reads half a file. On a Mac the scratch folder is under the long
  `TMPDIR`, so a plist path can take two lines; in Docker it is under `/tmp`. Check the program and the names, and do
  not match a plist path on one line (tasks/learnings.md, "Join the lines of the screen before an e2e test checks a
  long error").
- `docs/cli.md`: the row "The detail pane" of the table of parts (`:83`), "A service with no plist" (`:93-102`), the
  table of the lines of the detail pane (`:127-135`), "Refresh" (`:278-284`), and the note that `svcview status`
  prints each value of `EnvironmentVariables` (`:953-956`). The example screen at `:52-73` ends with an empty line under
  the events, which is the empty line above `Plist`, so it stays true.
- `man launchd.plist` on this Mac: `KeepAlive` is a boolean or a dictionary of conditions, and launchd keeps the job
  alive when any one condition holds. `SuccessfulExit` true restarts the job after an exit with the status 0, and false
  in the inverse case. `Crashed` true restarts it after a signal of a crash (SIGILL, SIGSEGV, …), and false in the
  inverse case. `PathState` keeps it alive while a path exists (true) or does not (false). `OtherJobEnabled` keeps it
  alive while another job is loaded in launchd. `NetworkState` "is no longer implemented". `AfterInitialDemand` is a
  condition that the story does not name. `StartCalendarInterval` takes `Minute`, `Hour`, `Day`, `Weekday` (0 and 7 are
  Sunday) and `Month`, a missing one is each value, and with both `Day` and `Weekday` either one starts the job.
- tasks/learnings.md entries that apply: "Text from outside goes to the terminal through
  terminal.ReplaceControlCharacters"; "A golden file already holds each line of the screen that it draws"; "A test of a
  cut at a width needs the exact-fit width and one cell less"; "Show no data of a read before the first read ends";
  "Plan each Bubble Tea message and private type in design.yaml" (the planned design names no new private type, so a
  helper type in `internal/launchd` needs `clerk design note`); "Check the name of a new type against the words of
  design.yaml before clerk design note"; "Proposed mutants test the words and the order of a text on the screen";
  "go test -run splits its pattern at / before it matches"; "Run clerk mutants and clerk audit run under mise exec";
  "task test:e2e:local leaves e2e/package-lock.json in the tree"; "Pass --integrate to clerk land when the request asks
  to land on main".

## Tasks

### Task 1: Read the program, the work folder, the log paths and the names of the environment as properties of a service

**Behavior:** `Domain.Services` and `Domain.AllServices` give each service with a plist the properties of its plist, as
`service.Property` values with a name in plain words and the texts of the value, in this order: `program` (each
argument of `ProgramArguments`, or the `Program` when the plist has no `ProgramArguments`), `work folder`
(`WorkingDirectory`), `stdout` (`StandardOutPath`), `stderr` (`StandardErrorPath`) and `environment` (the names of
`EnvironmentVariables`, sorted, with no value). A property that the plist does not have gives no `service.Property`.
An XML plist and a binary plist give the same properties. A change to the plist gives the new properties at the next
read. A service with no plist has no properties.

**Acceptance Criteria:**
- [x] A plist with `ProgramArguments` `/usr/local/bin/worker`, `--queue`, `jobs`, a `WorkingDirectory`, a
      `StandardOutPath`, a `StandardErrorPath`, and `EnvironmentVariables` with `WORKER_TOKEN` and `WORKER_STATUS_FILE`
      gives a service whose properties are exactly `program` with the three arguments in their order, `work folder`,
      `stdout`, `stderr`, and `environment` with `WORKER_STATUS_FILE` and `WORKER_TOKEN`. No text of a property holds
      the value of a variable.
- [x] The service of `binary.plist`, a binary plist, has `program` with `/usr/bin/true`, as the service of `xml.plist`
      has.
- [x] A plist with `Program` and no `ProgramArguments` gives `program` with the `Program`. A plist with both gives
      `program` with the arguments of `ProgramArguments` only.
- [x] A plist with only a `Label` gives a service with no properties.
- [x] A `ProgramArguments` that is not an array, or a `Program` or a `WorkingDirectory` that is not a text, gives no
      property, and the other properties keep theirs. An argument that is not a text is left out, and the other
      arguments keep their order. A log path that is not a text gives no `stdout` or `stderr`, and a variable whose
      value is not a text gives no name, as the environment and the log paths of the service do today.
- [x] An empty text gives a part that is empty. An empty `ProgramArguments` or `EnvironmentVariables` gives its property
      with no part.
- [x] The `Env`, `StdoutPath`, `StderrPath` and `PlistPath` of each service do not change, and no file in
      `internal/status` changes.
- [x] A plist whose `ProgramArguments` changes with its modification time gives the new program at the next read, and
      a plist whose modification time does not change keeps its properties.
- [x] With `AllServices`, a service with no plist has no properties, and a service with a plist has the properties of
      its plist.
- [x] A read of a plist with each of these properties records no call of `launchctl` other than `list`.
- [x] Each test that compares whole services read from `xml.plist`, `binary.plist` or an `environment*.plist` fixture
      expects the properties of the fixture, or compares the services without their properties, and still checks
      what its name says.

**Affected Files/Modules:**
- `internal/service/service.go` — the new type `Property` with `Name` and `Values`, and the field `Properties` of `Service`
- `internal/launchd/plist.go` — the read of `ProgramArguments`, `Program` and `WorkingDirectory`, and the properties of the plist
- `internal/launchd/domain.go` — `servicesOfPlists` gives each service the properties of its plist
- `internal/launchd/domain_test.go` — the properties of XML and binary plists, of each type of value, of a changed plist and of a service with no plist
- `internal/launchd/testdata/` — an XML fixture plist with these properties

**Patterns to Follow:**
- `internal/launchd/plist.go:10-45`
- `internal/launchd/plist.go:47-52`
- `internal/launchd/domain.go:84-92`
- `internal/service/service.go:35-50`
- `internal/launchd/domain_test.go:23-46`
- `internal/launchd/domain_test.go:175-188`
- `internal/launchd/domain_test.go:216-265`
- `internal/launchd/domain_test.go:664-679`
- `internal/launchd/domain_test.go:873-891`

**Testable:** Yes

**Certainty:** medium — the plist read already keeps the environment and the log paths with a check of the type of each
value (`internal/launchd/plist.go:24-43`, tested at `internal/launchd/domain_test.go:216-265`); this task varies it with
arrays of texts and a new exported type that carries the plist to the other packages, with its texts in a set order.

**Blast radius:** high — `EnvironmentVariables` can hold a token or a password, and this task decides that a property
holds only the names; a mistake here puts the value of a secret on the screen.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Show the plist path and the properties of a service in the detail pane

**Behavior:** For a service with a plist path, the detail pane shows, after the last line of the status, or after the
data from launchd when no line of the status shows, an empty line, then `Plist` with the plist path in the column of
`PID` and `last exit` (14 cells), then each property of the service: its name from the first column, the first part of
its value in the column of 14 cells, and each other part on a line of its own in that column. A property with no part
shows its name alone. A service with no plist path shows none of these lines. Each control character in the plist
path or in a part shows as `�`. The pane cuts a line that is wider than the pane with `…`, as it does today. The
scroll of the detail pane counts these lines.

**Acceptance Criteria:**
- [x] A golden file `plist`, at a width where each part fits (for example 120 columns), draws a running service with a
      plist path and the properties `program` with three parts, `work folder`, `stdout`, `stderr` and `environment`
      with two names, and no status: the two lines of the data from launchd, an empty line, `Plist` and the path, then
      `program       /usr/local/bin/worker`, `              --queue`, `              jobs`, and the other properties in
      their order.
- [x] With a status, the lines of the plist follow the last line of the status: the last event when the output has
      events, the last field when it has none, and the last line of a failure. The line `Status` and the lines under it
      do not move. Before the first run of the status ends, the lines of the plist follow the data from launchd.
- [x] A service with no plist path shows no line `Plist` and no property, and its detail pane ends with the same line
      as before this change. The golden files of services with no plist path, `detail.golden` and
      `all-services.golden` among them, do not change.
- [x] A property with no part shows its name alone on its line.
- [x] Each control character in the plist path and in a part of a value shows as `�`, and the screen sends no control
      character of them to the terminal.
- [x] A service whose `Env` has `WORKER_TOKEN` with a value, and whose property `environment` names `WORKER_TOKEN`,
      shows `environment   WORKER_TOKEN` and no text of the value anywhere on the screen.
- [x] With the focus on the detail pane and a pane shorter than the text, `↓` moves to the last property and stops when
      its last part is at the bottom of the pane.
- [x] `confirmation.golden` shows the line `Plist` of the selected service, cut with `…` at 80 columns, and no other
      golden file changes.
- [x] e2e: a plist with `ProgramArguments` `/usr/local/bin/worker`, `--queue`, `jobs`, a `WorkingDirectory`, a
      `StandardOutPath`, and `EnvironmentVariables` with `WORKER_TOKEN` and a value shows in the detail pane
      `program       /usr/local/bin/worker`, `--queue` in the column of the values on the next line, the work folder,
      the stdout and `environment   WORKER_TOKEN`; the screen does not contain the value, and the fake `launchctl`
      records no call.
- [x] e2e: a new plist with another program, written while the screen is open under another name and renamed onto the
      plist, shows the new program no later than the next refresh, with no key press.
- [x] e2e: with `--all`, a service with no plist shows no line `Plist`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `detailText` adds the lines of the plist after the lines of the status
- `internal/tui/screen_test.go` — the place of the lines, a service with no plist path, control characters, the names of the environment, the scroll
- `internal/tui/testdata/` — the new golden file `plist`, and `confirmation.golden`
- `e2e/tests/plist.test.ts` — the new e2e tests of the lines of the plist

**Patterns to Follow:**
- `internal/tui/screen.go:775-809`
- `internal/tui/screen.go:811-838`
- `internal/tui/screen.go:848-850`
- `internal/tui/screen_test.go:154-165`
- `internal/tui/screen_test.go:489-499`
- `internal/tui/screen_test.go:1580-1586`
- `internal/tui/screen_test.go:1774-1799`
- `internal/tui/screen_test.go:1854-1870`
- `internal/tui/screen_test.go:2117-2158`
- `internal/tui/screen_test.go:2750-2765`
- `internal/tui/screen_test.go:3300-3312`
- `e2e/tests/status.test.ts:23-37`
- `e2e/tests/all-services.test.ts:64-75`
- `e2e/testUtils.ts:34-58`
- `e2e/testUtils.ts:72-77`
- `task:1`

**Testable:** Yes

**Certainty:** medium — the fields and the events are blocks of lines that `detailText` adds after the data from launchd
(`internal/tui/screen.go:775-838`); this task varies them with names in the column of the launchd data, a value of
several lines, and a block whose place depends on whether lines of the status show.

**Blast radius:** high — the detail pane is where a value of `EnvironmentVariables` would reach the terminal; the tests
show the name of a variable and no text of its value.

**Verification:** `mise exec -- task test:unit` passes; `git status --porcelain internal/tui/testdata` names
`plist.golden` and `confirmation.golden` only; `mise exec -- task test:e2e` passes `e2e/tests/plist.test.ts` and each
other e2e test.

**Depends on:** Task 1

### Task 3: Continue a long part of a value on the next lines of the detail pane

**Behavior:** A part of a value of the plist, and the plist path, that is wider than the room after the column of 14
cells continues on the next lines, in that column, so each of its cells shows. The room is the width of the text of the
detail pane less 14 cells. A part breaks at the end of the room, cell by cell. The lines of the data from launchd and
of the status keep the cut with `…`. The scroll of the detail pane counts the lines as the pane draws them, so a change
of the width of the terminal changes the number of lines, and the scroll stays inside the text.

**Acceptance Criteria:**
- [x] At 80 columns the room is 30 cells: a part of exactly 30 cells shows on one line; a part of 31 cells shows its
      first 30 cells, and its last cell on the next line in the column of the values; a part of 61 cells takes three
      lines.
- [x] A wide character that does not fit at the end of the room moves whole to the next line. A control character shows
      as `�` before the break, and takes one cell.
- [x] A golden file `plist-wrapped` at 80 columns draws a plist path and a `stdout` that are each wider than 30 cells,
      each on two lines.
- [x] A field of the status, a line of a failure, and the data from launchd that are wider than the pane are still cut
      with `…`.
- [x] With the focus on the detail pane, `↓` stops when the last line of the last part is at the bottom of the pane.
      After the terminal gets wider, the same text takes fewer lines, the scroll stays inside the text, and no empty
      line shows under its last line.
- [x] A terminal of each width up to 32 columns, which gives a room of 2 cells, 1 cell, no cell or less, does not stop
      the screen after `↓`. With no cell of room, the pane shows each part on one line, which it cuts.
- [x] `confirmation.golden` shows the plist path of the selected service on two lines, and `plist.golden` does not
      change.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the lines of the plist take the width of the detail pane, and the scroll counts them at that width
- `internal/tui/screen_test.go` — the exact fit and the break, wide and control characters, the scroll after a change of the width, the narrow terminal
- `internal/tui/testdata/` — the new golden file `plist-wrapped`, and `confirmation.golden`

**Patterns to Follow:**
- `internal/tui/screen.go:674-681`
- `internal/tui/screen.go:593-607`
- `internal/tui/screen.go:492-495`
- `internal/tui/box.go:18-33`
- `internal/tui/screen_test.go:938-949`
- `internal/tui/screen_test.go:2992-3006`
- `internal/tui/screen_test.go:3025-3033`
- `task:2`

**Testable:** Yes

**Certainty:** medium — the list wraps each line of a read error at its width (`internal/tui/screen.go:674-681`); this
task varies it with a wrap that keeps a column, in text whose number of lines the scroll of the detail pane reads, so
that number now depends on the width of the terminal.

**Blast radius:** low — the change is where the text of the plist breaks on the screen; no read, no key and no command
changes.

**Verification:** `mise exec -- task test:unit` passes; `git status --porcelain internal/tui/testdata` names
`plist-wrapped.golden` and `confirmation.golden` only.

**Depends on:** Task 2

### Task 4: Show run at load and keep alive with each of its conditions

**Behavior:** `RunAtLoad` gives the property `run at load`, and `KeepAlive` gives `keep alive`, after `work folder` and
before `stdout`. A boolean shows as `yes` or `no`, also when it is false, because the plist has it. A `KeepAlive`
dictionary gives one part for each condition, in the order `SuccessfulExit`, `Crashed`, `PathState` (by path),
`OtherJobEnabled` (by name), and then each other condition by its name, with these texts:

| Condition | Text |
| --- | --- |
| `SuccessfulExit` true | `when the last exit is 0` |
| `SuccessfulExit` false | `when the last exit is not 0` |
| `Crashed` true | `when the last exit is a crash` |
| `Crashed` false | `when the last exit is not a crash` |
| `PathState`, a path with true | `while <path> exists` |
| `PathState`, a path with false | `while <path> does not exist` |
| `OtherJobEnabled`, a name with true | `while <name> is loaded or running` |
| `OtherJobEnabled`, a name with false | `while <name> is not loaded` |
| Each other condition, for example `NetworkState` or `AfterInitialDemand` | its name, then `yes` or `no` for a boolean |

**Acceptance Criteria:**
- [x] `RunAtLoad` true gives `run at load` with `yes`, and false gives `no`. `KeepAlive` true gives `keep alive` with
      `yes`, and false gives `no`.
- [x] A `KeepAlive` with `SuccessfulExit` false, `Crashed` true, `PathState` with `/b` true and `/a` false,
      `OtherJobEnabled` with `com.example.b` true and `com.example.a` false, `NetworkState` true and
      `AfterInitialDemand` true gives `keep alive` with exactly these parts in this order:
      `when the last exit is not 0`, `when the last exit is a crash`, `while /a does not exist`, `while /b exists`,
      `while com.example.a is not loaded`, `while com.example.b is loaded or running`, `AfterInitialDemand yes`,
      `NetworkState yes`.
- [x] `SuccessfulExit` true gives `when the last exit is 0`, and `Crashed` false gives
      `when the last exit is not a crash`.
- [x] A `RunAtLoad` that is not a boolean, and a `KeepAlive` that is neither a boolean nor a dictionary, give no
      property. A condition whose value has a type that launchd does not take (a `SuccessfulExit` or a `Crashed` that is
      not a boolean, a `PathState` or an `OtherJobEnabled` that is not a dictionary, a path or a name whose value is not
      a boolean) gives no part, and the other conditions keep theirs. Another condition whose value is not a boolean
      shows its name alone. An empty `KeepAlive` dictionary gives `keep alive` with no part.
- [x] A plist with each property of Task 1 and of this task gives exactly `program`, `work folder`, `run at load`,
      `keep alive`, `stdout`, `stderr`, `environment`.
- [x] A plist whose `KeepAlive` changes with its modification time gives the new conditions at the next read.
- [x] The services of each test that reads `xml.plist` or `binary.plist`, which have `RunAtLoad` true, have
      `run at load` with `yes` where the test expects their properties, so a boolean of a binary plist gives the same
      text as one of an XML plist.
- [x] e2e: a plist with `KeepAlive` and `SuccessfulExit` true, whose service `launchctl list` shows with no PID and the
      Status 1, shows `last exit 1` and `keep alive    when the last exit is 0` in the detail pane.

**Affected Files/Modules:**
- `internal/launchd/plist.go` — the read of `RunAtLoad` and `KeepAlive`, and the text of each condition
- `internal/launchd/domain_test.go` — each boolean, each condition, each type of value, the order, a changed `KeepAlive`
- `internal/launchd/testdata/` — an XML fixture plist with `KeepAlive` and its conditions
- `e2e/tests/plist.test.ts` — the keep alive of a service whose last exit is 1

**Patterns to Follow:**
- `internal/launchd/plist.go:24-43`
- `internal/launchd/domain_test.go:216-265`
- `internal/launchd/domain_test.go:873-891`
- `e2e/tests/screen.test.ts:119-139`
- `task:1`
- `task:2`

**Testable:** Yes

**Certainty:** medium — the read of `EnvironmentVariables` checks the type of each value of a dictionary
(`internal/launchd/plist.go:36-43`); this task varies it with a property that is a boolean or a dictionary of
conditions, two of which hold a dictionary of their own, with a text for each condition that this breakdown fixes.

**Blast radius:** low — the change is text on the screen; what launchd does, and each call of `launchctl`, stay the
same.

**Verification:** `mise exec -- task test:unit` passes; `mise exec -- task test:e2e` passes `e2e/tests/plist.test.ts`.

**Depends on:** Task 1, Task 2

### Task 5: Show the start interval, the start times, the watch paths and the throttle

**Behavior:** `StartInterval` gives `start every`, `StartCalendarInterval` gives `start at`, `WatchPaths` gives
`watch paths`, and `ThrottleInterval` gives `throttle`, in this order, after `keep alive` and before `stdout`.
`StartInterval` and `ThrottleInterval` show their seconds with `s`, for example `300s`. `start at` has one part for each
start time: one for a dictionary, and one for each dictionary of an array, in the order of the array. A start time shows
the `Hour` and the `Minute` as two digits joined by `:`, with `*` for one that the entry does not have; then
` on <weekday>` for `Weekday`, with 0 and 7 as Sunday and 1 to 6 as Monday to Saturday; with both `Weekday` and `Day`,
` on <weekday> or on day <Day>`, and with `Day` alone, ` on day <Day>`; then ` in <month>` for `Month`, with 1 to 12 as
January to December. A number outside these ranges shows as the number. `watch paths` has one part for each path, in
the order of the array.

**Acceptance Criteria:**
- [ ] `StartInterval` 300 gives `start every` with `300s`, and `ThrottleInterval` 10 gives `throttle` with `10s`. A value
      that is not a whole number, such as the real 10.5 or a text, gives no property.
- [ ] Each start time gives its text: `Hour` 3 and `Minute` 0 give `03:00`; `Minute` 30 gives `*:30`; `Hour` 9,
      `Minute` 0 and `Weekday` 1 give `09:00 on Monday`; `Hour` 0, `Minute` 0, `Day` 1 and `Month` 6 give
      `00:00 on day 1 in June`; `Weekday` 7 and `Day` 15 give `*:* on Sunday or on day 15`; `Weekday` 9 and `Month` 13
      give `*:* on weekday 9 in month 13`; `Hour` 25 gives `25:*`; an empty dictionary gives `*:*`.
- [ ] A `StartCalendarInterval` that is one dictionary gives `start at` with one part. An array of dictionaries gives one
      part for each, in the order of the array.
- [ ] A value of a start time that is not a whole number is left out of it, so its place shows as for a missing one. A
      negative number shows as the number. An element of the array that is not a dictionary gives no part, and a
      `StartCalendarInterval` that is neither a dictionary nor an array gives no property.
- [ ] `WatchPaths` gives `watch paths` with each path in the order of the array. An element that is not a text is left
      out, and a `WatchPaths` that is not an array gives no property.
- [ ] A plist with each property of the story gives exactly `program`, `work folder`, `run at load`, `keep alive`,
      `start every`, `start at`, `watch paths`, `throttle`, `stdout`, `stderr`, `environment`. An XML plist and a
      binary plist with each property of the story, a `KeepAlive` dictionary with `PathState` and an array of start
      times among them, give the same properties.

**Affected Files/Modules:**
- `internal/launchd/plist.go` — the read of `StartInterval`, `StartCalendarInterval`, `WatchPaths` and `ThrottleInterval`, and the text of a start time
- `internal/launchd/domain_test.go` — each start time, each type of value, the order of all properties, XML and binary
- `internal/launchd/testdata/` — an XML fixture plist with each property of the story, and the same plist as binary

**Patterns to Follow:**
- `internal/launchd/plist.go:24-43`
- `internal/launchd/domain_test.go:175-188`
- `internal/launchd/domain_test.go:248-265`
- `task:1`
- `task:4`

**Testable:** Yes

**Certainty:** medium — the plist read keeps the values of the right type (`internal/launchd/plist.go:24-43`); this task
varies it with whole numbers, which `howett.net/plist` gives as `uint64`, or `int64` when negative, from XML and from
binary (the probe in Codebase Context), and with a property that is one dictionary or an array of them.

**Blast radius:** low — the change is text on the screen; what launchd does, and each call of `launchctl`, stay the
same.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 4

### Task 6: Describe the properties of the plist in docs/cli.md

**Behavior:** `docs/cli.md` describes the lines of the plist in the detail pane: where they show, the plist property of
each line and what it shows, the texts of `keep alive`, the form of a start time, the names of the environment with no
values, a service with no plist, the continued lines, and that a change to a plist shows at the next refresh.

**Acceptance Criteria:**
- [ ] The row "The detail pane" of the table of parts says that the pane of a service with a plist shows the plist path
      and the properties of its plist, after the status.
- [ ] "The detail pane" has a row for each line: `Plist`, `program`, `work folder`, `run at load`, `keep alive`,
      `start every`, `start at`, `watch paths`, `throttle`, `stdout`, `stderr` and `environment`. Each row names the
      plist property that the line comes from and what the line shows, and the section says that a property that the
      plist does not have shows no line, and that a boolean shows `yes` or `no`.
- [ ] A table gives the text of each condition of `keep alive` with the condition and the value it comes from, and says
      that launchd starts the service again while any one condition holds, that a crash is an end by a signal such as
      `SIGSEGV` or `SIGILL`, and that launchd no longer acts on `NetworkState`, so svcview shows `NetworkState yes` or
      `NetworkState no` and no phrase.
- [ ] The section gives the form of a start time, with `*` for each hour or each minute, and examples.
- [ ] The section says that the pane shows the names of `EnvironmentVariables` and never their values, which can hold a
      token, and that `svcview status` prints the values as template values.
- [ ] The section says that a part that is wider than the pane continues on the next lines in the column of the values,
      and that the other lines of the pane are cut with `…`.
- [ ] An example of the detail pane at 80 columns shows a worker with a status and the lines of its plist, and each of
      its lines is a line that the tests of Tasks 2 to 5 expect.
- [ ] "A service with no plist" says that its detail pane shows no plist path and no property.
- [ ] "Refresh" says that a change to a plist shows at the next refresh, because svcview reads a plist again when its
      modification time changes, and that svcview runs no other command to read the properties.
- [ ] The section says that svcview shows only these properties, and not what launchd infers from them (a `KeepAlive`
      that implies `RunAtLoad`), a default of launchd that the plist does not state, or other properties that can
      start a service, such as `Sockets`, `MachServices`, `QueueDirectories` or `LaunchEvents`.
- [ ] `docs/cli.md` uses the word property for an entry of the plist, and keeps the word setting for an entry of a
      config file.

**Affected Files/Modules:**
- `docs/cli.md` — the table of parts, "A service with no plist", "The detail pane", and "Refresh"

**Patterns to Follow:**
- `docs/cli.md:79-91`
- `docs/cli.md:93-102`
- `docs/cli.md:127-135`
- `docs/cli.md:278-284`
- `docs/cli.md:953-956`
- `docs/proposal-launchd-tui.md:152-182`
- `task:2`
- `task:4`
- `task:5`

**Testable:** No

**Certainty:** high — `docs/cli.md:127-135` describes each line of the detail pane in one table, and the docs tasks of
the earlier stories of this repo extended it the same way.

**Blast radius:** low — documentation only.

**Verification:** Each line of the example in `docs/cli.md` matches a line that the tests of Tasks 2 to 5 expect;
`mise exec -- task test:unit` still passes.

**Depends on:** Task 3, Task 5

## Summary

- 6 tasks. Task 1 reads the properties in `internal/launchd` and gives them to the other packages through
  `service.Property`, with the high-blast rule that the environment holds names only, first, so that the screen of
  Task 2 has properties to draw and an audit fix of the read touches no file of the screen. Task 2 is the one change of
  the layout of the screen, and Task 3, the other change in `internal/tui`, comes right after it, so that no later task
  rewrites their lines. Tasks 4 and 5 change only `internal/launchd`, because the screen draws each property in the same
  way; Task 4 comes first because `KeepAlive` is the property that the story says only the plist explains. Task 6
  describes the result.
- Story criteria: the program, its arguments and the `WorkingDirectory` (Task 1, shown by Task 2); when launchd starts
  the service (Tasks 4 and 5); the plist path, the log paths and the names of the environment (Tasks 1 and 2); a service
  with no plist (Tasks 1 and 2); no new command and a change at the next refresh (Tasks 1, 2 and 4); `docs/cli.md`
  (Task 6). Task 3 adds the continued lines, so that a path shows whole at 80 columns. No criterion is deferred.
- Certainty is medium for Tasks 1 to 5 and high for Task 6: each code task has a precedent in this repo, and each varies
  it in a way that no instance covers. The story is routine work on two precedents (the plist read and the blocks of
  the detail pane), so no task is low.
- Tasks 1 and 2 have blast radius high, because a value of `EnvironmentVariables` can be a secret, and they decide that
  only its name reaches the screen.
