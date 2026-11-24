## Contents
1. Task 1: Give the template values of a service and the command line that they make
2. Task 2: Print the template values and the command line of a service with svcview status
3. Task 3: Keep the stdout of each run in its result
4. Task 4: Run the status command one time and print its output and its result
5. Task 5: Describe svcview status in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-006 "Test a status command with `svcview status`" (`launchd-tui/US-006`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Status Command" (with "Template Values", "Output",
"Failures" and "`svcview status <name>`"), "Config Files" and "Code Layout".

The author of a program that adds its services to svcview runs `svcview status <name>` to test the status command of
one service before the installer of the program ships it. svcview reads the config files and the services as the
screen does, finds the service with that name and the status command of its group, and runs that command one time.
It prints each step: the template values of the service, the command line after svcview applies the template values,
the output of the command, and the result, which is the parsed output or the failure. It exits with 0 when the status
is valid, and with 1 in each other case. It reads no input and needs no terminal, so a script can run it.
`svcview --help` lists `status`, and `docs/cli.md` describes it.

The story criteria that this change satisfies:

- `svcview status <name>` prints the template values of the service, the command line after svcview applies the
  templates, the output of the command, and the parse result.
- It exits with 0 when the status is valid. It exits with 1 when the command fails, runs longer than
  `status.timeout`, or gives an output that is not valid, and when a template value is not in the plist.
- For a name that no service has, or for a service whose group has no status command, it prints the cause and exits
  with 1.
- It works when stdin and stdout are not terminals, so a script can run it.
- `svcview --help` lists `status`, and `docs/cli.md` describes it.

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
- `svcview status` runs the status command of one service one time. It does not run the command again, watch it,
  or run the commands of a group or of all services.
- It says nothing about a stale status, and prints no age of the status.
- It does not start, stop, or change a service. It does not change the screen: the rows and the detail pane show
  what they show now.
- It prints no state, PID, uptime or last exit of the service.
- It prints text for a person, and no JSON. A script reads its exit code, which is 0 or 1.
- It reads the services as the screen does: only the services that have a plist in `~/Library/LaunchAgents`, with no
  `--all`. An error of `launchctl list` stops it.
- It reads the config files as the screen does. A config error stops it, also an error in a group of another service.
- It prints each variable of `EnvironmentVariables` of the plist, and hides no value, also a value that holds a token.
  It lists each variable as `{{.Env.X}}`, also a variable whose name a Go template cannot write after `.Env`.
- No change to the rules of the status command: the template values, the check of the texts at start, the parse, the
  failures and their texts stay as they are.
- The steps that it prints have no color.

**Deferred:** None.

## Codebase Context

- `internal/cli/root.go:22-29`: `Run` runs the command tree of `newCommand`. When an action gives an error, `Run`
  prints its text in red on stdout through `terminal.ReplaceControlCharacters`, and returns 1, which
  `cmd/svcview/main.go` gives to `os.Exit`. So an action that gives an error exits with 1, and the error prints once.
- `internal/cli/root.go:31-43`: `newCommand` has the subcommands `version` and `update`. The root action
  `showScreenOrHelp` (`root.go:44-52`) opens the screen only when stdin and stdout are terminals, and else prints the
  help. A subcommand in `Commands` runs before the root action, so `svcview status <name>` never reaches the terminal
  check.
- `internal/cli/root.go:54-72`: `showScreen` gets the home folder and the uid, reads the config with
  `config.Read(config.Folder(home, XDG_CONFIG_HOME), uid, config.ACLEntries)`, and makes `launchd.NewDomain(home,
  uid)`. The subcommand needs the same settings and the same domain; one function of `internal/cli` gives both, so the
  two paths cannot read the config in two ways.
- `internal/cli/root.go:74-83` and `internal/cli/update.go:20-31`: a subcommand with a `Usage` and an action that
  prints to `cmd.Root().Writer`. `internal/cli/adapter.go:17-26`: `asOneArgumentAction` gives an error when the
  first argument is empty; nothing calls it yet.
- `internal/cli/status_runs.go:24-48`: the screen runs `status.Command.Run` through `statusRuns.run`, which only adds
  the stop of each run when the screen quits. `Command.Run` returns only after it stopped the process group of the
  program, also when its context ends, so the subcommand calls `Command.Run` with the context of svcview and needs no
  `statusRuns`. `cmd/svcview/main.go` ends that context on SIGINT, SIGHUP and SIGTERM.
