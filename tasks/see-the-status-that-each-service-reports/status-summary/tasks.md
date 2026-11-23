## Contents
1. Task 1: Read the environment and the log paths of each service from its plist
2. Task 2: Run the status command of a service with the template values of its plist
3. Task 3: Show the summary of each status in the row, in the color of its level
4. Task 4: Show the age of the status in the detail pane
5. Task 5: Run the status command of each service every status.every, one run at a time
6. Task 6: Describe the status command in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-003 "See the status that each service reports" (`launchd-tui/US-003`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Data From launchd", "The Status Command"
(Settings of a Group, Template Values, Output, A Fast Status Command) and "Code Layout".

This change shows in the list what each service reports about itself. For each service of a group with
`status.command`, svcview runs that command every `status.every`, in each state of the service, with no shell, and
with no second run of a service until its first run ends. Each text of the command gets the template values of the
service from its plist: `{{.Name}}`, `{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}`. A value
that the plist does not have shows as the status of that service, for example `the plist has no
WORKER_STATUS_FILE`. When the command prints a JSON object with `version` 1, the row shows its `summary` after the
short name, in the default color for `info`, yellow for `warn` and red for `error`. The detail pane shows the age of
the status, for example `Status 1s ago`: the age of `at` when the output has `at`, and the age of the last run when
it does not. `docs/cli.md` describes the status settings, the template values and the output.

The first user is a worker whose status command is `cat` of a file that the worker writes every 2 s:
`command: [cat, '{{.Env.WORKER_STATUS_FILE}}']`.

The story criteria that this change satisfies:

- For each service of a group with `status.command`, svcview runs that command every `status.every` (default `2s`),
  in each state of the service. svcview runs the program and its arguments with no shell. svcview does not start a
  second run for a service until the first run of that service ends.
- Each argument can use the template values `{{.Name}}`, `{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}`, and
  `{{.PlistPath}}`, from the plist of the service. A value that the plist does not have shows as the status of that
  service, for example `the plist has no WORKER_STATUS_FILE`. An invalid template stops svcview at start, and the
  error names the file and the group.
- When the output is a JSON object with `version` 1, the row shows its `summary` after the short name. `level` sets
  the color of the summary: the default color for `info`, yellow for `warn`, and red for `error`. A property that
  svcview does not know changes nothing.
- The detail pane shows the age of the status, for example `Status 1s ago`. This is the age of `at` when the output
  has `at`, and the age of the last run when it does not.

svcview already reads the `status` settings of a group and checks each template at start
(`internal/status.Command`, `config.Group.Status`). This change gives the template values from the plist, runs the
command, and shows its output. Of the criterion "`docs/cli.md` describes the status settings, the template values,
and the output contract", this change describes the settings, the template values, and the output of a run that
succeeds.

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
- A status command runs with the environment and the working folder of svcview. The `EnvironmentVariables` of the
  plist reach the command only through `{{.Env.X}}`.
- The detail pane does not show the `fields` or the `events` of an output. svcview keeps them with the output, and
  checks their shape as part of version 1.
- No mark of a stale status. `stale_after` stays a setting that svcview does not know.
- No command of the CLI runs the status command of one service. The run of one status command needs nothing of the
  screen.
- No filter, no fold of a group, and no new key. The help and the keys do not change.
- `NO_COLOR` does not remove the color of a summary, as it does not remove the other colors of the screen.
- svcview reads only `config.yaml`. It reads no `conf.d/` folder.

**Deferred:**
- A run that exits with a code other than 0, that runs longer than `status.timeout`, or whose output is not a JSON
  object of version 1, shows no summary in the row and no reason in the detail pane. The detail pane shows only the
  age of that run.
- At `status.timeout`, svcview stops the program of the command, not the processes that the program started.
- svcview keeps no stderr of a run, and no text of an output that is not valid.
- `docs/cli.md` does not describe the failures of a status command.

## Codebase Context

- `internal/status` exists: `status.Command` holds the program and arguments of a status command as checked Go
  templates, with `Every` and `Timeout`, and `status.NewCommand` builds it. `config.Group.Status` is a pointer to the
  status command of a group, or nil. Each template uses only the five template values; svcview refuses any other at
  start. The config file that holds the command passes the check of its owner, its mode and its ACL.
- `internal/launchd/plist.go:15-27`: `readPlist` unmarshals only `Label` with `howett.net/plist`, and a plist that
  does not unmarshal gives no name, so no service. `plist_folder.go:15-45` keeps each plist by path and reads it
  again only when its modification time changes; `names()` gives only the names. `domain.go:20-45` builds each
  `service.Service` from a name and `launchctlList.service(name)`. The tests are at
  `internal/launchd/domain_test.go:130-152` (an XML and a binary plist) and `domain_test.go:598-660` (the
  modification time), with fixture plists in `internal/launchd/testdata/`.
- `internal/service/service.go:33-39`: `Service` is `{Name, State, LastExit, PID, Uptime}`. A value that can be
  missing and whose zero is a real value is a pointer (`tasks/learnings.md`); an empty value of an environment
  variable is a real value, so a missing variable is a missing key of the map, not an empty text.
- `internal/launchd/launchctl_list.go:24-41` runs `launchctl list` with `exec.CommandContext`, a deadline and a
  `WaitDelay` of 1 s (`tasks/learnings.md`, "Run each outside program with a deadline"). `launchctl_list.go:43-72`
  parses an output and gives errors that name what is wrong.
- `internal/launchd/domain_test.go:59-84` writes the fake programs one time in `TestMain`, because macOS checks each
  new executable file at its first run, which costs about 0.3 s (`tasks/learnings.md`). A status test that needs a
  script of its own pays that cost; a test that runs `sh -c`, `printf` or `cat` does not.
- `internal/tui/screen.go`: `readServices` (`screen.go:107-115`) is the `tea.Cmd` of a read; `Update`
  (`screen.go:90-96`) asks for the next `refresh` one tick after each read ends. `listText` and `selectedRow`
  (`screen.go:251-277`) draw the rows; `printable` (`screen.go:345-352`) shows each control character as `�`;
  `lastExitText` (`screen.go:310-319`) draws red text; `detailText`, `detailLine` and `uptimeText`
  (`screen.go:279-308`) draw the detail pane. `row` holds a service, its `config.Group` and the index of the group.
  `Screen.hasRead` is false until the first read ends (`tasks/learnings.md`, "Show no data of a read before the
  first read ends").
- `tui.NewScreen(domain, read, settings)` takes the read of the services as a function, and the tests give it fake
  reads (`internal/tui/screen_test.go:32-66`). A run of a real program in a `synctest` bubble blocks the fake clock
  while the program runs, so the screen takes the run of a status command as a function too, and the tests give it
  fake runs. `runFor` (`screen_test.go:183-203`) runs the real program on the fake clock, and
  `screen_test.go:566-584` checks one read at a time.
- The golden files in `internal/tui/testdata/` show screens with no status command, and they must not change. The
  TUI tests use a screen of 80 columns. A change to the row or to the detail pane rewrites the same lines of
  `screen.go` and `screen_test.go` (`tasks/learnings.md`, "Plan the layout change of the screen as the first
  task"), so the row and the detail pane change before the schedule of the runs.
- `internal/cli/root.go:52-66`: `showScreen` reads the config, makes the `launchd.Domain` and opens the screen. Only
  e2e tests reach it (`tasks/learnings.md`).
- e2e: `addPlist` (`e2e/testUtils.ts:33-50`) writes a plist with only `Label`. The e2e tests run real programs in
  Docker and on a Mac; `cat`, `sh` and `sleep` are in the image, and `e2e/fakes` holds only `launchctl` and `ps`.
  `e2e/tests/config.test.ts:99-109` checks the rows of a group.
- Words: a "run" is one start of the status command of one service, until it ends; the "output" is what the command
  prints on stdout; "summary", "level" and "at" are the properties of the output; the "age" is the time since `at`,
  or since the last run ended. "stale" is the word of `stale_after`, not of the age. "status" is what a service
  reports.

## Tasks

### Task 1: Read the environment and the log paths of each service from its plist

**Behavior:** Each service gets from its plist the `EnvironmentVariables`, the `StandardOutPath`, the
`StandardErrorPath`, and the path of the plist file. A plist that lacks one of them gives a service without it.
svcview reads the values again only when the modification time of the plist changes.

**Acceptance Criteria:**
- [x] A plist with `EnvironmentVariables`, `StandardOutPath` and `StandardErrorPath` gives a service with the same
      environment and log paths, and with the path of the plist file in `~/Library/LaunchAgents`.
- [x] An environment variable with an empty value is in the environment, with an empty value.
- [x] A plist with none of these properties gives a service with no environment and no log paths.
- [x] A plist whose `EnvironmentVariables` is not a dictionary of texts, or whose log path is not a text, still
      gives its service, with its name, and without the value of the wrong type.
- [x] A plist that changes its `EnvironmentVariables` gives the new environment at the next read after its
      modification time changes.
- [x] A symbolic link in `~/Library/LaunchAgents` gives the path of the link as the path of the plist.

**Affected Files/Modules:**
- `internal/service/service.go` — the environment, the log paths and the plist path of a service
- `internal/launchd/plist.go` — read the environment and the log paths
- `internal/launchd/plist_folder.go` — give each plist with its path, not only its name
- `internal/launchd/domain.go` — put the values of the plist into each service
- `internal/launchd/domain_test.go` — the tests of the values
- `internal/launchd/testdata/environment.plist` — new: a plist with an environment and log paths

**Patterns to Follow:**
- `internal/launchd/plist.go:15-27`
- `internal/launchd/plist_folder.go:15-45`
- `internal/launchd/domain.go:20-45`
- `internal/launchd/domain_test.go:130-152`
- `internal/launchd/domain_test.go:598-660`

**Testable:** Yes — through `launchd.Domain.Services` with fixture plists in a temporary home folder.

**Certainty:** high — `readPlist` (`plist.go:15-27`) unmarshals `Label` with `howett.net/plist`, and the same
unmarshal takes the other properties.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Run the status command of a service with the template values of its plist

**Behavior:** `status.Command.Run` takes a context and a service. It applies the template values of the service to
each text of the command, runs the program with its arguments and no shell, and parses what the program prints on
stdout. A JSON object with `version` 1 gives a result with its output: `summary`, `level`, `at`, `fields` and
`events`. A template value that the service does not have gives a result with a failure that names the value, and no
program runs. svcview stops the program at the timeout of the command, and reads no more than 1 MiB of its stdout.
Each other run gives a result with no output. The result keeps the time at which the run ended.

**Acceptance Criteria:**
- [ ] `{{.Name}}` gives the name of the service, `{{.Env.X}}` the value of `X` in its environment, and
      `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}` its log paths and the path of its plist. An
      environment variable with an empty value gives an empty text.
- [ ] A template value that the service does not have gives the failure `the plist has no X` for `{{.Env.X}}`,
      `the plist has no StandardOutPath` and `the plist has no StandardErrorPath` for the log paths, and
      `the service has no plist` for `{{.PlistPath}}`. No program runs.
- [ ] svcview runs the program with its arguments and no shell: an argument with `$HOME`, `;`, `|`, `*` or a space
      reaches the program as it is.
- [ ] An output that is a JSON object with `version` 1 gives its `summary`, its `level` (`info` when it has none) and
      its `at`, also with a new line after the object.
- [ ] The output keeps `fields` (each a `name`, a `value` and an optional `progress`) and `events` (each an `at`, a
      `level` and a `text`), in their order.
- [ ] A property that svcview does not know changes nothing.
- [ ] Each of these gives a result with no output: a `version` other than 1, or none; a `level` other than `info`,
      `warn` or `error`; an `at` that is not an RFC 3339 time; a `summary` that is not a text; `fields` or `events`
      whose shape is not the one above; an output that is not a JSON object, or that has a second JSON value after
      the object; an empty output.
- [ ] A program that exits with a code other than 0 gives a result with no output, also when it prints a valid
      output.
- [ ] A program that runs longer than the timeout of the command stops, and gives a result with no output, no later
      than 1 s after the timeout.
- [ ] A program that prints more than 1 MiB on stdout gives a result with no output, and svcview keeps no more than
      1 MiB of it.
- [ ] The result has the time at which the run ended.

**Affected Files/Modules:**
- `internal/status/command.go` — the run of the command, its result and its failure
- `internal/status/output.go` — new: the output of version 1, its level, its fields and events, and its parse
- `internal/status/command_test.go` — the tests of the run, with small commands such as `sh -c` and `printf`

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:24-41`
- `internal/launchd/launchctl_list.go:43-72`
- `internal/launchd/domain_test.go:59-84`

**Testable:** Yes — through `status.Command.Run` with services that the test builds and small real commands.

**Certainty:** medium — the run of `launchctl list` with a deadline (`launchctl_list.go:24-41`) and its parse
(`launchctl_list.go:43-72`) are the precedent, but no code here runs a program that the config names, applies Go
templates to its arguments, or parses JSON.

**Blast radius:** high — the first code that runs a program that the config names, as the user of svcview, with
values that come from the plists; a shell here would let a value of a plist run as a command. The output is a
contract that programs outside this repo write.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 1

### Task 3: Show the summary of each status in the row, in the color of its level

**Behavior:** When the first read of the services ends, svcview runs the status command of each service of a group
that has one, one run for each service, in each state. When a run ends with an output, the row of the service shows
the `summary` after the short name: in the default color for `info`, yellow for `warn`, and red for `error`. When a
run ends with a failure, the row shows its text in red, for example `the plist has no WORKER_STATUS_FILE`. svcview
gives the screen the run of the status commands when it opens it.

**Acceptance Criteria:**
- [ ] After the first read of the services, each service of a group with a status command gets one run, whatever
      its state. A service of a group with no status command gets no run.
- [ ] The row shows the `summary` of the last run after the short name, and the summaries of the rows of one group
      start in one column.
- [ ] The summary has the default color for `info`, yellow for `warn`, and red for `error`.
- [ ] A run that ends with a failure shows the text of the failure in red in place of the summary.
- [ ] A row shows nothing after the short name before its first run ends, when the last run gave no output and no
      failure, and in a group with no status command.
- [ ] The selected row shows the summary on its background. A summary that is longer than the list pane ends with
      `…`, and each control character in a summary shows as `�`.
- [ ] With no group that has a status command, the screen is as before: the golden files of
      `internal/tui/testdata/` do not change.
- [ ] `svcview` gives the screen the run of the status commands when it opens it.
- [ ] e2e test: a plist with the environment variable `WORKER_STATUS_FILE`, and a group with
      `status.command: [cat, '{{.Env.WORKER_STATUS_FILE}}']`, shows the `summary` of the file in the row of the
      service. A group whose command uses `{{.Env.MISSING}}` shows `the plist has no MISSING`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the first run of each status command, and the summary in the row
- `internal/tui/screen_test.go` — the tests of the summary, with fake runs
- `internal/tui/testdata/` — new golden files of rows with a summary
- `internal/cli/root.go` — give the run of the status commands to the screen
- `e2e/tests/status.test.ts` — new: the e2e tests of the status command, with a helper that writes a plist with an
  environment

**Patterns to Follow:**
- `internal/tui/screen.go:107-115`
- `internal/tui/screen.go:251-277`
- `internal/tui/screen.go:310-319`
- `internal/tui/screen.go:345-352`
- `internal/tui/screen_test.go:32-66`
- `e2e/tests/config.test.ts:99-109`
- `e2e/testUtils.ts:33-50`

**Testable:** Yes — through the `Screen` model with fake runs, golden files, and an e2e test.

**Certainty:** medium — the read of the services as a `tea.Cmd` (`screen.go:107-115`) and the rows
(`screen.go:251-277`) are the precedent, but no command of the screen runs once for each service at the same time,
with each result as a message of its own.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 4: Show the age of the status in the detail pane

**Behavior:** Under the data from launchd, the detail pane of a service whose run ended shows a line `Status` with
the age of the status, for example `Status 1s ago`. The age is the time since `at` when the output has `at`, and the
time since the last run ended when it has no `at` or when the run gave no output.

**Acceptance Criteria:**
- [ ] The detail pane of a service whose output has `at` shows the age of `at`.
- [ ] The detail pane of a service whose output has no `at`, or whose run gave a failure or no output, shows the age
      of the last run.
- [ ] The age has the units of the uptime, for example `1s`, `3m12s` or `3h12m`. An `at` after the current time
      shows `0s`.
- [ ] The age grows as the screen draws again, with no new run.
- [ ] The detail pane of a service with no status command, or before its first run ends, shows no `Status` line.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the line `Status` with the age in the detail pane
- `internal/tui/screen_test.go` — the tests of the age, on the fake clock of `synctest`
- `internal/tui/testdata/` — new golden files of a detail pane with a status

**Patterns to Follow:**
- `internal/tui/screen.go:279-308`
- `internal/tui/screen_test.go:183-203`
- `task:3`

**Testable:** Yes — through the `Screen` model, with fake runs and the fake clock of `synctest`.

**Certainty:** medium — `detailText` and `uptimeText` (`screen.go:279-308`) are the precedent, but no line of the
screen depends on the current time.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 3

### Task 5: Run the status command of each service every status.every, one run at a time

**Behavior:** For each group with a status command, svcview starts a run for each service of the group every
`status.every` of that group, in each state of the service. It starts no run for a service whose run has not ended.
The row and the detail pane show the last run that ended. When svcview quits, it stops each status command that
still runs.

**Acceptance Criteria:**
- [ ] With `every` of 2 s, a service gets a run every 2 s, and the row shows a new `summary` no later than 2 s after
      the output changes.
- [ ] Two groups with different `every` run their commands each at its own `every`.
- [ ] A run that lasts longer than `every` gets no second run of the same service while it runs; the next run starts
      at an `every` after it ends.
- [ ] A service that a read adds gets runs at the next `every` of its group; a service that a read removes gets no
      more runs.
- [ ] A service that is `not loaded` gets runs as a `running` service does.
- [ ] When svcview quits, each status command that still runs stops.
- [ ] e2e tests: when the status file changes, the row shows the new `summary` with no key press. A status command
      that writes a file after 2 s, with a `timeout` of 10 s, does not write it when svcview quits after the run
      starts.

**Affected Files/Modules:**
- `internal/tui/screen.go` — a timer for each group, and one run at a time for each service
- `internal/tui/screen_test.go` — the tests of the schedule, on the fake clock of `synctest`
- `internal/cli/root.go` — stop the status commands that still run when svcview quits
- `e2e/tests/status.test.ts` — the new summary, and the stop at quit

**Patterns to Follow:**
- `internal/tui/screen.go:90-96`
- `internal/tui/screen_test.go:183-203`
- `internal/tui/screen_test.go:566-584`
- `internal/cli/root.go:52-66`
- `task:3`

**Testable:** Yes — through the `Screen` model on the fake clock of `synctest`, with fake runs that take time, and in
e2e tests.

**Certainty:** medium — the `refresh` tick after each read (`screen.go:90-96`) and its test of one read at a time
(`screen_test.go:566-584`) are the precedent, but no timer runs for each group, no rule holds one run for each
service, and nothing stops a program when svcview quits.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 3, Task 4

### Task 6: Describe the status command in docs/cli.md

**Behavior:** `docs/cli.md` describes the `status` settings of a group, the template values, the output of a status
command, how the list and the detail pane show it, and how to write a fast status command. The author of a program
can write a status command that succeeds from these docs, with no read of the svcview code.

**Acceptance Criteria:**
- [ ] A section on the status command lists `status.command`, `status.every` and `status.timeout`, with their
      meaning and their defaults, and gives an example group with a status command.
- [ ] It says that svcview runs the program and its arguments with no shell, that `[sh, -c, '...']` gives a shell,
      that the command runs in each state of the service, and that a service gets no second run until its run ends.
- [ ] It lists the five template values, and says that a value that the plist does not have shows as the status,
      for example `the plist has no WORKER_STATUS_FILE`.
- [ ] It names the texts that stop svcview at start: a template value other than the five, `range`, `define`,
      `block` and `{{template ...}}`, a field without `$` in the body of a `with`, a field of a variable that the text
      declares, and a new value for `$`. It says that `every` and `timeout` must not be less than 0, and that 0 gives
      the default.
- [ ] It describes the output of version 1: `version`, `at`, `summary`, `level`, `fields` and `events`, with an
      example, the shape of each, and that a property that svcview does not know changes nothing. It says which
      properties the screen shows.
- [ ] It says how the row shows the summary and the colors of the levels, and what the line `Status` of the detail
      pane shows.
- [ ] It tells why a command that starts a large runtime is too slow, and how a service writes its status file in
      one step so that `cat` never reads half a file.
- [ ] The errors of `status` that stop svcview at start are in the list of errors, and the glyphs and colors table
      has the colors of the levels.

**Affected Files/Modules:**
- `docs/cli.md` — a section on the status command, and the list, the detail pane, the colors and the errors

**Patterns to Follow:**
- `docs/cli.md:35-111`
- `docs/cli.md:112-188`

**Testable:** No — a document.

**Certainty:** high — the sections on the screen and on the config file (`docs/cli.md:35-188`) are the precedent,
and the proposal gives the tables ("The Status Command").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the behaviour of `task cli` with a group whose status command is
`cat` of a status file.

**Depends on:** Task 5

## Summary

- 6 tasks. The plist values come first (Task 1), then the run of one status command (Task 2), which needs nothing
  of the screen. The screen then changes its layout before the schedule: the summary in the row (Task 3) and the
  age in the detail pane (Task 4). Task 5 runs each command again every `status.every`, one run at a time, and
  stops the runs when svcview quits. The document comes last.
- Covers the criteria on the runs every `status.every` (Tasks 3 and 5), on the template values (Tasks 1, 2 and 3),
  on the summary and its level (Tasks 2 and 3), and on the age (Task 4). Of the criterion on `docs/cli.md`, it covers
  the settings, the template values and the output (Task 6). The failures of a run, in the row and in the detail
  pane, are outside this change (Boundaries).
- Task 2 is `high` blast radius: it is the first code that runs a program that the config names. No task is `low`
  certainty; Task 1 is `high` (the plist read), and Tasks 2 to 5 vary a precedent of this repo.
- The planner chose these details, which the story does not give: the command runs with the environment of
  svcview; a missing `{{.PlistPath}}` reads `the service has no plist`; a `level` that is not `info`, `warn` or
  `error`, a second JSON value, or an empty output is not a valid output; the summaries of one group start in one
  column; the text of a failure is red; the first runs start when the first read ends; a service that a read adds
  waits for the next `every` of its group; svcview stops the runs that still run when it quits.
