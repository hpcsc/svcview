# Implementation learnings

## Test a Bubble Tea timer on the fake clock of synctest
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 4 — see-the-launchd-services-in-a-list-services-in-launchagents
- Learning: Run the real tea.NewProgram inside synctest.Test with tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignalHandler() and tea.WithWindowSize; quit it from a goroutine after time.Sleep, then call time.Sleep(time.Minute) and synctest.Wait() so the open tea.Tick timers end. internal/tui/screen_test.go runFor does this, and a refresh of 2 s costs no real time.
- Apply when: A task in internal/tui tests a refresh, a timer, or a command that runs over time, for example the refresh that also runs launchctl list or ps.

## internal/tui draws its boxes in box.go, with no Lip Gloss
- Type: convention
- Recorded: 2026-11-23
- Observed: task 3 — see-the-launchd-services-in-a-list-launchd-state
- Learning: Lip Gloss v2 draws no title in a border, and its Width and Height include the border, so internal/tui/box.go draws each rounded box with its title and cuts each line to fit with …. The view methods of Screen live in screen.go, so internal/tui has no view.go. Colors are SGR styles of github.com/charmbracelet/x/ansi with the 16 ANSI colors (Screen.stateLook in screen.go), so go.mod has no Lip Gloss.
- Apply when: A task in internal/tui adds a pane, a line in the header, a color or a glyph, or names internal/tui/view.go or Lip Gloss.

## Plan each Bubble Tea message and private type in design.yaml
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2,4 — see-the-launchd-services-in-a-list-services-in-launchagents
- Learning: clerk finish refuses each type that the planned design does not name, private types too, and each design note then costs one design-check agent. In internal/tui each result of a tea.Cmd (servicesRead) and each timer (refresh) is a message type, and a file such as plist_folder.go declares a type of its own.
- Apply when: Planning a task that adds a tea.Cmd, a timer, a file in internal/launchd or internal/tui, or a helper type.

## e2e tests redirect stdin or stdout through openCli
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 3 — see-the-launchd-services-in-a-list-services-in-launchagents
- Learning: openCli puts its arguments into the sh -c command line unquoted, so openCli(dir, ['<', '/dev/null']) and openCli(dir, ['>', 'output.txt']) run svcview without a terminal on stdin or stdout. Do not pipe to cat: EXIT:$? then shows the exit code of cat, not of svcview.
- Apply when: An e2e test needs svcview with stdin or stdout that is not a terminal, or reads what svcview printed outside the screen.

## Only e2e tests reach the terminal branch of the root command
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 3 — see-the-launchd-services-in-a-list-services-in-launchagents
- Learning: The Go tests have no terminal, so the branch of internal/cli that opens the screen (showScreenOrHelp, showScreen) and the code that only it calls show as LIVED, NOT COVERED or CALLER GAPS in clerk mutants. The e2e tests in e2e/tests/screen.test.ts kill them; accept each row with the e2e test that fails on it, after you check one of them by hand in Docker.
- Apply when: A task changes the root action, showScreen, or the terminal check in internal/cli, or wires a new launchd or tui type into the screen.

## Run clerk mutants and clerk audit run under mise exec
- Type: constraint
- Recorded: 2026-11-23
- Observed: see-the-launchd-services-in-a-list-launchd-state
- Learning: The shell GOROOT points at Go 1.27.1 and go.mod asks for 1.27.4, so clerk mutants from the bare shell fails with 'compile: version go1.27.1 does not match go tool version go1.27.4'. Run mise exec -- clerk mutants and mise exec -- clerk audit run, as go_tool_prefix says for Go commands.
- Apply when: Running clerk mutants, clerk audit run, gopls or another Go tool in this repo.