- `internal/config/config.go:108-115`: `Settings.GroupOf(name)` gives the first group whose `match` matches, or
  `config.Other`, which has no `Status`. `config.Group.Status` (`internal/config/group.go:14-19`) is nil for a group
  with no status command.
- `internal/launchd/domain.go:20-48`: `Domain.Services` reads the plists of `~/Library/LaunchAgents` and runs
  `launchctl list`, and runs `ps` only when a service is `running`. Each service has `Name`, `Env`, `StdoutPath`,
  `StderrPath` and `PlistPath` (`internal/service/service.go:33-43`).
- `internal/status/command.go:63-98`: `Command.Run(ctx, service) Result` applies the template values, runs the
  program with no shell in a process group of its own, stops the group at `Timeout`, reads at most 1 MiB of stdout
  into a `stdoutBuffer` (`internal/status/stdout_buffer.go:8-34`), and parses it. The `Result`
  (`internal/status/result.go:5-35`) has `Ended`, `Output` and `Failure`, and not the stdout. A context that ends
  gives a result with no output and no failure.
- `internal/status/command.go:135-162`: `commandLine` applies the template values, and gives the `*Failure` of a
  value that the service does not have. `commandLineText` and `shellText` give the command line as the text of a
  status timeout, with each text that a shell reads in another way in single quotes. Both are private.
- `internal/status/values.go:12-60`: `values` gives `Name`, `Env`, and the methods `StdoutPath`, `StderrPath` and
  `PlistPath`, each with the failure `the plist has no StandardOutPath`, `the plist has no StandardErrorPath` or
  `the service has no plist`. `apply` turns a missing key of `Env` into `the plist has no X`. Nothing lists the
  values.
- `internal/tui/screen.go:490-499` and `internal/tui/screen.go:540-566`: the row shows `status error`,
  `status timeout` or `status invalid` for a failure, and the detail pane shows under `Status` the text of the failure
  and then its lines. The subcommand shows a failure with the same words.
- Tests: `internal/status/command_test.go:189-196` (each template value), `:223-241` (a missing value),
  `:568-582` (the quotes of the command line), `:674-687` (the context ends), `:689-720` (1 MiB); the tests carry the
  build tag `unit`. `internal/status/result_test.go:13-35` is the umbrella of a type of `internal/status`.
  `internal/cli/root_test.go:24-37` runs `newCommand` with an input that fails the test when svcview reads it.
  `internal/cli` has no fake `launchctl`; `internal/launchd/domain_test.go:59-90` writes one fake `launchctl` and one
  fake `ps` in `TestMain` for all tests.
- e2e: `runCli` (`e2e/testUtils.ts:116-134`) runs svcview with pipes for stdin and stdout, and leaves stdin open, so a
  command that reads stdin does not end. It does not put `e2e/fakes` on `PATH`, as `openCli`
  (`e2e/testUtils.ts:138-158`) does; without the fake, `svcview status` runs no `launchctl` in Docker and the real one
  on a Mac. `homeWithPlists` and `addPlist` make a home folder with plists, and `writeConfig` writes `config.yaml`
  with a mode. `e2e/tests/status.test.ts:103-138` makes each failure of a status command; `e2e/tests/config.test.ts:49-60`
  checks a `config.yaml` that all users can write. `docs/e2e-tests.md:31-40` describes `runCli` and `openCli`.
- `docs/cli.md:328-546` describes the status command, its template values, its output and its failures.
  `docs/cli.md:625-646` describes the subcommands `version` and `update`.
- `tasks/learnings.md` entries that apply: "Text from outside goes to the terminal through
  terminal.ReplaceControlCharacters" (it names `svcview status`: Tasks 2 and 4); "Fake a program that svcview runs
  with a script first on PATH" (the fake `launchctl` of the `internal/cli` tests, Task 2); "e2e tests redirect stdin
  or stdout through openCli" and "Join the lines of the screen before an e2e test checks a long error" (Tasks 2 and
  4); "A word of the subtest name is in every t.TempDir path" (an error that names a config file); "A change to the
  template values changes the check of status.NewCommand" (a later template value must also go into the list of
  Task 1); "Check by hand a mutant that only the tests of another package kill" (the printed steps of Tasks 2 and 4
  show what `internal/status` gives); "Prefix the constants of an enum in internal/status with its type"; "Run
  clerk mutants and clerk audit run under mise exec"; "task test:e2e:local leaves e2e/package-lock.json in the
  tree"; "Check the name of a new type against the words of design.yaml before clerk design note".
