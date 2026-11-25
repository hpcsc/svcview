## Contents
1. Task 1: Read each service of the domain, also each service with no plist
2. Task 2: Show each service of the domain on the screen with svcview --all
3. Task 3: Describe --all in docs/cli.md and the README

## Story Reference

Story: `user-stories/launchd-tui.md`, US-013 "Show all services of the domain" (`launchd-tui/US-013`).
Proposal: `docs/proposal-launchd-tui.md`, the section "Data From launchd", the paragraph that starts "By default,
svcview shows only the services that have a plist".

The user opens the screen with `svcview --all` to see each service that launchd has in the domain `gui/<uid>`, also
the services of macOS, so that they can check a service that has no plist in `~/Library/LaunchAgents`. A service
with no plist shows its state, its PID, its uptime and its last exit, and goes into the group that its name matches,
or into `Other`, as each other service does. With no `--all`, the list shows only the services that have a plist in
`~/Library/LaunchAgents`, as it does today. `svcview --help` lists `--all`.

A service with no plist has no environment and no log paths. A status command or an action that needs one of these
values shows the reason of a value that the plist does not have, and `o` says that the service has no log path.

The story criteria that this change satisfies:

- With `--all`, the list shows each name that `launchctl list` prints, also the services of macOS.
- A service with no plist in `~/Library/LaunchAgents` shows its state, PID, uptime, and last exit. It goes into a
  group by its name, as the other services do.
- Without `--all`, the list shows only the services that have a plist in `~/Library/LaunchAgents`.
- `svcview --help` lists `--all`.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root. `--all` shows the
  services of the domain `gui/<uid>` of the user, which `launchctl list` prints without root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No work in the worker repo.
- svcview reads no plist outside `~/Library/LaunchAgents`. A service with no plist there has no environment, no log
  paths and no plist path, also when a plist in `/Library/LaunchAgents`, `/System/Library/LaunchAgents` or another
  folder defines it. svcview does not run `launchctl print` to find one.
- No setting of a config file turns `--all` on, and `--all` has no short form.
- The screen shows no mark that `--all` is on. No glyph, color or text marks a service with no plist, and the header
  counts each service of the read, as with no flag.
- The keys do not change. `s`, `x`, `r`, `o`, the keys of the actions and the confirmations act on a service with no
  plist as on any other service in the same state. No key refuses a service of macOS, and the confirmation of `x`
  reads `Stop <name>? (y/N)` for each service.
- The texts do not change. A template value that a service with no plist does not have gives the same reason as a
  value that a plist does not have, and `o` gives `<name> has no log path in its plist`.
- `svcview status`, `svcview version` and `svcview update` do not take `--all`. `svcview status` finds only a service
  that has a plist in `~/Library/LaunchAgents`, and its error for a name that no service has keeps its text.
- No limit on the status runs. A group whose match takes services with no plist runs its status command for each of
  them, at each `every`.
- No change to the order of the rows, the groups, the filter or the refresh.
- No layout for a narrow terminal, and no change for `NO_COLOR`.

**Deferred:** None.

## Codebase Context

- `internal/launchd/domain.go:21-48`: `Domain.Services` reads the plists of `~/Library/LaunchAgents` through
  `plistFolder.read` (`internal/launchd/plist_folder.go:15-45`), runs `launchctl list` one time through
  `readLaunchctlList` (`internal/launchd/launchctl_list.go:24-41`), and builds one service for each plist with
  `launchctlList.service` (`launchctl_list.go:74-79`), which gives a `not loaded` service for a name that the list
  does not print. It copies the environment, the log paths and the plist path of the plist, and runs `ps` one time
  with the PIDs of the running services (`readUptimes`, `internal/launchd/uptimes.go:15-32`). The comment at
  `domain.go:21` says that callers must not call `Services` from two goroutines at the same time: `plistFolder` keeps
  the plists that it read (`plist_folder.go:10-13`), and a new read of the same `Domain` shares it.
- `internal/launchd/launchctl_list.go:20-22`: `launchctlList` keeps the services in a map by name, so it gives no
  order of the names. `parseLaunchctlList` (`launchctl_list.go:43-72`) already parses each row of the output, also
  the rows of names that no plist has, so `--all` adds no new parse of the output of launchctl.
