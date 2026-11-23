## Contents
1. Task 1: Show a status command that exits with a code other than 0
2. Task 2: Stop a status command that runs longer than its timeout, with each process that it started
3. Task 3: Show an output that is not a JSON object of version 1
4. Task 4: Describe the failures of a status command in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-003 "See the status that each service reports" (`launchd-tui/US-003`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Status Command" (Output, Failures) and "The Screen".

This change shows why a status command failed, so that a command that fails or does not end never leaves a row
empty, and the user never trusts a status that is not true. When the command exits with a code other than 0, the row
shows `status error` in red, and the detail pane shows the exit code and the last 20 lines of stderr. When the
command runs longer than `status.timeout`, svcview stops the command and each process that it started, the row shows
`status timeout` in red, and the detail pane shows the command line. When the output is not a JSON object of version
1, or is larger than 1 MiB, the row shows `status invalid` in red, and the detail pane shows the parse error and the
first 20 lines of the output. `docs/cli.md` describes each failure, so that the author of a program can write a
status command from the docs, with no read of the svcview code.

The story criteria that this change satisfies:

- When the command exits with a code other than 0, the row shows `status error` in red. The detail pane shows the
  exit code and the last 20 lines of stderr.
- When the command runs longer than `status.timeout` (default `1s`), svcview stops the command and each process that
  it started. The row shows `status timeout` in red, and the detail pane shows the command line.
- When the output is not a JSON object of version 1, or is larger than 1 MiB, the row shows `status invalid` in red.
  The detail pane shows the parse error and the first 20 lines of the output.
- `docs/cli.md` describes the status settings, the template values, and the output contract. The author of a program
  can write a status command from these docs, with no read of the svcview code.

`docs/cli.md` already describes the status settings, the template values and the output of a run that succeeds.
This change adds the failures, which completes the last criterion.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none. The row and the detail pane show only the last run of
  a service that ended.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No work in the worker repo. The worker writes its status file, and its install script adds `WORKER_STATUS_FILE`
  to the plist and writes `conf.d/worker.yaml`. That work is outside svcview.
- A process that leaves the process group of the command, for example with `setsid`, does not stop at the timeout.
- The detail pane does not scroll. It shows the lines of a failure that fit in it, and cuts each line at its width.
- The detail pane does not show the `fields` or the `events` of an output, and a `progress` outside 0 to 1 does not
  make the output invalid.
- No mark of a stale status. `stale_after` stays a setting that svcview does not know.
- No command of the CLI runs the status command of one service.
- `NO_COLOR` does not remove the red of a failure, as it does not remove the other colors of the screen.

**Deferred:** None.

## Codebase Context

- `internal/status/command.go`: `status.Command.Run(ctx, service)` applies the template values, runs the program
  with no shell, stops it at the timeout of the command, reads no more than 1 MiB of stdout, and parses a JSON object
  of version 1 (`internal/status/output.go`). It gives a `status.Result` with the time at which the run ended, and an
  `Output`, a `Failure`, or neither. Today a `Failure` is only a template value that the service does not have, with
  its text; a program that exits with a code other than 0, that runs longer than the timeout, or whose output is not
  valid gives a result with neither, and svcview keeps no stderr and no text of the output.
- `internal/tui/screen.go`: the screen runs the status command of each service of a group every `status.every`, one
  run at a time, and stops the runs that still run when svcview quits. The row shows the `summary` after the short
  name in the color of its `level`, or the text of a failure in red. The detail pane shows the line `Status` with the
  age. The tests give the screen fake runs, on the fake clock of `synctest`; the golden files are in
  `internal/tui/testdata/`. `e2e/tests/status.test.ts` tests the status command with real programs.
- `internal/launchd/launchctl_list.go:24-41` reads the stderr of an exit error and gives it in the error.
  `launchctl_list.go:43-72` gives a parse error that names what is wrong. `internal/tui/screen.go:251-269` shows each
  line of an error on a row of its own, with each control character as `�`, and `screen.go:310-319` draws red text.
- No code in the repo starts a program in a process group of its own, or sends a signal to a process group. A
  signal to the process ID 0 goes to the process group of svcview itself, and a signal to -1 goes to each process
  of the user, so the stop must signal only the group that svcview started for the command.
- A change to the detail pane rewrites the same lines of `screen.go` and `screen_test.go` (`tasks/learnings.md`,
  "Plan the layout change of the screen as the first task"), so the first task puts the lines of a failure under
  `Status`, and the next tasks fill them.
- Words: "failure" is why a run gave no output; the row words are `status error`, `status timeout` and
  `status invalid`; the "command line" is the program and its arguments after svcview applies the template values;
  the "parse error" says why the output is not a JSON object of version 1. The names of the kinds of failure must
  not clash with the levels `info`, `warn` and `error` of the output.

## Tasks

### Task 1: Show a status command that exits with a code other than 0

**Behavior:** A run whose program exits with a code other than 0, or that svcview cannot start, is a status error.
svcview keeps the exit code, or the signal that ended the program, or why it could not start, and the last 20 lines
of stderr. The row shows `status error` in red. Under `Status`, the detail pane shows the exit code and the lines of
stderr.

**Acceptance Criteria:**
- [x] A program that exits with 3 gives a status error with the exit code 3, also when it prints a valid output.
- [x] svcview keeps only the last 20 lines of stderr, also for a program that writes many more.
- [x] A program that a signal ends gives a status error with the name of the signal.
- [x] A program that svcview cannot start, for example one that is not on the `PATH`, gives a status error that says
      why.
- [x] The row shows `status error` in red in place of the summary.
- [x] Under the line `Status`, the detail pane shows the exit code, or the signal, or why the program could not
      start, and then the lines of stderr. Each control character shows as `�`.
- [x] e2e test: a status command `[sh, -c, 'echo broken >&2; exit 3']` shows `status error` in the row, and the exit
      code 3 and `broken` in the detail pane.

**Affected Files/Modules:**
- `internal/status/command.go` — the status error, with its exit code and the last lines of stderr
- `internal/status/command_test.go` — the tests of the status error
- `internal/tui/screen.go` — `status error` in the row, and the lines of a failure under `Status`
- `internal/tui/screen_test.go` — the tests of the row and the detail pane
- `internal/tui/testdata/` — new golden files of a status error
- `e2e/tests/status.test.ts` — the status error

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:24-41`
- `internal/tui/screen.go:251-269`
- `internal/tui/screen.go:310-319`

**Testable:** Yes — through `status.Command.Run` with small commands, through the `Screen` model with fake runs, and
in an e2e test.

**Certainty:** medium — `readLaunchctlList` (`launchctl_list.go:24-41`) reads the stderr of a program that exits
with an error, but nothing here keeps only the last lines of a stderr of any size.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** None

### Task 2: Stop a status command that runs longer than its timeout, with each process that it started

**Behavior:** svcview starts each status command in a process group of its own. At the timeout of the command,
svcview stops the process group, so each process that the command started stops too. The run is a status timeout,
with the command line. The row shows `status timeout` in red, and under `Status` the detail pane shows the command
line. When svcview quits, it stops the process group of each status command that still runs.

**Acceptance Criteria:**
- [x] A command that runs longer than its `timeout` gives a status timeout no later than 1 s after the timeout.
- [x] A process that the command started, for example the second `sleep` of `sh -c 'sleep 30 & sleep 30'`, no longer
      runs after the timeout.
- [x] svcview sends a signal only to the process group that it started for the command, never to its own process
      group and never to each process of the user.
- [x] When svcview quits, the process group of each status command that still runs stops.
- [x] The row shows `status timeout` in red in place of the summary.
- [x] Under the line `Status`, the detail pane shows the command line: the program and its arguments after svcview
      applied the template values.
- [x] e2e test: a status command `[sleep, '30']` with a `timeout` of `500ms` shows `status timeout` in the row and
      `sleep 30` in the detail pane.

**Affected Files/Modules:**
- `internal/status/command.go` — the process group of the command, its stop, and the status timeout
- `internal/status/command_test.go` — the tests of the stop and of the status timeout
- `internal/tui/screen.go` — `status timeout` in the row, and the command line in the detail pane
- `internal/tui/screen_test.go` — the tests of the row and the detail pane
- `internal/tui/testdata/` — new golden files of a status timeout
- `e2e/tests/status.test.ts` — the status timeout

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:24-41`
- `task:1`

**Testable:** Yes — through `status.Command.Run` with commands that start other processes and print their process
IDs, through the `Screen` model with fake runs, and in an e2e test.

**Certainty:** low — no code here starts a program in a process group of its own or sends a signal to a process
group; this task adds both.

**Blast radius:** high — a signal to a wrong process group stops processes that svcview did not start: the process
ID 0 is the group of svcview, and -1 is each process of the user.

**Verification:** `mise exec -- task test:unit` passes on this Mac and in CI, and `task test:e2e` passes.

**Depends on:** Task 1

### Task 3: Show an output that is not a JSON object of version 1

**Behavior:** A run whose output is not a JSON object of version 1, or is larger than 1 MiB, is a status invalid,
with the parse error and the first 20 lines of the output. The row shows `status invalid` in red, and under `Status`
the detail pane shows the parse error and the lines of the output.

**Acceptance Criteria:**
- [x] Each output that is not valid gives a status invalid with a parse error that says why: an output that is not
      JSON; a JSON value that is not an object; a `version` other than 1, or none; a `level` other than `info`,
      `warn` or `error`; an `at` that is not an RFC 3339 time; a `summary` that is not a text; `fields` or `events`
      whose shape is not that of version 1; a second JSON value after the object; an empty output.
- [x] An output larger than 1 MiB gives a status invalid that says that the output is larger than 1 MiB.
- [x] svcview keeps only the first 20 lines of an output that is not valid.
- [x] The row shows `status invalid` in red in place of the summary.
- [x] Under the line `Status`, the detail pane shows the parse error and then the lines of the output. Each control
      character shows as `�`.
- [x] e2e test: a status command `[printf, 'busy']` shows `status invalid` in the row, and the parse error and
      `busy` in the detail pane.

**Affected Files/Modules:**
- `internal/status/output.go` — the parse error of an output that is not valid
- `internal/status/command.go` — the status invalid, with the first lines of the output
- `internal/status/command_test.go` — the tests of the status invalid
- `internal/tui/screen.go` — `status invalid` in the row, and the parse error and the lines in the detail pane
- `internal/tui/screen_test.go` — the tests of the row and the detail pane
- `internal/tui/testdata/` — new golden files of a status invalid
- `e2e/tests/status.test.ts` — the status invalid

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:43-72`
- `internal/tui/screen.go:251-269`
- `task:1`

**Testable:** Yes — through `status.Command.Run` with small commands that print each output, through the `Screen`
model with fake runs, and in an e2e test.

**Certainty:** high — the parse of `launchctl list` names what is wrong in its error (`launchctl_list.go:43-72`), and
Task 1 shows the lines of a failure.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1

### Task 4: Describe the failures of a status command in docs/cli.md

**Behavior:** `docs/cli.md` describes each failure of a status command, what the row and the detail pane show for
it, and the limits of a run. With the sections on the status settings, the template values and the output, the
author of a program can write a status command from the docs, with no read of the svcview code.

**Acceptance Criteria:**
- [x] A table lists each failure: a program that exits with a code other than 0 or cannot start, a run longer than
      `status.timeout`, and an output that is not valid or is larger than 1 MiB, with what the row and the detail
      pane show for each.
- [x] It says that svcview stops the process group of the command at the timeout, and that a process that leaves
      that group does not stop.
- [x] It says that svcview keeps the last 20 lines of stderr and the first 20 lines of an output that is not valid.
- [x] It says that a program that exits with a code other than 0 shows `status error` also when it prints a valid
      output.
- [x] The glyphs and colors table has the red of a failure.

**Affected Files/Modules:**
- `docs/cli.md` — the failures of a status command

**Patterns to Follow:**
- `docs/cli.md:72-111`

**Testable:** No — a document.

**Certainty:** high — the tables of `docs/cli.md:72-111` are the precedent, and the proposal gives the table of the
failures ("Failures").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the behaviour of `task cli` with a status command that exits with
3, one that runs `sleep 30`, and one that prints a text that is not JSON.

**Depends on:** Task 2, Task 3

## Summary

- 4 tasks. Task 1 puts the lines of a failure under `Status`, which is the layout change of this change, with the
  status error. Task 2 adds the process group and the status timeout, and Task 3 the status invalid, each with its
  row and its detail pane. The document comes last.
- Covers the criteria on a status error (Task 1), a status timeout (Task 2) and a status invalid (Task 3), and
  completes the criterion on `docs/cli.md` (Task 4).
- Task 2 is `low` certainty and `high` blast radius: it is the first code that starts a process group and sends a
  signal to it, and a wrong group stops processes that svcview did not start.
- The planner chose these details, which the story does not give: a program that svcview cannot start is a status
  error; a program that a signal ends shows the name of the signal; an exit code other than 0 wins over a valid
  output; the stop at quit stops the process group too.