- Words: "status command" is the program of a group, so the subcommand `svcview status` is the status subcommand,
  never a status command. The function that makes it must not be `newStatusCommand`, which reads as a status command
  and is near `status.NewCommand`. "Output" is the word of the docs for what the program prints; in the code,
  `status.Output` is the parsed JSON object, so the bytes that the program printed are `Stdout`. "Result" is what one
  run gives. "Command line" is the program and its arguments after the template values.

## Tasks

### Task 1: Give the template values of a service and the command line that they make

**Behavior:** `internal/status` gives the template values of one service: each value as a text of a status command
writes it, and what it gives for that service, or the failure for a value that the service does not have. A status
command gives its command line for one service: the program and its arguments after the template values, as the
text that a status timeout shows. A value that the service does not have gives the failure that a run gives.

**Acceptance Criteria:**
- [x] `status.TemplateValues` of a service lists `{{.Name}}`, then one `{{.Env.X}}` for each variable of its
      environment in the order of the names, then `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}`, each with
      the value of the service.
- [x] A variable with an empty value gives an empty value, not a failure. A service with no environment lists no
      `{{.Env.X}}`.
- [x] A service with no `StdoutPath`, no `StderrPath` or no `PlistPath` gives for that value a failure of the kind
      template value, with the text `the plist has no StandardOutPath`, `the plist has no StandardErrorPath` or
      `the service has no plist`: the failure that a run of a text with that value gives.
- [x] `Command.CommandLine` of a service gives the command line with each text that a shell reads in another way in
      single quotes. For a command and a service whose run lasts longer than the timeout, it is the text of the status
      timeout.
- [x] `Command.CommandLine` with a template value that the service does not have gives an error that is the failure
      that `Command.Run` gives for it, of the kind template value, also for a missing variable (`the plist has no
      WORKER_STATUS_FILE`). It runs no program.
- [x] The tests of the list are in `TestTemplateValue`, and the tests of the command line are in `TestCommand`, in a
      group of their own.

**Affected Files/Modules:**
- `internal/status/values.go` — the list of the template values of a service, and the type of one value
- `internal/status/command.go` — the exported command line of a service
- `internal/status/values_test.go` — new: the tests of the list
- `internal/status/command_test.go` — the tests of the command line

**Patterns to Follow:**
- `internal/status/values.go:12-60`
- `internal/status/command.go:135-162`
- `internal/status/command_test.go:189-196`
- `internal/status/command_test.go:223-241`
- `internal/status/command_test.go:568-582`
- `internal/status/result_test.go:13-35`

**Testable:** Yes — through `status.TemplateValues` and `Command.CommandLine`.

**Certainty:** high — `values` gives each value or its failure (`values.go:12-60`), and `commandLine` with
`commandLineText` gives the command line of a status timeout (`command.go:135-162`); this task lists the first and
exports the second.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Print the template values and the command line of a service with svcview status

**Behavior:** `svcview status <name>` reads the config files and the services as the screen does, finds the service
with that name and the status command of its group, and prints two parts on stdout: the template values of the
service under the heading `Template values`, and the command line under the heading `Command line`. When the command
uses a template value that the plist does not have, the failure shows in place of the command line, no program runs,
and svcview exits with 1. With no name, a name that no service has, or a service whose group has no status command,
svcview prints the cause and exits with 1. The subcommand reads no input and does not check for a terminal.
`svcview --help` lists `status`.

**Acceptance Criteria:**
- [x] `svcview status <name>` prints the heading `Template values` and one line for each value of
      `status.TemplateValues`: the value as a text writes it, and what it gives, or the text of its failure. Then it
      prints the heading `Command line` and the command line.
