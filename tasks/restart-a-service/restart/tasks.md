## Contents
1. Task 1: Restart a service with launchctl kickstart -k in the gui domain of the user
2. Task 2: Show r restart in the help line and in the help
3. Task 3: Restart the selected service with r after a confirmation
4. Task 4: Say on the message line that r found the service not loaded
5. Task 5: Describe the restart in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-009 "Restart a service" (`launchd-tui/US-009`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Keys" and "Actions on launchd".

The user restarts the selected service with one key, so that the service reads its new config, or leaves a bad
state, with no separate stop and start. `r` on a service that is `loaded` or `running` asks for a confirmation on the
message line. On a yes, svcview runs `launchctl kickstart -k gui/<uid>/<name>`, which kills the running process of
the service, when it has one, and runs the service again. On a no, svcview runs no command. After the restart of a
`running` service, the detail pane shows the new PID and the new uptime no later than the next refresh. `r` on a
service that is `not loaded` runs no command, and the message line says that the service is not loaded. The help line
shows `r restart`.

The story criteria that this change satisfies:

- `r` on a service that is `loaded` or `running` asks for a confirmation. On a yes, svcview runs
  `launchctl kickstart -k gui/<uid>/<name>`. On a no, svcview runs no command.
- After a restart of a service that is `running`, the detail pane shows a new PID and a new uptime no later than the
  next refresh.
- `r` on a service that is `not loaded` runs no command, and the message line says that the service is not loaded.
- The help line shows `r restart`.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- A restart runs only `launchctl kickstart -k`. It runs no `bootout` and `bootstrap` pair, and no `kill`, `load`,
  `unload`, `enable` or `disable`.
- A restart does not load the plist again: launchd runs the service with the plist that it loaded at the bootstrap.
  A change to the plist takes effect after a stop and a start.
- `r` on a `not loaded` service does not start it, and asks nothing. `s` starts a service.
- The confirmation of `r` always shows. No setting turns it off, and no group adds a key or an action.
- No log, no `o` or `L` key, and no filter of the list.
- No line or glyph says that a restart runs. The row and the detail pane show the last read until the next refresh.
- svcview does not read the services when a restart ends. The next refresh shows the new PID and uptime, because
  `Domain.Services` must not run in two goroutines at the same time (`internal/launchd/domain.go:21`).
- svcview does not wait for a restart that still runs when it quits.
- The start and the stop do not change: `s` and `x` run the same commands, the confirmation of `x` still reads
  `Stop <name>? (y/N)`, and `y` after `x` still stops. Only the help line, which shows all actions, changes.
- No change to the status commands: a restart does not start or skip a run of a status command, and the status of
  the service shows its last run until its next run.
- No change to `svcview status`.
- The message line has no color for the text of `r` on a `not loaded` service, as for each of its texts.
- No layout for a narrow terminal, no change for `NO_COLOR`, and no `--all`.

**Deferred:** None.

## Codebase Context

- `internal/launchd/domain.go:54-75`: `Domain.Start` and `Domain.Stop` choose the launchctl command from the state of
  the service that they get, and run it through `launchctlCommand` (`internal/launchd/launchctl_command.go:22-88`):
  no shell, a deadline of 30 s, `Setpgid` and a kill of the process group, and an error that holds what launchctl
  printed, stderr first, or that names the command and its exit code. `serviceTarget` gives `gui/<uid>/<name>`.
  `Start` of a `loaded` service runs `kickstart` with no `-k`. `Stop` of a `not loaded` service runs no command.
- `launchctl` man page on this Mac: `kickstart [-kp] service-target` runs the service at once; `-k`: "If the service
  is already running, kill the running instance before restarting the service"; `-p`: "Upon success, print the PID
  of the new process". Because `-p` can print the new PID on success, launchd has started the new process when
  `kickstart` returns, so the first `launchctl list` after a restart ends shows the new PID. On a `loaded` service
  with no process, `kickstart -k` starts the service, as `kickstart` does. The man page does not say which signal
  `-k` sends, or how long it waits for the old process to exit; the deadline of 30 s of `launchctlCommand` covers a
  slow exit. The planner did not run `kickstart -k` against a real service.
- `internal/launchd/domain_test.go:64-90`: `TestMain` writes one fake launchctl for all tests. It records each call
  other than `list`, with its arguments separated by `\x1f` (`fakeLaunchctlCalls` and `launchctlCalls`,
  `domain_test.go:118-138`), and fails with `FAKE_LAUNCHCTL_STDERR` and `FAKE_LAUNCHCTL_EXIT`. The groups `start`
  (`domain_test.go:815-940`) and `stop` (`domain_test.go:942-1003`) of `TestDomain` are the precedent for a group
  `restart`.
- `internal/tui/screen.go:21` holds the help line, and `screen.go:421-433` the help, with 9 lines. `keys.golden` is
  80×15 (`screen_test.go:3276-3281`), and its help box has room for 9 lines, so one more line needs 80×16. Each of the
  26 golden files in `internal/tui/testdata` ends with the help line. `screen_test.go` holds the text of the help line
  in the groups `message line` (`screen_test.go:2945`) and `stop a service` (`screen_test.go:3077`), and in the test
  of the help line (`screen_test.go:3333-3340`); `e2e/tests/start-stop.test.ts:70` finds it too. The test of the cut
  at 12 columns (`screen_test.go:3342-3347`) keeps its text, because the new help line starts with the same 12 cells.
- `internal/tui/screen.go:165-220`: `press` clears `message` at each key. `x` sets `confirmation` for a selected
  service that is `loaded` or `running` and whose action has not ended. While a confirmation shows, `Update` gives
  each key to `answer` (`screen.go:131-135`), which calls `stop` with the service of the confirmation on `y`, quits on
  `ctrl+c`, and does nothing on each other key. `answer` knows one action, so the confirmation of `r` needs the
  confirmation to hold the action that `y` runs.
- `internal/tui/screen.go:270-278`: `actionOn` runs an action of `NewScreen` in a `tea.Cmd`, marks the service in
  `actionNotEnded`, and ends with `actionEnded`, whose error the message line shows (`screen.go:149-153`,
  `screen.go:388-397`). The end of an action starts no read.
- `internal/tui/screen.go:105-118`: `NewScreen(domain, read, runStatus, start, stop, settings)`. `screen_test.go`
  calls `tui.NewScreen` and `openActing` about 120 times, so a new parameter changes each call.
- `internal/tui/screen.go:555-566`: the detail pane shows the `PID` and the `up` of the service in the last read.
  `screen_test.go:786-798` shows a new PID and uptime at the next read, and `screen_test.go:2923-2938` shows the new
  state after a start ends, at the next refresh.
- Tests of the screen: `openActing` (`screen_test.go:347-354`), `serviceCalls` (`screen_test.go:324-340`),
  `failingWith`, `bottomLines`, `runAll`, `readNow`, `readsInTurn`, `changingAt` and `runFor`. The group
  `stop a service` (`screen_test.go:3072-3265`) is the precedent for the tests of `r` and its confirmation.
- `internal/cli/root.go:62-74`: `showScreen` gives the screen the start and the stop of the domain, with the context
  of svcview. Only e2e tests reach it.
- e2e: `e2e/fakes/launchctl` records each call other than `list` in `$HOME/launchctl-calls.txt`, and
  `launchctlCalls(home)` and `failLaunchctl(home, text)` are in `e2e/testUtils.ts:72-80`. `e2e/fakes/ps` prints
  `03:12:07` for each PID, so no e2e test can show a new uptime yet. `docs/e2e-tests.md:103-148` describes both fakes.
  `e2e/tests/start-stop.test.ts:75-140` is the precedent for the e2e tests of a confirmation. In Docker the tests run
  as root, so the domain is `gui/0`. Both fakes stay mode `100755` in git.
- `docs/cli.md`: the example screen and the table of the parts of the screen (`docs/cli.md:44-79`), the Keys table
  (`docs/cli.md:146-158`), "Start and stop a service" (`docs/cli.md:160-196`) and "The message line"
  (`docs/cli.md:198-212`). The table of parts and the Keys table link to `#start-and-stop-a-service`.
- `tasks/learnings.md` entries that apply: "Plan the layout change of the screen as the first task" (Task 2 rewrites
  every golden file before `r` does something); "Each key of the screen goes into the help and docs/cli.md" (Tasks 2
  and 5); "Add a parameter to tui.NewScreen with gofmt -r" (Task 3); "A screen test of a read error must select a
  service before the error" (Tasks 3 and 4); "A test that keys change nothing presses each key from its own start
  state" (Task 3); "Text from outside goes to the terminal through terminal.ReplaceControlCharacters" (Tasks 3 and 4);
  "Fake a program that svcview runs with a script first on PATH" (Tasks 1 and 3); "Test a Bubble Tea timer on the
  fake clock of synctest" (a fake restart that sleeps must end within the minute); "Only e2e tests reach the terminal
  branch of the root command" (Task 3); "Run clerk mutants and clerk audit run under mise exec"; "task test:e2e:local
  leaves e2e/package-lock.json in the tree".
- Words: restart is the action of `r`, beside start and stop. The launchctl words `kickstart` and `-k` stay in
  `internal/launchd`. The question of `r` is a confirmation, on the message line.

## Tasks

### Task 1: Restart a service with launchctl kickstart -k in the gui domain of the user

**Behavior:** `launchd.Domain` restarts one service. A restart of a service that is `loaded` or `running` runs
`launchctl kickstart -k gui/<uid>/<name>`. A restart of a service that is `not loaded` runs no command, and gives an
error that says that the service is not loaded. When launchctl fails or does not end, the restart gives the error
that a start or a stop gives.

**Acceptance Criteria:**
- [x] `Domain.Restart` of a `loaded` and of a `running` service runs `launchctl kickstart -k gui/<uid>/<name>`, with
      the uid of the domain and the name of the service, and gives no error when launchctl exits with 0.
- [x] `Domain.Restart` of a `not loaded` service runs no command, and gives an error that names the service and says
      that it is not loaded.
- [x] Each restart runs one launchctl command, `kickstart` with `-k`, and never `bootout`, `bootstrap`, `kill`,
      `load`, `unload`, `enable` or `disable`.
- [x] A name with a space, a quote and a `$` reaches launchctl unchanged.
- [x] A launchctl that exits with a code other than 0 gives an error whose text is what launchctl printed.
- [x] A launchctl that does not end before the context of the call ends is killed, and the error says that
      `launchctl kickstart -k gui/<uid>/<name>` did not end. The test ends the context, and does not wait 30 s.
- [x] The tests are in a group `restart` of `TestDomain`, with the fake launchctl that all tests share.

**Affected Files/Modules:**
- `internal/launchd/domain.go` — `Restart` of `Domain`
- `internal/launchd/domain_test.go` — the group for the restart

**Patterns to Follow:**
- `internal/launchd/domain.go:54-75`
- `internal/launchd/launchctl_command.go:26-49`
- `internal/launchd/domain_test.go:942-1003`
- `internal/launchd/domain_test.go:118-138`

**Testable:** Yes — through `Domain.Restart`, with the shared fake launchctl first on `PATH`.

**Certainty:** high — `Domain.Start` runs `kickstart` for a `loaded` service and `Domain.Stop` runs no command for a
`not loaded` one (`domain.go:54-75`), both through `launchctlCommand`; a restart adds one argument.

**Blast radius:** high — `kickstart -k` kills the running process of a service: a wrong target kills the process of
another service, and a wrong subcommand stops a service in place of a restart.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Show r restart in the help line and in the help

**Behavior:** The help line at the bottom of the screen shows `s start · x stop · r restart · ? help`. `?` lists `r`
with "restart the selected service, after a confirmation", on the line after `x`, with each other key as before. This
task changes only the text of the help; Task 3 gives `r` its action.

**Acceptance Criteria:**
- [x] The help line shows `s start · x stop · r restart · ? help` under the panes and under the help, cut at the width
      of the terminal as before.
- [x] `?` shows `r` with its action on the line after `x`, and each other key with its action as before. The golden
      file of the help shows all of its lines, in a terminal that is high enough for them.
- [x] Each golden file in `internal/tui/testdata` shows the new help line, and no other line of them changes, except
      the help in `keys.golden`.
- [x] The e2e test that finds the help line under a message line finds the new help line.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the help line and the help
- `internal/tui/screen_test.go` — the text of the help line in the tests, and the size of the test of the help
- `internal/tui/testdata/` — each golden file
- `e2e/tests/start-stop.test.ts` — the help line under the message line

**Patterns to Follow:**
- `internal/tui/screen.go:21`
- `internal/tui/screen.go:421-433`
- `internal/tui/screen_test.go:3276-3281`
- `internal/tui/screen_test.go:3333-3340`

**Testable:** Yes — through the `Screen` model and its golden files.

**Certainty:** high — the help line (`screen.go:21`), the help (`screen.go:421-433`) and their tests
(`screen_test.go:3276-3281`, `screen_test.go:3333-3340`) are the precedent; this task changes their text.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, `task test:e2e` passes, and `git diff internal/tui/testdata`
shows only the help line changed in each golden file other than `keys.golden`.

**Depends on:** None

### Task 3: Restart the selected service with r after a confirmation

**Behavior:** `r` on the selected service, when it is `loaded` or `running`, shows the confirmation
`Restart <name>? (y/N)` on the message line, with the full name of the service. `y` calls the restart of the screen
with that service, in the background. Each other key answers no: it hides the confirmation and does nothing else.
`ctrl+c` still quits. `y` runs the action of the key that asked: the restart after `r`, and the stop after `x`.
svcview gives the screen the restart of its domain, so a yes runs `launchctl kickstart -k gui/<uid>/<name>`. An error
of the restart shows on the message line, as an error of a start or a stop does. After the restart ends, the detail
pane shows the PID and the uptime that the next refresh reads.

**Acceptance Criteria:**
- [x] `r` on a `loaded` and on a `running` service shows `Restart <name>? (y/N)` on the message line, with the full
      name of the service, and calls nothing yet.
- [x] `y` hides the confirmation and calls the restart one time with the service that the confirmation names, also
      when a refresh changed the selected row, or removed the service from the list, while the confirmation showed.
- [x] `y` after `r` calls the restart and not the stop, and `y` after `x` calls the stop and not the restart.
- [x] Each key other than `y` and `ctrl+c` answers no to the confirmation of `r`, for example `n`, `esc`, `q`, `↓`,
      `r`, `x` and `s`: it hides the confirmation, calls nothing, and does nothing else. Each key is pressed from its
      own start state. `r` answers no to the confirmation of `x`. `ctrl+c` quits while the confirmation of `r` shows.
- [x] `r` shows no confirmation and calls nothing while the help shows, with no rows, while the list shows a read
      error after a read that selected a service, and while the start, the stop or the restart of the service has
      not ended. `s` and `x` do nothing on a service whose restart has not ended.
- [x] The confirmation of `r` shows each control character of the name as `�`.
- [x] An error of the restart shows on the message line, as an error of a start or a stop does.
- [x] After the restart of a `running` service ends, the detail pane shows the PID and the uptime that the next
      refresh reads, no later than one `refresh` after the restart ends. The end of the restart starts no read.
- [x] `tui.NewScreen` takes the restart, and `internal/cli/root.go` gives it the restart of the domain with the
      context of svcview.
- [x] The e2e fake `ps` prints, for each PID, the elapsed time that a helper in `e2e/testUtils.ts` writes in the home
      folder, and `03:12:07` when no test wrote one. "The fake ps" in `docs/e2e-tests.md` names the helper.
- [x] e2e: `r` and `y` on a `running` and on a `loaded` service record `kickstart -k gui/<uid>/<name>`; `r` and `n`
      record nothing. After `r`, `y`, a new output of `launchctl list` with a new PID, and a new elapsed time of the
      fake `ps`, the detail pane shows the new PID and the new uptime at the next refresh, with no key press.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `r`, the restart, and the action that the confirmation runs
- `internal/tui/screen_test.go` — the tests of `r` and its confirmation, and the new parameter of each call of
  `tui.NewScreen`
- `internal/cli/root.go` — the restart of the domain for the screen
- `e2e/fakes/ps` — the elapsed time that a test sets
- `e2e/testUtils.ts` — the helper that sets the elapsed time
- `e2e/tests/restart.test.ts` — new: the e2e tests of `r`
- `docs/e2e-tests.md` — the elapsed time of the fake `ps`

**Patterns to Follow:**
- `internal/tui/screen.go:186-220`
- `internal/tui/screen.go:270-278`
- `internal/tui/screen_test.go:3072-3265`
- `internal/tui/screen_test.go:2923-2938`
- `internal/tui/screen_test.go:786-798`
- `internal/cli/root.go:62-74`
- `e2e/tests/start-stop.test.ts:75-140`
- `e2e/fakes/launchctl`
- `e2e/testUtils.ts:72-80`
- `task:1`
- `task:2`

**Testable:** Yes — through the `Screen` model with a fake restart and fake reads, and in e2e tests with the fake
launchctl and the fake ps.

**Certainty:** medium — `x` and `answer` are the precedent for a key that asks before it acts (`screen.go:190-193`,
`screen.go:210-220`), but `answer` runs only the stop, so the confirmation must now hold the action that `y` runs,
and the e2e fake `ps` prints one fixed elapsed time.

**Blast radius:** high — the confirmation is the one step between a key and the kill of the process of a service: a
`y` that restarts another service than the confirmation names, a restart with no `y`, or a stop after `r`, ends the
work of a service that the user needs.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1, Task 2

### Task 4: Say on the message line that r found the service not loaded

**Behavior:** `r` on a selected service that is `not loaded` shows `<name> is not loaded, so svcview cannot restart it`
on the message line, with the full name of the service. It shows no confirmation and calls nothing. The next key
press hides the text, as it hides each text of the message line.

**Acceptance Criteria:**
- [ ] `r` on a `not loaded` service shows `<name> is not loaded, so svcview cannot restart it` on the message line,
      shows no confirmation, and calls nothing. `y` after it calls nothing.
- [ ] The text shows each control character of the name as `�`.
- [ ] The next key press hides the text and does what it does: for example `↓` also selects the next row.
- [ ] `r` shows no text while the help shows, with no rows, while the list shows a read error after a read that
      selected a service, and while the start of the service has not ended.
- [ ] e2e: `r` on a `not loaded` service shows the text on the line above the help line, and records no launchctl
      call.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `r` on a `not loaded` service
- `internal/tui/screen_test.go` — the tests of `r` on a `not loaded` service
- `e2e/tests/restart.test.ts` — the e2e test of `r` on a `not loaded` service

**Patterns to Follow:**
- `internal/tui/screen.go:165-193`
- `internal/tui/screen.go:388-397`
- `internal/tui/screen_test.go:2943-3070`
- `e2e/tests/start-stop.test.ts:60-72`
- `task:3`

**Testable:** Yes — through the `Screen` model with a fake restart, and in an e2e test with the fake launchctl.

**Certainty:** high — the message line shows `message` until the next key press (`screen.go:149-153`, `screen.go:166`,
`screen.go:388-397`); this task sets it from a key in place of the end of an action.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 3

### Task 5: Describe the restart in docs/cli.md

**Behavior:** `docs/cli.md` describes `r`, its confirmation, the command that it runs, the text of `r` on a
`not loaded` service, and the new help line. The Keys table matches the help of the screen.

**Acceptance Criteria:**
- [ ] The Keys table lists `r` as the help shows it, and says that `y` answers yes to its confirmation and each other
      key answers no.
- [ ] The example screen and the table of the parts of the screen show the help line
      `s start · x stop · r restart · ? help`. The row of the message line names the confirmation of `x` and of `r`,
      the error of a restart, and the text of `r` on a `not loaded` service.
- [ ] A section on the restart says that `r` on a `loaded` or a `running` service asks `Restart <name>? (y/N)`, and
      that `y` runs `launchctl kickstart -k gui/<uid>/<name>`, which kills the process of a `running` service and
      runs the service again; that `r` on a `not loaded` service runs nothing and says so on the message line; and
      that the detail pane shows the new PID and the new uptime at the next refresh after the restart ends.
- [ ] It says that a restart does not load the plist again, so a change to the plist takes effect after `x` and `s`.
- [ ] The rules of the keys while an action runs, and "The message line", name the restart beside the start and the
      stop.
- [ ] Each link to a section of `docs/cli.md` still finds its section.

**Affected Files/Modules:**
- `docs/cli.md` — the restart, its confirmation, the message line, and the help line

**Patterns to Follow:**
- `docs/cli.md:146-212`

**Testable:** No — a document.

**Certainty:** high — the Keys table, "Start and stop a service" and "The message line" (`docs/cli.md:146-212`) are
the precedent, and the proposal gives the key ("Keys") and the command ("Actions on launchd").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the help of the screen and against the tests of Tasks 3 and 4.

**Depends on:** Task 4

## Summary

- 5 tasks. Task 1 puts the restart in `internal/launchd`, the only package that knows launchctl. Task 2 changes the
  help line, and with it every golden file, before `r` does something, so an audit fix of a later task folds into its
  own commit. Task 3 gives `r` its confirmation and its restart, makes the confirmation hold the action that `y` runs,
  and wires the restart into svcview, with the e2e fake `ps` that lets a test see a new uptime. Task 4 adds the text
  for `r` on a `not loaded` service, on the guards that Task 3 sets. The document comes last.
- Covers each criterion of the story: the confirmation and `kickstart -k` (Tasks 1 and 3), the new PID and uptime at
  the next refresh (Task 3), `r` on a `not loaded` service (Tasks 1 and 4), and the help line (Task 2). None is
  deferred.
- Certainty: Task 3 is medium, because the confirmation now runs one of two actions. Tasks 1, 2, 4 and 5 are high:
  the start, the stop, the confirmation and the message line of US-008 are the precedent for each.
- Blast radius: Tasks 1 and 3 are high. Task 1 runs `kickstart -k`, which kills the process of a service, and Task 3
  changes the confirmation that guards both the restart and the stop.
- The planner chose these details, which the story does not give: the confirmation reads `Restart <name>? (y/N)`, and
  each key other than `y` and `ctrl+c` answers no, as for `x`; the text for a `not loaded` service reads
  `<name> is not loaded, so svcview cannot restart it`, and `Domain.Restart` of a `not loaded` service gives an error
  with no command; `r` does nothing while the help shows and while an action of the same service runs; the help line
  puts `r restart` after `x stop`, in the order of the proposal; the end of a restart starts no read; the e2e fake
  `ps` gets an elapsed time that a test sets.
