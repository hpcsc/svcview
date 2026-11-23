## Contents
1. Task 1: Refuse a progress outside 0 to 1 as a status invalid
2. Task 2: Show the fields of a status under Status, each with its name and its value
3. Task 3: Show a bar of 10 cells under the value of a field with a progress
4. Task 4: Show the events of a status under Events
5. Task 5: Describe the fields, the bars and the events in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-004 "See the fields and the events of a status" (`launchd-tui/US-004`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen" (the detail pane with `Status`, the fields, the
bar and `Events`), "Colors and Glyphs", and "The Status Command" (Output).

This change shows in the detail pane what a worker reports beyond its summary: which job it does, how far the job
is, and what it did in the last hour. Under `Status`, the detail pane shows each field of the last output as its
`name` and its `value`, in the order of the output, and svcview shows each field in the same way, whatever its name.
Thus a service can add a field with no change to svcview. Under the value of a field with `progress`, a bar of 10
cells shows how far the job is. A `progress` outside 0 to 1 makes the output invalid, so the row shows
`status invalid`. Under `Events`, the detail pane shows each event in the order of the output: the hours and minutes
of its `at` in the local time zone, a `●` in the color of its `level`, and its `text`. `docs/cli.md` describes each
of these, and says that svcview refuses a `progress` outside 0 to 1.

The names, the values and the texts come from a program outside svcview. Each control character in them shows as
`�`, so an output cannot send an escape sequence to the terminal.

The story criteria that this change satisfies:

- Under `Status`, the detail pane shows each field as its `name` and its `value`, in the order of the output. svcview
  shows each field in the same way, whatever its name. Thus a service can add a field with no change to svcview.
- A field with `progress` shows a bar of 10 cells under its value: `▰` in the accent color for the done part, and a
  dim `▱` for the rest. For example, `0.6` shows 6 `▰` and 4 `▱`.
- A `progress` outside 0 to 1 makes the row show `status invalid`.
- Under `Events`, the detail pane shows each event in the order of the output: the time of its `at` as hours and
  minutes, a `●` in the color of its `level`, and its `text`.

The stories file gives the time zone of the event times as the local time zone (Open Questions).

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none. The detail pane shows the fields and the events of the
  last run only. After a failure, it shows the failure, and no fields and no events of an earlier run.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No work in the worker repo. The worker writes its status file, and its install script adds `WORKER_STATUS_FILE`
  to the plist and writes `conf.d/worker.yaml`. That work is outside svcview.
- svcview gives no meaning to the name of a field. No name hides a field or shows it in another way.
- svcview does not sort, filter, count or limit the events. It shows each event of the output, in the order of the
  output, also an event that is older than one hour.
- The time of an event has no date and no age. An event of another day shows only its hours and minutes.
- The row in the list does not change: it shows the summary, or a failure, as before.
- The only new rule of the output is the range of `progress`. Each other rule of the parse stays as it is.
- The detail pane does not scroll, and no key moves the focus. A detail pane whose lines do not fit shows the lines
  that fit, as the lines of a failure do, and `↑` and `↓` select a service as before. The borders of the panes keep
  their look.
- No mark of a stale status. `stale_after` stays a setting that svcview does not know.
- No layout for a narrow terminal, and no change for `NO_COLOR`.

**Deferred:**
- The focus of a pane, and the scroll of a detail pane whose lines do not fit.

## Codebase Context

- `internal/status/output.go:68-141`: `parseOutput` reads `fields` (a `name`, a `value`, and an optional `progress`)
  and `events` (`at`, `level`, `text`), and keeps their order in `Output.Fields` and `Output.Events`. A parse error
  names the place in the output as a JSON pointer and says what version 1 needs there, for example
  `"/version" is 2, and it must be 1` (`output.go:101-106`) and `"/fields/1/name" is missing` (`output.go:116-124`).