- [x] For a command that uses a template value that the plist does not have, it prints the template values, then
      the failure under `Command line`, for example `the plist has no WORKER_STATUS_FILE`, and exits with 1. No program
      runs, and the failure shows one time.
- [x] With no name, it says that it needs the name of a service and exits with 1. With more than one name, it says
      that it takes one name and exits with 1.
- [x] For a name that no service has, it prints a cause that names the name, and says that svcview knows only the
      services that have a plist in `~/Library/LaunchAgents`. It exits with 1.
- [x] For a service whose group has no status command, also the group `Other`, it prints a cause that names the
      service and the group, and exits with 1.
- [x] It reads the config through the same function as the screen. A `config.yaml` or a file of `conf.d` that
      another user can write, and each other config error, stops it with the error that stops the screen, with exit
      code 1, before it prints a step. `showScreen` and the subcommand get the settings and the domain from one function
      of `internal/cli`.
- [x] It reads the services through `Domain.Services`, as the screen does. An error of `launchctl list` stops it with
      that error and exit code 1.
- [x] It reads no input. With stdin and stdout that are not terminals, it prints the same steps as in a terminal. In
      a terminal, it prints the steps and ends, and opens no screen.
- [x] Each control character in a value, a name, a group, the command line or a failure prints as `�`, so a plist,
      a config file or a name cannot send an escape sequence to the terminal.
- [x] `svcview --help`, and the help that svcview prints with no command and no terminal, list `status` with its
      usage. `svcview status --help` shows that it takes a `<name>`, and runs nothing.
- [x] The tests are in `TestStatusSubcommand` in `internal/cli/status_test.go`. They run the command tree of
      `newCommand` with an input that fails the test when svcview reads it, a home folder and a config folder in
      `t.TempDir`, and a fake `launchctl` first on `PATH` that all tests of the package share.
- [x] `runCli` puts `e2e/fakes` first on `PATH`, as `openCli` does, and `docs/e2e-tests.md` says so.
- [x] e2e, in `e2e/tests/status-subcommand.test.ts`: with `runCli`, `svcview status <name>` prints the template
      values with a variable of the plist, and the command line, and exits with 0; a missing variable prints `the plist
      has no` and the variable, writes no marker file of the command, and exits with 1; a name that no service has and a
      service in `Other` each exit with 1 with their cause; a `config.yaml` that all users can write exits with 1 with
      the error that names the file; `--help` lists `status`. With `openCli`, `svcview status <name>` prints the
      template values and `EXIT:0`.

**Affected Files/Modules:**
- `internal/cli/root.go` — the subcommand in the command tree, and one function that gives the settings and the
  domain to `showScreen` and to the subcommand
- `internal/cli/status.go` — new: the status subcommand
- `internal/cli/status_test.go` — new: the tests of the subcommand, and the fake `launchctl` of the package
- `e2e/testUtils.ts` — `runCli` puts `e2e/fakes` first on `PATH`
- `e2e/tests/status-subcommand.test.ts` — new: the e2e tests of the subcommand
- `docs/e2e-tests.md` — `runCli` runs the fakes too

**Patterns to Follow:**
- `internal/cli/root.go:31-43`
- `internal/cli/root.go:54-72`
- `internal/cli/root.go:74-83`
- `internal/cli/update.go:20-31`
- `internal/cli/adapter.go:17-26`
- `internal/cli/root_test.go:24-37`
- `internal/config/config.go:108-115`
- `internal/launchd/domain_test.go:59-90`
- `internal/terminal/control_characters.go`
- `e2e/testUtils.ts:116-158`
- `e2e/tests/config.test.ts:49-60`
- `task:1`

**Testable:** Yes — through the command tree of `newCommand` with a fake `launchctl`, and in e2e tests with
`runCli` and `openCli`.

**Certainty:** medium — `version` and `update` are subcommands that print to the writer of the root
(`root.go:74-83`, `update.go:20-31`), and `showScreen` reads the config and the domain (`root.go:54-72`), but no
subcommand yet takes an argument, reads the config or the services, or exits with 1 for a cause that is not an error
of a program, and the tests of `internal/cli` have no fake `launchctl`.

**Blast radius:** high — a second path that reads the config files, which hold the commands that svcview runs as the
user: it must refuse a config file that another user can write, as the screen does. It also prints each variable of
the environment of a plist, which can hold a token.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 1

