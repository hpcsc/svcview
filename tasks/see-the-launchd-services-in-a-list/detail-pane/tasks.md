## Contents
1. Task 1: Read the PID and the uptime of each running service
2. Task 2: Select a service with the up and down keys
3. Task 3: Show the selected service in the detail pane
4. Task 4: Show all keys with ?
5. Task 5: Describe the screen and its keys in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-001 "See the launchd services in a list" (`launchd-tui/US-001`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Data From launchd" and "Tests".

This change adds a detail pane beside the list. `↑` and `↓` select a service, and the selected row has a `▌` in the
accent color and a background color. The title of the detail pane is the full name of the selected service. The
pane shows the state, the PID, the uptime, and the last exit, which is the name of the signal when a signal ended
the last run. A last exit other than 0 is red. svcview gets the uptime from one `ps` call for all the PIDs at each
refresh. `?` shows all keys. `docs/cli.md` describes the screen and its keys.

The story criteria that this change satisfies:

- The title of the detail pane shows the full name of the selected service. The pane shows the state, the PID, the
  uptime, and the last exit. When a signal ended the last run, the pane shows the signal, not an exit code.
- A last exit other than 0 makes the glyph in the list and the `last exit` value red.
- `↑` and `↓` select a service. The selected row has a `▌` in the accent color and a background color. `?` shows
  all keys, and `q` quits.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- svcview does not parse `launchctl print`.
- No focus on a pane, no `tab`, and no scroll of the detail pane. `↑` and `↓` always select a service.
- No one-pane layout for a terminal with fewer than 60 columns, and no `enter` to open the detail pane.
- No `Status` and no `Events` in the detail pane, no config file, no groups other than `Other`, no actions on a
  service, no filter, no `--all`, and no `NO_COLOR` handling.
- The detail pane shows no program, no environment, and no log path of the service.
- The fake `ps` of the e2e tests answers the elapsed time of the PIDs that it gets, and nothing else.

**Deferred:**
- The help line shows no keys of actions, because the screen has no actions.

## Codebase Context

- The screen exists: `internal/tui` draws the header with the domain and the counts of each state, and the list in
  the group `Other`, with the glyph of each state, red when the last exit is other than 0. It refreshes every 2 s.
  `internal/launchd` reads the plists and `launchctl list` into `service.Service` values with the name, the state
  and the last exit. The e2e tests start svcview with a scratch home folder and a fake `launchctl` first on
  `PATH`.
- `ps -o pid=,etime= -p <PID>,<PID>` prints one line for each PID that exists: the PID, and the elapsed time in
  the form `[[dd-]hh:]mm:ss`. A PID that does not exist gives no line. On macOS, `ps` exits with 1 when no PID
  exists. The Docker image of the e2e tests has no `ps`.
- `internal/cli/release/updater.go:101-114` reads a text file line by line. `internal/cli/progress/line_test.go:13-60`
  checks terminal output as exact text.
- The proposal: the list takes 40 percent of the width, the accent color is blue, and the colors come from the 16
  colors of the ANSI palette. "The Screen" shows the detail pane: `● running     PID 4122` on the first line, and
  `up 3h12m      last exit 0` on the second.
- `docs/cli.md` describes each command in a section of its own (`docs/cli.md:35-56`).
- `internal/tui` has no `view.go`. `Screen` keeps all its view methods in `screen.go`, and the private type `box` in
  `box.go` draws each rounded box with its title and cuts each line with `…`. The colors are SGR styles of
  `github.com/charmbracelet/x/ansi` (`Screen.stateLook`, `stateGlyph`, `rowGlyph`), and `go.mod` has no Lip Gloss.
  `Screen.hasRead` is false until the first read ends, and a pane that shows data of the read must check it.
- `internal/launchd/launchctl_list.go` declares the private type `launchctlList`, which reads the output of
  `launchctl list`. `launchctl` runs with a deadline of 5 s and a `WaitDelay` of 1 s.
- `openCli` in `e2e/testUtils.ts` puts `e2e/fakes` first on `PATH`, and `e2e/fakes/launchctl` prints
  `$HOME/launchctl-list.txt`.
- Words: "selected" is the row that `↑` and `↓` move. A later feature gives a pane the "focus", so no code here
  uses "focus" for the selected row. "uptime" is the time since the process of the service started.

## Tasks

### Task 1: Read the PID and the uptime of each running service

**Behavior:** `service.Service` has the PID and the uptime of its process. At each read, `launchd.Domain` runs `ps`
one time for the PIDs of all running services, and gives each running service the elapsed time that `ps` reports
for its PID as its uptime.

**Acceptance Criteria:**
- [x] A `running` service has the PID that `launchctl list` shows for it, and the uptime that `ps` reports for that
      PID. A `loaded` or `not loaded` service has no PID and no uptime.
- [x] A read runs `ps` one time for all the PIDs, and does not run `ps` when no service is `running`.
- [x] Each form of elapsed time that `ps` prints, `[[dd-]hh:]mm:ss`, gives the right uptime.
- [x] A PID that `ps` does not report gives its service no uptime, and the other services keep theirs.
- [x] When `ps` cannot start or fails, no service has an uptime, and the read still gives the services with their
      state and last exit.
- [x] A row of `launchctl list` whose PID is not `-` or a positive number gives an error that names the row, and no
      services.

**Affected Files/Modules:**
- `internal/service/service.go` — the PID and the uptime of a `Service`
- `internal/launchd/launchctl_list.go` — the PID of each running service
- `internal/launchd/uptimes.go` — new: the private type `uptimes`, the run of `ps`, and the read of its lines
- `internal/launchd/domain.go` — the uptime of each running service
- `internal/launchd/domain_test.go` — the tests of the PID and the uptime
- `internal/launchd/testdata/` — the fixture outputs of `launchctl list` and `ps`

**Patterns to Follow:**
- `internal/cli/release/updater.go:101-114`
- `internal/cli/release/updater_test.go:26-84`

**Testable:** Yes — through `Domain` and its list of services, with fixture output of `launchctl list` and of `ps`.

**Certainty:** medium — `internal/cli/release/updater.go:101-114` reads text line by line, but no instance matches the lines of a second program to services by PID, or keeps the read when that program fails.

**Blast radius:** low

**Verification:** `task test:unit` passes.

**Depends on:** None

### Task 2: Select a service with the up and down keys

**Behavior:** One row of the list is selected. `↓` selects the next row and `↑` the row before it. The selected
row starts with a `▌` in the accent color and has a background color. A refresh keeps the same service selected.

**Acceptance Criteria:**
- [ ] At start, the first row is selected.
- [ ] `↓` selects the next row, and `↑` the row before it. At the last row `↓` does nothing, and at the first row
      `↑` does nothing.
- [ ] The selected row starts with a `▌` in the accent color, blue of the ANSI palette, and has a background color.
      The other rows have no `▌`.
- [ ] A refresh keeps the selection on the same name, also when rows come or go before it. When the selected
      service is gone, the row that takes its place is selected, or the last row when it was the last.
- [ ] With no rows, no row is selected, and `↑` and `↓` do nothing.
- [ ] An e2e test presses `↓`, and sees the `▌` on the second row.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the selected service, the keys `↑` and `↓`, and the `▌` and the background of the
  selected row
- `internal/tui/screen_test.go` — the tests of the selection
- `internal/tui/testdata/` — the golden files
- `e2e/tests/screen.test.ts` — the selection

**Patterns to Follow:**
- `internal/cli/progress/line_test.go:13-60`

**Testable:** Yes — through the `Screen` model with key messages and a fake list of services that changes between reads, and in an e2e test.

**Certainty:** low — the rule that keeps a selection across a refresh that adds and removes rows is decided here.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** None

### Task 3: Show the selected service in the detail pane

**Behavior:** The list and the detail pane stand side by side, with the list at 40 percent of the width. The title
of the detail pane is the full name of the selected service. The pane shows the glyph and the state, the PID, the
uptime, and the last exit. A signal that ended the last run shows as its name. A last exit other than 0 is red.

**Acceptance Criteria:**
- [ ] The list and the detail pane stand side by side, and the list takes 40 percent of the width.
- [ ] The title of the detail pane is the full name of the selected service, also when the list cuts the name
      with `…`.
- [ ] The pane shows the state with its glyph, the PID, the uptime in the form `up 3h12m`, and `last exit` with its
      value, as in "The Screen" of the proposal.
- [ ] A service with no PID shows no PID value, and a service with no uptime shows no uptime value.
- [ ] When a signal ended the last run, the `last exit` value is the name of the signal, for example `SIGKILL`,
      and not a number.
- [ ] A last exit other than 0 makes the `last exit` value red, and the glyph in the list stays red.
- [ ] With no rows, the detail pane shows no service.
- [ ] The e2e tests have a fake `ps`, an executable file in the repository, that prints a fixed elapsed time for
      each PID that it gets. Each screen test starts svcview with it first on `PATH`.
- [ ] An e2e test sees the name, the PID, the uptime and the last exit of a running service in the detail pane, sees
      the name of a signal for a service whose last run a signal ended, and sees the title change after `↓`.
- [ ] `docs/e2e-tests.md` describes the fake `ps`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the detail pane, and the two panes side by side
- `internal/tui/box.go` — a title that is longer than the box ends with `…`, and a box with no title
- `internal/tui/screen_test.go` — the tests of the detail pane
- `internal/tui/testdata/` — the golden files
- `e2e/fakes/ps` — new: the fake `ps`, executable
- `e2e/testUtils.ts` — the fake `ps` first on `PATH` for each screen test
- `e2e/tests/screen.test.ts` — the detail pane
- `docs/e2e-tests.md` — the fake `ps`

**Patterns to Follow:**
- `task:1`
- `task:2`
- `docs/proposal-launchd-tui.md:70-108`

**Testable:** Yes — through the view of the `Screen` model, compared with golden files, and in an e2e test.

**Certainty:** medium — the selection of Task 2 is the precedent, but no instance lays out a second pane beside the list or formats an uptime.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1, Task 2

### Task 4: Show all keys with ?

**Behavior:** `?` shows each key of the screen with what it does. `?` again, or `esc`, hides the keys. The help line
at the bottom of the screen shows `? help`.

**Acceptance Criteria:**
- [ ] `?` shows each key of the screen with its action: `↑` and `↓` select a service, `?` shows all keys, and `q`
      quits.
- [ ] `?` or `esc` hides the keys, and the screen shows the same service selected as before.
- [ ] `q` quits also while the keys show.
- [ ] The help line at the bottom of the screen shows `? help`.
- [ ] An e2e test presses `?`, sees the keys, presses `q`, and sees `EXIT:0`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — `?` and `esc`, the list of keys with the action of each, and the help line
- `internal/tui/screen_test.go` — the tests of `?`
- `internal/tui/testdata/` — the golden files
- `e2e/tests/screen.test.ts` — the keys

**Patterns to Follow:**
- `task:2`

**Testable:** Yes — through the `Screen` model with key messages, compared with golden files, and in an e2e test.

**Certainty:** medium — the keys of Task 2 are the precedent, but no instance shows a view in place of the panes and returns to them.

**Blast radius:** low

**Verification:** `task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 2

### Task 5: Describe the screen and its keys in docs/cli.md

**Behavior:** `docs/cli.md` tells how to open the screen and what each part of it shows, and lists the keys.

**Acceptance Criteria:**
- [ ] `docs/cli.md` has a section on the screen. It says that `svcview` with no command opens the screen in a
      terminal, and prints the help when stdin or stdout is not a terminal.
- [ ] The section tells what the header, the list and the detail pane show, the glyph and the color of each state,
      the red mark of a last exit other than 0, and the refresh every 2 s.
- [ ] The section says that the list shows each service that has a plist in `~/Library/LaunchAgents`, in the group
      `Other`.
- [ ] The section lists each key of the screen with its action.

**Affected Files/Modules:**
- `docs/cli.md` — a section on the screen

**Patterns to Follow:**
- `docs/cli.md:35-56`

**Testable:** No — a document.

**Certainty:** high — the sections of `docs/cli.md:35-56` are the precedent.

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the screen of `task cli`.

**Depends on:** Task 3, Task 4

## Summary

- 5 tasks. The read of the PID and the uptime, and the selection, do not depend on each other and come first. The
  detail pane needs both. `?` needs the keys of the selection. The document comes last, when the screen is
  complete.
- Covers the story criteria on the detail pane, on a last exit other than 0 (the `last exit` value, which
  completes the red glyph in the list), and on the keys (`↑`, `↓`, the selected row, `?`).
- One task is `low` certainty: the rule that keeps the selection across a refresh is decided here. No task is
  `high` blast radius: svcview runs only `launchctl list` and `ps`, which change nothing.