## Fake a program that svcview runs with a script first on PATH
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 1 — see-the-launchd-services-in-a-list-detail-pane
- Learning: macOS checks each new executable file at its first run, which costs about 0.3 s, so internal/launchd/domain_test.go writes one fake launchctl and one fake ps in TestMain and each test gives them its output file through FAKE_LAUNCHCTL_LIST, FAKE_PS_OUTPUT and FAKE_PS_CALLS (fakeLaunchctlList, fakePs); only a test that needs another script writes its own with fakeProgram. A fake script for each test made the package tests 27 s in place of 15 s, and clerk mutants then stopped at its limit of 10 minutes. e2e tests keep the fakes in e2e/fakes, which openCli puts first on PATH: launchctl prints $HOME/launchctl-list.txt, and ps prints 03:12:07 for each PID. Put a command after a long one in a fake (sleep 60; echo never), because dash runs the last command in place of the shell.
- Apply when: A task runs another program from internal/launchd, for example launchctl kickstart or bootout for an action, or fakes one in a unit or e2e test.
## Run each outside program with a deadline
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2 — see-the-launchd-services-in-a-list-launchd-state
- Learning: Screen asks for the next refresh only after a read ends, so a program that never exits stops the refresh with no error on the screen. readLaunchctlList runs launchctl with exec.CommandContext, a deadline of 5 s and a WaitDelay of 1 s, and gives an error at the deadline.
- Apply when: A task adds a call to an outside program in the read of the services, for example ps.

## Show no data of a read before the first read ends
- Type: convention
- Recorded: 2026-11-23
- Observed: task 4 — see-the-launchd-services-in-a-list-launchd-state
- Learning: Screen.lastRead is the zero servicesRead until the first read ends, and Screen.hasRead says whether one ended. A pane or a line that shows what a read gave must check hasRead, or it shows zeros or empty values as if a read happened (the header counts did until an audit found it).
- Apply when: A task in internal/tui adds a pane or a line that shows data from the read, for example the detail pane.

## A value that can be 0 and can be missing is a pointer
- Type: convention
- Recorded: 2026-11-23
- Observed: task 1 — see-the-launchd-services-in-a-list-detail-pane
- Learning: service.Service.Uptime is *time.Duration: nil is no uptime, and a pointer to 0 is a process that ps reports at 00:00, which the detail pane shows as up 0s. An audit found an uptime of 0 shown as no uptime, and a service that launchd starts again after each crash often has that uptime. PID stays an int, because launchctl never shows a PID of 0 for a service.
- Apply when: A task adds a field to service.Service or to a result type that can be missing and whose zero is a real value, for example the age of a status or the time of an event.

## Each key of the screen goes into the help and docs/cli.md
- Type: convention
- Recorded: 2026-11-23
- Observed: task 4 — see-the-launchd-services-in-a-list-detail-pane
- Learning: The story says that ? shows all keys, so Screen.helpText must list every key that Screen.Update handles, ctrl+c too, and the Keys table of docs/cli.md must match it. The match-request step found ctrl+c missing from the help after the audit; the golden file keys.golden shows the help.
- Apply when: A task adds or changes a key of the screen, for example s, x, r or l for an action, or plans the criteria of a story that says that ? shows all keys.

## Plan the layout change of the screen as the first task
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2,3,4 — see-the-launchd-services-in-a-list-detail-pane
- Learning: Each change to the layout of the screen (a pane, a line, a height) rewrites every golden file in internal/tui/testdata and the same lines of screen.go and screen_test.go, so an audit fix to the code of an earlier task cannot fold into its commit: clerk fixup mark needs --onto, and the fix of a task before the layout change becomes a commit of its own. In detail-pane, the selection (task 2) came before the detail pane and the help line (tasks 3 and 4), and two fixes of the selection became their own commits.
- Apply when: Planning a deliverable with more than one task in internal/tui, where one task adds a pane, a line or a view to the screen.

## A check that another user can write a file must read the macOS ACL
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2 — put-the-services-in-groups-with-a-config-file-config-file
- Learning: The mode bits and the owner do not show a macOS ACL: chmod +a "everyone allow write" leaves the mode 0644, and internal/config.Read still reads the file. On Linux the mode check covers a POSIX ACL, because a write entry for another user sets the group write bit through the ACL mask. x/sys/unix has no Getattrlist on darwin, so the check needs a raw getattrlist call or a parse of ls -le, and only a Mac can test it.
- Apply when: Planning US-003 or any task that refuses a config file, or any other file, that another user can write, before a setting of the config can hold a command.

## A word of the subtest name is in every t.TempDir path
- Type: recurring-finding
- Recorded: 2026-11-23
- Observed: task 1 — put-the-services-in-groups-with-a-config-file-config-file
- Learning: Go names the folder of t.TempDir after the test, so require.ErrorContains(t, err, "refresh") passes in a subtest whose name holds refresh when the error names the path. Check a text that the path cannot hold, for example the setting in quotes (`"refresh"`).
- Apply when: A Go test checks that an error, which also names a file in t.TempDir, holds a word that the name of the test holds too.