### Task 3: Keep the stdout of each run in its result

**Behavior:** The result of a run of a status command keeps what the program printed on stdout, as svcview read it:
no more than 1 MiB. The screen shows nothing new.

**Acceptance Criteria:**
- [ ] A run that gives a valid output has the stdout byte for byte, with its new lines.
- [ ] A status error has what the program printed on stdout before it ended. A status timeout has what the program
      printed before the stop. A status invalid has the stdout; for a stdout larger than 1 MiB, it has the start of
      the stdout, no more than 1 MiB.
- [ ] A run that runs no program has no stdout: a template value that the service does not have, a timeout that ends
      before the start, and a context that ended before the start.
- [ ] The screen does not change: each golden file in `internal/tui/testdata` stays the same.

**Affected Files/Modules:**
- `internal/status/result.go` — the stdout of the result
- `internal/status/command.go` — `Run` keeps the stdout in the result
- `internal/status/command_test.go` — the tests of the stdout

**Patterns to Follow:**
- `internal/status/stdout_buffer.go:8-34`
- `internal/status/command.go:63-98`
- `internal/status/command_test.go:689-720`

**Testable:** Yes — through `Command.Run` and its `Result`.

**Certainty:** high — `stdoutBuffer` holds the bytes that `Run` reads (`stdout_buffer.go:8-34`), and `Run` builds
the result from it (`command.go:63-98`); this task keeps the bytes in the result.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `git status internal/tui/testdata` shows no change.

**Depends on:** None

### Task 4: Run the status command one time and print its output and its result

**Behavior:** After the command line, `svcview status <name>` runs the status command of the service one time with
`status.Command.Run`, the method that the screen runs, with the timeout of the group and the context of svcview. It
prints the stdout of the run under the heading `Output`, and the result under the heading `Result`: `valid` and what
svcview read from the output, or the failure with the word that the row shows and the lines that the detail pane
shows. It exits with 0 only when the status is valid.

**Acceptance Criteria:**
- [ ] For a valid output, it prints under `Output` the stdout as the program printed it, line by line, and under
      `Result` the word `valid`, then the `at` as an RFC 3339 time or that the output has no `at`, the `summary`, the
      `level` (`info` when the output has none), each field with its `name`, its `value` and its `progress` when it has
      one, and each event with its `at`, its `level` and its `text`, in the order of the output. It exits with 0.
- [ ] A command that exits with a code other than 0, that a signal ends, or that svcview cannot start prints under
      `Result` the words `status error`, then the exit code, the signal or the reason, and the last 20 lines of
      stderr. It exits with 1. `Output` shows what the program printed on stdout.
- [ ] A command that runs longer than `status.timeout` of its group (default `1s`) prints `status timeout` and the
      command line under `Result`, and exits with 1 no later than 1 s after the timeout. The program and each process
      that it started stop, as on the screen.
- [ ] An output that is not a JSON object of version 1, that is larger than 1 MiB, or that has a field with a
      `progress` outside 0 to 1, prints `status invalid`, the parse error and the lines of the failure under `Result`,
      and exits with 1.
- [ ] The result is the result that the screen gets for the same service and the same output: the subcommand gets it
      from `Command.Run`, and parses and classifies nothing itself.
- [ ] When svcview gets SIGINT, SIGTERM or SIGHUP while the command runs, the program and each process that it
      started stop, and svcview exits with 1 and does not print `valid`.
- [ ] A failure shows one time in what svcview prints.
- [ ] When the program prints nothing on stdout, `Output` says that it printed nothing.
- [ ] Each control character in the stdout other than the new line, and each control character in the result,
      prints as `�`, as on the screen.
- [ ] e2e, in `e2e/tests/status-subcommand.test.ts`: `cat` of a valid status file prints the output and `valid`
      with the summary, and exits with 0; `[sh, -c, 'echo broken >&2; exit 3']` prints `status error`, `exit code 3`
      and `broken`, and exits with 1; `[sleep, '30']` with `timeout: 500ms` prints `status timeout` and `sleep 30`, and
      exits with 1; `[printf, 'busy']` prints `status invalid` and `the output is not valid JSON`, and exits with 1.