- The parse does not check the range of `progress` today. `internal/status/command_test.go:429-437` checks that a
  `progress` of 1.5 and of -0.1 gives an output with that progress, and `docs/cli.md:277` says that svcview does not
  check the range. Both say the opposite of the story and of the proposal, and change in Task 1. A `progress` of
  0, 0.6 and 1 gives an output with that progress (`command_test.go:272-310`).
- `internal/status/command.go:90-95`: a parse error becomes a status invalid with the first 20 lines of the output.
  The row shows `status invalid` in red (`internal/tui/screen.go:352-373`), and the detail pane shows the parse
  error and the lines under `Status` (`screen.go:417-425`). Thus Task 1 needs no change in `internal/tui`.
- `internal/tui/screen.go:405-427`: `detailText` draws the launchd lines, the line `Status` with the age of the
  status, and under it the text and the lines of a failure. `nameWidths` and `rowText` (`screen.go:335-350`) start
  the summaries of one group in one column, two spaces after the widest short name. `levelColor`
  (`screen.go:375-383`) gives the default color for `info`, yellow for `warn` and red for `error`. `accent` is blue
  (`screen.go:22`), and the `▌` of the selected row is drawn in it (`screen.go:396-403`).
- `screen.go:485-492`: `printable` shows each control character as `�`. Each text of a status command goes through
  it today: the summary, the text of a failure and its lines. ESC and the C1 controls are control characters, so this
  rule keeps an output from sending an escape sequence to the terminal.
- `internal/tui/box.go:21-31` cuts each line at the width of the pane with `…`, and draws only the lines that fit in
  the height of the pane.
- No code here draws dim text. `github.com/charmbracelet/x/ansi` has the SGR attribute faint, which terminals call
  dim (`tput dim`). The proposal uses two words: gray for the glyph of `not loaded` (`ansi.BrightBlack`,
  `screen.go:474-483`), and dim for `▱`.
- No code in `internal/tui` draws a time of day or depends on the time zone. The unit tests run in the zone of the
  machine: a Mac, or UTC in CI. The e2e tests run on a Mac and in Docker, and `openCli` gives svcview the environment
  that a test passes (`e2e/testUtils.ts:117-139`). Go reads `TZ=UTC` with no zoneinfo files.
- Tests: `runsGiving` fakes the runs and `detailPane` gives the lines of the detail pane
  (`internal/tui/screen_test.go:256-276`). `requireGolden` writes a golden file with `-update`
  (`screen_test.go:81-91`), and the golden files are in `internal/tui/testdata/`. The test of the lines of a failure
  with control characters is at `screen_test.go:1127-1143`. `e2e/tests/status.test.ts:11-21` opens svcview with a
  status file that `cat` prints.
- `docs/cli.md`: the table of the output says that `fields` and `events` show nothing (`docs/cli.md:271-278`); the
  table of the detail pane (`docs/cli.md:90-97`); the table of the glyphs and colors (`docs/cli.md:72-85`); the
  section "In the list and the detail pane" (`docs/cli.md:287-300`); the table of the parse errors
  (`docs/cli.md:326-339`).
- `tasks/learnings.md`, "Plan the layout change of the screen as the first task": Task 2 is the first task that
  changes the screen. It puts the lines of the fields under `Status`, and Tasks 3 and 4 add lines under them.
- Words: a field is one entry of `fields`, with a name, a value and an optional progress (`status.Field`). An event
  is one entry of `events` (`status.Event`). The bar has 10 cells, and a cell is done or not done. The time of an
  event is the hours and minutes of its `at`. The proposal uses "property" for an entry of the JSON object, so a
  field is not a property.

## Tasks

### Task 1: Refuse a progress outside 0 to 1 as a status invalid

**Behavior:** An output with a field whose `progress` is less than 0 or more than 1 is not a valid output. The run
is a status invalid, with a parse error that names the field by its JSON pointer and gives its progress, for example
`"/fields/2/progress" is 1.5, and it must be from 0 to 1`. The row shows `status invalid` in red, and the detail pane
shows the parse error and the first lines of the output, as for each other status invalid. `docs/cli.md` says that a
`progress` outside 0 to 1 makes the output invalid.