- `internal/service/service.go:35-45`: `Service` has `Env`, `StdoutPath`, `StderrPath` and `PlistPath`, which stay
  empty for a service with no plist. `Uptime` is a pointer: nil is no uptime (tasks/learnings.md, "A value that can
  be 0 and can be missing is a pointer").
- `internal/launchd/domain_test.go:175-662`: the group `list` of `TestDomain` tests `Services` with one fake
  launchctl and one fake ps that `TestMain` writes (`domain_test.go:64-90`); a test gives them its output through
  `fakeLaunchctlList` and `fakePs` (`domain_test.go:109-116`, `140-150`) and compares with `require.ElementsMatch`,
  because the order of the services is not part of the read. `domain_test.go:353-362` checks that `Services` gives no
  service for a name that no plist has. The fixtures `launchctl-list-each-state.txt` (a loaded
  `com.example.no-plist`) and `launchctl-list-two-running.txt` with `ps-two-pids.txt` (PID 4123 with Status -9) in
  `internal/launchd/testdata` already hold the rows that a test of a service with no plist needs.
- A probe of the real launchd on this Mac: `launchctl list` printed 554 rows, 302 with a PID; 523 names start with
  `com.apple.` and 14 with `application.com.`, while `~/Library/LaunchAgents` holds 3 plists. `ps -o pid=,etime= -p`
  with the 302 PIDs took 73 ms, and `launchctl list` 24 ms. macOS `ps` exits with 0 when at least one PID of the list
  exists and with 1 only when none does, so a process that ends between `launchctl list` and `ps` costs only its own
  uptime (`readUptimes` gives no uptimes on an exit code other than 0, `uptimes.go:28-31`).
- The screen needs no change. `Screen.readServices` (`internal/tui/screen.go:325-333`) builds one row for each
  service that the read gives, `Screen.rowOf` (`screen.go:395-398`) puts it in the group that
  `config.Settings.GroupOf` gives for its name, or in `Other`, and `Screen.detailText` (`screen.go:736-747`) shows the
  state, the PID, the uptime and the last exit from the service alone. Each name that the screen draws goes through
  `terminal.ReplaceControlCharacters` (`screen.go:63`, `559`, `661`), so a name from `launchctl list` with a control
  character shows as `�`.
- The template values already give the reasons of the story Context for a service with no plist:
  `internal/status/values.go:26-44` gives `the plist has no StandardOutPath`, `the plist has no StandardErrorPath` and
  `the service has no plist`, and `values.go:56-59` gives `the plist has no X` for a key that `Env` does not have. A
  probe of `text/template` with `missingkey=error` on a nil map gave `map has no entry for key`, so a service with no
  plist (nil `Env`) gives `the plist has no X`, as a plist with no `EnvironmentVariables` does
  (`internal/launchd/plist.go:36-43` leaves the environment nil). Tests: `internal/status/values_test.go:63-75`,
  `internal/status/command_test.go:224-240`. `o` on a service with no log path shows
  `<name> has no log path in its plist` (`internal/tui/screen.go:243-244`).
- `internal/launchd/domain.go:54-78`: `Start` of a `loaded` service runs `launchctl kickstart gui/<uid>/<name>` with
  no plist path; `Stop` and `Restart` use only the name. A read gives a service with no plist only when
  `launchctl list` prints it, so the service is `loaded` or `running`, and the error of `Start` for a `not loaded`
  service with no plist path (`domain.go:60-61`) stays for a service that launchd dropped after the last read.
- `internal/cli/root.go:39-51`: the root command has no flag. `showScreenOrHelp` (`root.go:53-61`) prints the help
  without a terminal, and `showScreen` (`root.go:63-80`) gives `domain.Services` to `tui.NewScreen` as the read
  (`root.go:72`). `readSettingsAndDomain` (`root.go:94-105`) also serves `svcview status`, whose `findStatusCommand`
  (`internal/cli/status.go:66-91`) calls `domain.Services` and says that svcview knows only the services that have a
  plist.
- `internal/cli/update.go:24-28` declares the bool flags of `svcview update` with a usage each, and `update.go:52`
  reads one with `cmd.Bool`.
- urfave/cli v3.14.0 (`~/go/pkg/mod/github.com/urfave/cli/v3@v3.14.0`): a flag is persistent by default, so each
  subcommand takes a flag of the root unless the flag sets `Local` (`flag_impl.go:67`, `command_parse.go:44`).
  `HelpFlag` and `VersionFlag` set `Local` and `HideDefault` (`flag.go:27-44`). The help of the root lists the
  visible flags of the root under `GLOBAL OPTIONS` (`help_notemplate.go:90-119`, `254-276`), with or without `Local`.
  Today `svcview --help` shows `--help, -h  show help` and `--version, -v  print the version` there.
- `internal/cli/root_test.go:23-56`: `TestRootCommand` runs `newCommand` with a buffer as output, which is not a
  terminal, and an input that fails the test when svcview reads it. The fake launchctl of the package
  (`internal/cli/status_test.go:20-38`) prints `$HOME/launchctl-list.txt`.
- e2e: `openCli` puts its arguments in the command line of `sh -c` (`e2e/testUtils.ts:161-181`), `homeWithPlists`
  (`testUtils.ts:105-111`) writes plists, `setLaunchctlList` (`testUtils.ts:67-70`) writes rows of names with or
  without a plist, and the fake ps prints `03:12:07` for each PID, which the detail pane shows as `up 3h12m`.
  `e2e/tests/screen.test.ts:119-139` checks the detail pane, `screen.test.ts:221-240` the header counts,
  `e2e/tests/config.test.ts:140-150` a group with its short names through `listLines`, and
  `e2e/tests/status.test.ts:78-86`, `e2e/tests/actions.test.ts:103-118` and `e2e/tests/log.test.ts:66-80` the reasons
  of a value that the plist does not have.
- tasks/learnings.md entries that apply: "Only e2e tests reach the terminal branch of the root command" (Task 2),
  "Run clerk mutants and clerk audit run under mise exec", "Fake a program that svcview runs with a script first on
  PATH" (Task 1 uses the shared fakes and needs no new script), "A word of the subtest name is in every t.TempDir
  path" (Task 1, the error that names the LaunchAgents folder), "An e2e test finds the help line by a short part of
  it" (Task 2, the message line of `o` and of an action), "svcview status reads the whole config file" (Task 3, say
  what `svcview status` does not do), and "Check the name of a new type against the words of design.yaml before clerk
  design note" (the planned design adds no type; a helper type needs a note).

