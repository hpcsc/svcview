# Implementation learnings

## Test a Bubble Tea timer on the fake clock of synctest
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 4 — see-the-status-that-each-service-reports-status-summary
- Learning: Run the real tea.NewProgram inside synctest.Test with tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignalHandler() and tea.WithWindowSize; quit it from a goroutine after time.Sleep, then call time.Sleep(time.Minute) and synctest.Wait() so the open tea.Tick timers end. internal/tui/screen_test.go runFor and runSized do this, and a refresh of 2 s costs no real time. A fake read or run that sleeps in the bubble must end within that minute: a goroutine still asleep when the main goroutine of the bubble returns makes synctest panic with 'main bubble goroutine has exited but blocked goroutines remain'.
- Apply when: A task in internal/tui tests a refresh, a timer, or a command that runs over time, for example the refresh that also runs launchctl list or ps, or a fake status run that takes time.
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
- Observed: see-the-status-that-each-service-reports-status-failures
- Learning: The shell GOROOT points at Go 1.27.1 and go.mod asks for 1.27.4, so clerk mutants from the bare shell fails with 'compile: version go1.27.1 does not match go tool version go1.27.4'. Run mise exec -- clerk mutants and mise exec -- clerk audit run, as go_tool_prefix says for Go commands. An audit round started from the bare shell gets agents that cannot build Go: round 1 of status-failures lost its mutants lens and each runtime check.
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
- Observed: task 1,2 — see-the-status-that-each-service-reports-status-settings
- Learning: The mode bits and the owner do not show a macOS ACL: chmod +a "everyone allow write" leaves the mode 0644. config.ACLEntries(file) counts the ACL entries of an open file (fgetattrlist through unix.Syscall6 on darwin, 0 on other systems, where a write entry of a POSIX ACL shows in the group bits of the mode), and config.Read refuses a count of 1 or more after the check of the mode and the owner. A check of another file calls config.ACLEntries on the open file and does not write a new read of the ACL.
- Apply when: A task refuses a file that another user can write, for example a file of conf.d or a status file, or changes the check of config.yaml.
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

## A change to the template values changes the check of status.NewCommand
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 3 — see-the-status-that-each-service-reports-status-settings
- Learning: status.NewCommand checks each text of a status command with a walk of its parse tree, because an execution visits only the branches that it takes. The walk knows where . is the service: it refuses a field without $ in a with, a range (each value is text), define and template, a field of a declared variable, and a new value for $. Two audit rounds found texts that passed an earlier walk and failed at run time. A new template value, or a value that is not text, changes valuesWithoutKey, otherFields and the struct that Run executes against in one task, with a test that runs the text.
- Apply when: A task adds or changes a template value of a status command, adds a function to its templates, or adds the struct that status.Command.Run executes the templates against.

## Check by hand a mutant that only the tests of another package kill
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 3 — see-the-status-that-each-service-reports-status-settings
- Learning: clerk mutants and clerk mutants --id run only the tests of the package of a mutant, so a mutant in internal/status that only a test of internal/config kills (the drop of the templates of status.Command) stays LIVED. Apply the mutant to the file by hand with a copy in the scratchpad, run the test of the other package, put the file back with /bin/cp -f, and accept the row with the name of that test and its failure text.
- Apply when: A mutant lives in a package whose result only the tests of another package can see, for example a private field of status.Command that only config tests compare.

## An ACL test removes the ACL before the cleanup of its folder
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 1 — see-the-status-that-each-service-reports-status-settings
- Learning: An ACL entry that denies delete, such as /bin/chmod +a "everyone deny delete", stops the cleanup of t.TempDir, so the test fails after its assertions pass and the folder stays in TMPDIR. The chmod helper of internal/config/acl_test.go registers /bin/chmod -h -N on the path in t.Cleanup before it adds the entry. A leftover folder needs /bin/chmod -h -N on each file before rm.
- Apply when: A Go or e2e test adds an ACL entry with /bin/chmod +a, or removes the folder of a file with an ACL.

## task test:e2e:local leaves e2e/package-lock.json in the tree
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2 — see-the-status-that-each-service-reports-status-settings
- Learning: task test:e2e:local runs npm install in e2e/, which writes e2e/package-lock.json, and the repo does not track that file, although docs/e2e-tests.md tells to commit it. The file makes the tree dirty, so clerk refuses the next step and a commit can take it by mistake. Remove it after each local e2e run until the repo decides to commit it.
- Apply when: A run uses task test:e2e:local, or git status shows e2e/package-lock.json as untracked.