**Acceptance Criteria:**
- [x] An output with a field whose `progress` is 1.5 gives a status invalid with the parse error
      `"/fields/0/progress" is 1.5, and it must be from 0 to 1`, and no output.
- [x] A `progress` of -0.1, and a `progress` a little more than 1, such as 1.000001, give a status invalid with the
      same form of parse error.
- [x] When two fields have a `progress` outside 0 to 1, the parse error names the first of them in the order of the
      output, by its place in `fields`.
- [x] A `progress` of 0, of 1, and of each number between them gives an output with that progress.
- [x] `internal/status/command_test.go:429-437` checks the opposite today. It changes to check the status invalid.
- [x] In `docs/cli.md`, the table of the output says that `progress` is a number from 0 to 1, and that a `progress`
      outside 0 to 1 makes the output invalid. The sentence that says that svcview does not check the range is gone.
      The table of the parse errors has a row for a `progress` outside 0 to 1.
- [x] e2e test: a status command that prints an output with a field whose `progress` is 1.5 shows `status invalid`
      in the row, and the parse error that names the field in the detail pane.

**Affected Files/Modules:**
- `internal/status/output.go` — the check of the range of `progress`, with its parse error
- `internal/status/command_test.go` — the tests of the parse error, in place of the test of a `progress` outside 0 to 1
- `docs/cli.md` — the range of `progress` in the table of the output, and its parse error
- `e2e/tests/status.test.ts` — a `progress` outside 0 to 1

**Patterns to Follow:**
- `internal/status/output.go:101-106`
- `internal/status/output.go:116-124`
- `internal/status/command_test.go:338-391`
- `e2e/tests/status.test.ts:72-81`

**Testable:** Yes — through `status.Command.Run` with a command that prints the output, and in an e2e test.

**Certainty:** high — the parse error of `version` (`output.go:101-106`) gives the form, and the table test of the
parse errors (`command_test.go:338-391`) gives the test.

**Blast radius:** low — it narrows the output contract that the authors of status commands write to, but it makes
the contract what the proposal and the story give, and the row shows each refused output as `status invalid`.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** None

### Task 2: Show the fields of a status under Status, each with its name and its value

**Behavior:** After a run that gives an output with fields, the detail pane shows under the line `Status` one line for
each field, in the order of the output: the name of the field, and then its value. The values of one output start in
one column, two spaces after the widest name of that output. svcview shows each field in the same way, whatever its
name, so a service can add a field with no change to svcview.

**Acceptance Criteria:**
- [ ] An output with the fields `mode` `busy`, `ticket` `ENG-692`, `done` `14` and `failed` `1` shows four lines under
      the line `Status`, in this order: `mode    busy`, `ticket  ENG-692`, `done    14` and `failed  1`.
- [ ] The values of one output start in one column, two spaces after its widest name. Another output with another
      widest name moves the column.
- [ ] A field whose name svcview has never seen, and a field named `Status`, `Events`, `summary` or `progress`, show
      as their name and their value, as each other field does.
- [ ] A field with an empty value shows its name, and nothing after it.
- [ ] Each control character in a name or a value shows as `�`, and no escape sequence of a name or a value reaches
      the terminal.
- [ ] An output with no fields shows no line under `Status`. After a failure, the detail pane shows the text of the
      failure and its lines under `Status`, as before, and no field.
- [ ] When the output of the next run has other fields, the detail pane shows the fields of that run.
- [ ] A golden file shows the detail pane of a service with the fields of the proposal.
- [ ] e2e test: a status file with the fields `mode` `busy` and `ticket` `ENG-692` shows `mode    busy` and
      `ticket  ENG-692` in the detail pane.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the lines of the fields under `Status`
