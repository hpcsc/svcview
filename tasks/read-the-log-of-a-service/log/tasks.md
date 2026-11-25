## Contents
1. Task 1: Show o log in the help line and in the help
2. Task 2: Open the StandardOutPath of the selected service in less +F with o
3. Task 3: Keep svcview open when ctrl-c reaches a program that has the terminal
4. Task 4: Open the StandardErrorPath, or say on the message line that the service has no log path
5. Task 5: Describe the log in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-010 "Read the log of a service" (`launchd-tui/US-010`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Keys", "Actions on launchd" (the row "Log"), and "Actions of
a Group" (a group action gives the terminal to its command "as for the log action").

The user opens the log of the selected service with one key, so that they read what the service prints with no
search for its log path. `o` gives the terminal to `less +F` with the `StandardOutPath` of the service. `less +F`
shows the end of the file and each new line, as `tail -f` does; `ctrl-c` stops the follow, and `q` ends `less`.
When `less` ends, the screen shows again, with the same service selected. When the plist has no `StandardOutPath`,
`o` opens the `StandardErrorPath`. When the service has neither path, `o` does not open `less`, and the message line
says that the service has no log path. The help line shows `o log`.

The screen gives the terminal to a program in one way, which this change adds: the program and its arguments, with
no shell, until the program ends, and then the screen again. A failure of the program shows on the message line, and
a `ctrl-c` that the program gets does not end svcview. Each key that gives the terminal to a program uses this way.

The story criteria that this change satisfies:

- `o` gives the terminal to `less +F` with the `StandardOutPath` of the service. When `less` ends, the screen shows
  again, with the same service selected.
- When the plist has no `StandardOutPath`, `o` opens the `StandardErrorPath`.
- When the service has neither path, `o` does not open `less`, and the message line says that the service has no
  log path.
- The help line shows `o log`.

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
- No keys that a group adds, and no `actions` setting. `o` is the only key that gives the terminal to a program.
- svcview runs only `less +F -- <log path>`. It reads no `PAGER`, has no setting for the program of the log, and
  does not set or clear `LESS`: `less` gets the environment of svcview, with the `LESS` of the user.
- svcview does not read the log file, does not check that it exists, and does not resolve a relative log path.
  `less` gets the path as the plist gives it, and `less` reports a file that it cannot open.
- svcview does not show what the program writes to stderr after the program ends. The program writes it to the
  terminal, and the screen covers it when it shows again. The message line shows the exit code.
- The program that has the terminal runs with no deadline, until the user ends it. svcview does not stop it at
  `SIGTERM`: the status runs stop at once, and svcview ends when the program ends.
- While a program has the terminal, svcview draws nothing, reads no key, reads no services, and starts no status
  run. A start, a stop, a restart or a status run that started before goes on, and the screen shows its end after
  the program ends.
- `o` asks no confirmation, and no setting adds one.
- The start, the stop, the restart and their confirmations do not change. Only the help line, which shows all
  actions, changes.
- No change to the status commands, and no change to the output of `svcview status`. `svcview status` still ends
  its run at `SIGINT`, `SIGTERM` and `SIGHUP`.
- No layout for a narrow terminal, no change for `NO_COLOR`, no `--all`, and no filter of the list.

**Deferred:** None.

## Codebase Context

- `internal/service/service.go:40-41`: `service.Service` has `StdoutPath` and `StderrPath`. `Domain.Services` fills
  them from the `StandardOutPath` and `StandardErrorPath` of the plist (`internal/launchd/domain.go:35`,
  `internal/launchd/plist.go:24-35`); an empty text is a plist with no such property, or with a value that is not a
  text. No change to `internal/launchd` or `internal/service`.
- Bubble Tea v2.1.2 (`charm.land/bubbletea/v2`, file `exec.go` in the module cache): `tea.ExecProcess` releases the
  terminal, runs an `exec.Cmd` with the stdin and stdout of the program and the stderr of svcview, takes the
  terminal back and draws again, and then sends the message of its callback with the error of the run. The event
  loop waits in the run (`tea.go:869-871`), so no key, refresh, status tick or end of a run reaches `Update` while
  the program runs; they arrive after it. While the terminal is released, the signal handler of Bubble Tea ignores
  `SIGINT` and `SIGTERM` (`tea.go:692-711`, `tea.go:1449-1471`). No code of svcview gives the terminal to a program.
- Probe 1, `ctrl-c` ends svcview today. The terminal sends `SIGINT` for `ctrl-c` to its foreground process group,
  which holds svcview and the program. `cmd/svcview/main.go:13` ends the context of svcview at `os.Interrupt`, and
  the screen runs with that context (`internal/cli/root.go:73`). The planner ran a Bubble Tea program with the
  signals of `main.go` in a pty; it gave the terminal to a script that traps `SIGINT` and ends at `q`. After
  `ctrl-c` and `q`, the script got the interrupt, but `Run` returned `program was killed: context canceled`, and the
  screen did not show again. With only `SIGHUP` and `SIGTERM` in the context, the script got `ctrl-c`, ended at
  `q`, and the screen showed again with no error. `svcview status` needs `SIGINT` to end its run: `docs/cli.md:698`
  says so, `internal/cli/status.go:52` prints the error of a run that a signal stopped, and no test sends it
  `SIGINT`.
- Probe 2, the e2e shell. The e2e image runs the `sh -c` of `openCli` with dash (`/usr/bin/dash`). The same probe in
  the image `svcview-e2e`, under `sh -c '<probe>; echo "EXIT:$?"'`, showed the screen again after `ctrl-c` and `q`,
  but after the probe ended, dash ended itself by `SIGINT` and printed no `EXIT:` line. An e2e test that presses
  `ctrl-c` must not wait for `EXIT:0`.
- Probe 3, the Go tests. `tea.ExecProcess` of `sh -c 'sleep 0.2; exit 3'` ran in a `synctest` bubble, in a program
  with the options of `runSized` (`internal/tui/screen_test.go:225-245`): the child ran 0.2 s of real time, the
  fake clock held still meanwhile, and the end message (`exit status 3`) reached `Update` before the quit that the
  test sent after 100 ms of fake time.
- `less` on this Mac is less 668. `less +F <missing file>` prints `<path>: No such file or directory` and exits with
  1; `less +F <folder>` prints `<path> is a directory` and exits with 1. `less +F +Fx.log` reads the name as a
  command of `less` and opens no file; `less +F -- +Fx.log` opens the file. A command of `less` that starts with `!`
  runs a shell command. With an empty `LESS`, `ctrl-c` stops the follow and `q` ends `less` with 0. The `LESS` of
  the shell of this machine holds `-F` (quit if one screen): with it, `less +F` on a log that fits one screen ends
  with 0 at the first `ctrl-c`.
- The e2e image (`node:24-trixie-slim`) has no `less`, as it has no `ps`. e2e tests need a fake `less` in
  `e2e/fakes`, as `e2e/fakes/launchctl` and `e2e/fakes/ps` are (`docs/e2e-tests.md:103-152`); each fake stays mode
  `100755` in git. `addPlist` (`e2e/testUtils.ts:33-52`) writes only `Label` and `EnvironmentVariables`, and
  `launchctlCalls` (`e2e/testUtils.ts:82-86`) is the precedent for a helper that reads the calls of a fake.
- `internal/tui/screen.go:172-224`: `press` clears `message` at each key. The `r` branch (`screen.go:201-209`) sets
  `message` from a key for a `not loaded` service. While a confirmation shows, `Update` gives each key to `answer`
  (`screen.go:226-236`), which answers no to each key other than `y` and `ctrl+c`, so `o` answers no with no new
  code. `serviceToActOn` (`screen.go:356-359`) is the guard of `s`, `x` and `r`: a selected service, no help, and no
  action of the service that has not ended. `actionEnded` shows an error on the message line on one line, through
  `terminal.ReplaceControlCharacters` (`screen.go:155-160`).
- `internal/tui/screen.go:21` holds the help line, and `screen.go:442-455` the help, with 10 lines. `keys.golden` is
  80×16 (`screen_test.go:3498-3505`), which has room for 10 lines, so one more line needs 80×17. Each of the 26
  golden files in `internal/tui/testdata` ends with the help line. `screen_test.go` holds the text of the help line
  in the groups `message line` (`screen_test.go:2946`), `stop a service` (`screen_test.go:3078`) and
  `restart a service` (`screen_test.go:3273`), and in the test of the help line (`screen_test.go:3556-3562`). The
  test of the cut at 12 columns (`screen_test.go:3564-3569`) keeps its text. No e2e test holds the whole help line:
  they find it by `? help`.
- The tests of the screen are in package `tui_test`, with the build tag `unit`, and `internal/tui` has no
  `TestMain`. No test in `internal/tui` runs in parallel, so `t.Setenv` is safe there. Helpers: `runSized`,
  `runWithInput` (`screen_test.go:167-178`), `openActing` (`screen_test.go:348-355`), `readsInTurn`, `readNow`,
  `bottomLines`, `selectedName`. The groups `restart a service` (`screen_test.go:3268-3489`) and `message line`
  (`screen_test.go:2944-3072`) are the precedent for the tests of `o`.
- `internal/launchd/domain_test.go:64-102`: `TestMain` writes one copy of each fake program for all tests, because
  macOS checks each new executable file at its first run, and each test gives the fake its files through
  environment variables and puts the folder first on `PATH` (`putFirstOnPath`, `fakeProgram`).
- `internal/launchd/launchctl_command.go:76-88`: the words of a failed program: `<command> exited with code N`, and
  `<command> ended by <signal>` with `service.SignalName`. `launchctl_command.go:45-46` words a program that did
  not start.
- `internal/cli/root.go:62-75`: `showScreen` starts the status runs with the context of svcview and stops them after
  `Run` returns, whatever ended it (`internal/cli/status_runs.go:42-48`). `internal/cli/status.go:31-56`:
  `printStatusSteps` runs the status command with the same context.
- e2e signals: `e2e/tests/status.test.ts:189-203` sends `SIGHUP` and `SIGTERM` to svcview, whose PID a status
  command writes as its `$PPID`. `e2e/tests/status-subcommand.test.ts` runs `svcview status` with `runCli`.
- `tasks/learnings.md` entries that apply: "Plan the layout change of the screen as the first task" (Task 1);
  "Each key of the screen goes into the help and docs/cli.md" (Tasks 1 and 5); "Fake a program that svcview runs
  with a script first on PATH" (Tasks 2 and 3); "Test a Bubble Tea timer on the fake clock of synctest" (Task 2);
  "Text from outside goes to the terminal through terminal.ReplaceControlCharacters" (Tasks 2 and 4); "A screen test
  of a read error must select a service before the error" (Tasks 2 and 4); "A test that keys change nothing presses
  each key from its own start state" (Task 2); "An e2e test finds the help line by a short part of it" (Tasks 2 and
  4); "A program in a process group of its own gets no SIGHUP from the terminal" and "An e2e test gets the PID of
  svcview as the PPID of a status command" (Task 3); "Only e2e tests reach the terminal branch of the root command"
  (Task 3); "Plan each Bubble Tea message and private type in design.yaml" (Task 2); "Run clerk mutants and clerk
  audit run under mise exec"; "task test:e2e:local leaves e2e/package-lock.json in the tree".