## Tasks

### Task 1: Read each service of the domain, also each service with no plist

**Behavior:** `launchd.Domain.AllServices` gives one service for each name that `launchctl list` prints, also for a
name that no plist in `~/Library/LaunchAgents` has, and one `not loaded` service for each plist whose name
`launchctl list` does not print. A service with no plist has the state, the PID, the last exit and the uptime that
`launchctl list` and `ps` give, and no environment, no log paths and no plist path. A service with a plist has the same
values that `Services` gives it. `Services` still gives only the services that have a plist.

**Acceptance Criteria:**
- [ ] A name with a PID that no plist has gives a `running` service with that PID, the last exit of its Status, and
      the uptime that `ps` reports for the PID; a name with `-` as the PID that no plist has gives a `loaded` service
      with no PID and no uptime.
- [ ] A service with no plist has no environment, no `StdoutPath`, no `StderrPath` and no `PlistPath`.
- [ ] A name that a plist has and that `launchctl list` prints gives one service, with the environment, the log paths
      and the plist path of the plist, and the state, the PID and the last exit of `launchctl list`.
- [ ] A plist whose name `launchctl list` does not print gives a `not loaded` service.
- [ ] `ps` runs one time, with the PIDs of all running services, also of the services with no plist; with no running
      service, `ps` does not run.
- [ ] An output of `launchctl list` with an unknown header line, a `launchctl` that exits with a code other than 0,
      and a LaunchAgents folder that cannot be read each give the error and no services, as `Services` does. A home
      with no `Library/LaunchAgents` folder gives the services of `launchctl list`, with no error.
- [ ] `Services` still gives no service for a name that no plist has: the test at
      `internal/launchd/domain_test.go:353-362` passes unchanged.

**Affected Files/Modules:**
- `internal/launchd/domain.go` — `AllServices` beside `Services`, and the comment on calls from two goroutines
  covers both
- `internal/launchd/domain_test.go` — a group `list all` in `TestDomain`
- `internal/launchd/launchctl_list.go` — only when the read needs each name of the list from `launchctlList`
- `internal/launchd/testdata/` — a new fixture only when no fixture has the rows that a test needs