- `internal/tui/screen_test.go` — the tests of the lines of the fields
- `internal/tui/testdata/` — a new golden file of a detail pane with fields
- `e2e/tests/status.test.ts` — the fields of a status file in the detail pane

**Patterns to Follow:**
- `internal/tui/screen.go:335-350`
- `internal/tui/screen.go:405-427`
- `internal/tui/screen.go:485-492`
- `internal/tui/screen_test.go:1127-1143`
- `e2e/tests/status.test.ts:11-21`

**Testable:** Yes — through the `Screen` model with fake runs and golden files, and in an e2e test.

**Certainty:** high — the lines of a failure under `Status` (`screen.go:417-425`) and the column of the summaries
(`screen.go:335-350`) are the precedent.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** None

### Task 3: Show a bar of 10 cells under the value of a field with a progress

**Behavior:** Under the value of a field with a `progress`, the detail pane shows a bar of 10 cells, on a line of its
own that starts in the column of the values. The done cells are `▰` in the accent color, and the other cells are a
dim `▱`. The number of done cells is the progress times 10, rounded down, so only a progress of 1 fills the bar.

**Acceptance Criteria:**
- [ ] A field `step` with the value `review · 12m / 20m` and a `progress` of 0.6 shows its value, and on the next
      line, in the column of the values, 6 `▰` and then 4 `▱`.
- [ ] A progress of k tenths shows k `▰` and 10 − k `▱`, for each k from 0 to 10.
- [ ] The number of done cells is rounded down: 0.65 shows 6 `▰`, 0.99 shows 9 `▰`, 0.05 shows no `▰`, and only 1
      shows 10 `▰`.
- [ ] Each `▰` is in the accent color, blue. Each `▱` is dim: the SGR attribute faint, in the default color of the
      terminal.
- [ ] A field with no `progress` has no line for a bar, and the field after it follows its value.
- [ ] A golden file shows the detail pane of a service with the fields of the proposal, with the bar under `step`.
- [ ] e2e test: a status file with a field whose `progress` is 0.6 shows `▰▰▰▰▰▰▱▱▱▱` on the line under its value.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the bar under the value of a field with a progress
- `internal/tui/screen_test.go` — the tests of the bar
- `internal/tui/testdata/` — a golden file of the bar
- `e2e/tests/status.test.ts` — the bar of a field from a status file

**Patterns to Follow:**
- `internal/tui/screen.go:396-403`
- `internal/tui/screen.go:461-472`
- `task:2`

**Testable:** Yes — through the `Screen` model with fake runs and golden files, and in an e2e test.

**Certainty:** medium — glyphs in a color (`screen.go:461-472`) and the accent `▌` (`screen.go:396-403`) are the
precedent, but no code here turns a number into a count of cells, and no code draws dim text.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1, Task 2

### Task 4: Show the events of a status under Events

**Behavior:** After the fields, the detail pane shows an empty line, the line `Events`, and one line for each event of
the output, in the order of the output. Each line has the time of its `at` as hours and minutes in the local time
zone, a `●` in the color of its `level`, and its `text`, for example `09:04 ● step review`.

**Acceptance Criteria:**
- [ ] An output with the events `step review` at 09:04:40 and `claimed ENG-692` at 09:00:12, in the local time zone,
      shows under the fields an empty line, the line `Events`, `09:04 ● step review` and `09:00 ● claimed ENG-692`, in
      the order of the output.
- [ ] The time has two digits for the hours, 0 to 23, and two digits for the minutes.
- [ ] An `at` in another time zone shows the hours and minutes of the same time in the local time zone.
- [ ] An event of another day shows only its hours and minutes.
- [ ] The `●` of an event is in the default color for `info`, yellow for `warn`, and red for `error`, as the summary
      of each level.
- [ ] Each control character in the text of an event shows as `�`, and no escape sequence of a text reaches the
      terminal.
- [ ] An output with no events shows no line `Events`, and no empty line for it. An output with events and no fields
      shows the empty line and `Events` under the line `Status`.