- Words: the log of a service is the file that `o` shows; its log path is the `StandardOutPath`, or the
  `StandardErrorPath` when the plist has no `StandardOutPath`. svcview gives the terminal to a program. The text of
  `o` and the failure of a program show on the message line.

## Tasks

### Task 1: Show o log in the help line and in the help

**Behavior:** The help line at the bottom of the screen shows `s start · x stop · r restart · o log · ? help`. `?`
lists `o` with "show the log of the selected service", on the line after `r`, with each other key as before. This
task changes only the text of the help; Task 2 gives `o` its action.

**Acceptance Criteria:**
- [x] The help line shows `s start · x stop · r restart · o log · ? help` under the panes and under the help, cut at
      the width of the terminal as before.
- [x] `?` shows `o` with its action on the line after `r`, and each other key with its action as before. The golden
      file of the help shows all of its lines, in a terminal that is high enough for them.
- [x] Each golden file in `internal/tui/testdata` shows the new help line, and no other line of them changes, except
      the help in `keys.golden`.
- [x] The tests that hold the text of the help line hold the new text, and the test of the cut at 12 columns keeps
      its text.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the help line and the help
- `internal/tui/screen_test.go` — the text of the help line in the tests, and the size of the test of the help
- `internal/tui/testdata/` — each golden file