**Affected Files/Modules:**
- `internal/cli/status.go` — the run, the output and the result, and the exit code
- `internal/cli/status_test.go` — the tests of the run and the exit code
- `e2e/tests/status-subcommand.test.ts` — the e2e tests of each result

**Patterns to Follow:**
- `internal/status/command.go:63-98`
- `internal/cli/status_runs.go:24-30`
- `internal/tui/screen.go:490-499`
- `internal/tui/screen.go:540-566`
- `internal/status/command_test.go:674-687`
- `e2e/tests/status.test.ts:103-138`
- `task:2`
- `task:3`

**Testable:** Yes — through the command tree of `newCommand` with status commands that are small programs, and in e2e
tests with `runCli`.

**Certainty:** medium — the screen runs `Command.Run` (`status_runs.go:24-30`) and shows each failure with its word
and its lines (`screen.go:490-499`, `screen.go:540-566`), but no code yet prints an output or a result as text
outside the screen, or turns a result into an exit code.

**Blast radius:** high — the exit code is the contract that the install script of another program reads to decide
that its status command works, and this is a second path that runs a program from the config as the user.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2, Task 3

### Task 5: Describe svcview status in docs/cli.md

**Behavior:** `docs/cli.md` describes `svcview status <name>`, so the author of a program can test a status command
with no read of the svcview code.

**Acceptance Criteria:**
- [ ] A section under "The status command" describes `svcview status <name>`: it reads the config files and the
      services as the screen does, runs the status command of the service one time with the timeout of its group, and
      prints the four parts in order, with an example of each part.
- [ ] It gives the exit codes: 0 for a valid status; 1 for a status error, a status timeout, a status invalid, a
      template value that the plist does not have, a name that no service has, a service whose group has no status
      command, no name, and a config error.
- [ ] It says that the subcommand reads no input and needs no terminal, so a script or an installer can run it.
- [ ] It says that the template values show each variable of `EnvironmentVariables`, also a value that holds a
      token, and that each control character shows as `�`.
- [ ] The section "Template values" and the section "Failures" link to it.

**Affected Files/Modules:**
- `docs/cli.md` — the section on `svcview status`

**Patterns to Follow:**
- `docs/cli.md:328-546`
- `docs/cli.md:625-646`

**Testable:** No — a document.

**Certainty:** high — the sections of the status command (`docs/cli.md:328-546`) and of the subcommands `version` and
`update` (`docs/cli.md:625-646`) are the precedent, and the proposal gives the subcommand ("`svcview status
<name>`").

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the tests of Tasks 2 and 4 and against `svcview status --help`.

**Depends on:** Task 4

## Summary

- 5 tasks. Task 1 gives what the subcommand prints before a run from `internal/status`, which owns the template
  values and the command line. Task 2 adds the subcommand with the read of the config and the services that the
  screen does, and the causes that stop it before a run. Task 3 keeps the stdout in the result of a run, which the
  screen and the subcommand both get from `Command.Run`. Task 4 runs the command and sets the exit code. The document
  comes last.
- Covers each criterion of the story: the four steps (Tasks 1, 2, 3 and 4), the exit codes (Task 2 for a missing
  template value, Task 4 for each failure of a run), the causes of a name and a group (Task 2), no terminal (Task 2,
  and the e2e tests of Tasks 2 and 4 run with pipes), and the help and the docs (Tasks 2 and 5). None is deferred.
- Certainty: Tasks 1, 3 and 5 are high; Tasks 2 and 4 are medium, each with its precedent and its variation.
- Blast radius: Tasks 2 and 4 are high. Task 2 is a second read of the config files that hold commands, and prints
  the environment of a plist. Task 4 runs a program from the config, and its exit code is what a script reads.
- The planner chose these details, which the story does not give: the headings `Template values`, `Command line`,
  `Output` and `Result`; the template values list each variable of the environment, not only the values that the
  command uses; the command line prints before the run; the subcommand calls `Command.Run` with the context of svcview
  and does not use `statusRuns`; the result of each run keeps its stdout, so the screen keeps no more than 1 MiB more
  for each service; an error of `launchctl list` stops the subcommand; each control character other than the new line
  prints as `�`, also a tab; no name and more than one name each exit with 1; the function that makes the subcommand
  is not `newStatusCommand`.