- [ ] A golden file shows the detail pane of a service with the fields and the events of the proposal.
- [ ] e2e test: a status file with two events shows each under `Events`, in the order of the file, with its time in
      the time zone that the test gives svcview.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the line `Events` and the line of each event
- `internal/tui/screen_test.go` — the tests of the events
- `internal/tui/testdata/` — a golden file of the events
- `e2e/tests/status.test.ts` — the events of a status file in the detail pane

**Patterns to Follow:**
- `internal/tui/screen.go:375-383`
- `internal/tui/screen.go:461-464`
- `task:2`

**Testable:** Yes — through the `Screen` model with fake runs and golden files, and in an e2e test.

**Certainty:** medium — the color of a level (`screen.go:375-383`) and a glyph in a color (`screen.go:461-464`) are
the precedent, but no code in `internal/tui` draws a time of day, and the tests must not depend on the time zone of
the machine.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes on this Mac and in Docker.

**Depends on:** Task 2

### Task 5: Describe the fields, the bars and the events in docs/cli.md

**Behavior:** `docs/cli.md` describes what the detail pane shows of an output: the fields with their bars, and the
events. The author of a status command can tell from the docs how each property of the output shows on the screen.

**Acceptance Criteria:**
- [ ] The table of the output says, for `fields` and for `events`, what the detail pane shows, in place of `Nothing`.
- [ ] The section "In the list and the detail pane" says that the detail pane shows each field under `Status` in the
      order of the output, with the values in one column, and that svcview shows each field in the same way, whatever
      its name, so a service can add a field with no change to svcview.
- [ ] It says that the bar has 10 cells, that the number of `▰` is the progress times 10 rounded down, and that only a
      progress of 1 fills the bar.
- [ ] It says that each event shows under `Events` in the order of the output, with the hours and minutes of its `at`
      in the local time zone and no date, a `●` in the color of its `level`, and its `text`.
- [ ] It says that each control character in a name, a value or a text shows as `�`.
- [ ] The table of the detail pane, and the table of the glyphs and colors, have the fields, the bar (`▰` blue, `▱`
      dim) and the `●` of an event in the color of its level.

**Affected Files/Modules:**
- `docs/cli.md` — the fields, the bars and the events in the detail pane

**Patterns to Follow:**
- `docs/cli.md:72-97`
- `docs/cli.md:271-300`

**Testable:** No — a document.

**Certainty:** high — the tables and the section of the summary in `docs/cli.md:72-97` and `docs/cli.md:271-300` are
the precedent, and the proposal gives the look of each part ("Colors and Glyphs").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the screen of `task cli` with a status command that prints the
output of the proposal.

**Depends on:** Task 3, Task 4

## Summary

- 5 tasks. Task 1 adds back the check of the range of `progress`, in `internal/status` and in `docs/cli.md`, before
  any code draws a progress. Task 2 is the first change to the screen: the lines of the fields under `Status`. Task 3
  adds the bar under a value, and Task 4 adds the events under the fields. The document comes last.
- Covers the criteria on the fields (Task 2), the bar (Task 3), a `progress` outside 0 to 1 (Task 1) and the events
  (Task 4). Task 5 describes them for the authors of status commands.
- Certainty: Tasks 1, 2 and 5 repeat a pattern of the repo. Tasks 3 and 4 are medium: the count of the cells of a
  bar, the dim text, and a time of day in the local time zone have no precedent here. No task has a high blast
  radius. The texts from the output reach the terminal only through the rule that shows each control character as
  `�`, and Tasks 2 and 4 check it for each new text.
- The planner chose these details, which the story does not give: the parse error of a `progress` outside 0 to 1
  names the first such field; the values of one output start two spaces after its widest name, and the bar starts in
  that column, as in the screen of the proposal; the number of done cells is rounded down, so that a full bar means a
  job that is done; dim is the SGR attribute faint, not the gray of `not loaded`; the line `Events` shows only when
  the output has events; the time of an event has no date.
