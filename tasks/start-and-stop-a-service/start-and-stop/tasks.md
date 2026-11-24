## Contents
1. Task 1: Start and stop a service with launchctl in the gui domain of the user
2. Task 2: Show s start and x stop in the help line and in the help
3. Task 3: Start the selected service with s
4. Task 4: Show the error of a failed start on a message line above the help line
5. Task 5: Stop the selected service with x after a confirmation
6. Task 6: Describe the start, the stop and the message line in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-008 "Start and stop a service" (`launchd-tui/US-008`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Keys", "Data From launchd" and "Actions on
launchd".

The user starts and stops a service of the list with one key, with no `launchctl` command and no label to type.
`s` starts the selected service: `launchctl bootstrap gui/<uid> <plist path>` for a service that is `not loaded`,
`launchctl kickstart gui/<uid>/<name>` for a service that is `loaded`, and no command for a service that is
`running`. `x` asks for a confirmation before it stops a service that is `loaded` or `running`; on a yes, svcview
runs `launchctl bootout gui/<uid>/<name>`. `bootout` removes the service from launchd until the next login, also
when its plist has `KeepAlive`, which `launchctl kill` does not do. The row shows the new state no later than the
next refresh. When `launchctl` fails, a message line above the help line shows what it printed. The help line shows
`s start · x stop`.

The story criteria that this change satisfies:

- `s` on a service that is `not loaded` runs `launchctl bootstrap gui/<uid> <plist path>`. `s` on a service that is
  `loaded` runs `launchctl kickstart gui/<uid>/<name>`. `s` on a service that is `running` runs no command.
- `x` on a service that is `loaded` or `running` asks for a confirmation. On a yes, svcview runs
  `launchctl bootout gui/<uid>/<name>`. On a no, or on a service that is `not loaded`, svcview runs no command.
- After a stop, the service stays `not loaded` until the next login or until a start in svcview, also when its plist
  has `KeepAlive`.
- After a start or a stop, the row shows the new state no later than the next refresh.
- When `launchctl` exits with an error, a message line above the help line shows its output.
- The help line shows `s start · x stop`.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No `launchctl kill`, `load`, `unload`, `enable` or `disable`. A start runs only `bootstrap` or `kickstart`, and a
  stop runs only `bootout`.
- No restart, no `r` key, no `kickstart -k`, no log, no `l` key, no keys of a group, and no filter of the list. The
  confirmation asks only before a stop.
- svcview starts no service on its own. After a stop, a service stays as launchd keeps it until a key starts it.
- `s` on a `running` service and `x` on a `not loaded` service show no message: they do nothing.
- No line or glyph says that a start or a stop runs. The row shows the state of the last read until the next
  refresh, and `s` and `x` act on that state.
- svcview does not read the services when a start or a stop ends. The next refresh shows the new state, because
  `Domain.Services` must not run in two goroutines at the same time (`internal/launchd/domain.go:19-20`).
- svcview does not wait for a start or a stop that still runs when it quits.
- The message line has no color, keeps no history of messages, and shows one line. It shows only a confirmation or
  the error of a start or a stop: a read error stays in the list, and a status failure stays in the row and the
  detail pane, as before.
- No layout for a narrow terminal, no `enter` that opens the detail pane, and no change for `NO_COLOR`.
- No `--all`, and no start or stop of a service that has no plist in `~/Library/LaunchAgents`.
- No change to the status commands: a start or a stop does not start or skip a run of a status command.

**Deferred:** None.

## Codebase Context

- `internal/launchd/domain.go:10-50`: `Domain` holds the uid and the plist folder. `NewDomain(home, uid)` makes it,
  `Services` reads the plists and runs `launchctl list` and `ps`, and `String` gives `gui/<uid>`. The comment on
  `Services` says that it must not run in two goroutines at the same time, because the plist folder keeps the plists
  that it read. Start and stop use only the uid and the service that they get, so they can run while a read runs.
- `internal/launchd/launchctl_list.go:24-41`: `readLaunchctlList` runs `launchctl` with `exec.CommandContext`, a
  deadline of 5 s and a `WaitDelay` of 1 s. An exit with a code other than 0 gives an error with the exit code and what
  launchctl printed, stderr first. The deadline gives the error `launchctl list did not end within 5s`.
- `internal/status/command.go:63-98`: `status.Command.Run(ctx, service)` takes the context of its caller, and
  `internal/cli/status_runs.go` gives it the context of svcview. A start and a stop take a context in the same way,
  so a test can end the context and need not wait for the deadline.
- launchctl, from `man launchctl` on this Mac: `bootstrap | bootout domain-target [service-path ...] |
  service-target`; `kickstart [-kp] service-target`; `enable | disable` persists across boots. `man launchd.plist`
  says that `ExitTimeOut`, the time between `SIGTERM` and `SIGKILL` when launchd stops a service, is system-defined.
  `launchctl print gui/<uid>/<name>` shows `exit timeout = 5` for each agent in `~/Library/LaunchAgents` of this
  Mac. The man page does not say whether `bootout` waits for the process of the service to exit, so the deadline of
  a start and a stop is 30 s: a `bootout` that waits for a long `ExitTimeOut` ends before it. The planner did not run
  `bootstrap`, `kickstart` or `bootout` against a real service.
- `internal/service/service.go:5-43`: `service.Service` has `Name`, `State` (`NotLoaded`, `Loaded`, `Running`) and
  `PlistPath`. A service in the list always has a plist path, because svcview lists only the services that have a
  plist in `~/Library/LaunchAgents`.
- `internal/tui/screen.go:90-100`: `NewScreen(domain, read, runStatus, settings)`. `screen_test.go` calls
  `tui.NewScreen` 81 times, most of them with `nil` for `runStatus`, so a new parameter changes each call.
- `internal/tui/screen.go:106-159`: `Update`. The keys are at `screen.go:112-136`; while the help shows, `tab`, `↑`
  and `↓` do nothing. At the end of each `Update`, `detailTopInView` and `listTopInView` (`screen.go:242-259`) keep
  the selected row in view and the scroll inside the text, from the height of the panes.
- `internal/tui/screen.go:191-207` and `screen.go:147-149`: `runStatuses` starts one `tea.Cmd` for each service that
  has no run that has not ended (`runNotEnded`), and `runStatusOf` calls the `runStatus` of `NewScreen` in it and
  gives the message `runEnded`. This is the precedent for a start or a stop that runs in the background and ends with
  a message of the screen.
- `internal/tui/screen.go:289-306`: `content` draws the header, the panes or the help, and then the help line, cut
  with `ansi.Truncate`. `paneHeight` is the height of the terminal minus the header and the help line.
  `screen.go:21` holds the help line, and `screen.go:330-338` the help that `?` shows.
- `internal/tui/screen.go:355-362`: the list shows the lines of a read error through
  `terminal.ReplaceControlCharacters` (`internal/terminal/control_characters.go`).
- Tests of the screen: `open`, `openWith`, `openIn` (`screen_test.go:52-70`); `readsInTurn` and `changingAt`
  (`screen_test.go:169-186`) change what a read gives; `runsGiving` (`screen_test.go:274-282`) is a fake
  `runStatus`; `runFor` and `runSized` (`screen_test.go:203-229`) run the real program in a `synctest` bubble. The
  group `keys` is at `screen_test.go:2280-2353`. Its subtest "another key does not end the screen"
  (`screen_test.go:2347-2351`) uses `x`, which becomes a key of the screen in Task 5. `keys.golden` is 80×12, and the
  help box has room for 6 lines there; the help has 5 lines now. Each of the 23 golden files in
  `internal/tui/testdata` ends with the help line.
- `internal/cli/root.go:53-69`: `showScreen` makes the domain and the screen, and runs the program with the context
  of svcview. Only e2e tests reach it.
- Unit fakes: `internal/launchd/domain_test.go:59-82` writes one fake `launchctl` and one fake `ps` for all tests;
  the fake `ps` adds each call to `$FAKE_PS_CALLS` (`fakePs`, `domain_test.go:110-120`). The fake `launchctl`
  answers only `list`. `fakeLaunchctl` (`domain_test.go:96-99`) writes a script for one test, as the tests of an
  error and of the deadline do (`domain_test.go:426-447`).
- e2e: `e2e/fakes/launchctl` prints `$HOME/launchctl-list.txt` for `list`, and exits with 64 for any other
  arguments. `e2e/fakes/ps` is the other fake. `openCli` puts `e2e/fakes` first on `PATH`. `setLaunchctlOutput`
  (`e2e/testUtils.ts:65-70`) writes the file with one rename. In Docker the tests run as root, so the domain is
  `gui/0`, and the tests compare it with `process.getuid()`. Both fakes must stay mode `100755` in git.
  `docs/e2e-tests.md:103-141` describes the fakes. `e2e/tests/screen.test.ts:76-92` waits for the glyph of a new
  state at the next refresh.
- `tasks/learnings.md` entries that apply: "Plan the layout change of the screen as the first task" (Task 2 changes
  the help line, and with it every golden file, before any key does something); "Each key of the screen goes into the
  help and docs/cli.md" (Tasks 2 and 6); "Plan each Bubble Tea message and private type in design.yaml"
  (`actionEnded`, `confirmation`); "Run each outside program with a deadline" (Task 1); "Fake a program that svcview
  runs with a script first on PATH" (Tasks 1, 3 and 4); "Text from outside goes to the terminal through
  terminal.ReplaceControlCharacters" (Tasks 4 and 5); "Bubble Tea does not wait for the goroutines of its commands"
  (svcview does not wait for a start or a stop at quit, so no code after `Program.Run` waits for one); "Test a
  Bubble Tea timer on the fake clock of synctest" (a fake start or stop that sleeps must end within the minute);
  "Only e2e tests reach the terminal branch of the root command" (Tasks 3 and 5); "task test:e2e:local leaves
  e2e/package-lock.json in the tree".
- Words: "status" is the status command of a group, so the line above the help line is the message line, never a
  status line. In `internal/tui`, start and stop are the actions of the keys `s` and `x`, and the launchctl words
  `bootstrap`, `kickstart` and `bootout` stay in `internal/launchd`. The question of `x` is a confirmation, the word
  of the proposal.

## Tasks

### Task 1: Start and stop a service with launchctl in the gui domain of the user

**Behavior:** `launchd.Domain` starts and stops one service. A start of a service that is `not loaded` runs
`launchctl bootstrap gui/<uid> <plist path>`, a start of a service that is `loaded` runs
`launchctl kickstart gui/<uid>/<name>`, and a start of a service that is `running` runs no command. A stop of a
service that is `loaded` or `running` runs `launchctl bootout gui/<uid>/<name>`, and a stop of a service that is
`not loaded` runs no command. When launchctl fails, the error holds what it printed.

**Acceptance Criteria:**
- [x] `Domain.Start` of a `not loaded` service runs `launchctl bootstrap gui/<uid> <plist path>`, with the uid of the
      domain and the plist path of the service, and gives no error when launchctl exits with 0.
- [x] `Domain.Start` of a `loaded` service runs `launchctl kickstart gui/<uid>/<name>`.
- [x] `Domain.Start` of a `running` service runs no command and gives no error.
- [x] `Domain.Start` of a `not loaded` service with no plist path runs no command, and gives an error that says that
      the service has no plist.
- [x] `Domain.Stop` of a `loaded` and of a `running` service runs `launchctl bootout gui/<uid>/<name>`. `Domain.Stop`
      of a `not loaded` service runs no command and gives no error.
- [x] Each start and each stop runs one launchctl command, and never `kill`, `load`, `unload`, `enable` or `disable`.
- [x] launchctl gets each argument as it is, with no shell: a name or a plist path with a space, a quote or a `$`
      reaches launchctl unchanged.
- [x] A launchctl that exits with a code other than 0 gives an error whose text is what launchctl printed, stderr
      first and then stdout, with each line kept. When launchctl printed nothing, the text names the command and the
      exit code.
- [x] With no launchctl on `PATH`, a start and a stop give the error that launchctl is not found.
- [x] A launchctl that does not end within 30 s, or before the context of the call ends, is stopped, and the error
      names the command and says that it did not end. A child of launchctl that keeps the output open does not keep
      the call waiting. The test ends the context, and does not wait 30 s.
- [x] The tests are in `TestDomain`, in a group for the start and a group for the stop. The fake launchctl that all
      tests share records each call other than `list`, as the fake `ps` records its calls.

**Affected Files/Modules:**
- `internal/launchd/domain.go` — `Start` and `Stop` of `Domain`
- `internal/launchd/` — a new file beside `launchctl_list.go` for the run of a launchctl command that starts or stops
  a service, with its deadline and its error
- `internal/launchd/domain_test.go` — the groups for the start and the stop, and the shared fake launchctl that records
  its calls

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:24-41`
- `internal/status/command.go:63-98`
- `internal/launchd/domain_test.go:59-82`
- `internal/launchd/domain_test.go:110-120`
- `internal/launchd/domain_test.go:426-447`

**Testable:** Yes — through `Domain.Start` and `Domain.Stop`, with a fake launchctl first on `PATH`.

**Certainty:** medium — `readLaunchctlList` runs launchctl with a deadline and an error that holds what it printed
(`launchctl_list.go:24-41`), but no call yet changes the domain, puts the uid, the name or the plist path into the
arguments, or takes the context of its caller.

**Blast radius:** high — the first code that changes launchd: a wrong subcommand or a wrong target starts or stops a
service of the user, and a stop ends the process of the service in the middle of its work.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Show s start and x stop in the help line and in the help

**Behavior:** The help line at the bottom of the screen shows `s start · x stop · ? help`. `?` lists `s` with
"start the selected service" and `x` with "stop the selected service, after a confirmation", with each other key as
before. This task changes only the text of the help; Tasks 3 and 5 give the keys their actions.

**Acceptance Criteria:**
- [x] The help line shows `s start · x stop · ? help` under the panes and under the help, cut at the width of the
      terminal as before.
- [x] `?` shows `s` and `x` with their actions, and each other key with its action as before. The golden file of
      the help shows all of its lines, in a terminal that is high enough for them.
- [x] Each golden file in `internal/tui/testdata` shows the new help line, and no other line of them changes.
- [x] The e2e test of `?` still finds `? help` on the screen.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the help line and the help
- `internal/tui/screen_test.go` — the test of the help line, and the size of the test of the help
- `internal/tui/testdata/` — each golden file

**Patterns to Follow:**
- `internal/tui/screen.go:21`
- `internal/tui/screen.go:330-338`
- `internal/tui/screen_test.go:2289-2295`
- `internal/tui/screen_test.go:2338-2345`

**Testable:** Yes — through the `Screen` model and its golden files.

**Certainty:** high — the help line (`screen.go:21`), the help (`screen.go:330-338`) and their tests
(`screen_test.go:2289-2295`, `screen_test.go:2338-2345`) are the precedent; this task changes their text.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, `task test:e2e` passes, and `git diff internal/tui/testdata`
shows only the help line changed, except in the golden file of the help.

**Depends on:** None

### Task 3: Start the selected service with s

**Behavior:** `s` on the selected service, when it is `not loaded` or `loaded`, calls the start of the screen with
that service, in the background. `s` on a `running` service calls nothing. svcview gives the screen the start of its
domain, so `s` runs `launchctl bootstrap` or `launchctl kickstart`. The screen goes on with its reads, its runs and
its keys while the start runs. The row shows the new state at the next refresh after the start ends.

**Acceptance Criteria:**
- [x] `s` on a `not loaded` service and on a `loaded` service calls the start one time, with the service of the
      selected row. `s` on a `running` service calls nothing.
- [x] The screen reads, refreshes and takes keys while a start runs: a start that lasts longer than `refresh` does
      not move the next read.
- [x] While the start of a service has not ended, `s` on that service calls nothing, and `s` on another service still
      calls the start.
- [x] `s` calls nothing while the help shows, with no rows, and while the list shows a read error.
- [x] After a start ends, the row shows the state that the next read gives, no later than one `refresh` after the
      start ends. The end of a start starts no read.
- [x] `tui.NewScreen` takes the start, and `internal/cli/root.go` gives it the start of the domain with the context
      of svcview.
- [x] The e2e fake launchctl adds each call other than `list` to `$HOME/launchctl-calls.txt`, one line with its
      arguments for each call, and exits with 0. A helper in `e2e/testUtils.ts` gives the calls of a home folder.
- [x] e2e: `s` on a `not loaded` service records `bootstrap gui/<uid> <home>/Library/LaunchAgents/<name>.plist`;
      `s` on a `loaded` service records `kickstart gui/<uid>/<name>`; `s` on a `running` service records nothing.
      After `s`, a new output of `launchctl list` that shows the service `running` shows its `●` at the next
      refresh, with no key press.
- [x] `docs/e2e-tests.md`, "The fake launchctl", says that the fake records each other call, and names the helper.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `s`, the start, and the services whose start or stop has not ended
- `internal/tui/screen_test.go` — the tests of `s`, and the new parameter of each call of `tui.NewScreen`
- `internal/cli/root.go` — the start of the domain for the screen
- `e2e/fakes/launchctl` — record each call other than `list`
- `e2e/testUtils.ts` — the helper that gives the calls
- `e2e/tests/start-stop.test.ts` — new: the e2e tests of `s`
- `docs/e2e-tests.md` — the fake launchctl records its calls

**Patterns to Follow:**
- `internal/tui/screen.go:112-136`
- `internal/tui/screen.go:191-207`
- `internal/tui/screen.go:147-149`
- `internal/tui/screen_test.go:274-282`
- `internal/tui/screen_test.go:169-186`
- `internal/tui/screen_test.go:203-229`
- `internal/cli/root.go:53-69`
- `e2e/fakes/ps`
- `e2e/testUtils.ts:65-70`
- `e2e/tests/screen.test.ts:76-92`
- `task:1`

**Testable:** Yes — through the `Screen` model with a fake start and fake reads, and in e2e tests with the fake
launchctl.

**Certainty:** medium — a function of `NewScreen` that runs in a `tea.Cmd`, one at a time for each service, and ends
with a message (`runStatusOf` and `runNotEnded`, `screen.go:191-207`), is the precedent, but no key has run anything
yet, and no e2e fake records its calls.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1, Task 2

### Task 4: Show the error of a failed start on a message line above the help line

**Behavior:** When the start gives an error, a message line between the panes and the help line shows the text of the
error. The panes are one line shorter while the message line shows. The next key press hides it, and that key does
what it does without a message line.

**Acceptance Criteria:**
- [x] When the start gives an error, the line above the help line shows its text, and the panes end one line higher.
      Under the help, the help box ends one line higher.
- [x] The text of an error with more than one line shows on the one line, each line after the line before it with a
      space between them. A text that is wider than the terminal is cut with `…`. Each control character shows as
      `�`.
- [x] The message line stays through each refresh and each run, until the next key press. That key hides it and does
      what it does without a message line: for example `↓` also selects the next row.
- [x] A start that ends with no error shows no message line. The error of a later start takes the place of an
      earlier one.
- [x] Before the first error, and after a key hides it, the screen has no message line, so the golden files of the
      earlier tasks do not change.
- [x] When the message line shows and hides, the selected row stays in view in the list, and the scroll of the detail
      pane stays inside its text.
- [x] A new golden file shows a screen with a message line.
- [x] e2e: when a helper in `e2e/testUtils.ts` tells the fake launchctl to fail, the fake prints the text of the
      helper to stderr and exits with a code other than 0; `s` then shows that text above the help line, for example
      `Bootstrap failed: 5: Input/output error`. `docs/e2e-tests.md` names the helper.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the message line, and the height of the panes while it shows
- `internal/tui/screen_test.go` — the tests of the message line
- `internal/tui/testdata/` — a new golden file with a message line
- `e2e/fakes/launchctl` — fail when a test tells it to
- `e2e/testUtils.ts` — the helper that makes the fake fail
- `e2e/tests/start-stop.test.ts` — the e2e test of a failed start
- `docs/e2e-tests.md` — the failure of the fake launchctl

**Patterns to Follow:**
- `internal/tui/screen.go:289-306`
- `internal/tui/screen.go:355-362`
- `internal/tui/screen.go:242-259`
- `internal/terminal/control_characters.go`
- `internal/tui/screen_test.go:540-560`
- `task:3`

**Testable:** Yes — through the `Screen` model with a fake start that gives an error, golden files, and an e2e test.

**Certainty:** medium — `content` stacks the header, the panes and the help line (`screen.go:289-306`), and the list
shows the lines of a read error through `ReplaceControlCharacters` (`screen.go:355-362`), but no line of the screen
shows and hides, so the height of the panes now changes with the state of the screen.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 3

### Task 5: Stop the selected service with x after a confirmation

**Behavior:** `x` on the selected service, when it is `loaded` or `running`, shows the confirmation
`Stop <name>? (y/N)` on the message line, with the full name of the service. `y` calls the stop of the screen with
that service, in the background. Each other key answers no: it hides the confirmation and does nothing else.
`ctrl+c` still quits. `x` on a `not loaded` service shows nothing and calls nothing. svcview gives the screen the stop
of its domain, so a yes runs `launchctl bootout gui/<uid>/<name>`. An error of the stop shows on the message line, as
an error of a start does.

**Acceptance Criteria:**
- [x] `x` on a `loaded` and on a `running` service shows `Stop <name>? (y/N)` on the message line, with the full name
      of the service, and calls nothing yet.
- [x] `y` hides the confirmation and calls the stop one time with the service that the confirmation names, also when
      a refresh changed the selected row, or removed the service from the list, while the confirmation showed.
- [x] Each key other than `y` and `ctrl+c` answers no, for example `n`, `esc`, `q`, `↓` and `x`: it hides the
      confirmation, calls nothing, and does nothing else. `ctrl+c` quits while the confirmation shows.
- [x] `x` on a `not loaded` service shows no confirmation and calls nothing.
- [x] `x` calls nothing and asks nothing while the help shows, with no rows, while the list shows a read error, and
      while the start or the stop of the service has not ended.
- [x] After a stop ends, the row shows the state that the next read gives, no later than one `refresh` after the stop
      ends. The screen starts no service on its own: later reads that show the service `not loaded` call nothing.
- [x] An error of the stop shows on the message line as an error of a start does. An error that ends while a
      confirmation shows does not hide the confirmation, and shows after the answer.
- [x] The confirmation shows each control character of the name as `�`, and is cut with `…` at the width of the
      terminal.
- [x] `tui.NewScreen` takes the stop, and `internal/cli/root.go` gives it the stop of the domain with the context of
      svcview.
- [x] A new golden file shows a screen with a confirmation. The subtest "another key does not end the screen" uses a
      key that the screen does not handle, in place of `x`.
- [x] e2e: `x` and `y` on a `running` service record `bootout gui/<uid>/<name>`; `x` and `n` record nothing; `x` on
      a `not loaded` service shows no confirmation and records nothing. After `x`, `y` and a new output of
      `launchctl list` without the service, the row shows `○` at the next refresh, still shows it at the refresh after
      that, and the calls hold only the `bootout`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `x`, the confirmation, the answer, and the stop
- `internal/tui/screen_test.go` — the tests of `x` and the confirmation, and the new parameter of each call of
  `tui.NewScreen`
- `internal/tui/testdata/` — a new golden file with a confirmation
- `internal/cli/root.go` — the stop of the domain for the screen
- `e2e/tests/start-stop.test.ts` — the e2e tests of `x`

**Patterns to Follow:**
- `internal/tui/screen.go:112-136`
- `internal/tui/screen_test.go:2280-2353`
- `task:3`
- `task:4`

**Testable:** Yes — through the `Screen` model with a fake stop and fake reads, a golden file, and e2e tests with the
fake launchctl.

**Certainty:** medium — the help is a state of the screen that changes what the keys do (`screen.go:112-136`), and
Task 3 calls a function of `NewScreen` for the selected service, but no state yet takes the next key as an answer, or
keeps a service to act on while a refresh changes the selection.

**Blast radius:** high — the confirmation is the one step between a key and the stop of a service: a yes that stops
another service than the confirmation names, or a stop with no yes, ends the process of a service that the user needs.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 4

### Task 6: Describe the start, the stop and the message line in docs/cli.md

**Behavior:** `docs/cli.md` describes `s`, `x`, the confirmation, the message line and the new help line. The Keys
table matches the help of the screen.

**Acceptance Criteria:**
- [ ] The Keys table lists `s` and `x` as the help shows them, and says that `y` answers yes to the confirmation and
      each other key answers no.
- [ ] The table of the parts of the screen shows the help line `s start · x stop · ? help`, and the message line
      above it. The example screen at the start of "The screen" shows the new help line.
- [ ] A section says which launchctl command `s` runs for a service that is `not loaded` and for one that is
      `loaded`, and that `s` on a `running` service does nothing; that `x` asks for a confirmation, `y` runs
      `launchctl bootout gui/<uid>/<name>`, and `x` on a `not loaded` service does nothing.
- [ ] It says that a stopped service stays `not loaded` until the next login or until a start in svcview, also when
      its plist has `KeepAlive`, because svcview runs `bootout` and not `kill`; and that svcview does not run
      `launchctl disable`.
- [ ] It says that the row shows the new state at the next refresh.
- [ ] It says that the message line shows what launchctl printed when it fails, until the next key press, and that a
      launchctl that does not end within 30 s is stopped with an error.

**Affected Files/Modules:**
- `docs/cli.md` — the start, the stop, the confirmation, the message line, and the help line

**Patterns to Follow:**
- `docs/cli.md:35-147`

**Testable:** No — a document.

**Certainty:** high — the tables of "The screen" and its sections (`docs/cli.md:35-147`) are the precedent, and the
proposal gives the keys ("Keys") and the commands ("Actions on launchd").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the help of the screen and against the tests of Tasks 3 to 5.

**Depends on:** Task 5

## Summary

- 6 tasks. Task 1 puts the launchctl commands in `internal/launchd`, the only package that knows launchd. Task 2
  changes the help line, and with it every golden file, before any key does something, so an audit fix of a later
  task folds into its own commit. Task 3 runs the start from `s` and wires it into svcview, with the e2e fake that
  records the calls. Task 4 adds the message line for the error that Task 3 can give. Task 5 adds the confirmation on
  that message line, and the stop. The document comes last.
- Covers each criterion of the story: the start by state (Tasks 1 and 3), the stop with its confirmation (Tasks 1 and
  5), the stop that lasts until the next login with `KeepAlive` (Tasks 1 and 5: `bootout` and no `kill`, and no start
  on its own; Task 6 says why), the new state at the next refresh (Tasks 3 and 5), the message line (Tasks 4 and 5),
  and the help line (Task 2). None is deferred.
- Certainty: Tasks 1, 3, 4 and 5 are medium, each with its precedent and its variation; Tasks 2 and 6 are high. The
  one point that the repo cannot answer is how long a real `bootout` takes: the deadline of 30 s covers it.
- Blast radius: Tasks 1 and 5 are high. Task 1 is the first code that changes launchd, and Task 5 is the
  confirmation that guards the stop.
- The planner chose these details, which the story does not give: the confirmation reads `Stop <name>? (y/N)`, and
  each key other than `y` and `ctrl+c` answers no; the message line has no color and shows until the next key press,
  which also does what it does; the lines of an error show on one line; the panes are one line shorter while the
  message line shows; `s` and `x` do nothing while the help shows and while a start or a stop of the same service
  runs; a start or a stop has a deadline of 30 s; the end of a start or a stop starts no read; svcview does not wait
  for a start or a stop at quit; a start of a service with no plist path gives an error with no command; the e2e fake
  records the calls in `$HOME/launchctl-calls.txt`.
