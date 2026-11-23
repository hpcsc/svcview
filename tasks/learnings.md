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
- Observed: task 2 — see-the-launchd-services-in-a-list-services-in-launchagents
- Learning: Lip Gloss v2 draws no title in a border, and its Width and Height include the border, so internal/tui/box.go draws each rounded box with its title and cuts each line to fit with …. The view methods of Screen live in screen.go, so internal/tui has no view.go, and go.mod has no Lip Gloss until a task adds colors.
- Apply when: A task in internal/tui adds a pane, a line in the header, a color or a glyph, or names internal/tui/view.go.

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