**Patterns to Follow:**
- `internal/tui/screen.go:21`
- `internal/tui/screen.go:442-455`
- `internal/tui/screen_test.go:3498-3505`
- `internal/tui/screen_test.go:3556-3570`

**Testable:** Yes — through the `Screen` model and its golden files.

**Certainty:** high — the help line (`screen.go:21`), the help (`screen.go:442-455`) and their tests
(`screen_test.go:3498-3505`, `screen_test.go:3556-3570`) are the precedent; this task changes their text, as the
help of `r` did.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `git diff internal/tui/testdata` shows only the help line
changed in each golden file other than `keys.golden`.

**Depends on:** None

### Task 2: Open the StandardOutPath of the selected service in less +F with o

**Behavior:** `o` on the selected service, when its plist has a `StandardOutPath`, gives the terminal to
`less +F -- <StandardOutPath>`. svcview stops drawing and reading keys, and runs `less` with no shell in the
terminal. When `less` ends, the screen shows again, with the same service selected and the same focus. The screen
gives the terminal to a program in one way, a method that takes the program and its arguments, so that each key
that gives the terminal to a program uses it. When the program cannot start, exits with a code other than 0, or a
signal ends it, the screen shows again, and the message line says why.

**Acceptance Criteria:**
- [x] `o` on a selected service whose plist has a `StandardOutPath` runs `less +F -- <StandardOutPath>`, with no
      shell, for a `not loaded`, a `loaded` and a `running` service.
