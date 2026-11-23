## Contents
1. Task 1: Read the settings of config.yaml in the config folder
2. Task 2: Refuse a config file that another user can write
3. Task 3: Read the config file before the screen opens
4. Task 4: Refresh the screen at the time that the config file sets
5. Task 5: Describe the config file in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-002 "Put the services in groups with a config file" (`launchd-tui/US-002`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Config Files", "Names That Do Not Say launchd" and "Code
Layout".

This change gives svcview a config file. When svcview opens the screen, it first reads `config.yaml` in
`$XDG_CONFIG_HOME/svcview/`, or in `~/.config/svcview/` when `XDG_CONFIG_HOME` is not set. The setting `refresh`
sets the time between two refreshes of the screen, and the default is `2s`. With no config file, the screen is the
same as before. An unknown setting, or a `refresh` that is not a time, stops svcview at start, and the error names
the file. svcview refuses to start when another user can write the config file, and the error names the file.
`docs/cli.md` describes the config file and its setting.

The story says why the file mode matters: the config file holds commands that svcview runs (status commands, and
the actions of a group). A config file that another user can write then lets that user run commands as the user of
svcview. This change runs no command from the config, but the refusal must be correct before any setting can name
a command.

The story criteria that this change satisfies:

- svcview reads `config.yaml` in `$XDG_CONFIG_HOME/svcview/`, or in `~/.config/svcview/` when `XDG_CONFIG_HOME` is
  not set. With no config file, the screen is as in US-001.
- `refresh` sets the time between two refreshes of the screen. The default is `2s`.
- svcview refuses to start when another user can write the config file. The error names the file.

Of the criterion "An unknown setting, an invalid regular expression, or two groups with the same name stop svcview
at start. The error names the file and the group.", this change gives the unknown setting. Of the criterion
"`docs/cli.md` describes the config file, its settings, and the rules that put a service in a group.", this change
describes the config file and `refresh`.

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
- The config file has one setting, `refresh`. Any other setting stops svcview at start. No setting holds a command,
  and svcview runs no command from the config.
- svcview checks the mode and the owner of the config file, not of the folders that hold it.
- svcview does not use `~/Library/Application Support`.
- `refresh` has no limit other than that it is more than 0.
- `svcview version`, `svcview update`, and the help that svcview prints without a terminal do not read the config
  file.

**Deferred:**
- The list keeps every service in the group `Other`. No setting puts a service in a group, and each row shows the
  full name.
- svcview does not read the ACL of `config.yaml`. On macOS, an ACL can let another user write a file whose mode is
  `0644`, and svcview reads that file. On Linux, the mode check covers a POSIX ACL, because an ACL entry that lets
  another user write sets the group write bit through the ACL mask. `docs/cli.md` states the limit. The plan of
  US-003 must refuse a `config.yaml` whose ACL lets another user write it, before any deliverable that runs a
  command from the config (`decisions.md` of US-002, entry 2).

## Codebase Context

- `internal/cli/root.go:51-60`: `showScreen` gets the home folder, makes the `launchd.Domain` with the user ID from
  `os.Getuid()`, and runs the screen. An error there, for example no home folder, ends svcview with exit 1 before
  the screen opens, and `Run` prints the error. Only e2e tests reach this branch, because the Go tests have no
  terminal (`tasks/learnings.md`, "Only e2e tests reach the terminal branch of the root command").
- `internal/tui/screen.go`: the constant `refreshEvery` is 2 s. `Update` asks for the next refresh with a tick
  after each read ends (`screen.go:73-82`). `screen_test.go` calls `tui.NewScreen(domain, read)` 22 times, and
  `root.go` calls it one time. The refresh tests run the real program on the fake clock of `synctest` (`runFor`,
  `screen_test.go:158-178`; the tests at `screen_test.go:364-384`).
- No package reads YAML yet. `go.mod` has `go.yaml.in/yaml/v3` v3.0.5 as an indirect dependency, and the proposal
  names it for the config. The proposal puts the config code in `internal/config` ("Code Layout").
- No code checks the mode or the owner of a file. The release builds only for linux and darwin
  (`.goreleaser.yaml`), so each build target is a Unix system.
- `internal/launchd/plist_folder.go:15-45` gives no names when its folder does not exist. The launchd tests write
  their files into `t.TempDir()` (`internal/launchd/domain_test.go:18-40`). The unit tests have the build tag
  `unit`. Run Go through `mise exec --`, because the shell `GOROOT` is Go 1.27.1 and `go.mod` needs 1.27.4.
- e2e: `openCli` (`e2e/testUtils.ts:104-123`) gives svcview the environment of the test run, with `PATH` and the
  `env` of the test in front. On a Mac whose user sets `XDG_CONFIG_HOME`, `task test:e2e:local` would then read the
  real config of that user. The helpers at `e2e/testUtils.ts:56-74` write files into the scratch home folder.
  `e2e/tests/screen.test.ts:50-56` tests an error before the screen opens. In Docker the tests run as root.
- Words: a "setting" is an entry in a config file, and "settings" are all of them. "key" is a key of the keyboard,
  and "property" is an entry in a plist or in the output of a status command, so neither word names a setting.
  "refresh" is the read of the services again. The code says "folder", not "directory" (`plistFolder`).

## Tasks

### Task 1: Read the settings of config.yaml in the config folder

**Behavior:** The new package `internal/config` gives the config folder of svcview, and reads the settings in
`config.yaml` in that folder. `refresh` is the time between two refreshes, `2s` by default. Each error names the
file.

**Acceptance Criteria:**
- [x] The config folder is `$XDG_CONFIG_HOME/svcview` when `XDG_CONFIG_HOME` is set, and `~/.config/svcview` when
      it is not. An empty or a relative `XDG_CONFIG_HOME` counts as not set, as the XDG Base Directory
      Specification says.
- [x] With no `config.yaml` in the config folder, with no config folder, or with an empty `config.yaml`, the
      settings are the defaults: a refresh of 2 s.
- [x] `refresh` takes a Go duration, for example `5s` or `1m30s`.
- [x] A `refresh` that is not a duration, or that is not more than 0, gives an error that names the file and the
      setting.
- [x] A setting that svcview does not know gives an error that names the file and the setting.
- [x] A `config.yaml` that is not valid YAML, or whose top level is not a set of settings, gives an error that
      names the file.
- [x] A `config.yaml` that svcview cannot read, for example a folder with that name, gives an error that names the
      file.

**Affected Files/Modules:**
- `internal/config/config.go` — new: the settings, the config folder, and the read of `config.yaml`
- `internal/config/config_test.go` — new: the tests of the read and of the config folder
- `go.mod` — `go.yaml.in/yaml/v3` becomes a direct dependency

**Patterns to Follow:**
- `internal/launchd/plist_folder.go:15-45`
- `internal/launchd/launchctl_list.go:43-72`
- `internal/launchd/domain_test.go:18-40`

**Testable:** Yes — through the read of the settings and the config folder, with config files in a temporary
folder.

**Certainty:** low — it uses `go.yaml.in/yaml/v3` directly for the first time, and it sets the shape of
`internal/config`, which the status settings, the actions and `conf.d/` extend.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Refuse a config file that another user can write

**Behavior:** svcview refuses `config.yaml` when another user can write it: its group or all users can write it, or
a user other than the user of svcview owns it. The error names the file, and svcview gives no settings.

**Acceptance Criteria:**
- [x] A `config.yaml` that its group can write, or that all users can write, gives an error that names the file
      and says that another user can write it.
- [x] A `config.yaml` that a user other than the user of svcview owns gives the same error, also when only its
      owner can write it.
- [x] A `config.yaml` that the user of svcview owns, and that only its owner can write, for example with the mode
      `0644` or `0600`, gives its settings.
- [x] The mode and the owner that svcview checks are those of the file that it reads, also when `config.yaml` is a
      symbolic link.
- [x] With no `config.yaml`, svcview checks nothing, and the settings are the defaults.

**Affected Files/Modules:**
- `internal/config/config.go` — the check of the mode and the owner of `config.yaml`
- `internal/config/config_test.go` — the tests of the check

**Patterns to Follow:**
- `internal/launchd/domain_test.go:18-40`

**Testable:** Yes — through the read of the settings, with a temporary `config.yaml` whose mode the test sets, and
with a user ID other than the owner of the file.

**Certainty:** low — no code in this repo checks the mode or the owner of a file, and the rule for "another user
can write" is decided here.

**Blast radius:** high — the check decides whether another user of the Mac can put settings into svcview, and the
config file holds the commands that svcview runs as its user.

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 1

### Task 3: Read the config file before the screen opens

**Behavior:** When `svcview` opens the screen, it first reads `config.yaml` in the config folder. An error in the
config file stops svcview before the screen opens: svcview prints the error, which names the file, and exits with
1. With no config file, the screen opens as before.

**Acceptance Criteria:**
- [x] `svcview` with no command, in a terminal, reads `config.yaml` in `$XDG_CONFIG_HOME/svcview/`, or in
      `~/.config/svcview/` when `XDG_CONFIG_HOME` is not set, before it opens the screen.
- [x] Each error of the config file stops svcview before the screen opens. svcview prints the error, which names
      the file, and exits with 1.
- [x] svcview reads the config file only when it opens the screen. `svcview version`, `svcview update`, and the
      help that svcview prints when stdin or stdout is not a terminal do not read it.
- [x] The e2e tests start svcview with no `XDG_CONFIG_HOME` from the environment of the test run, so a test run
      never reads the config file of the user who runs it. A test can set `XDG_CONFIG_HOME` for itself.
- [x] An e2e helper writes a `config.yaml` into a config folder.
- [x] e2e tests: an unknown setting in `$XDG_CONFIG_HOME/svcview/config.yaml`, and one in
      `$HOME/.config/svcview/config.yaml` with no `XDG_CONFIG_HOME`, each stop svcview with `EXIT:1` and an error
      that names that file. A `config.yaml` that all users can write stops svcview with `EXIT:1` and an error that
      names the file. A valid `config.yaml` opens the screen.
- [x] `docs/e2e-tests.md` describes the helper that writes a `config.yaml`, and says that the tests start svcview
      with no `XDG_CONFIG_HOME` of their own environment.

**Affected Files/Modules:**
- `internal/cli/root.go` — read the config file in `showScreen`, before the screen opens
- `e2e/testUtils.ts` — no `XDG_CONFIG_HOME` from the environment of the test run in `openCli`, and a helper that
  writes a `config.yaml`
- `e2e/tests/config.test.ts` — new: the e2e tests of the config file
- `docs/e2e-tests.md` — the config helper, and the `XDG_CONFIG_HOME` of the tests

**Patterns to Follow:**
- `internal/cli/root.go:51-60`
- `e2e/tests/screen.test.ts:50-56`
- `e2e/testUtils.ts:56-74`
- `docs/e2e-tests.md:81-101`

**Testable:** Yes — in e2e tests only, because the Go tests do not reach the branch of the root command that opens
the screen.

**Certainty:** medium — `showScreen` (`internal/cli/root.go:51-60`) already stops before the screen when there is
no home folder, and `e2e/tests/screen.test.ts:50-56` tests that, but no instance reads a second source before the
screen, or keeps the environment of the test run away from svcview.

**Blast radius:** high — the refusal of a config file that another user can write takes effect here. An error of
the read that does not stop svcview lets that file in.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 2

### Task 4: Refresh the screen at the time that the config file sets

**Behavior:** The screen reads the services again at the `refresh` of the config file, not at a fixed 2 s.
`svcview` opens the screen with the settings that it read.

**Acceptance Criteria:**
- [ ] The screen takes the settings when svcview makes it, and reads the services again one `refresh` after each
      read ends.
- [ ] With a refresh of 5 s, the screen does not read again before 5 s have passed, and shows a change no later
      than 5 s after the read before it.
- [ ] With the default settings, the screen reads again every 2 s, as before.
- [ ] `svcview` opens the screen with the settings of its config file.
- [ ] An e2e test sets `refresh: 1m` in `config.yaml`, and sees that a plist added while the screen is open does not
      show within 4 s.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the time between two refreshes comes from the settings
- `internal/tui/screen_test.go` — each screen gets the settings, and the tests of a refresh that is not 2 s
- `internal/cli/root.go` — give the settings to the screen
- `e2e/tests/config.test.ts` — the refresh of the config file

**Patterns to Follow:**
- `internal/tui/screen.go:73-82`
- `internal/tui/screen_test.go:158-178`
- `internal/tui/screen_test.go:364-384`

**Testable:** Yes — through the `Screen` model on the fake clock of `synctest`, and in an e2e test.

**Certainty:** high — the refresh tick at `internal/tui/screen.go:73-82` and its tests at
`internal/tui/screen_test.go:364-384` are the precedent, and only the source of the time changes.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` (or `task test:e2e:local`) passes.

**Depends on:** Task 1, Task 3

### Task 5: Describe the config file in docs/cli.md

**Behavior:** `docs/cli.md` tells where svcview finds its config file, what `refresh` does, which errors stop
svcview at start, and why svcview refuses a config file that another user can write.

**Acceptance Criteria:**
- [ ] `docs/cli.md` has a section on the config file. It says where svcview looks for `config.yaml`, with
      `XDG_CONFIG_HOME` and without it, and that svcview reads the file only at start.
- [ ] The section lists `refresh`, with its meaning and its default `2s`, and gives an example `config.yaml`.
- [ ] The section says that an unknown setting, or a `refresh` that is not a time more than 0, stops svcview at
      start, and that the error names the file.
- [ ] The section says that svcview refuses a config file that another user can write, why it does, and how to
      give the file a mode that svcview accepts.
- [ ] The section on the refresh says that the time comes from `refresh`.

**Affected Files/Modules:**
- `docs/cli.md` — a section on the config file, and the refresh

**Patterns to Follow:**
- `docs/cli.md:35-106`

**Testable:** No — a document.

**Certainty:** high — the sections of `docs/cli.md:35-106` are the precedent.

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the behaviour of `task cli` with a config file.

**Depends on:** Task 3, Task 4

## Summary

- 5 tasks. The package `internal/config` comes first: Task 1 reads the settings, and Task 2 adds the refusal of a
  file that another user can write, so the check has its own commit and its own tests. Task 3 reads the config
  file before the screen opens, so each error stops svcview at start. Task 4 then gives the refresh of the config
  file to the screen. The document comes last.
- Covers the story criteria on the place of the config file (Tasks 1 and 3), on `refresh` (Tasks 1 and 4), and on a
  config file that another user can write (Tasks 2 and 3). Of the criterion on errors at start, it covers the
  unknown setting (Tasks 1 and 3). Of the criterion on `docs/cli.md`, it covers the config file and `refresh`
  (Task 5). The invalid regular expression, two groups with the same name, and the rules of a group are outside
  this change (Boundaries).
- Tasks 1 and 2 are `low` certainty: Task 1 uses the YAML library directly for the first time and sets the shape of
  the config package, and Task 2 decides the rule for "another user can write". Tasks 2 and 3 are `high` blast
  radius: the check, and the read that makes its error stop svcview.
- The planner chose these details, which the story does not give: an empty or a relative `XDG_CONFIG_HOME` counts
  as not set; an empty `config.yaml` gives the defaults; `refresh` must be more than 0; a file that another user
  owns counts as a file that another user can write; svcview reads the config file only when it opens the screen.
