## Contents
1. Task 1: Judge a result stale when its at is older than an age
2. Task 2: Read status.stale_after from the status of a group
3. Task 3: Show a stale status dim in its row, with stale and the age of at after it
4. Task 4: Keep the stale mark in view when the summary is too long for the row
5. Task 5: Describe status.stale_after and the stale mark in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-007 "See when a status is stale" (`launchd-tui/US-007`).
Proposal: `docs/proposal-launchd-tui.md`, the row "A stale status" of "Colors and Glyphs", `status.stale_after` in
"Settings of a Group", `at` in "Output", and the row of a stale `at` in "Failures".

A worker that crashes or does not respond stops the writes to its status file, but `cat` still prints the last file,
so the list shows `busy` for a worker that does nothing. A group of the config can set `status.stale_after`, the age
of `at` after which svcview does not trust a status. When the `at` of the last output of a service is older than
`stale_after`, the row of the service shows the summary dim, and then `stale` with the age of `at`, for example
`busy  stale 3m`. svcview runs the status command in each state of a service, so a service that is `not loaded` shows
the mark too when its status file is old. When the service writes a new `at`, the mark goes away at the end of the
next run. A group with no `stale_after`, and an output with no `at`, never show the mark.

The story criteria that this change satisfies:

- `status.stale_after` in a group sets the age of `at` after which a status is stale. Without this setting, no status
  of the group is stale.
- The row of a stale status shows the summary dim, with the age of `at` after it, for example `stale 3m`.
- An output with no `at` is never stale.
- When the service writes a new `at`, the mark goes away at the next run of the status command.
- svcview runs the status command for a service that is `not loaded` too. When its `at` is older than `stale_after`,
  the row shows the status as stale.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No work in the worker repo. The worker writes `at` into its status file, and its install script writes
  `stale_after` into `conf.d/worker.yaml`. That work is outside svcview.
- The detail pane does not change. It shows the age of `at` on the line `Status`, for example `Status 3m12s ago`, as it
  does for each status, with no stale mark and no dim. That line is what the proposal asks the detail pane to show for
  a stale status.
- No default `stale_after`. A group with no `stale_after` has no check, and no status of it is stale.
- A run that gives a failure is not stale. The row shows the failure in red in place of the summary, as it does now.
- A run that gives an output with no `at` is not stale, whatever the age of the run.
- An `at` after the current time is not stale.
- svcview does not change when it runs the status command for a stale status. The runs keep the `every` of the group.
- `svcview status <name>` does not read `stale_after`, and does not say whether a status is stale. It prints `at` as
  it does now.
- The header does not count stale services, and the help line and the help show no new key.
- No `NO_COLOR`. The dim is the faint attribute of the terminal, as for the `▱` of a bar. The mark is text, so it shows
  the stale status with no color too.
- No change to the rules of the status command: the template values, the parse of the output, the failures and their
  texts stay as they are.

**Deferred:** None.

## Codebase Context

- `internal/config/group.go:83-115`: `readStatus` reads the status of a group. `checkSettings` (`group.go:88`) refuses a
  setting that is not `command`, `every` or `timeout`, and a setting given two times. `every` and `timeout` go through
  `readDuration` (`internal/config/config.go:219-225`), which refuses a value that is not a Go duration more than 0, with
  an error that names the line and the setting in quotes. The error of `readStatus` gets the file from
  `readConfigFile`. `config.Read` gives each group its status as a `*status.Command`.
- `internal/config/config_test.go:680-690` today checks that `stale_after` in a status is a setting that svcview does
  not know. This change turns that check around, so the test changes in the same task.
- `internal/config/config_test.go:573-582` compares the whole `status.Command` of a group with one that
  `status.NewCommand` makes. A new field with the value 0 keeps these tests green.
- `internal/status/command.go:31-35`: `Command` holds its templates and the exported `Every` and `Timeout`.
  `status.NewCommand` (`command.go:37-61`) has 17 callers in five packages and their tests; this change does not add a
  parameter to it.
- `internal/status/output.go:58-64`: `Output.At` is a `*time.Time`, nil when the output has no `at`.
  `internal/status/result.go:12-17`: `Result.Time` gives the `at` of the output, or the time at which the run ended.
  A run with a failure has no output (`command.go:81-96`).
- `internal/tui/screen.go:244-253`: `runStatuses` runs the command of each row of the group, whatever its state.
  `internal/tui/screen_test.go:779-826` checks that a `not loaded` service gets its runs. So the first half of the
  criterion about `not loaded` holds today, and this change adds the mark for such a service.