- [x] A path with a space, a quote and a `$` reaches `less` unchanged, and a path that starts with `+` or `-`
      reaches `less` after `--`.
- [x] When `less` exits with 0, the screen shows again with the service that was selected before `o`, also when it
      is not the first row, with the same focus, and with no text on the message line.
- [x] When `less` exits with a code other than 0, the screen shows again, and the message line names the command
      and the code, for example `less +F -- /Users/me/Library/Logs/worker.out.log exited with code 1`. When a
      signal ends it, the message line says so, for example `... ended by SIGTERM`. When svcview cannot start it,
      the message line shows why, for example with `exec: "less": executable file not found in $PATH`.
- [x] The text of a failure shows on one line, with each control character of the path as `�`. The next key press
      hides it and does what it does, as for each text of the message line.
- [x] `o` runs nothing while the help shows, with no rows, and while the list shows a read error after a read that
      selected a service. While a confirmation shows, `o` answers no, as each key other than `y` does, and runs
      nothing.
- [x] `o` opens the log also while the start, the stop or the restart of the selected service has not ended.
- [x] The Go tests of the screen run a fake `less` from `PATH`, which `TestMain` of `internal/tui` writes one time
      and which records its arguments and exits with the code that the test gives. They run the screen in a real
      Bubble Tea program in a `synctest` bubble.
- [x] e2e: `e2e/fakes/less` records its arguments in the home folder, shows a line on the terminal, and ends when
      the test types `q` and `enter`. A helper makes it exit with a code at once. `addPlist` can write a
      `StandardOutPath` and a `StandardErrorPath`, and a helper reads the calls of the fake `less`.
      `docs/e2e-tests.md` describes the fake `less` and the helpers.
- [x] e2e: `↓` and `o` on a service with a `StandardOutPath` show the fake `less`, which records
      `+F -- <StandardOutPath>`. After `q` and `enter`, the screen shows again, with `▌` on the same row.
- [x] e2e: when the fake `less` exits with 1, the line above the help line shows
      `less +F -- <StandardOutPath> exited with code 1`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `o`, the way the screen gives the terminal to a program, and the message of its end
- `internal/tui/screen_test.go` — `TestMain` with the fake `less`, and the tests of `o`
- `e2e/fakes/less` — new: the fake `less`
- `e2e/testUtils.ts` — the log paths of `addPlist`, and the helpers of the fake `less`
- `e2e/tests/log.test.ts` — new: the e2e tests of `o`
- `docs/e2e-tests.md` — the fake `less` and its helpers

**Patterns to Follow:**
- `internal/tui/screen.go:201-209`
- `internal/tui/screen.go:155-160`
- `internal/tui/screen.go:286-294`
- `internal/launchd/launchctl_command.go:76-88`
- `internal/launchd/domain_test.go:64-102`
- `internal/tui/screen_test.go:225-245`
- `internal/tui/screen_test.go:3268-3489`
- `e2e/fakes/launchctl`
- `e2e/testUtils.ts:33-52`
- `e2e/testUtils.ts:82-86`
- `e2e/tests/restart.test.ts:50-64`
- `docs/e2e-tests.md:103-152`
- `task:1`

**Testable:** Yes — through the `Screen` model in a real Bubble Tea program with a fake `less` first on `PATH`, and
in e2e tests with the fake `less` in `e2e/fakes`.

**Certainty:** medium — a key that sets the message line (`screen.go:201-209`), the shared fake programs of
`internal/launchd` (`domain_test.go:64-102`) and the words of a failed launchctl (`launchctl_command.go:76-88`) are
the precedent, but no code of the repo gives the terminal to a program; Probe 3 in Codebase Context shows
`tea.ExecProcess` in a `synctest` bubble with the options of `runSized`.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1