## Join the lines of the screen before an e2e test checks a long error
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 3 — put-the-services-in-groups-with-a-config-file-config-file
- Learning: openCli gives svcview 160 columns, and an error that names a path under the macOS TMPDIR continues on the next line, so toContain(path) fails on a Mac and passes in Docker. Check (await session.text()).replaceAll('\n', ''), as e2e/tests/config.test.ts does.
- Apply when: An e2e test checks the text of an error that svcview prints and that names a file in a scratch folder.

## internal/config is the precedent for a setting and for the check of the config file
- Type: convention
- Recorded: 2026-11-23
- Observed: task 1,2 — put-the-services-in-groups-with-a-config-file-config-file
- Learning: A task that adds a setting reads it in (*Settings).set of internal/config/config.go from yaml.Node values, with an error that gives the line and the setting in quotes, and tests it with a config.yaml in t.TempDir. The check of the mode and the owner with fstat on the open file is in config.Read. Tasks of this kind were planned as low certainty and went green on the first try.
- Apply when: Planning or assessing a task that adds a setting to config.yaml, for example groups, status or actions, or that changes the check of the config file.

## A fix whose fold conflicts gets a commit of its own after the last task that changed its lines
- Type: convention
- Recorded: 2026-11-23
- Observed: task 1,2 — put-the-services-in-groups-with-a-config-file-config-file
- Learning: When a later task rewrites the lines of an earlier one, as task 2 rewrote config.Read and its tests, clerk fixup replay of an audit fix into the earlier task conflicts. Do not fold the fix into the later task: commit it on its own at the end, with a message that names the commit it fixes, so git blame gives the right reason.
- Apply when: clerk fixup replay reports a conflict, or a breakdown has two tasks that change the same new function and its tests.

## Pass --integrate to clerk land when the request asks to land on main
- Type: constraint
- Recorded: 2026-11-23
- Observed: put-the-services-in-groups-with-a-config-file-config-file
- Learning: Bare clerk land reads only the integrate setting of the repo (integrate_source default), not the --integrate of the request, so it archives the breakdown and does not integrate. The run is then at the learn step, and clerk land refuses again; the branch then needs git merge --ff-only in the main checkout.
- Apply when: A run whose request says --integrate, or land it on main, reaches the land step.

## The list draws and counts its lines through linesAbove and row.compare
- Type: convention
- Recorded: 2026-11-23
- Observed: task 2,3 — put-the-services-in-groups-with-a-config-file-groups
- Learning: Screen sorts the rows of a read with row.compare: the index of the group in groupOrder, then the name. selectedAfterRead finds the place of a gone name with the same compare, and lineOf counts the lines that linesAbove gives above each row, as listText draws them, so the scroll and the list cannot disagree.
- Apply when: A task in internal/tui adds a line to the list, a filter of the list, a fold of a group, or changes the order of the rows or what a row shows.

## Search the learnings for the run procedure too
- Type: pattern
- Recorded: 2026-11-23
- Observed: put-the-services-in-groups-with-a-config-file-groups
- Learning: The entries about the procedure of a run (clerk mutants under mise exec, clerk land --integrate, clerk fixup) name no package of a story, so clerk learn index --match with only the nouns of the story misses them, and the groups run repeated both problems. Add the terms clerk mise mutants land integrate fixup to the --match of the ground step.
- Apply when: The ground step of any run reads clerk learn index --match.

## Record the design note of an export for tests in the task that adds it
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 1,4 — put-the-services-in-groups-with-a-config-file-groups
- Learning: clerk finish counts only callers outside test files when it checks an exported name, and it checks only at the last task, so config.NewGroup, which only the tui tests call, refused the docs task at the end. Record clerk design note for such an export in the task that adds it, or plan the constructor in design.yaml.
- Apply when: A task exports a constructor or a function that only the tests of another package call, for example to build config.Settings with groups in the tui tests.

## Run gofmt before clerk finish
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 1,2 — put-the-services-in-groups-with-a-config-file-groups
- Learning: clerk finish lints the staged files but does not run gofmt, and CI has no format check, so a field rename by a script committed a struct with unaligned fields in two tasks. Run gofmt -l internal before clerk finish and before clerk fixup mark.
- Apply when: A task edits Go files with a script, or renames or adds a field of a struct whose fields are aligned.