- `internal/tui/screen.go:440-525`: `listText` draws each row from `rowText` and `statusText`, which give the summary
  and the color of its level (`levelColor`). `selectedRow` draws the selected row on a `BrightBlack` background, with
  the summary in the color of its level. The summaries of one group start in one column, two spaces after the widest
  short name (`nameWidths`).
- `internal/tui/box.go:22-33`: `box.render` cuts each line at the width of the pane with `…`, at its end. A mark after a
  long summary is cut away by this, so the row itself must cut the summary to keep the mark.
- `internal/tui/screen.go:596-615`: `statusAge` is the time since `Result.Time`, never below 0, and `durationText` gives
  it in two units (`3m12s`). The detail pane draws `Status <age> ago` at each draw, so the age grows with no new run.
  The screen draws again at each message, at the latest at each refresh (2 s by default).
- `internal/tui/screen_test.go`: `runSized` (`:220-240`) runs the real program on the fake clock of `synctest`;
  `runsGiving` (`:310-319`) and a `run` func give results; `listRows` (`:136-146`) gives the rows with no styles;
  `requireGolden` (`:83-93`) compares the screen with a file in `internal/tui/testdata`. `statusEvery` (`:256-261`)
  makes a command with an `every`.
- `e2e/tests/status.test.ts:7-8` writes the config of a group with a status command; `:88-101` changes the status file
  with a rename and waits for the new row with no key press. The e2e fake `launchctl` lists no service by default, so a
  service with only a plist is `not loaded`. `openCli` gives svcview 160 columns.
- `docs/cli.md`: "Glyphs and colors" (`:81-101`), "The status command" (`:328-362`), the table of properties
  (`:423-431`), "In the list and the detail pane" (`:439-485`), and the errors in `status` (`:680-686`).
- `tasks/learnings.md`: "Prefix the constants of an enum in internal/status with its type", "A word of the subtest name
  is in every t.TempDir path", "Test a Bubble Tea timer on the fake clock of synctest", "internal/tui draws its boxes in
  box.go, with no Lip Gloss", "Text from outside goes to the terminal through terminal.ReplaceControlCharacters",
  "Fold the fixes of two tasks in one file with one clerk fixup mark for each task in turn" (Tasks 3 and 4 both change
  `screen.go` and `screen_test.go`), and "Run gofmt before clerk finish" (Task 2 adds a field to an aligned struct).

## Tasks

### Task 1: Judge a result stale when its at is older than an age

**Behavior:** `status.Result` gains `Stale`, which says whether the result is stale at a given time for a given age
limit: a result is stale only when its output has an `at` and the age of that `at` is more than the limit. A limit of 0
means no check.

**Acceptance Criteria:**
- [x] A result whose output has an `at` older than the limit is stale.
- [x] A result whose output has an `at` exactly as old as the limit, or newer, is not stale; an `at` after the given
      time is not stale.
- [x] A result whose output has no `at` is never stale, whatever the time at which the run ended.
- [x] A result with a failure, and a result with no output, are never stale.
- [x] With a limit of 0, no result is stale, also one whose `at` is days old.
- [x] The tests are a `stale` group under the umbrella `TestResult`, beside the group `time`.

**Affected Files/Modules:**
- `internal/status/result.go` — `Result.Stale`
- `internal/status/result_test.go` — the group `stale` of `TestResult`

**Patterns to Follow:**
- `internal/status/result.go:12-17`
- `internal/status/result_test.go:13-35`
- `internal/status/output.go:58-64`

**Testable:** Yes

**Certainty:** high — `Result.Time` at `internal/status/result.go:12-17` reads the same optional `at`, with its tests in
`result_test.go:13-35`.

**Blast radius:** low

**Verification:** `task test:unit` passes.

**Depends on:** None

### Task 2: Read status.stale_after from the status of a group

**Behavior:** The status of a group accepts `stale_after`, a Go duration more than 0, and gives it to the status
command of the group as `Command.StaleAfter`. Without it, `StaleAfter` is 0, so the group has no check.

**Acceptance Criteria:**
- [x] A group whose status sets `stale_after: 30s` gives a status command with a `StaleAfter` of 30 s, with its
      program, its arguments, its `every` and its `timeout` as before.
- [x] A group whose status has no `stale_after` gives a status command with a `StaleAfter` of 0, and so does a status
      with `every` and `timeout` but no `stale_after`.
- [x] A `stale_after` that is not a Go duration more than 0 (for example `soon`, `0s`, `-5s`, an empty value, or a
      list) gives an error that names the file, the group and `"stale_after"`, and no settings, as for `every` and
      `timeout`.
- [x] A `stale_after` that the status sets two times gives the error that names the file, the group and the line of
      the first, as for the other settings of the status.
- [x] The test at `internal/config/config_test.go:680-690`, which checks today that svcview does not know
      `stale_after`, checks other settings that svcview does not know, and no longer names `stale_after`.