### Task 3: Keep svcview open when ctrl-c reaches a program that has the terminal

**Behavior:** While a program has the terminal, `ctrl-c` sends `SIGINT` to the program and to svcview, which are in
the foreground process group of the terminal. svcview does not end at that `SIGINT`: `less` stops its follow, `q`
ends `less`, and the screen shows again. A `SIGINT` while the screen shows and no program has the terminal still
ends svcview and stops each status run. `svcview status` still stops its run at `SIGINT`. `SIGHUP` and `SIGTERM`
end svcview as before.

**Acceptance Criteria:**
- [ ] e2e: after `o`, `ctrl-c` in the terminal reaches the fake `less`, which shows that it got it and goes on.
      After `q` and `enter`, the screen shows again with the same service selected, and `↓` then selects the next
      row, so svcview still runs. The test does not wait for `EXIT:0` after `ctrl-c`.
- [ ] e2e: a `SIGINT` to svcview while the screen shows, with no program in the terminal, ends svcview, and the
      child process of a status command does not write its file, as for `SIGHUP` and `SIGTERM`.
- [ ] e2e: `svcview status` that gets `SIGINT` during its run stops the program with each process that it started,
      prints no `valid`, and exits with 1.
- [ ] The fake `less` traps `SIGINT`, shows that it got it, and goes on, as `less +F` does. `docs/e2e-tests.md`
      says so.

**Affected Files/Modules:**
- `cmd/svcview/main.go` — the signals that end the context of svcview
- `internal/cli/status.go` — `svcview status` ends its run at `SIGINT`
- `internal/cli/root.go` — the context of the screen, if the change of the signals needs it
- `e2e/fakes/less` — the trap of `SIGINT`
- `e2e/tests/log.test.ts` — `ctrl-c` in the fake `less`
- `e2e/tests/status.test.ts` — `SIGINT` to the screen
- `e2e/tests/status-subcommand.test.ts` — `SIGINT` to `svcview status`
- `docs/e2e-tests.md` — the trap of the fake `less`

**Patterns to Follow:**
- `cmd/svcview/main.go:12-16`
- `internal/cli/status.go:31-56`
- `internal/cli/root.go:62-75`
- `internal/cli/status_runs.go:42-48`
- `e2e/tests/status.test.ts:189-203`
- `e2e/tests/status-subcommand.test.ts:1-40`
- `task:2`

**Testable:** Yes — in e2e tests only. The Go tests have no terminal and send no signal to svcview, so mutants in
`cmd/svcview` and in the screen branch of `internal/cli` live in `clerk mutants`; accept each with the e2e test that
kills it, after a check by hand in Docker.

**Certainty:** medium — `main.go:13` and the signal tests of `e2e/tests/status.test.ts:189-203` are the precedent,
but no code lets `SIGINT` end `svcview status` and not the screen; Probes 1 and 2 in Codebase Context show the change
and its e2e shell.

