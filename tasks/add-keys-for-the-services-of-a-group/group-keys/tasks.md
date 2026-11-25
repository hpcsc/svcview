## Contents
1. Task 1: Move the texts of a command into status.CommandTemplate
2. Task 2: Read the actions of a group from the config file
3. Task 3: Refuse a key of svcview and a key that two actions of one group use
4. Task 4: Show the keys of the actions of the selected group in the help line and in the help
5. Task 5: Give the terminal to the command of an action with its key
6. Task 6: Ask for a confirmation before the command of an action with confirm
7. Task 7: Describe the actions of a group in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-011 "Add keys for the services of a group" (`launchd-tui/US-011`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Keys", "Settings of a Group", "Template Values", "Actions of
a Group", and "Config Files" (the row "Errors": a key conflict stops svcview at start).

The author of a program that adds its services to svcview gives its group its own keys, for example a key that opens
the worker log, so that the users of the program reach its tools from svcview. Each entry in `actions` of a group has
a `key`, a `name`, a `command`, and an optional `confirm`. The key acts only when a service of the group is selected.
The help line then shows the key and its name, for example `L worker log`, and `?` shows them too. The command gets
the template values of the selected service, and svcview gives the terminal to the command and shows the screen again
when the command ends. With `confirm: true`, svcview asks first, and runs no command on a no. When a template value
is not in the plist, svcview runs no command, and the message line shows the error, for example
`the plist has no WORKER_LOG`. A key that svcview uses, or a key that two actions of one group use, stops svcview at
start, with an error that names the file and the group.

The story criteria that this change satisfies:

- Each entry in `actions` of a group has a `key`, a `name`, a `command`, and an optional `confirm`. The key works
  only when a service of the group is selected. The help line then shows the key and its `name`, for example
  `L worker log`, and `?` shows it too.
- The command gets the template values of the selected service. svcview gives the terminal to the command, and shows
  the screen again when the command ends.
- With `confirm: true`, svcview asks for a confirmation first, and runs no command on a no.
- When a template value is not in the plist, svcview runs no command, and the message line shows the error, for
  example `the plist has no WORKER_LOG`.
- A key that svcview uses, or a key that two actions of one group use, stops svcview at start. The error names the
  file and the group.
- `docs/cli.md` describes the `actions` settings.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No work in the worker repo. Its install script adds `WORKER_LOG` to the plist and writes the actions of its group
  in `conf.d/worker.yaml`. That work is outside svcview.
- A key of an action is one character that prints and is not a space. Names of keys such as `ctrl+l`, `f2`, `enter`,
  `tab`, `esc` or `space` are not keys of an action.
- An action is a setting of a group in a config file. The group `Other` has no actions, and no key acts on a service
  of a group other than its own.
- The key of an action acts on the selected service only, never on all the services of the group.
- No new template value, and no change to the template values, to the failure of a value that the plist does not
  have, or to the check of a text of a command. The status commands, their failures, and the output of
  `svcview status` do not change.
- `svcview status` shows no action and runs no action. It reads the config file as before, so an error in `actions`
  stops it as each other error of a config file does.
- No change to the signals of svcview. `cmd/svcview/main.go`, `endAtInterrupt` and the catch of `SIGINT` in
  `showScreen` stay as they are; the command of an action gets the terminal in the one way that `o` uses.
- The command of an action runs with no shell, with the environment and the working folder of svcview, and with no
  deadline, until it ends. svcview does not stop it at `SIGTERM`, and ends when it ends, as for `less` after `o`.
- svcview does not show what the command writes after it ends. The screen covers it when it shows again, and the
  message line shows the exit code.
- svcview does not check that the program of an action exists before the key press. A program that cannot start
  shows on the message line.
- The keys `s`, `x`, `r` and `o`, the confirmation of `x` and `r`, and the help line and the help of a service in a
  group with no actions do not change.
- Two actions of one group can have the same name. Only their keys must differ.
- No setting for the place of the keys of a group in the help line, and no color for them.
- No filter of the list, no layout for a narrow terminal, no change for `NO_COLOR`, and no `--all`. `/` is a key of
  svcview here only so that an action cannot take the key of the filter of the list.

**Deferred:** None.

## Codebase Context

- `internal/status/command.go:38-62`: `status.NewCommand` parses each text of a command as a Go template with
  `missingkey=error`, and refuses an empty command, an empty program, a text that is not a Go template, a text that
  defines a template, and a text that uses a value or an action that a text cannot use (the walk of the parse tree,
  `command.go:173-240`). `command.go:144-155` (`commandLine`) applies the template values of a service to each text,
  and `internal/status/values.go:47-62` (`values.apply`) gives the failure of the kind template value, with the text
  `the plist has no X` for a missing variable and the texts of `values.go:26-45` for a missing path. This code is
  private to `status.Command`, which also holds `Every`, `Timeout` and `StaleAfter`, which an action has no use for.
  Task 1 moves the texts and their checks into `status.CommandTemplate`, so that an action uses the same code.
  The error of a text that defines a template names "a text of a status command" (`command.go:54`); no test asserts
  that part of the text.
- `tasks/learnings.md`, "A change to the template values changes the check of status.NewCommand": the walk, the
  struct that a run applies, `valuesWithoutKey`, and `values.list` belong together. Task 1 moves them as they are and
  adds no template value.
- `internal/config/group.go:39-66` (`readGroup`) knows the settings `name`, `match` and `status` of a group through
  `checkSettings` (`group.go:118-131`), and labels a group `group "Workers"`, or `group 2` with no name
  (`group.go:45-48`). `group.go:83-116` (`readStatus`) is the read of a set of settings inside a group, with
  `textsOf` (`group.go:148-160`) for a list of texts and an error per fault. `internal/config/config.go:73-96`
  (`setGroups`) reads a list of sets of settings with a label by place, and refuses a name that an earlier entry has,
  with the line of that entry (`config.go:80-89`). `config.go:158-160` puts the path of the file before each error of
  its content, so each error of a group names the file. No setting of a config file is a boolean yet.
- `internal/config/config_test.go:542-556` uses `actions: [restart]` as a setting of a group that svcview does not
  know (lines 544-545). When `actions` is a setting, those two cases must use another name.
  `config_test.go:702-715` uses `actions` as a setting of `status` that svcview does not know, which stays true.
- Group is comparable today. The planned design of US-003
  (`tasks/see-the-status-that-each-service-reports/status-settings/design.yaml`) says "a group stays comparable, so
  its status command is a pointer". `internal/tui/screen.go:292` compares the group of a row with `==`, and
  `screen.go:337` finds the index of a group in `groupOrder` with `slices.Index`, which needs a comparable type. A list
  of actions in `config.Group` stops both lines from compiling. The name of a group is unique across all config files
  (`config.go:98-106`), and no group can have the name `Other` (`group.go:27-28`), so a name tells two groups apart.
- Probe of `go.yaml.in/yaml/v3` v3.0.5, the version of `go.mod`: `confirm: true` and `confirm: True` have the tag
  `!!bool`; `confirm: yes` and `confirm: 'true'` have the tag `!!str`; `key: 1` has the tag `!!int` and the value `1`;
  `key: ' '` is the text of a space; `key: ~` is `!!null`; `key: ?` with no quotes is not valid YAML.
- Bubble Tea v2.1.2: `KeyPressMsg.String()` gives the text of the key when it has one that is not a space
  (`github.com/charmbracelet/ultraviolet` `key.go:391-396`), so `shift+l` gives `L` and the space bar gives `space`.
  `internal/tui/screen.go:184-245` (`press`) switches on that text. The one-character keys that `press` handles are
  `q`, `?`, `l`, `h`, `s`, `x`, `r`, `o`, `k` and `j`. The Keys table of the proposal
  (`docs/proposal-launchd-tui.md:128-143`) also gives `/` to the filter of the list (US-012) and `enter` to the
  narrow terminal (US-014). `y` acts only while a confirmation shows (`screen.go:247-257`).
- `internal/tui/screen.go:24` holds the help line as a constant; `content` (`screen.go:428-442`) adds it under the
  panes or the help, cut at the width with `ansi.Truncate`. `helpText` (`screen.go:481-495`) has 11 lines, and
  `keys.golden` is 80×17 (`screen_test.go:3735-3742`), which has room for 11 lines. Each of the golden files in
  `internal/tui/testdata` ends with the help line, and none of them has a group with actions, so a help line that is
  the same for a group with no actions keeps their bytes. `screen_test.go:3793-3807` tests the help line and its cut.
- `internal/tui/screen.go:222-230` is `o`: a selected service and no help, then the terminal for
  `less +F -- <log path>` through `giveTerminalTo` (`screen.go:317-321`). `programEnded` (`screen.go:169-172`) puts
  the failure of the program on the message line with `programFailureText` (`screen.go:323-333`): the command line,
  then `exited with code N`, `ended by SIGx`, or `run <command line>: <error>`. `serviceToActOn`
  (`screen.go:395-398`) is the guard of `s`, `x` and `r`, and adds "no action on launchd of the service that has not
  ended" to the guard of `o`.
- The rules of `SIGINT` that US-010 built: `cmd/svcview/main.go` ends the context only at `SIGHUP` and `SIGTERM`;
  `endAtInterrupt` (`internal/cli/root.go:82-92`) ends each subcommand at `SIGINT`; `showScreen`
  (`internal/cli/root.go:63-80`) catches `SIGINT` while the screen runs. A key that uses `giveTerminalTo` keeps these
  rules with no change outside `internal/tui`.
- The confirmation (`internal/tui/screen.go:56-64`) holds the question, the service, and the action on launchd that
  `y` runs; `answer` (`screen.go:247-257`) runs it through `actionOn` (`screen.go:307-315`), which marks the action of
  the service as not ended when it is called and runs it in the background. The command of an action must get the
  terminal in the foreground on `y`. The question of `x` is `Stop <name>? (y/N)` (`screen.go:62-64`).
- The word action: the planned designs of US-008 and US-009 give it to the start, the stop or the restart that a key
  does to the selected service (`tasks/restart-a-service/restart/design.yaml`). The proposal calls both the actions on
  launchd and the actions of a group "actions", so the word now covers what any key does to the selected service.
  `actionOn`, `actionEnded` and `actionNotEnded` in `internal/tui` stay the actions on launchd that run in the
  background.
- `internal/cli/root.go:94-105` (`readSettingsAndDomain`) reads the config for the screen and for `svcview status`,
  so an error in `actions` stops both. `config.Read` keeps its parameters.
- e2e: `e2e/fakes/less` adds each call to `$HOME/less-calls.txt` (`lessCalls`), prints
  `the fake less shows: <arguments>`, traps `SIGINT`, and ends at `q` and enter, so the action of the proposal,
  `[less, +F, '{{.Env.WORKER_LOG}}']`, reaches it from `PATH` with no new fake. `addPlist`
  (`e2e/testUtils.ts:33-58`) writes `EnvironmentVariables`, and `writeConfig` (`e2e/testUtils.ts:113-119`) writes
  `config.yaml`. `e2e/tests/config.test.ts:84-100` is the precedent for an error at start that names the file and the
  group, and `e2e/tests/log.test.ts` for a key that gives the terminal to a program.
- Go tests of the screen: `TestMain` writes the one fake `less` (`internal/tui/screen_test.go:37-55`), `fakeLess`
  reads its calls (`screen_test.go:57-74`), `groups` builds settings from `config.NewGroup`
  (`screen_test.go:112-121`), and `workersWithStatus` sets a field of a group after `config.NewGroup`
  (`screen_test.go:295-300`), which is the precedent for a group with actions in a test. The group
  `read the log of a service` (`screen_test.go:3531-3726`) is the precedent for the tests of a key that gives the
  terminal to a program, and the groups `stop a service` and `restart a service` (`screen_test.go:3114-3530`) for a
  confirmation.
- `tasks/learnings.md` entries that apply: "internal/config is the precedent for a setting and for the check of the
  config file" and "A word of the subtest name is in every t.TempDir path" (Tasks 2 and 3); "Check the name of a new
  type against the words of design.yaml before clerk design note" (Tasks 1 and 2); "Check by hand a mutant that only
  the tests of another package kill" (Task 1); "Join the lines of the screen before an e2e test checks a long error"
  (Task 3); "Plan the layout change of the screen as the first task", "Each key of the screen goes into the help and
  docs/cli.md", "A test of a cut at a width needs the exact-fit width and one cell less" and "The list draws and
  counts its lines through linesAbove and row.compare" (Tasks 2 and 4); "An e2e test finds the help line by a short
  part of it" (Tasks 4 to 6); "Text from outside goes to the terminal through terminal.ReplaceControlCharacters"
  (Tasks 4 to 6); "A method that holds the guard of the keys replaces each copy of the guard", "A screen test of a
  read error must select a service before the error", "A screen test gives each run of a program through a key its
  own Bubble Tea program" and "Each subcommand ends at SIGINT through endAtInterrupt, and the screen catches SIGINT
  itself" (Task 5); "A test that keys change nothing presses each key from its own start state" (Tasks 5 and 6);
  "svcview status reads the whole config file, so it reads each new setting" (Task 7); "Run clerk mutants and clerk
  audit run under mise exec"; "task test:e2e:local leaves e2e/package-lock.json in the tree".
- Words: an action of a group is one entry of `actions`; its command template is the program and its arguments as the
  config file gives them; its command line is what the template values make of them for one service; a key of
  svcview is a key that the screen handles itself; `confirm` makes the key ask for a confirmation; svcview gives the
  terminal to the command; a failure shows on the message line.

## Tasks

### Task 1: Move the texts of a command into status.CommandTemplate

**Behavior:** The texts of a command and their template values are one type, `status.CommandTemplate`, which a status
command and an action of a group can each use. `status.NewCommandTemplate` takes the texts of a command and refuses
each text that `status.NewCommand` refuses today, with the same error. Its `Apply` gives the command line with the
template values of one service, or the failure of the kind template value, such as `the plist has no WORKER_LOG`, and
runs no program. `status.Command` holds a `CommandTemplate`, so each status command, its failures, `svcview status`
and each error of a config file stay as they are. The error of a text that defines a template says "a text of a
command" in place of "a text of a status command", because the rule is now the rule of each command.

**Acceptance Criteria:**
- [x] `status.NewCommandTemplate` refuses an empty command, an empty program, a text that is not a Go template, a text
      that defines a template, and a text that uses a value or an action that a text cannot use, each with the error
      that `status.NewCommand` gives today; the error of a text that defines a template names "a text of a command".
- [x] `Apply` gives the command line of a service with each template value of that service, also in the program, and
      gives the failure of the kind template value for a value that the plist does not have, with the same text that
      `status.TemplateValues` gives for that value, for example `the plist has no WORKER_LOG`.
- [x] `status.Command` holds a `CommandTemplate`. Each test in `internal/status`, `internal/config`, `internal/cli`
      and `internal/tui` passes with no change to what it asserts.
- [x] `TestCommandTemplate`, in `internal/status/command_template_test.go`, is the umbrella test of the new type.

**Affected Files/Modules:**
- `internal/status/command_template.go` — new: `CommandTemplate`, `NewCommandTemplate`, `Apply`, and the check of the
  texts
- `internal/status/command.go` — `Command` holds a `CommandTemplate`
- `internal/status/command_template_test.go` — new: `TestCommandTemplate`

**Patterns to Follow:**
- `internal/status/command.go:38-62`
- `internal/status/command.go:144-155`
- `internal/status/values.go:47-62`
- `internal/status/values_test.go:131-139`
- `internal/status/command_test.go:189-264`

**Testable:** Yes — through `status.NewCommandTemplate` and `Apply`, and through the tests of `status.Command`.

**Certainty:** high — the code moves out of `status.Command` (`command.go:38-62`, `command.go:144-155`) with its
checks and its failures as they are.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** None

### Task 2: Read the actions of a group from the config file

**Behavior:** A group can have the setting `actions`, a list in which each entry has a `key`, a `name`, a `command`,
and an optional `confirm`. svcview reads each entry into a `config.Action` of the group, in the order of the file. A
`key` is one character that prints and that is not a space. A `name` is a text that is not empty. A `command` is a
list of texts with the program first, each a Go template of the template values, as for `status.command`, through
`status.NewCommandTemplate`. A `confirm` is `true` or `false`, and `false` when the entry has none. An error in
`actions` stops svcview at start, and names the file, the group, and the action by its place in `actions`, for
example `action 2 of group "Workers"`. A group with a list of actions is no longer comparable, so the screen tells two
groups apart by their names. The screen shows no change.

**Acceptance Criteria:**
- [ ] A group with `actions` gives a group whose actions have the key, the name, the command and the confirm of each
      entry, in the order of the file, and the command gives the command line of a service through `Apply`. An entry
      with no `confirm` has a confirm of false. A group with no `actions` has no actions, and so does the group
      `Other`.
- [ ] Each of these gives an error that names the file, the group, and the action by its place, and gives no
      settings: an `actions` that is not a list; an entry that is not a set of settings; a setting of an action that
      svcview does not know, or that an action sets two times; an action with no `key`, no `name`, or no `command`; a
      `key` that is not one character that prints, or that is a space, for example `ctrl+l`, `LL`, a tab, or `' '`; an
      empty `name`; a `command` that is not a list of texts, that is empty, or that `status.NewCommandTemplate`
      refuses; a `confirm` that is not `true` or `false`, for example `yes`.
- [ ] The two cases of `internal/config/config_test.go` that use `actions` as a setting of a group that svcview does
      not know use another name, and the case that uses `actions` as a setting of `status` that svcview does not know
      stays as it is.
- [ ] The screen tells two groups apart by their names: the order of the groups in the list, the status runs of each
      group, and each test in `internal/tui` stay as they are.

**Affected Files/Modules:**
- `internal/config/action.go` — new: `Action`, and the read of an entry of `actions`
- `internal/config/group.go` — the setting `actions` of a group, and the field `Actions`
- `internal/config/config_test.go` — the tests of `actions`, through `config.Read`
- `internal/tui/screen.go` — the screen tells two groups apart by their names

**Patterns to Follow:**
- `internal/config/group.go:39-66`
- `internal/config/group.go:83-116`
- `internal/config/group.go:118-160`
- `internal/config/config.go:73-96`
- `internal/config/config_test.go:573-720`
- `internal/tui/screen.go:289-298`
- `internal/tui/screen.go:335-338`
- `task:1`

**Testable:** Yes — through `config.Read` with a config folder in `t.TempDir`.

**Certainty:** medium — `readStatus` (`group.go:83-116`) and the read of the list of groups (`config.go:73-96`) are
the precedent, but no setting is a list of sets of settings inside a group, no setting is a boolean, and no change of
a type of `internal/config` has reached `internal/tui` before.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes.

**Depends on:** Task 1

### Task 3: Refuse a key of svcview and a key that two actions of one group use

**Behavior:** An action whose key is a key of svcview stops svcview at start. The keys of svcview are each
one-character key that `Screen.press` handles, and `/`, which the Keys table of the proposal gives to the filter of the
list. Two actions of one group with the same key stop svcview at start too. Each error names the file, the group, the
action and the key; the error of two actions also gives the line of the first action. The case counts, so `L`, `S`
and `X` are not keys of svcview, and two groups can each have an action with the same key.

**Acceptance Criteria:**
- [ ] An action whose key is a key of svcview gives an error that names the file, the group, the action and the key,
      and says that the key is a key of svcview, and gives no settings. The keys of svcview are each one-character key
      that `Screen.press` handles (`?`, `h`, `j`, `k`, `l`, `o`, `q`, `r`, `s` and `x` in `screen.go:184-245`), and
      `/`.
- [ ] Two actions of one group with the same key give an error that names the file, the group, the second action and
      the key, and gives the line of the first action, and no settings.
- [ ] An action whose key is a key of svcview in upper case, for example `L`, `S` or `X`, and two groups that each
      have an action with the key `L`, give their actions with no error.
- [ ] An e2e test starts svcview with a `config.yaml` whose action has the key `s`, and with one whose group has two
      actions with the key `L`. Each stops svcview with the exit code 1 before the screen opens, with an error that
      names the file and the group.

**Affected Files/Modules:**
- `internal/config/action.go` — the keys of svcview, and the check of the key of an action
- `internal/config/group.go` — no two actions of a group have the same key
- `internal/config/config_test.go` — the tests of the keys, through `config.Read`
- `e2e/tests/actions.test.ts` — new: svcview stops at start

**Patterns to Follow:**
- `internal/config/config.go:80-89`
- `internal/config/group.go:23-37`
- `internal/config/config_test.go:558-571`
- `e2e/tests/config.test.ts:84-100`
- `task:2`

**Testable:** Yes — through `config.Read`, and through the binary in the e2e test.

**Certainty:** high — the refusal of a group name that an earlier entry has (`config.go:80-89`) and of the name
`Other` (`group.go:27-28`) are the precedent.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 4: Show the keys of the actions of the selected group in the help line and in the help

**Behavior:** When the selected service is in a group with actions, the help line shows the key and the name of each
action of that group after `o log` and before `? help`, in the order of the file, for example
` s start · x stop · r restart · o log · L worker log · ? help`. `?` lists each of these actions with its key and its
name, on the lines after `o`. With no selected service, or with a selected service in a group with no actions or in
`Other`, the help line and the help are as before. The keys of the actions do nothing yet; Task 5 gives each one its
command.

**Acceptance Criteria:**
- [ ] With a service of a group with actions selected, the help line under the panes and under the help shows each
      action of the group as its key and its name, after `o log` and before `? help`, in the order of the file, and
      is cut at the width of the terminal as before.
- [ ] The help line follows the selection: it shows the actions of the group of the service that `↓` or `↑` selects,
      and no action with a service of a group with no actions, with a service of `Other`, with no rows, and while the
      list shows a read error.
- [ ] `?` lists each action of the group of the selected service with its key and its name, on the lines after `o`,
      and each key of svcview as before. A new golden file shows this help, in a terminal that is high enough for each
      of its lines.
- [ ] The name of an action shows each control character as `�`, in the help line and in the help.
- [ ] Each golden file that exists in `internal/tui/testdata` keeps its bytes.
- [ ] An e2e test opens svcview with a group that has the action `L` with the name `worker log`: the help line shows
      `L worker log` while a service of the group is selected, and does not after `↓` selects a service of `Other`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the help line and the help
- `internal/tui/screen_test.go` — the tests of the help line and the help with actions
- `internal/tui/testdata/` — a new golden file of the help with actions
- `e2e/tests/actions.test.ts` — the help line

**Patterns to Follow:**
- `internal/tui/screen.go:24`
- `internal/tui/screen.go:428-442`
- `internal/tui/screen.go:481-495`
- `internal/tui/screen_test.go:295-300`
- `internal/tui/screen_test.go:3735-3742`
- `internal/tui/screen_test.go:3793-3807`
- `e2e/tests/log.test.ts:66-79`
- `task:2`

**Testable:** Yes — through the `Screen` model, its golden files, and the e2e test.

**Certainty:** medium — the help line and the help (`screen.go:24`, `screen.go:481-495`) are the precedent, but the
help line is a constant today, and no part of the screen changes with the group of the selected service.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, `git status internal/tui/testdata` shows only the new golden
file, and `task test:e2e` passes.

**Depends on:** Task 3

### Task 5: Give the terminal to the command of an action with its key

**Behavior:** The key of an action, while a service of the group of that action is selected, applies the template
values of that service to the command, and gives the terminal to the command line through `giveTerminalTo`, with no
shell. When the command ends, the screen shows again with the same service selected and the same focus. A command
that cannot start, exits with a code other than 0, or ends by a signal shows why on the message line, with the words
that `o` uses. When a text of the command uses a template value that the plist does not have, svcview runs no
command, and the message line shows the failure, for example `the plist has no WORKER_LOG`. On a service of another
group, or of `Other`, the key does nothing. As `o` does, the key acts in each state of the service, also while a
start, a stop or a restart of the service has not ended; it does nothing while the help shows, with no rows, and
while the list shows a read error; while a confirmation shows, it answers no. A `ctrl-c` that the command gets does
not end svcview.

**Acceptance Criteria:**
- [ ] The key of an action, on a selected service of its group, runs the command line with the template values of
      that service, each text as one argument and with no shell, for a not loaded, a loaded and a running service.
- [ ] Two groups with an action on the same key each run the command of their own group, and the key on a service of
      `Other` runs nothing and shows nothing.
- [ ] An action whose key is a key of svcview in upper case, for example `L`, runs its command, and `l` still moves
      the focus to the detail pane.
- [ ] When the command exits with 0, the screen shows again with the same service selected, the same focus, and no
      text on the message line. When it exits with a code other than 0, ends by a signal, or cannot start, the message
      line says so with the command line, as it does for `o`.
- [ ] When a text of the command uses a template value that the plist does not have, the key runs no command, and the
      message line shows the failure, for example `the plist has no WORKER_LOG`, with each control character as `�`.
      The next key press hides it.
- [ ] The key does nothing while the help shows, with no rows, and while the list shows a read error after a read
      that selected a service. It runs its command while a start, a stop or a restart of the service has not ended.
      While a confirmation shows, it answers no.
- [ ] An e2e test with the action `[less, +F, '{{.Env.WORKER_LOG}}']` on `L`: `L` gives the terminal to the fake
      `less` with the `WORKER_LOG` of the plist of the selected service, and after `q` the screen shows again with
      the same service selected; `ctrl-c` while the fake `less` has the terminal reaches it and does not end svcview;
      `L` on a service whose plist has no `WORKER_LOG` shows `the plist has no WORKER_LOG` on the line above the help
      line and runs no `less`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the key of an action
- `internal/tui/screen_test.go` — the tests of the key of an action
- `e2e/tests/actions.test.ts` — the key of an action through the binary

**Patterns to Follow:**
- `internal/tui/screen.go:222-230`
- `internal/tui/screen.go:317-333`
- `internal/tui/screen.go:387-398`
- `internal/tui/screen_test.go:37-74`
- `internal/tui/screen_test.go:3531-3726`
- `e2e/tests/log.test.ts:17-52`
- `e2e/tests/log.test.ts:66-79`
- `e2e/testUtils.ts:33-58`
- `task:4`

**Testable:** Yes — through the `Screen` model with the fake `less` of `TestMain`, and through the binary in the e2e
test.

**Certainty:** medium — `o` (`screen.go:222-230`) and `giveTerminalTo` (`screen.go:317-321`) are the precedent, but
the key and its command come from the group of the selected row and not from a case of `press`, and the command line
can fail before the program runs.

**Blast radius:** high — the key runs a command that a config file names, with the template values of the selected
service, on one key press: a key that reaches the action of another group, or the values of another service, runs
that command against the wrong service with no question.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 4

### Task 6: Ask for a confirmation before the command of an action with confirm

**Behavior:** The key of an action with `confirm: true` applies the template values first; a failure shows on the
message line, and svcview asks nothing. Else the message line shows `Run <name> for <service>? (y/N)`, for example
`Run worker log for com.example.worker.01? (y/N)`, and svcview runs nothing yet. `y` gives the terminal to the command
line that the key made, for the service that the confirmation names, also when a refresh changed the selected row
while the confirmation showed. Each other key answers no: it hides the confirmation, runs nothing, and does nothing
else. `ctrl+c` quits. The confirmation of `x` and `r` stays as it is: `y` after `x` stops the service, and `y` after
`r` restarts it.

**Acceptance Criteria:**
- [ ] The key of an action with `confirm: true` shows `Run <name> for <full name of the service>? (y/N)` on the
      message line, with each control character of the name of the action and of the service as `�`, cut at the
      width of the terminal with `…`, and runs no command.
- [ ] `y` runs the command line that the key made, one time, with the template values of the service that the
      confirmation names, also when a refresh changed the selected row while the confirmation showed.
- [ ] Each key other than `y` and `ctrl+c` answers no, the key of the action too: it hides the confirmation, runs no
      command, and does nothing else. `ctrl+c` ends the screen.
- [ ] On a service whose plist does not have a template value of the command, the key of an action with
      `confirm: true` shows the failure and no confirmation, and `y` after it runs nothing.
- [ ] `y` after `x` still stops, and `y` after `r` still restarts, the service that the confirmation names, and calls
      nothing else. Each test of the stop and the restart passes with no change to what it asserts.
- [ ] An e2e test with `confirm: true`: `L` shows the confirmation on the line above the help line, `n` hides it and
      runs no `less`, and `L` and then `y` give the terminal to the fake `less`.

**Affected Files/Modules:**
- `internal/tui/screen.go` — the confirmation, what `y` does, and the key of an action with confirm
- `internal/tui/screen_test.go` — the tests of the confirmation of an action
- `e2e/tests/actions.test.ts` — the confirmation through the binary

**Patterns to Follow:**
- `internal/tui/screen.go:56-64`
- `internal/tui/screen.go:209-221`
- `internal/tui/screen.go:247-257`
- `internal/tui/screen_test.go:3114-3308`
- `internal/tui/screen_test.go:3345-3356`
- `task:5`

**Testable:** Yes — through the `Screen` model with the fake `less`, and through the binary in the e2e test.

**Certainty:** medium — the confirmation of `x` and `r` (`screen.go:56-64`, `screen.go:247-257`) is the precedent,
but `y` must now give the terminal to a program in the foreground, where today it always runs an action on launchd in
the background.

**Blast radius:** high — it changes the confirmation that guards the stop and the restart, and the confirmation is
what keeps a command that its author marked with `confirm` from running on a no: a mistake runs a command, a stop or
a restart that the user answered no to.

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 5

### Task 7: Describe the actions of a group in docs/cli.md

**Behavior:** `docs/cli.md` describes the setting `actions` of a group and its four settings, the keys that an action
can take and the keys of svcview that it cannot take, what the key does and when, the template values in the command
of an action, the confirmation, the help line and the help, the texts that an action shows on the message line, the
errors of `actions` that stop svcview at start, and that `svcview status` shows and runs no action.

**Acceptance Criteria:**
- [ ] The table of the settings of a group lists `actions`, and a section on the actions of a group has a table of
      `key`, `name`, `command` and `confirm`, with the example of the proposal: the key `L`, the name `worker log`,
      and the command `[less, +F, '{{.Env.WORKER_LOG}}']`.
- [ ] The section says that a key is one character that prints and that is not a space, that the case counts, which
      keys of svcview an action cannot take, that no two actions of one group can have the same key, and that two
      groups can each have an action with the same key.
- [ ] The section says that the key acts only on a selected service of its group, in each state of the service, also
      while a start, a stop or a restart of the service has not ended, and does nothing while the help shows and while
      the list shows an error; that svcview gives the terminal to the command with no shell, with the environment and
      the working folder of svcview, with no deadline, and shows the screen again when the command ends; and that a
      `ctrl-c` that the command gets does not end svcview.
- [ ] The section of the template values says that a text of the command of an action can use them, and that a value
      that the plist does not have shows on the message line, for example `the plist has no WORKER_LOG`, and runs no
      command.
- [ ] The Keys table, the help line, the confirmation, and the section of the message line describe the keys of the
      actions, the question `Run <name> for <service>? (y/N)`, and the texts of a command that exits with a code
      other than 0, ends by a signal, or cannot start.
- [ ] The section Errors lists each error of `actions`, and says that each one names the file and the group.
- [ ] The text says that `svcview status` shows and runs no action, and does not say that it does not read them.
- [ ] Each link in `docs/cli.md` points to a heading that exists.

**Affected Files/Modules:**
- `docs/cli.md` — the actions of a group, the keys, the help line, the message line, and the errors

**Patterns to Follow:**
- `docs/cli.md:146-160`
- `docs/cli.md:219-253`
- `docs/cli.md:255-277`
- `docs/cli.md:372-401`
- `docs/cli.md:441-478`
- `docs/cli.md:767-802`

**Testable:** No — documentation.

**Certainty:** high — the sections of the log (`docs/cli.md:219-253`) and of the status command
(`docs/cli.md:403-478`) are the precedent.

**Blast radius:** low

**Verification:** Read the changed sections against Tasks 2 to 6, and check each link of `docs/cli.md` by hand.

**Depends on:** Task 3, Task 6

## Summary

- 7 tasks. Tasks 1 to 3 build the config side (the shared texts of a command, the read of `actions`, and the keys
  that an action cannot take), so that the screen gets complete, checked actions. Task 4 is the change of the help
  line, the first task of the screen, as the learnings ask; it rewrites no golden file that exists. Task 5 gives the
  key its command, and Task 6 adds the confirmation on top of it, because the confirmation holds the command line
  that the key makes. Task 7 describes the result.
- Story criteria: the settings of an entry (Task 2), the help line and `?` (Task 4), and the key that acts only on
  its group (Task 5) cover the first criterion. The template values of the selected service (Tasks 1 and 5) and the
  terminal (Task 5) cover the second. The confirmation covers the third (Tasks 2 and 6). The failure of a template
  value covers the fourth (Task 5). The keys that stop svcview at start cover the fifth (Task 3). `docs/cli.md`
  covers the sixth (Task 7). No criterion is deferred.
- Certainty: high for Tasks 1, 3 and 7, medium for Tasks 2, 4, 5 and 6. Blast radius: high for Tasks 5 and 6, low for
  the others.
