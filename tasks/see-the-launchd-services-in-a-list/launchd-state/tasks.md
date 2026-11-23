## Contents
1. Task 1: Read the state of each service from launchctl list
2. Task 2: Show the error of launchctl list in place of the rows
3. Task 3: Start each row with the glyph of its state
4. Task 4: Count the services in each state in the header
5. Task 5: Draw the glyph red when the last exit is other than 0

## Story Reference

Story: `user-stories/launchd-tui.md`, US-001 "See the launchd services in a list" (`launchd-tui/US-001`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Data From launchd", "Colors and Glyphs" and "Tests".

This change gives each service on the screen the state that launchd reports for it. svcview runs `launchctl list`
at each refresh. A service is `running` when the list shows its name with a PID, `loaded` when the list shows its
name with no PID, and `not loaded` when only a plist in `~/Library/LaunchAgents` has its name. Each row starts with
the glyph of its state, and the glyph is red when the last exit of the service is other than 0. The header counts
the services in each state. When `launchctl list` fails, or its header line is not `PID`, `Status`, `Label`, the
screen shows the error and no rows. The e2e tests put a fake `launchctl` first on `PATH`.

The story criteria that this change satisfies:

- The list: each row starts with the glyph of the state, a green `●` for `running`, a yellow `◐` for `loaded`, and a
  gray `○` for `not loaded`.
- The header: the number of services in each state, for example `● 2 running   ◐ 0 loaded   ○ 1 not loaded`.
- The refresh: every 2 s, the screen shows a service that starts, stops, or exits, with no key press.
- When the header line of `launchctl list` is not `PID`, `Status`, `Label`, the screen shows an error and no rows.
- From the criterion on the last exit: a last exit other than 0 makes the glyph in the list red.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- svcview does not parse `launchctl print`, and runs no `launchctl` command other than `list`.
- A name that `launchctl list` shows, but that no plist in `~/Library/LaunchAgents` has, is not on the screen.
- No behaviour for a session over SSH with no console session, other than the error that `launchctl` gives.
- No config file, no groups other than `Other`, no status command, no actions on a service, no filter, no `--all`,
  no one-pane layout for a narrow terminal, and no `NO_COLOR` handling.
- The fake `launchctl` of the e2e tests answers `list` only.

**Deferred:**
- No PID, no uptime, and no last exit value on the screen. The last exit shows only as the color of the glyph.
- No selection of a row, no detail pane, and no list of the keys on `?`.

## Codebase Context

- The screen and the read of the plists exist: `internal/launchd` reads the plists in `~/Library/LaunchAgents`
  into `service.Service` values, and refreshes every 2 s, and `internal/tui` draws the header with the domain and
  the list in the group `Other`. The e2e tests start svcview with a scratch home folder that holds plists.
- The module runs no other program yet. `launchctl list` prints a header line and then one line for each name,
  with three columns split by tabs: the PID, or `-` when the process does not run; the Status, which is the exit
  code of the last run, or the negative number of the signal that ended it (for example `-9`); and the label.
- `internal/cli/release/updater.go:101-114` reads a text file line by line, and its error names what it did not
  find. `internal/cli/release/updater.go:24-28` puts an outside source behind an unexported interface, and
  `internal/cli/release/updater_test.go:26-84` fakes that source per test.
- `e2e/tests/update.test.ts:20-78` fakes an outside service for the e2e tests, and `docs/e2e-tests.md:54-76`
  describes that fake. The Docker image of the e2e tests has no `launchctl`, and `task test:e2e:local` must not run
  the real one, so every screen test needs the fake first on `PATH`.
- `internal/cli/progress/line_test.go:13-60` checks terminal output as exact text with escape codes.
- The proposal: the colors come from the 16 colors of the ANSI palette, so that the theme of the terminal sets the
  real colors. The code builds and its unit tests run on Linux, but the numbers in the Status column are the
  signal numbers of macOS.
- Words: "state" is what launchd says about a service. "last exit" is the exit code of the last run, or the signal
  that ended it. The Status column of `launchctl list` is the last exit: no code uses "status" for it, because
  "status" is what a service reports about itself, through a command of a later feature.

## Tasks

### Task 1: Read the state of each service from launchctl list

**Behavior:** At each read, `launchd.Domain` runs `launchctl list` and gives each service that has a plist its
`service.State`: `running` when the list shows its name with a PID, `loaded` when the list shows its name with no
PID, and `not loaded` when the list does not show its name. The e2e tests start svcview with a fake `launchctl`
first on `PATH`.

**Acceptance Criteria:**
- [ ] A service whose name `launchctl list` shows with a PID is `running`.
- [ ] A service whose name `launchctl list` shows with `-` as the PID is `loaded`.
- [ ] A service with a plist whose name `launchctl list` does not show is `not loaded`.
- [ ] A name that `launchctl list` shows, but that no plist has, gives no service.
- [ ] svcview finds `launchctl` on `PATH`.
- [ ] The states read as `running`, `loaded` and `not loaded`.
- [ ] The e2e tests have a fake `launchctl`, an executable file in the repository, that prints the content of a
      fixture file for `list`. A test writes the fixture file, and can change it while the screen is open. Each
      screen test starts svcview with the fake first on `PATH`, so no e2e test runs the real `launchctl`.
- [ ] The screen tests that exist pass with the fake.
- [ ] `docs/e2e-tests.md` describes the fake `launchctl` and its fixture file.

**Affected Files/Modules:**
- `internal/service/service.go` — the type `State`, and the state of a `Service`
- `internal/launchd/list.go` — new: the run of `launchctl list`, and the read of its rows
- `internal/launchd/domain.go` — the state of each service from the plists and the list
- `internal/launchd/domain_test.go` — the tests of the states
- `internal/launchd/testdata/` — a fixture output of `launchctl list`, with a name in each state
- `e2e/fakes/launchctl` — new: the fake `launchctl`, executable
- `e2e/testUtils.ts` — the fake first on `PATH`, and the fixture file, for each screen test
- `docs/e2e-tests.md` — the fake `launchctl`

**Patterns to Follow:**
- `internal/cli/release/updater.go:101-114`
- `internal/cli/release/updater.go:24-28`
- `internal/cli/release/updater_test.go:26-84`
- `e2e/tests/update.test.ts:20-78`
- `docs/e2e-tests.md:54-76`

**Testable:** Yes — through `Domain` and its list of services, with fixture output of `launchctl list`.

**Certainty:** medium — `internal/cli/release/updater.go:101-114` reads text line by line and `e2e/tests/update.test.ts:20-78` fakes an outside service, but no instance runs a program from `PATH` or fakes one.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** None

### Task 2: Show the error of launchctl list in place of the rows

**Behavior:** When `launchctl list` exits with an error, cannot start, or prints a header line that is not `PID`,
`Status`, `Label`, the read of the services gives an error, and svcview does not read the rows of that output.
The screen shows the error and no rows. The next read that succeeds shows the rows again.

**Acceptance Criteria:**
- [ ] When the header line of `launchctl list` is not `PID`, `Status`, `Label`, the screen shows an error that names
      the header line that it got, and no rows.
- [ ] When `launchctl` exits with a code other than 0, the screen shows the output of `launchctl` and no rows.
- [ ] When `launchctl` cannot start, the screen shows that error and no rows.
- [ ] After an error, the first refresh whose output is valid shows the rows again.
- [ ] An e2e test with a fixture whose header line is not `PID`, `Status`, `Label` sees the error and no rows.

**Affected Files/Modules:**
- `internal/launchd/list.go` — the check of the header line, and the errors of `launchctl`
- `internal/launchd/domain_test.go` — the tests of each error
- `internal/launchd/testdata/` — a fixture output with an unknown header line
- `e2e/tests/screen.test.ts` — the error on the screen

**Patterns to Follow:**
- `task:1`
- `internal/cli/release/updater.go:101-114`

**Testable:** Yes — through `Domain` for each error, and in an e2e test for the error on the screen.

**Certainty:** medium — the read of Task 1 is the precedent, but no instance checks a header line before it reads the rows.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1

### Task 3: Start each row with the glyph of its state

**Behavior:** Each row of the list starts with the glyph of the state of its service: a green `●` for `running`, a
yellow `◐` for `loaded`, and a gray `○` for `not loaded`. When the state of a service changes, its row shows the
new glyph at the next refresh.

**Acceptance Criteria:**
- [ ] A `running` service has a green `●` before its name, a `loaded` service a yellow `◐`, and a `not loaded`
      service a gray `○`.
- [ ] Green, yellow and gray are colors of the 16 colors of the ANSI palette.
- [ ] A name that is cut with `…` fits the list pane with its glyph.
- [ ] The golden files show each glyph with its color.
- [ ] An e2e test sees the glyph of each state from the fixture of the fake `launchctl`. When the test changes the
      fixture so that a service has a PID, its row shows `●` with no key press.

**Affected Files/Modules:**
- `internal/tui/view.go` — the glyph and its color on each row
- `internal/tui/screen_test.go` — the tests of the glyphs
- `internal/tui/testdata/` — the golden files
- `e2e/tests/screen.test.ts` — the glyphs, and a change of state at a refresh

**Patterns to Follow:**
- `internal/cli/progress/line_test.go:13-60`
- `docs/proposal-launchd-tui.md:110-126`

**Testable:** Yes — through the view of the `Screen` model with a fake list of services, compared with golden files, and in an e2e test.

**Certainty:** medium — `internal/cli/progress/line_test.go:13-60` checks terminal output with escape codes, but no instance sets colors with Lip Gloss styles or fixes the color profile of a test.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1

### Task 4: Count the services in each state in the header

**Behavior:** Under the line with `svcview` and the domain, the header shows the number of services in each state,
each with the glyph of the state in its color.

**Acceptance Criteria:**
- [ ] The header shows the counts in the form `● 2 running   ◐ 0 loaded   ○ 1 not loaded`, with each glyph in the
      color of its state.
- [ ] A state with no service shows the count 0.
- [ ] The counts change at the refresh that changes a state.
- [ ] When the read of the services gives an error, the header shows no counts.
- [ ] An e2e test sees the counts for a fixture with services in each state.

**Affected Files/Modules:**
- `internal/tui/view.go` — the counts in the header
- `internal/tui/screen_test.go` — the tests of the counts
- `internal/tui/testdata/` — the golden files
- `e2e/tests/screen.test.ts` — the counts

**Patterns to Follow:**
- `task:3`

**Testable:** Yes — through the view of the `Screen` model, compared with golden files, and in an e2e test.

**Certainty:** high — the glyphs and colors of Task 3 are the precedent.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 3

### Task 5: Draw the glyph red when the last exit is other than 0

**Behavior:** `launchd.Domain` reads the Status column of `launchctl list` as the `service.LastExit` of each
service: an exit code, or, for a negative number, the name of the signal that ended the last run. The glyph of a
service whose last exit is other than 0 is red, whatever its state.

**Acceptance Criteria:**
- [ ] A Status of 0 is a last exit of 0, and a positive Status is that exit code.
- [ ] A negative Status is the signal of that number, named as macOS numbers signals, also in a build for Linux.
      For example, `-9` is `SIGKILL` and `-15` is `SIGTERM`.
- [ ] A service that `launchctl list` does not show has a last exit of 0.
- [ ] A last exit other than 0 is an exit code other than 0, or any signal.
- [ ] The glyph of a service with a last exit other than 0 is red, also when the service is `running`. The glyph of
      a service with a last exit of 0 keeps the color of its state.
- [ ] The golden files show a red glyph.

**Affected Files/Modules:**
- `internal/service/service.go` — the type `LastExit`, and the last exit of a `Service`
- `internal/launchd/list.go` — the read of the Status column, and the names of the signals of macOS
- `internal/launchd/domain_test.go` — the tests of the last exit
- `internal/launchd/testdata/` — rows with a code other than 0 and with a signal
- `internal/tui/view.go` — the red glyph
- `internal/tui/screen_test.go` — the tests of the red glyph
- `internal/tui/testdata/` — the golden files

**Patterns to Follow:**
- `task:1`
- `task:3`

**Testable:** Yes — through `Domain` for the last exit, and through the view of the `Screen` model for the red glyph.

**Certainty:** medium — the read of Task 1 and the glyphs of Task 3 are the precedent, but no instance names signals, and the names must follow macOS also in the Linux build that CI tests.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 3

## Summary

- 5 tasks. The read of the states comes first, with the fake `launchctl`, because from that task on svcview runs
  `launchctl` and every screen test needs the fake. The errors of the read come next, while the read is fresh.
  The glyphs, the counts and the red glyph then draw what the read gives.
- Covers the story criteria on the list (the glyphs), on the header (the counts), on the refresh (a service that
  starts, stops, or exits), and on an unknown header line of `launchctl list`, and the list part of the criterion
  on a last exit other than 0. The PID, the uptime, the last exit value, the selection, the detail pane and `?`
  are not in this change.
- No task is `low` certainty: each one varies a precedent that is named. No task is `high` blast radius: svcview
  runs only `launchctl list`, which changes nothing in launchd.