**Blast radius:** high — it changes the signals that end svcview (`tasks/learnings.md`, "A program in a process group
of its own gets no SIGHUP from the terminal"): a mistake leaves a status command and its processes running after
`svcview status` ends, or ends the screen at each `ctrl-c` in `less`.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 4: Open the StandardErrorPath, or say on the message line that the service has no log path

**Behavior:** `o` on a selected service whose plist has no `StandardOutPath` gives the terminal to
`less +F -- <StandardErrorPath>`. When the plist has neither path, `o` runs nothing, and the message line shows
`<name> has no log path in its plist`, with the full name of the service. The next key press hides the text, as it
hides each text of the message line.

**Acceptance Criteria:**
- [ ] `o` on a service with a `StandardErrorPath` and no `StandardOutPath` runs `less +F -- <StandardErrorPath>`.
- [ ] `o` on a service with both paths runs `less +F -- <StandardOutPath>`.
- [ ] `o` on a service with neither path runs nothing, and shows `<name> has no log path in its plist` on the
      message line, with each control character of the name as `�`.
- [ ] The next key press hides the text and does what it does: for example `↓` also selects the next row.
- [ ] `o` shows no text while the help shows, with no rows, and while the list shows a read error after a read that
      selected a service.
- [ ] e2e: `o` on a service with only a `StandardErrorPath` makes the fake `less` record
      `+F -- <StandardErrorPath>`. `o` on a service with neither path shows the text on the line above the help
      line, and the fake `less` records no call.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the `StandardErrorPath`, and the text of a service with no log path
- `internal/tui/screen_test.go` — the tests of both
- `e2e/tests/log.test.ts` — the e2e tests of both

**Patterns to Follow:**
- `internal/tui/screen.go:201-209`
- `internal/tui/screen_test.go:3368-3390`
- `e2e/tests/restart.test.ts:50-64`
- `task:2`

**Testable:** Yes — through the `Screen` model with the fake `less`, and in e2e tests with the fake `less`.

**Certainty:** high — `r` on a `not loaded` service sets the message line from a key and runs nothing
(`screen.go:201-209`, `screen_test.go:3368-3390`), and `o` of Task 2 opens a path.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 5: Describe the log in docs/cli.md

**Behavior:** `docs/cli.md` describes `o`, the program that it runs, the log path that it opens, what the screen
does while `less` runs, the text of a service with no log path, the failure of `less`, the new help line, and the
`SIGINT` rule of the screen. The Keys table matches the help of the screen.

**Acceptance Criteria:**
- [ ] The Keys table lists `o` as the help shows it.
- [ ] The example screen and the table of the parts of the screen show the help line
      `s start · x stop · r restart · o log · ? help`. The row of the message line names the text of `o` on a
      service with no log path, and the failure of `less`.
- [ ] A section on the log says that `o` gives the terminal to `less +F -- <StandardOutPath>`, or to the
      `StandardErrorPath` when the plist has no `StandardOutPath`; that `less` shows the end of the file and each
      new line, `ctrl-c` stops the follow, and `q` ends `less`; that the screen then shows again with the same
      service selected; that `o` works in each state and while an action of the service runs; and that while `less`
      runs, svcview draws nothing, reads no services and starts no status run.
- [ ] It says that `less` gets the `LESS` of the user, so that with `-F`, `less` ends at `ctrl-c` when the log fits
      on one screen; and that svcview does not check the file, so `less` reports a file that it cannot open, and the
      message line shows its exit code.
- [ ] "The message line" names the text of `o` and the failure of a program. The text on the signals of the screen
      (`docs/cli.md:596-597`) names `SIGINT`, and says that a `ctrl-c` that `less` gets does not end svcview.
- [ ] Each link to a section of `docs/cli.md` still finds its section.

**Affected Files/Modules:**
- `docs/cli.md` — the log, the message line, the help line, and the signals of the screen

**Patterns to Follow:**
- `docs/cli.md:146-233`
- `docs/cli.md:590-600`

**Testable:** No — a document.

**Certainty:** high — the Keys table, "Start, stop and restart a service" and "The message line"
(`docs/cli.md:146-233`) are the precedent, and the proposal gives the key ("Keys") and the program ("Actions on
launchd").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the help of the screen and against the tests of Tasks 2, 3 and 4.

**Depends on:** Task 3, Task 4

## Summary

- 5 tasks. Task 1 changes the help line, and with it every golden file, before `o` does something, so an audit fix
  of a later task folds into its own commit. Task 2 gives `o` its action on the `StandardOutPath` and adds the one
  way the screen gives the terminal to a program, with the fake `less` of the Go tests and of the e2e tests. Task 3
  comes next because it is the risk of the change: without it, the `ctrl-c` that the story names ends svcview.
  Task 4 adds the `StandardErrorPath` and the text for a service with no log path, on the guards of Task 2. The
  document comes last.
- Covers each criterion of the story: `o` with the `StandardOutPath` and the screen again (Tasks 2 and 3), the
  `StandardErrorPath` (Task 4), no log path (Task 4), and the help line (Task 1). None is deferred.
- Certainty: Tasks 2 and 3 are medium, because no code of the repo gives the terminal to a program, and no code
  lets `SIGINT` end `svcview status` and not the screen; three probes in Codebase Context show both. Tasks 1, 4 and
  5 are high.
- Blast radius: Task 3 is high, because it changes the signals that end svcview and stop its status runs.
- The planner chose these details, which the story does not give: svcview runs `less +F -- <path>`, so a path that
  starts with `+` or `-` is a file for `less` and not a command; the text for no log path reads
  `<name> has no log path in its plist`; the help line puts `o log` after `r restart`, in the order of the proposal,
  and the help reads "show the log of the selected service"; `o` asks no confirmation, works in each state, and
  works while an action of the service runs; a failure of `less` shows on the message line with the words of a
  failed launchctl; svcview passes the `LESS` of the user to `less`; `SIGINT` no longer ends the context of the
  screen, and Bubble Tea ends the screen at a `SIGINT` when no program has the terminal; svcview does not stop the
  program at `SIGTERM`.