## A macOS system call that the planner probed is not low certainty
- Type: convention
- Recorded: 2026-11-23
- Observed: task 1 — see-the-status-that-each-service-reports-status-settings
- Learning: The ACL read of status-settings (fgetattrlist through unix.Syscall6, the first raw system call and the first file with a build constraint of the repo) was planned low and went green on the first try, because the planning probe gave the layout of the answer. The work was in the tests: real ACLs with /bin/chmod, their cleanup, and a parse function tested with bytes in a _darwin_test.go file. Plan such a task medium when Codebase Context has a probe of the call.
- Apply when: Planning or assessing a task that calls the kernel through golang.org/x/sys/unix or adds a file with a build constraint, with a probe of the call in Codebase Context.

## An audit fix that adds a file is a commit of its own
- Type: constraint
- Recorded: 2026-11-23
- Observed: see-the-status-that-each-service-reports-status-summary
- Learning: clerk fixup mark refuses a file that no commit of the branch touches, also with --onto, so an audit fix that adds a file (a new type, or a _test.go for a type whose tests were under another umbrella) becomes a commit of its own. Put each changed file that needs the new file into that commit, and fold only the changes that compile without it: in status-summary, root.go needed status_runs.go, and a fold of root.go alone would have broken the build of the Task 5 commit.
- Apply when: An audit fix adds a file, for example a new private type that a fix needs, or a test file for the umbrella test of a type.

## Bubble Tea does not wait for the goroutines of its commands
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 5 — see-the-status-that-each-service-reports-status-summary
- Learning: After Program.Run returns, the goroutines of the tea.Cmd values that the screen started still run, and can still call the functions that the screen got. So a sync.WaitGroup.Add in such a function can race with the Wait after Run, which the audit showed with -race. Code after Run that waits for that work must refuse new work under a lock before it waits, as internal/cli/status_runs.go does.
- Apply when: A task in internal/cli or internal/tui waits after Program.Run for work that a tea.Cmd starts, for example an action of US-008 that runs launchctl, or the stop of the status runs.

## Prefix the constants of an enum in internal/status with its type
- Type: convention
- Recorded: 2026-11-23
- Observed: task 2 — see-the-status-that-each-service-reports-status-summary
- Learning: In package status a bare constant reads with the package name: status.Error read as the failure 'status error' of the story, so the levels are LevelInfo, LevelWarn and LevelError. Name each constant of a new enum in internal/status with its type as the prefix.
- Apply when: A task adds an enum or exported constants to internal/status, for example FailureKind of status-failures.

## Stop the process group of a program with context.AfterFunc, not with the Cancel of exec.CommandContext
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 2 — see-the-status-that-each-service-reports-status-failures
- Learning: os/exec calls Cancel only while Wait waits for the process. After the program exits, a child that keeps stdout open keeps Wait alive until WaitDelay, past the timeout, and nothing stops the child. status.Command.runProgram starts the program with Setpgid, stops its process group in a context.AfterFunc of the context of the run, and stops the group again after Wait.
- Apply when: A task runs a program with a timeout or must stop a program and each process that it starts, for example an action of US-008 that runs a command, or a change to status.Command.Run.

## A program in a process group of its own gets no SIGHUP from the terminal
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 2 — see-the-status-that-each-service-reports-status-failures
- Learning: The terminal sends SIGHUP only to its foreground process group, and Go ends svcview at a SIGHUP that it does not catch. So cmd/svcview/main.go catches SIGHUP and SIGTERM with signal.NotifyContext, and the end of the context stops the screen and the status runs.
- Apply when: A task starts a program with Setpgid or Setsid, or changes the signals that cmd/svcview/main.go catches.

## Stage a new file before clerk fixup mark to fold it into a commit
- Type: pattern
- Recorded: 2026-11-23
- Observed: see-the-status-that-each-service-reports-status-failures
- Learning: clerk fixup mark refuses a file that no commit of the branch touches, also with --onto. A new file that an audit fix splits out of a file of the branch goes into the fixup when it is staged before clerk fixup mark of the touched files, as signal_name.go went into the commit that added service.SignalName.
- Apply when: An audit fix moves code to a new file, or clerk fixup mark refuses a path with the reason untouched.

