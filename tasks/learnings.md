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
- Observed: task 1 — see-the-launchd-services-in-a-list-launchd-state
- Learning: Go unit tests write a shell script with the name of the program and put its folder first on PATH with t.Setenv (fakeLaunchctl and fakeLaunchctlList in internal/launchd/domain_test.go), so a test runs the real exec, exit code, stderr and deadline. e2e tests keep the fake in e2e/fakes, which openCli puts first on PATH, and the fake prints a fixture file in $HOME (launchctl-list.txt). Put a command after a long one in a fake (sleep 60; echo never), because dash runs the last command in place of the shell.
- Apply when: A task runs another program from internal/launchd, for example ps for the uptime, or fakes one in a unit or e2e test.

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
