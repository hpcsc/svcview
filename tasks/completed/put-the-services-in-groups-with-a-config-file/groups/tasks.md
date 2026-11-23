## Contents
1. Task 1: Read the groups of the config file
2. Task 2: Show the services in the groups of the config file
3. Task 3: Show the short name of each service in a group
4. Task 4: Describe the groups in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-002 "Put the services in groups with a config file" (`launchd-tui/US-002`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "The Screen", "Settings of a Group" and "Config Files".

This change puts the services in named groups. The setting `groups` of `config.yaml` is a list of groups, and each
group has a `name` and a `match`, which is a Go regular expression. The list shows the groups in the order of the
config file, and then the group `Other` with each service that no group matches. A service that matches two groups
is in the first one, and in each group the services are sorted by name. A row in a group shows the short name of
its service, and the title of the detail pane always shows the full name. An invalid regular expression, or two
groups with the same name, stops svcview at start, and the error names the file and the group. With no config
file, or with no groups, the screen is the same as before. `docs/cli.md` describes the groups and the rules that
put a service in a group.

The story criteria that this change satisfies:

- Each entry in `groups` has a `name` and a `match`, which is a Go regular expression. The list shows the groups in
  the order of the config, and then the group `Other` with each service that no group matches. A service that
  matches two groups is in the first one. In each group, the services are sorted by name.
- A row in a group shows the short name: the part of the name after the text that `match` matched, when the match
  starts at the first character. For example, `^com\.example\.worker\.` gives `01` for `com.example.worker.01`. In
  all other cases, and in the group `Other`, the row shows the full name.
- An unknown setting, an invalid regular expression, or two groups with the same name stop svcview at start. The
  error names the file and the group.
- `docs/cli.md` describes the config file, its settings, and the rules that put a service in a group.

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
- svcview reads the config file only at start. A change to the config file while svcview runs takes effect at the
  next start.
- svcview reads only `config.yaml`. It reads no `conf.d/` folder.
- A group sets only `name` and `match`. A group has no `status` and no `actions`, and svcview runs no command from
  the config.
- A row shows no status after the short name.
- `↑` and `↓` move one row at a time. No key moves to the next group, and a group does not fold.
- A group with no service shows no line in the list.
- The header counts all services, whatever their group.
- No filter of the list.

**Deferred:**
- The row of a service in a group shows only the glyph of its state and its short name. The `summary` of its status
  is not on the row.

## Codebase Context

- `internal/config` reads `config.yaml` in the config folder into `config.Settings`: `refresh` with its default
  `2s`, an error for a setting that svcview does not know, and the refusal of a file that another user can write.
  Each error names the file. `internal/cli/root.go` reads the settings before the screen opens, an error stops
  svcview with exit 1, and `tui.NewScreen` takes the settings. `e2e/tests/config.test.ts` tests the config file,
  and a helper in `e2e/testUtils.ts` writes a `config.yaml`. The e2e tests start svcview with no `XDG_CONFIG_HOME`
  of their own environment.
- `internal/tui/screen.go`: `Update` sorts the services of each read by name (`screen.go:73-79`). `listText`
  (`screen.go:199-219`) writes one group line `Other` and then one row for each service. The selection keeps a
  name, not a row number. `selectedAfterRead` finds the place of a gone name with a binary search over rows sorted
  by name, and `listTopInView` keeps the selected row in view with one group line before the first row
  (`screen.go:105-138`). Both assume one list sorted by name with one group line, so this change must change both
  and their tests (`tasks/learnings.md`, "The selection of the list assumes one list sorted by name").
- The title of the detail pane is the full name of the selected service (`screen.go:159-173`). `printable` shows
  each control character in a name as `�`, and the list cuts a long name with `…`.
- The tests of the selection are at `internal/tui/screen_test.go:588-846`. The golden files in
  `internal/tui/testdata/` show the screen with no groups. With no groups in the settings, they must not change.
- `internal/launchd/launchctl_list.go:43-72` reads entries and names the bad entry in its error.
- The proposal draws an empty line between two groups ("The Screen", `docs/proposal-launchd-tui.md:68-108`).
- Words: "group" is a named set of services from the config file. "match" is its regular expression, not "pattern"
  or "filter", because a later feature filters the list by name. "short name" is the part of the name after the
  match. "group line" is the line with the name of a group in the list, not "header", which is the box at the top
  of the screen. "selected" is the row that `↑` and `↓` move.

## Tasks

### Task 1: Read the groups of the config file

**Behavior:** The setting `groups` of `config.yaml` is a list of groups. Each group has a `name` and a `match`, a Go
regular expression, and the groups keep the order of the file. The settings give the group of a service: the first
group whose `match` matches its name. svcview refuses a list of groups that it cannot show, and the error names the
file and the group.

**Acceptance Criteria:**
- [x] Each entry in `groups` gives a group with its `name` and its `match`, in the order of the file.
- [x] A service is in a group when the `match` of the group, a Go regular expression, matches its name. A match in
      any part of the name counts, unless the expression has `^` or `$`.
- [x] A service that the `match` of two groups matches is in the first of them. A service that no group matches is
      in no group of the settings.
- [x] With no `groups`, or with no config file, the settings have no groups.
- [x] `refresh` and `groups` are the settings at the top of the file. Any other setting there gives an error that
      names the file and the setting.
- [x] An invalid regular expression gives an error that names the file and the group.
- [x] A group with no `name`, or with no `match`, gives an error that names the file and the group. A group with no
      name is named by its place in `groups`, for example `group 2`.
- [x] An unknown setting in a group gives an error that names the file, the group, and the setting.
- [x] Two groups with the same name give an error that names the file and the group. A group named `Other` gives
      the same error, because `Other` is the group of the services that no group matches. Two names that differ in
      case, for example `Workers` and `workers`, are two names.

**Affected Files/Modules:**
- `internal/config/config.go` — the setting `groups`, the checks of each group, and the group of a service
- `internal/config/group.go` — new: `Group`, with its name and its match
- `internal/config/config_test.go` — the tests of the groups

**Patterns to Follow:**
- `internal/launchd/launchctl_list.go:43-72`
- `internal/launchd/domain_test.go:18-40`

**Testable:** Yes — through the read of the settings and the group of a service, with config files in a temporary
folder.

**Certainty:** medium — `internal/launchd/launchctl_list.go:43-72` reads a list of entries and names the bad entry
in its error, but no instance compiles a regular expression from a setting or checks a rule across the entries.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Show the services in the groups of the config file

**Behavior:** The list shows each group of the settings that has a service, in the order of the config file, and
then the group `Other`. Each group starts with its group line, and in each group the services are sorted by name.
`↑` and `↓` move through the rows in the order of the list, and a refresh keeps the same service selected.

**Acceptance Criteria:**
- [x] The list shows the groups in the order of the config file, and then the group `Other` with each service that
      no group matches. Each group starts with a line with its name, and in each group the services are sorted by
      name.
- [x] An empty line separates two groups, as in "The Screen" of the proposal.
- [x] A group with no service shows no line. When each service is in a group of the settings, the list shows no
      line `Other`.
- [x] With no groups in the settings, the screen is the same as before, and the golden files in
      `internal/tui/testdata/` do not change.
- [x] At start, the first row of the first group is selected. `↓` selects the next row in the order of the list,
      also when the next row is in the next group, and `↑` selects the row before it.
- [x] A refresh keeps the same name selected. When the selected service is gone, the row that takes its place in
      the order of the list is selected, or the last row when it was the last.
- [x] The list scrolls to keep the selected row in view, with a group line for each group and the empty lines
      between them. When the selected row is the first row of its group, the group line of its group shows too.
- [x] An e2e test writes a `config.yaml` with two groups whose names are not in alphabetical order, and sees the
      groups in the order of the file, then `Other`, with the services of each group sorted by name.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the order of the rows, the group lines, and the selection and the scroll over the
  groups
- `internal/tui/screen_test.go` — the tests of the groups, and of the selection and the scroll over them
- `internal/tui/testdata/` — new golden files of a list with groups
- `e2e/tests/config.test.ts` — the groups of the config file on the screen

**Patterns to Follow:**
- `internal/tui/screen.go:98-138`
- `internal/tui/screen.go:199-219`
- `internal/tui/screen_test.go:588-846`
- `task:1`

**Testable:** Yes — through the `Screen` model with key messages and reads that change, compared with golden files,
and in an e2e test.

**Certainty:** medium — the selection and the scroll at `internal/tui/screen.go:105-138` are the precedent, but they
assume one list sorted by name with one group line. Here the order is the group and then the name, and each group
adds lines.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1

### Task 3: Show the short name of each service in a group

**Behavior:** A row in a group shows the short name of its service: the part of the name after the text that
`match` matched, when the match starts at the first character. In all other cases, and in the group `Other`, the
row shows the full name. The title of the detail pane always shows the full name.

**Acceptance Criteria:**
- [x] `^com\.example\.worker\.` gives the short name `01` for `com.example.worker.01`.
- [x] When the match does not start at the first character, for example `worker` for `com.example.worker.01`, the
      row shows the full name.
- [x] When the match is the whole name, the row shows the full name, so no row is empty.
- [x] A row in the group `Other` shows the full name.
- [x] The title of the detail pane shows the full name of the selected service, also when its row shows the short
      name.
- [x] A short name that is too long for the list ends with `…`, and each control character in it shows as `�`, as
      in a full name.
- [x] An e2e test sees `01` in a row of a group, and `com.example.worker.01` in the title of the detail pane when
      that row is selected.

**Affected Files/Modules:**
- `internal/config/group.go` — the short name of a service in the group
- `internal/config/config_test.go` — the tests of the short name
- `internal/tui/screen.go` — the short name in the rows of a group
- `internal/tui/screen_test.go` — the tests of the short name in the list and the full name in the detail pane
- `internal/tui/testdata/` — the golden files of a list with groups
- `e2e/tests/config.test.ts` — the short name on the screen

**Patterns to Follow:**
- `internal/tui/screen.go:199-227`
- `internal/tui/screen.go:159-173`
- `task:2`

**Testable:** Yes — through the short name of a group, through the view of the `Screen` model compared with golden
files, and in an e2e test.

**Certainty:** medium — the rows at `internal/tui/screen.go:199-227` are the precedent, but no row shows a name
other than the full name of its service.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1, Task 2

### Task 4: Describe the groups in docs/cli.md

**Behavior:** `docs/cli.md` describes the setting `groups`, the rules that put a service in a group, the short
name, and the errors of a group that stop svcview at start. The section on the screen shows a list with a group.

**Acceptance Criteria:**
- [x] The section on the config file lists `groups`, with `name` and `match`, and gives an example group with the
      `match` `^com\.example\.worker\.`.
- [x] It gives the rules that put a service in a group: the order of the config file, the first group whose `match`
      matches the name, the group `Other` for each service that no group matches, and the sort by name in each
      group.
- [x] It gives the rule of the short name, with the example `01` for `com.example.worker.01`, and says that the
      rows of `Other` and the title of the detail pane show the full name.
- [x] It lists the errors of a group that stop svcview at start: an invalid regular expression, a group with no
      name or no match, an unknown setting in a group, two groups with the same name, and a group named `Other`.
      Each error names the file and the group.
- [x] The section on the screen says that the list shows the groups of the config file, and its example screen
      shows a group with short names, and `Other`.

**Affected Files/Modules:**
- `docs/cli.md` — the groups in the section on the config file, and in the section on the screen

**Patterns to Follow:**
- `docs/cli.md:35-106`
- `docs/proposal-launchd-tui.md:68-108`

**Testable:** No — a document.

**Certainty:** high — the sections of `docs/cli.md:35-106` are the precedent, and the proposal draws the list with
groups at `docs/proposal-launchd-tui.md:68-108`.

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the screen of `task cli` with a config file that has groups.

**Depends on:** Task 2, Task 3

## Summary

- 4 tasks. The groups of the config file come first, because the list needs them. Task 2 changes the layout of the
  list, so it comes before Task 3, which changes only the text of the rows of a group. The order of the rows, the
  group lines, the selection and the scroll stay in one task: a list in groups with a selection that still
  assumes one list sorted by name moves the selection to the wrong row at each refresh. The document comes last.
- Covers the story criteria on the groups and their order (Tasks 1 and 2), on the short name (Task 3), on the
  invalid regular expression and two groups with the same name (Task 1), and on `docs/cli.md` (Task 4).
- No task is `low` certainty and no task is `high` blast radius: the code reads the config file and draws the list,
  and the selection precedent covers most of Task 2. Task 2 is the task to watch: it changes `selectedAfterRead`
  and `listTopInView`, which `tasks/learnings.md` names.
- The planner chose these details, which the story does not give: an empty line between two groups (from the
  proposal); a group with no service shows no line; a group named `Other` is refused as a duplicate name; names
  that differ in case are two names; a match that is the whole name gives the full name; when the first row of a
  group is selected, its group line shows; a group with no name is named by its place in the error.