## An e2e test gets the PID of svcview as the PPID of a status command
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 2 — see-the-status-that-each-service-reports-status-failures
- Learning: svcview starts a status command with no shell, so $PPID of sh -c in the command is the PID of svcview. e2e/tests/status.test.ts writes it to a file, reads it, and sends SIGHUP or SIGTERM to svcview with process.kill.
- Apply when: An e2e test must send a signal to svcview, or must find the PID of svcview while the screen is open.

## A boundary that turns off a check must name the code that has the check today
- Type: convention
- Recorded: 2026-11-23
- Observed: see-the-status-that-each-service-reports-status-failures
- Learning: The boundary of status-failures said that a progress outside 0 to 1 does not make the output invalid, but parseOutput of status-summary refused it. The run had to remove that check with no criterion for it, and the proposal and US-004 still ask for it. Plan the removal as a criterion of a task, and tell the story that owns the rule.
- Apply when: Planning a deliverable whose boundaries say that a rule, a check or a validation does not apply, in a package that an earlier deliverable changed.

## clerk finish cannot commit the deletion of a file
- Type: constraint
- Recorded: 2026-11-23
- Observed: task 4 — see-the-fields-and-the-events-of-a-status-fields-and-events
- Learning: clerk finish refuses a path that does not exist, and it refuses a deletion staged with git rm as another-task-still-staged when an earlier task recorded the file. Task 4 of fields-and-events planned to replace internal/tui/testdata/status-fields.golden with status-fields-events.golden; it kept the old golden file and its test, and added the new one. Keep the file, or delete it in a commit of its own outside clerk finish and say why in the message.
- Apply when: A task deletes or renames a file, for example a golden file in internal/tui/testdata.

## A test of the local time zone sets time.Local to a fixed zone that is not UTC
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 4 — see-the-fields-and-the-events-of-a-status-fields-and-events
- Learning: CI runs task test:unit on ubuntu-latest, where the zone is UTC, so a test that builds its time in time.Local cannot catch a time that shows in UTC in place of the local zone; an audit found this in fields-and-events. The test of an event in another zone in internal/tui/screen_test.go sets time.Local = time.FixedZone("UTC+5:30", ...) and restores it with t.Cleanup; no test in internal/tui runs in parallel, so this is safe. The e2e Docker image has zoneinfo files, so an e2e test can give svcview a TZ such as Asia/Kathmandu too.
- Apply when: A unit or e2e test checks a time of day that svcview shows in the local time zone, for example the time of an event.

## Split an audit fix across earlier commits with git commit-tree, and keep the tip tree the same
- Type: pattern
- Recorded: 2026-11-23
- Observed: task 2,3,4 — see-the-fields-and-the-events-of-a-status-fields-and-events
- Learning: clerk fixup mark folds a change made at HEAD into one commit. In fields-and-events the audit fix (a test of a task 2 criterion, and a rename of a golden subtest of tasks 2 and 3) went into the task 4 commit, and run-verifier reported tasks 2 and 3 as scattered. The split: write the file as each earlier commit must have it, build each commit with git commit-tree from a temporary GIT_INDEX_FILE with the old message and author, rebuild the later commits on their old trees, check that git diff of the old and the new tip is empty, run the tests at each new commit in a temporary worktree, and move the branch with git update-ref. The suite receipt stays valid, because it is bound to the tip tree. Run the script with bash: zsh here has an alias mk.
- Apply when: An audit fix changes lines that two or more task commits added, or run-verifier reports a task whose work is in a later commit, on a branch that is not pushed.

## Match the learnings index with the clerk commands as well as the story
- Type: constraint
- Recorded: 2026-11-23
- Observed: see-the-fields-and-the-events-of-a-status-fields-and-events
- Learning: clerk learn index --match with only the packages and nouns of the story does not list the entries about clerk itself. fields-and-events missed 'Run clerk mutants and clerk audit run under mise exec' and 'Pass --integrate to clerk land when the request asks to land on main', and repeated both: its audit lost the mutants lens, and bare clerk land archived the breakdown without the integration. Add clerk, mutants, audit, land, finish and fixup to the terms of the first read.
- Apply when: The ground step of a run in this repo reads the learnings index.