- [x] A `stale_after` in a file of `conf.d` is read as in `config.yaml`.
- [x] Each check of a word in an error uses the setting in quotes, so the name of the `t.TempDir` path cannot make it
      pass.

**Affected Files/Modules:**
- `internal/status/command.go` — the field `StaleAfter` of `Command`
- `internal/config/group.go` — `readStatus` knows and reads `stale_after`
- `internal/config/config_test.go` — the tests of `stale_after` in the group `read` of `TestSettings`

**Patterns to Follow:**
- `internal/config/group.go:83-115`
- `internal/config/config.go:219-225`
- `internal/config/config_test.go:573-588`
- `internal/config/config_test.go:645-666`
- `internal/config/config_test.go:680-706`
- `internal/status/command.go:31-35`

**Testable:** Yes

**Certainty:** high — `every` and `timeout` are read the same way at `internal/config/group.go:100-109`, with the tests at
`internal/config/config_test.go:645-706`.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `gofmt -l internal` prints nothing.

**Depends on:** None

### Task 3: Show a stale status dim in its row, with stale and the age of at after it

**Behavior:** In a group whose status command has a `StaleAfter`, the row of a service whose last result is stale at
the time of the draw shows the summary dim, in the color of its level, and then two spaces and the stale mark: `stale`
and the age of `at`, for example `busy  stale 3m`. The mark is drawn at each draw, so it shows with no new run once
`at` gets older than `stale_after`, and it goes away at the end of the first run whose output has a newer `at`.

**Acceptance Criteria:**
- [ ] With a `StaleAfter` of 30 s, the row of a service whose last output has the summary `busy` and an `at` 3 minutes
      old shows `busy  stale 3m`: the summary dim, then two spaces, then the mark in the default color of the terminal.
- [ ] The age in the mark is the age of `at` in its largest whole unit, rounded down: `45s`, `3m` for 3 minutes and 12
      seconds, `2h`, `1d`. The detail pane keeps `Status 3m12s ago`.
- [ ] A dim summary keeps the color of its level: dim in the default color for `info`, dim yellow for `warn`, dim red
      for `error`. The mark is in the default color for each level.
- [ ] On the selected row, the dim summary and the mark are on the background of the selected row.
- [ ] An output with no summary and a stale `at` shows only the mark, in the column of the summaries of the group.
- [ ] The summaries of one group still start in one column, stale or not.
- [ ] These rows show the summary as before, with no dim and no mark: an output with no `at`, also when the run ended
      long ago; an `at` that is not older than `stale_after`; a failure, which stays in red in place of the summary; a
      service of a group whose status command has no `StaleAfter`; a service of a group with no status command.
- [ ] A `running` service and a `not loaded` service whose last outputs have an `at` older than `stale_after` each
      show the mark.
- [ ] With an `at` 20 s old at the end of the run and a `stale_after` of 30 s, the row shows no mark at first, and
      shows the mark after more than 10 s more, while the runs give the same `at`; the age in the mark grows as the
      screen draws again.
- [ ] After a run that gives a stale `at`, the next run whose output has an `at` newer than `stale_after` makes the
      row show its summary with no dim and no mark, at the end of that run.
- [ ] A golden file `status-stale` in `internal/tui/testdata` shows a stale row of each level, a row that is not stale,
      and the selected row stale.
- [ ] e2e: with `stale_after: 30s` in the status of the group and a status file whose `at` is 5 minutes old, the row of
      the `not loaded` service shows `01  busy  stale 5m`; after a rename puts a file with a new `at` and the summary
      `idle` in its place, the row shows `01  idle` with no mark, with no key press.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the row draws the dim summary and the stale mark from the last result and the
  `StaleAfter` of the group
- `internal/tui/screen_test.go` — the tests of the stale mark in the group `status` of `TestScreen`
- `internal/tui/testdata/status-stale.golden` — new golden file
- `e2e/tests/status.test.ts` — the e2e test of the stale mark and of its end

**Patterns to Follow:**
- `internal/tui/screen.go:440-525`
- `internal/tui/screen.go:596-615`
- `internal/tui/screen_test.go:803-826`
- `internal/tui/screen_test.go:930-968`
- `internal/tui/screen_test.go:1159-1238`
- `e2e/tests/status.test.ts:7-21`
- `e2e/tests/status.test.ts:88-101`
- `task:1`
- `task:2`

**Testable:** Yes