**Patterns to Follow:**
- `internal/launchd/domain.go:21-48`
- `internal/launchd/launchctl_list.go:20-22`
- `internal/launchd/launchctl_list.go:74-79`
- `internal/launchd/domain_test.go:317-336`
- `internal/launchd/domain_test.go:338-362`
- `internal/launchd/domain_test.go:446-465`
- `internal/launchd/domain_test.go:489-533`

**Testable:** Yes

**Certainty:** high — `Domain.Services` (`internal/launchd/domain.go:21-48`) is the same read with the same join, and
the group `list` of `TestDomain` tests it with the same fakes and fixtures.

**Blast radius:** low

**Verification:** `mise exec -- go test -tags=unit ./internal/launchd/...` passes.

**Depends on:** None

### Task 2: Show each service of the domain on the screen with svcview --all

**Behavior:** The root command takes the flag `--all`. `svcview --all` in a terminal opens the screen with
`Domain.AllServices` as its read, so the list shows each name that `launchctl list` prints, also the services of
macOS, each in the group that its name matches or in `Other`, with its state, PID, uptime and last exit. `svcview`
with no flag reads with `Domain.Services`, as today. `svcview --help` lists `--all` with its usage. `--all` is a flag
of the root command only: `svcview status`, `svcview version` and `svcview update` do not take it.

**Acceptance Criteria:**
- [ ] `svcview --help` lists `--all` under `GLOBAL OPTIONS`, with the usage
      `show each service that launchctl list prints, also the services of macOS` and no default value, as `--help`
      and `--version` show none.
- [ ] `svcview --all` without a terminal prints the help and reads no input, as `svcview` does.
- [ ] `svcview status --all <name>` fails with the error of a flag that the command does not define, and runs no
      status command.
- [ ] In a terminal, `svcview --all` with plists for some names and a `launchctl list` that also prints names that no
      plist has shows a row for each name, and the header counts each of these services.
- [ ] With `--all`, the detail pane of a `running` service with no plist shows its state, its PID, `up 3h12m` and its
      last exit, for example `last exit SIGKILL` for a Status of -9.
- [ ] With `--all` and a group of `config.yaml` whose match takes the name of a service with no plist, the service
      shows under the line of the group with its short name; a service with no plist that no group matches shows
      under `Other`.
- [ ] `svcview` with no flag, on the same plists and the same `launchctl list`, shows only the services that have a
      plist, and the header counts only them.
- [ ] With `--all`, a group whose status command uses `{{.Env.WORKER_STATUS_FILE}}` shows
      `the plist has no WORKER_STATUS_FILE` in the row of a service with no plist; the key of an action whose command
      uses `{{.Env.WORKER_LOG}}` shows `the plist has no WORKER_LOG` on the message line and runs no `less`; and `o`
      shows `<name> has no log path in its plist` and runs no `less`.

**Affected Files/Modules:**
- `internal/cli/root.go` — the flag `--all` of the root command, and the read that `showScreen` gives the screen
- `internal/cli/root_test.go` — the help with `--all`, `--all` without a terminal, and `svcview status --all`
- `e2e/tests/all-services.test.ts` — new: the screen with and without `--all`

**Patterns to Follow:**
- `internal/cli/update.go:24-28`
- `internal/cli/update.go:52`
- `internal/cli/root.go:39-80`
- `internal/cli/root_test.go:23-56`
- `e2e/tests/screen.test.ts:119-139`
- `e2e/tests/screen.test.ts:221-240`
- `e2e/tests/config.test.ts:140-150`
- `e2e/tests/status.test.ts:78-86`
- `e2e/tests/actions.test.ts:103-118`
- `e2e/tests/log.test.ts:66-80`

**Testable:** Yes

**Certainty:** high — the bool flags of `svcview update` (`internal/cli/update.go:24-28`, `52`) and the read that
`showScreen` gives the screen (`internal/cli/root.go:72`) are the precedent; the e2e files above check each part of
the screen that the criteria name.

**Blast radius:** high — the flag decides which read the screen uses, so a mistake puts each service of macOS under
`s`, `x` and `r` without `--all`; `x` on a service with no plist runs `launchctl bootout`, after which
`launchctl list` does not print it and svcview cannot start it until the next login.

