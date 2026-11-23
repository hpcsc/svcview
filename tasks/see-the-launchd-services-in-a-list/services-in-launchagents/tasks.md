## Contents
1. Task 1: Read the services that have a plist in the LaunchAgents folder
2. Task 2: Show the services in a list on the screen
3. Task 3: Open the screen from the root command in a terminal
4. Task 4: Refresh the list every 2 seconds
5. Task 5: Replace the example package with the svcview packages in the layout

## Story Reference

Story: `user-stories/launchd-tui.md`, US-001 "See the launchd services in a list" (`launchd-tui/US-001`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Data From launchd", "Code Layout" and "Tests".

This change makes `svcview` with no command, in a terminal, open a screen. The header shows the domain of the
user, for example `gui/501`. The list shows one row for each service that has a plist in `~/Library/LaunchAgents`,
in the group `Other`, sorted by name. A name that is too long for the list ends with `…`. The screen reads the
plists again every 2 s, so a plist that the user adds, removes, or changes shows with no key press. `q` quits.
When stdin or stdout is not a terminal, `svcview` prints the help, as it does now.

The story criteria that this change satisfies:

- `svcview` with no command, in a terminal, opens the screen. When stdin or stdout is not a terminal, it prints the
  help and reads no input.
- From the criterion on the list: one row for each service that has a plist in `~/Library/LaunchAgents`, in the
  group `Other`, sorted by name. A name that is too long for the list ends with `…`.
- From the criterion on the header: the domain, for example `gui/501`.
- From the criterion on the refresh: every 2 s, the screen shows a plist that the user adds, removes, or changes,
  with no key press.
- From the criterion on the keys: `q` quits.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No config file, no groups other than `Other`, no short names, no status command, no actions on a service, no
  filter, no `--all`, no one-pane layout for a narrow terminal, and no `NO_COLOR` handling.
- No behaviour for a session over SSH with no console session.
- svcview reads no property of a plist other than `Label`.

**Deferred:**
- The screen does not run `launchctl`. A row has no state and no glyph, and the header has no count of the
  services in each state.
- No selection of a row, no detail pane, no uptime, and no list of the keys on `?`.
- The time between two refreshes is fixed at 2 s. No setting changes it.

## Codebase Context

- The module is a Go CLI scaffold. `cmd/svcview/main.go` calls `cli.Run` with a context that ends on an interrupt.
  `internal/cli/root.go:24-34` builds the root command with urfave/cli v3, with the commands `version` and
  `update`. The root command has no action, so with no command it prints the help. The e2e test
  `e2e/tests/cli.test.ts:4-12` checks this through pipes, which are not a terminal, and it must stay valid.
- `internal/cli/update.go:90-93` is the only terminal check in the module. It uses go-isatty, which is a direct
  dependency already.
- The module has no TUI library, no plist library, and no code that runs another program. The proposal names
  Bubble Tea, Lip Gloss and Bubbles for the screen, and `howett.net/plist` for XML and binary plists.
- The proposal fixes the packages: `internal/service` holds the types that the other packages use,
  `internal/launchd` is the only package that knows launchd, `internal/tui` holds the Bubble Tea model and the
  views, and `internal/cli` starts the screen. `internal/tui` does not import `internal/launchd`. The code has no
  `darwin` build tag: it builds, and its unit tests run, on Linux in CI.
- The unit tests have the build tag `unit`, use testify `require`, and put the scenarios of a type under one
  umbrella test (`internal/cli/version/version_test.go:1-24`). `task test:unit` runs them with `-tags=unit -race`.
- `internal/cli/release/updater.go:45-56` is a type whose constructor takes the source that it reads, and
  `internal/cli/release/updater_test.go:26-84` gives each test a fake of that source.
- `internal/cli/progress/line_test.go:13-60` checks terminal output as exact text with escape codes. It is the
  nearest precedent for golden files of the screen.
- e2e: `e2e/testUtils.ts:24-28` makes a scratch folder per test, and `e2e/testUtils.ts:58-77` starts the binary in
  a pseudo-terminal of 160 by 40 with environment values, and writes `EXIT:<code>` when it ends.
  `docs/e2e-tests.md` describes the harness. The Docker image of the e2e tests copies `e2e/`, and has no `ps` and
  no `launchctl`.
- `README.md:3-4` and `README.md:39` describe the example package `internal/greeting`.
- Words: use the words of the proposal. A service is a launchd job in the domain of the user, and its name is its
  launchd label. "status" is what a service reports about itself, through a command of a later feature, so no
  code here uses "status" for data from launchd.

## Tasks

### Task 1: Read the services that have a plist in the LaunchAgents folder

**Behavior:** `launchd.Domain` gives one `service.Service` for each plist in the `Library/LaunchAgents` folder of a
home folder, named by the `Label` property of the plist. The domain reads as `gui/<uid>` for the uid that it is
made with.

**Acceptance Criteria:**
- [x] Each file with the extension `.plist` in `Library/LaunchAgents` that has a `Label` property gives one service
      with that name. An XML plist and a binary plist both give their service.
- [x] A file that is not a plist, a plist with no `Label`, and a file without the extension `.plist` give no
      service, and do not stop the read of the other files.
- [x] Two plists with the same `Label` give one service.
- [x] A home folder with no `Library/LaunchAgents` folder gives no services and no error.
- [x] A `Library/LaunchAgents` folder that cannot be read gives an error that names the folder.
- [x] The domain reads as `gui/` and the uid, for example `gui/501`.

**Affected Files/Modules:**
- `internal/service/service.go` — new: the type `Service` with its name
- `internal/launchd/domain.go` — new: the type `Domain`, which lists the services and names the domain
- `internal/launchd/plist.go` — new: the read of the `Label` of each plist in the folder
- `internal/launchd/domain_test.go` — new: the tests of `Domain`
- `internal/launchd/testdata/` — new: fixture plists, one XML and one binary
- `go.mod`, `go.sum` — add `howett.net/plist`

**Patterns to Follow:**
- `internal/cli/release/updater.go:45-56`
- `internal/cli/release/updater_test.go:110-136`
- `internal/cli/version/version_test.go:1-24`

**Testable:** Yes — through `Domain` and its list of services, against fixture plists in a scratch home folder.

**Certainty:** low — the first use of `howett.net/plist`, and the first package of svcview's own data, so its shape is decided here.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `go build ./...` succeeds.

**Depends on:** None

### Task 2: Show the services in a list on the screen

**Behavior:** `tui.Screen` draws a header with `svcview` and the domain, and a list pane that shows the group
`Other` with one row for each service, sorted by name. A name that is too long for the list pane ends with `…`.
When the read of the services gives an error, the list shows the error and no rows. `q` and `ctrl+c` end the
screen.

**Acceptance Criteria:**
- [ ] At start, with no key press, the screen reads the services one time and shows them.
- [ ] The header shows `svcview` and the domain, for example `gui/501`.
- [ ] The list shows the group `Other`, and under it one row for each service with its full name, sorted by name.
- [ ] A name that is longer than the list pane is cut to fit and ends with `…`. A name that fits is not cut.
- [ ] With no services, the list shows no rows.
- [ ] When the read gives an error, the list shows the error and no rows.
- [ ] `q` and `ctrl+c` end the screen.
- [ ] Golden files hold the screen at a fixed width and height.

**Affected Files/Modules:**
- `internal/tui/screen.go` — new: the type `Screen`, the Bubble Tea model, with its keys and the read at start
- `internal/tui/view.go` — new: the header, the list pane, the cut of a long name
- `internal/tui/screen_test.go` — new: the tests of the screen with a fake list of services
- `internal/tui/testdata/` — new: the golden files
- `go.mod`, `go.sum` — add Bubble Tea and Lip Gloss

**Patterns to Follow:**
- `internal/cli/progress/line_test.go:13-60`
- `internal/cli/release/updater_test.go:26-84`
- `docs/proposal-launchd-tui.md:70-103`

**Testable:** Yes — through the `Screen` model, with a fake read of the services and key messages, and its view compared with golden files.

**Certainty:** low — Bubble Tea and Lip Gloss are new to this repository, and the shape of the screen model is decided here.

**Blast radius:** low

**Verification:** `task test:unit` passes.

**Depends on:** Task 1

### Task 3: Open the screen from the root command in a terminal

**Behavior:** `svcview` with no command opens the screen with the services of the home folder and the uid of the
user, when stdin and stdout are both terminals. In all other cases it prints the help, reads no input, and exits
with 0, as it does now. The e2e tests can start svcview with a scratch home folder that holds the plists that a
test names.

**Acceptance Criteria:**
- [ ] In a terminal, `svcview` with no command opens the screen, with one row for each plist in
      `$HOME/Library/LaunchAgents`, sorted by name.
- [ ] `q` ends svcview with the exit code 0.
- [ ] When stdin or stdout is not a terminal, `svcview` prints the help, reads no input, and exits with 0. The e2e
      test "with no command, prints its commands and reads no input" passes with no change.
- [ ] `svcview version`, `svcview update`, `--help` and `--version` work as before.
- [ ] An e2e helper makes a scratch home folder with the plists that a test names in `Library/LaunchAgents`, and
      the screen tests start svcview with `HOME` set to it.
- [ ] `docs/e2e-tests.md` describes the scratch home folder and the screen tests.

**Affected Files/Modules:**
- `internal/cli/root.go` — the root command opens the screen in a terminal, and prints the help in all other cases
- `e2e/testUtils.ts` — the helper for a scratch home folder with plists
- `e2e/tests/screen.test.ts` — new: the screen tests
- `docs/e2e-tests.md` — the scratch home folder and the screen tests

**Patterns to Follow:**
- `internal/cli/root.go:24-34`
- `internal/cli/update.go:90-93`
- `e2e/testUtils.ts:24-28`
- `e2e/testUtils.ts:58-77`
- `e2e/tests/cli.test.ts:4-12`
- `docs/e2e-tests.md:54-76`

**Testable:** Yes — the e2e tests start the binary in a pseudo-terminal for the screen, and with pipes for the help.

**Certainty:** medium — the terminal check of `internal/cli/update.go:90-93` and the pseudo-terminal of `e2e/testUtils.ts:58-77` exist, but no instance gives the root command an action of its own or reads rows from a full-screen program.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes, the existing tests included.

**Depends on:** Task 2

### Task 4: Refresh the list every 2 seconds

**Behavior:** The screen reads the services again every 2 s with no key press. A plist that the user adds, removes,
or changes shows at the next refresh. `launchd.Domain` reads a plist again only when the modification time of the
file changes.

**Acceptance Criteria:**
- [ ] A plist that is added to `Library/LaunchAgents` gives a row no later than 2 s after the change, with no key
      press. The row of a plist that is removed goes away in the same time.
- [ ] A plist whose `Label` changes shows the new name at the next refresh.
- [ ] A plist whose modification time did not change is not read again: a change to its content that keeps the
      modification time does not change its service.
- [ ] A refresh does not start a read while the previous read runs.
- [ ] After a read that gives an error, the next read that succeeds shows the rows again.
- [ ] An e2e test adds a plist to the scratch home folder while the screen is open, and sees its row with no key
      press.

**Affected Files/Modules:**
- `internal/launchd/plist.go` — keep each plist that was read, with its modification time
- `internal/launchd/domain_test.go` — the tests of the read again on a change
- `internal/tui/screen.go` — the refresh every 2 s
- `internal/tui/screen_test.go` — the tests of the refresh
- `e2e/tests/screen.test.ts` — a plist that is added while the screen is open

**Patterns to Follow:**
- `task:1`
- `task:2`
- `task:3`

**Testable:** Yes — through `Domain` for the read again on a change, through the `Screen` model for the refresh, and in an e2e test for a plist that is added.

**Certainty:** medium — the read and the screen come from Task 1 and Task 2, but no instance has a timer that reads again, or keeps a file until its modification time changes.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 3

### Task 5: Replace the example package with the svcview packages in the layout

**Behavior:** The example package `internal/greeting` is gone. The layout table of `README.md` names the packages of
svcview and the job of each, as in "Code Layout" of the proposal.

**Acceptance Criteria:**
- [ ] The folder `internal/greeting` does not exist, `go build ./...` succeeds, and `task test:unit` passes.
- [ ] `README.md` has no text about `internal/greeting`.
- [ ] The layout table of `README.md` names `internal/service`, `internal/launchd`, `internal/tui` and
      `internal/cli`, with the job of each.

**Affected Files/Modules:**
- `internal/greeting/` — delete
- `README.md` — the first lines and the layout table

**Patterns to Follow:**
- `README.md:35-42`
- `docs/proposal-launchd-tui.md:350-364`

**Testable:** No — a deletion and a document; the build and the unit tests show that nothing used the package.

**Certainty:** high — the layout table of `README.md:35-42` is the precedent, and the compiler shows any use of the deleted package.

**Blast radius:** low

**Verification:** `go build ./...` succeeds, and `task test:unit` passes.

**Depends on:** Task 3

## Summary

- 5 tasks. The read of the plists comes first, because the screen needs its type. The screen comes next, on a
  fake list of services. Task 3 makes the screen reachable and brings the e2e harness for it. The refresh comes
  after the screen opens, so that its e2e test has a harness. Task 5 tidies the layout when all packages exist.
- Covers the story criterion on how `svcview` opens, and the parts of the criteria on the list (the rows, the
  group `Other`, the order, `…`), on the header (the domain), on the refresh (a plist that is added, removed, or
  changed), and on the keys (`q`). The state of each service, its glyph and the counts, the detail pane, the
  selection and `?` are not in this change.
- Two tasks are `low` certainty because they bring Bubble Tea, Lip Gloss and `howett.net/plist` into the
  repository and decide the shape of `internal/launchd` and `internal/tui`. No task is `high` blast radius: the
  change reads files and draws a screen, and the output of `svcview` with no terminal stays the help.