**Certainty:** medium — the row draws a summary in the color of its level at `internal/tui/screen.go:470-525`, and the
detail pane draws an age at each draw at `screen.go:547-548`, but no row draws two styled parts after the short name,
and no text shows an age in one unit.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` passes the test of the stale mark.

**Depends on:** Task 1, Task 2

### Task 4: Keep the stale mark in view when the summary is too long for the row

**Behavior:** When a stale summary, the two spaces and the mark are wider than the list pane, the row cuts the
summary with `…` so that the mark shows in full at the end of the row. A row with no mark is cut as before, at its end.

**Acceptance Criteria:**
- [ ] In a terminal of 80 columns, a stale row whose summary is `busy · ENG-692 · 12m` shows a cut summary that ends
      with `…`, then two spaces and the whole mark, for example `stale 3m`, and the line fits in the pane.
- [ ] The cut summary stays dim, in the color of its level, also on the background of the selected row.
- [ ] Each control character of a cut summary shows as `�`, and no control character reaches the terminal.
- [ ] A row that is not stale and whose summary is too long is cut at its end with `…`, as before.
- [ ] A stale row whose summary fits is not cut.
- [ ] In a pane too narrow for the short name and the mark, the line is cut at its end with `…`, as any row.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the row cuts the summary of a stale status to make room for the mark
- `internal/tui/screen_test.go` — the tests of the cut in the group `status` of `TestScreen`

**Patterns to Follow:**
- `internal/tui/box.go:22-33`
- `internal/tui/screen.go:470-477`
- `internal/tui/screen.go:518-525`
- `internal/tui/screen_test.go:1047-1060`
- `task:3`

**Testable:** Yes

**Certainty:** medium — `box.render` cuts a line with `…` at `internal/tui/box.go:28`, tested at
`internal/tui/screen_test.go:1047-1060`, but no row cuts one part of its text to keep a later part in view.

**Blast radius:** low

**Verification:** `task test:unit` passes.

**Depends on:** Task 3

### Task 5: Describe status.stale_after and the stale mark in docs/cli.md

**Behavior:** `docs/cli.md` tells a user how to set `status.stale_after`, when svcview marks a status as stale, and
what the row of a stale status shows.

**Acceptance Criteria:**
- [ ] The example of "The status command" sets `stale_after: 30s`, and its table of settings has `status.stale_after`:
      the age of `at` after which svcview marks the status as stale, a Go duration more than 0, with the default "No
      check".
- [ ] "Glyphs and colors" has a row for a stale status: the summary dim in the color of its level, with the mark in
      the default color after it.
- [ ] The table of properties says that svcview compares `at` with `stale_after`.
- [ ] "In the list and the detail pane" says when a status is stale (an `at` older than `stale_after`; never without
      `at`, after a failure, or in a group with no `stale_after`), that the mark shows `stale` and the age of `at` in its
      largest unit, that it shows in each state of the service, also `not loaded`, that it shows and grows with no new
      run, that it goes away at the end of the first run with a newer `at`, and that the row cuts a long summary to keep
      the mark. It says that the detail pane shows the age of `at` on the line `Status`.
- [ ] The errors in `status` name `stale_after` beside `every` and `timeout`.
- [ ] Each statement matches what Tasks 1 to 4 built.

**Affected Files/Modules:**
- `docs/cli.md` — "Glyphs and colors", "The status command", "Output", "In the list and the detail pane", and
  "Errors"

**Patterns to Follow:**
- `docs/cli.md:81-101`
- `docs/cli.md:328-362`
- `docs/cli.md:423-456`
- `docs/cli.md:680-686`

**Testable:** No

**Certainty:** high — each earlier change to the status command described itself in these sections of
`docs/cli.md:328-536`.

**Blast radius:** low

**Verification:** Manual check of `docs/cli.md` against the tests of Tasks 1 to 4.

**Depends on:** Task 4

## Summary

- 5 tasks.
- Order: the rule first (Task 1) and the setting (Task 2), which have no effect on the screen alone but are each
  tested through their public API; then the row (Task 3), where both meet and the e2e test runs the real config file;
  then the cut of a long summary (Task 4), which needs the mark of Task 3; then the docs (Task 5). Tasks 1 and 2 do not
  depend on each other.
- No task changes the layout of the screen: no pane, line or height changes, so no existing golden file changes. Task 3
  adds one golden file.
- Criteria: the first (`stale_after` sets the age, no check without it) in Tasks 1, 2 and 3; the second (the dim
  summary and the mark) in Tasks 3 and 4; the third (no `at`, never stale) in Tasks 1 and 3; the fourth (the mark goes
  away at the next run) in Task 3 and its e2e test; the fifth (`not loaded`) in Task 3 and its e2e test, whose service
  is `not loaded`. None is deferred.
- The story is routine on the blast-radius side: no task touches a check of who can write a config file, a command
  that svcview runs, or text from outside that bypasses `terminal.ReplaceControlCharacters`.