**Verification:** `mise exec -- go test -tags=unit ./internal/cli/...` and `task test:e2e` pass. The branch of
`showScreen` that picks the read runs only in a terminal, so `clerk mutants` shows its mutants as LIVED or CALLER
GAPS, also the row of `internal/launchd/domain.go:(*Domain).AllServices`; accept each with the e2e test that fails on
it, checked by hand in Docker (tasks/learnings.md, "Only e2e tests reach the terminal branch of the root command").

**Depends on:** Task 1

### Task 3: Describe --all in docs/cli.md and the README

**Behavior:** `docs/cli.md` describes `svcview --all`: which services the list shows with and without it, the name of
a service with no plist, what such a service shows and which values it does not have, what the keys do to it, and
that `svcview status` does not take `--all`. The README says that the screen lists the services that have a plist,
or with `--all` each service of the domain.

**Acceptance Criteria:**
- [ ] The section "The screen" shows `svcview --all` in its shell block, and says that it opens the screen with each
      name that `launchctl list` prints, also the services of macOS, and that with no flag the list shows only the
      services that have a plist in `~/Library/LaunchAgents`.
- [ ] The row "The list" of the table of the screen, and the paragraph on the name of a service, say that with
      `--all` the list also shows each service with no plist, whose name is the name that `launchctl list` prints, and
      which has no environment and no log paths.
- [ ] "Template values" says that a service with no plist gives the reasons of its table for `{{.Env.X}}`,
      `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}`, and that `{{.Name}}` is the name that `launchctl list`
      prints.
- [ ] "The log of a service" says that `o` on a service with no plist shows `<name> has no log path in its plist`.
- [ ] "Start, stop and restart a service" says that `<name>` is also the name that `launchctl list` prints, that `s`
      on a service with no plist runs `kickstart` because the service is `loaded` or does nothing because it is
      `running`, and that after `x` on a service with no plist `launchctl list` does not print it, so the list does
      not show it and svcview cannot start it until the next login.
- [ ] "Test a status command with `svcview status`" says that `svcview status` does not take `--all`, and finds only a
      service that has a plist in `~/Library/LaunchAgents`.
- [ ] `README.md` says in its first paragraph that the screen lists each service that has a plist in
      `~/Library/LaunchAgents`, or with `--all` each service that `launchctl list` prints, and the row
      `internal/launchd` of its Layout says that the package also runs `launchctl list`.

**Affected Files/Modules:**
- `docs/cli.md` — "The screen" (`docs/cli.md:35-80`), "Start, stop and restart a service" (`docs/cli.md:243-300`),
  "The log of a service" (`docs/cli.md:301-336`), "Template values" (`docs/cli.md:533-565`), and "Test a status
  command with `svcview status`" (`docs/cli.md:766-842`)
- `README.md` — the first paragraph (`README.md:3-5`) and the row `internal/launchd` of the Layout (`README.md:42`)

**Patterns to Follow:**
- `docs/cli.md:35-80`
- `docs/cli.md:243-300`
- `docs/cli.md:533-565`

**Testable:** No

**Certainty:** high — the sections of `docs/cli.md` named above describe the list, the keys, the template values and
`svcview status` in the same form, and each new sentence states what the tests of Tasks 1 and 2 check.

**Blast radius:** low

**Verification:** Manual check: each sentence matches the output of `svcview --help` and the tests of Tasks 1 and 2;
`mise exec -- bash -c 'task test:unit && task test:e2e'` still passes.

**Depends on:** Task 2

## Summary

- 3 tasks. Task 1 adds the read of each service of the domain to `launchd.Domain`, with no caller outside its tests.
  Task 2 gives the screen that read with `--all`, so it carries each criterion of the story, and its e2e tests show a
  service with no plist in a group, in the detail pane, and with a status command, an action and `o` that need a
  value that it does not have. Task 3 describes `--all` once its behaviour is in place.
- The order follows the dependency: the flag has nothing to read before Task 1, and the docs describe the behaviour of
  Task 2.
- Criteria covered: "With `--all`, the list shows each name that `launchctl list` prints" (Tasks 1 and 2); "A service
  with no plist shows its state, PID, uptime, and last exit, and goes into a group by its name" (Tasks 1 and 2);
  "Without `--all`, the list shows only the services that have a plist" (Tasks 1 and 2); "`svcview --help` lists
  `--all`" (Task 2). The story Context (the reasons of a value that a service with no plist does not have) is checked
  in Task 2 and described in Task 3.
