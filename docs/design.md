# The Design of svcview

This document describes how svcview works inside: its parts, what it reads from launchd, and what happens on each
refresh, status run, and key. It is for a developer who changes svcview. `docs/cli.md` describes the behaviour for a
user, and `docs/proposal-launchd-tui.md` gives the reasons for the design.

## The Parts

svcview is one Go program. It talks to launchd only through the `launchctl` and `ps` programs and the plist files.
It does not use the XPC API of launchd, so the code builds on Linux, and the e2e tests replace `launchctl` and `ps`
with fake scripts.

```mermaid
flowchart LR
    user([Terminal of the user])

    subgraph svcview
        main[cmd/svcview<br/>signals]
        cli[internal/cli<br/>commands and wiring]
        tui[internal/tui<br/>Screen: Bubble Tea model]
        launchd[internal/launchd<br/>Domain]
        status[internal/status<br/>status commands]
        config[internal/config<br/>config files]
        service[internal/service<br/>Service, State, Property]
    end

    launchctl[[launchctl]]
    ps[[ps]]
    plists[("~/Library/LaunchAgents/*.plist")]
    configFiles[("~/.config/svcview<br/>config.yaml, conf.d/*.yaml")]
    statusCmds[[status commands<br/>of the groups]]
    programs[[less, and the commands<br/>of the actions]]

    user <--> tui
    main --> cli
    cli --> config
    cli --> launchd
    cli -- "read, start, stop, restart,<br/>run status as functions" --> tui
    launchd --> launchctl
    launchd --> ps
    launchd --> plists
    config --> configFiles
    status --> statusCmds
    tui -- gives the terminal to --> programs
    cli --> status
    launchd -.-> service
    tui -.-> service
    status -.-> service
```

| Package | Job |
| --- | --- |
| `cmd/svcview` | The `main` package. It ends the context of svcview at `SIGHUP` and `SIGTERM`, and runs the root command. |
| `internal/cli` | The commands. With no command, in a terminal, the root command reads the config, makes a `launchd.Domain`, and opens the screen. It also has `svcview status`, `version`, and `update`. |
| `internal/tui` | `Screen`, the Bubble Tea model, and its views. It does not import `internal/launchd`: `internal/cli` gives it the reads and the actions as functions. |
| `internal/launchd` | `Domain`. It runs `launchctl` and `ps`, reads the plists, and returns `service.Service` values. It is the only package that knows launchd. |
| `internal/status` | Runs a status command for one service, and parses its JSON output. |
| `internal/config` | Reads and checks `config.yaml` and `conf.d/`. |
| `internal/service` | The types `Service`, `State`, `LastExit`, and `Property`, which the other packages share. |
| `internal/terminal` | Replaces the control characters of a text from outside, so that a name or an output cannot change the terminal. |

The rule that only `internal/launchd` knows launchd keeps the other packages free of launchd words. A later version
can add a second service manager beside it.

## What svcview Reads From launchd

`Domain` joins three sources into one list of services:

| Source | What svcview gets | When |
| --- | --- | --- |
| `launchctl list` | The name, the PID, and the last exit of each service that launchd has in `gui/<uid>` | Each read |
| `~/Library/LaunchAgents/*.plist` | The `Label`, the environment, the log paths, and the other properties of each plist | Each read, but svcview parses a plist again only when its modification time changes |
| `ps -o pid=,etime= -p <PID>,<PID>` | The uptime of each running service | Each read, one call for all PIDs |

svcview checks the header line of `launchctl list`. When the header is not `PID`, `Status`, `Label`, svcview shows an
error and reads no row. It does not parse `launchctl print`, because the man page says that its output is not an
API.

The state of a service comes from the two first sources:

```mermaid
flowchart TD
    start{Is the name in<br/>launchctl list?}
    start -- yes --> pid{Does the row<br/>have a PID?}
    pid -- yes --> running([running])
    pid -- no --> loaded([loaded])
    start -- no --> plist{Does a plist in<br/>~/Library/LaunchAgents<br/>have the name?}
    plist -- yes --> notLoaded([not loaded])
    plist -- no --> none([not in the list])
```

`Domain.Services` gives the services that have a plist. `Domain.AllServices` also gives each name of
`launchctl list` that has no plist, for `--all` and the key `a`. A service with no plist has no environment, no log
paths, and no properties.

## The Refresh Cycle

Bubble Tea calls `Screen.Update` for each message, and runs each `tea.Cmd` in a goroutine of its own. `Update` does
no I/O: each read, status run, and action is a `tea.Cmd` that sends a message when it ends.

```mermaid
sequenceDiagram
    participant BT as Bubble Tea
    participant S as Screen
    participant D as launchd.Domain
    participant L as launchctl and ps

    BT->>S: Init
    S-->>BT: readServices (tea.Cmd)
    BT->>D: Services() or AllServices()
    D->>D: read the plists that changed
    D->>L: launchctl list
    D->>L: ps -o pid=,etime= -p ...
    D-->>BT: services
    BT->>S: servicesRead
    S->>S: show the rows, keep the selection
    S-->>BT: tea.Tick(refresh)
    Note over S: the first read also starts the status runs
    BT->>S: refresh (after the refresh setting, default 2s)
    S-->>BT: readServices (tea.Cmd)
```

`Domain` must not read from two goroutines at the same time, so the screen runs one read at a time:

- The next read starts one `refresh` after the last read ends. So two reads never overlap, also when `launchctl`
  is slow.
- `a` starts a read at once when no read runs. When a read runs, its result has the other value of `all`, so the
  screen drops it and starts the read that `a` asks for.
- A count of the reads that started marks each `refresh` tick. A tick from an earlier read does nothing, so `a`
  starts no second cycle.

## The Status Runs

A group with a `status` setting gets one status run for each of its services every `status.every`. The runs start
after the first read ends.

```mermaid
sequenceDiagram
    participant BT as Bubble Tea
    participant S as Screen
    participant R as statusRuns (internal/cli)
    participant C as status.Command
    participant P as status command

    BT->>S: statusTick for a group
    loop each service of the group whose last run ended
        S-->>BT: runStatusOf (tea.Cmd)
        BT->>R: run(command, service)
        R->>C: Run(ctx, service)
        C->>C: apply the template values
        C->>P: start, with no shell, in a process group of its own
        P-->>C: stdout (max 1 MiB), stderr, exit
        C->>C: parse one JSON object of version 1
        C-->>BT: Result
        BT->>S: runEnded
    end
    S-->>BT: tea.Tick(statusTick, every)
```

- **Templates:** `{{.Name}}`, `{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}` come from the
  plist. A value that the plist does not have is a failure, for example `the plist has no WORKER_STATUS_FILE`, and
  svcview runs no command.
- **Timeout:** at `status.timeout`, svcview kills the process group of the run, so a child of the command also ends.
- **Failures:** an exit code other than 0, a timeout, and an output that is not valid each give a `Failure`, which
  the row shows as `status error`, `status timeout`, or `status invalid`.
- **Stale:** the screen compares `at` of the output with `status.stale_after` when it draws, so a status gets stale
  with no new run.
- **Quit:** `statusRuns.stop` ends the context of the runs and waits until each run ends, so no command outlives
  svcview.

## The Actions on launchd

`s`, `x` and `r` call the functions that `internal/cli` gives to the screen. Each function runs one `launchctl`
command through `Domain`:

| Key | State of the service | Command |
| --- | --- | --- |
| `s` | `not loaded` | `launchctl bootstrap gui/<uid> <plist path>` |
| `s` | `loaded` | `launchctl kickstart gui/<uid>/<name>` |
| `x` | `loaded` or `running` | `launchctl bootout gui/<uid>/<name>` |
| `r` | `loaded` or `running` | `launchctl kickstart -k gui/<uid>/<name>` |

```mermaid
sequenceDiagram
    actor U as User
    participant S as Screen
    participant BT as Bubble Tea
    participant D as launchd.Domain
    participant L as launchctl

    U->>S: x
    S->>S: show the confirmation Stop NAME? (y/N)
    U->>S: y
    S-->>BT: the action (tea.Cmd), one at a time for each service
    BT->>D: Stop(ctx, service)
    D->>L: launchctl bootout gui/UID/NAME
    L-->>D: exit code and output
    D-->>BT: error or nil
    BT->>S: launchdActionEnded
    S->>S: an error goes on the message line
    Note over S: the next refresh shows the new state
```

The confirmation keeps the service that was selected when the question showed, and the action that the key asked
for. `y` runs that action, and each other key answers no. A `launchctl` command that does not end in 30 s is killed
with its process group.

## Giving the Terminal to a Program

`o` opens the log in `less +F`, and the key of an action runs the command of the action. Both go through
`Screen.giveTerminalTo`, which uses `tea.ExecProcess`: Bubble Tea gives the terminal to the program, and draws the
screen again when the program ends.

```mermaid
sequenceDiagram
    actor U as User
    participant S as Screen
    participant BT as Bubble Tea
    participant P as less, or the command of an action

    U->>S: o
    S-->>BT: tea.ExecProcess(less +F -- the log path)
    BT->>BT: release the terminal
    BT->>P: start
    U->>P: ctrl-c (stop the follow), then q
    Note over P,S: ctrl-c sends SIGINT to svcview too, which ignores it while the program runs
    P-->>BT: exit
    BT->>BT: take the terminal back
    BT->>S: programEnded
    S->>S: a failure goes on the message line
```

While the program has the terminal, `ctrl-c` sends `SIGINT` to each process in the foreground process group of the
terminal: to the program, and to svcview too. These rules keep svcview open:

| Where | Rule |
| --- | --- |
| `cmd/svcview` | The context of svcview ends only at `SIGHUP` and `SIGTERM`. |
| `showScreen` in `internal/cli` | It catches `SIGINT` while the screen runs. Bubble Tea ends the screen at `SIGINT` only while no program has the terminal. |
| `endAtInterrupt` in `internal/cli` | Each subcommand, for example `svcview status`, still ends at `SIGINT`. |

## The Config

svcview reads the config once, before it opens the screen. A change takes effect at the next start.

```mermaid
flowchart TD
    folder["Config folder:<br/>$XDG_CONFIG_HOME/svcview, or ~/.config/svcview"]
    folder --> main["config.yaml"]
    folder --> confd["conf.d/*.yaml, in byte order of the names"]
    main --> check
    confd --> check
    check{"For each file: a regular file,<br/>owned by the user, no write for others,<br/>no ACL?"}
    check -- no --> stop([svcview stops with an error<br/>that names the file])
    check -- yes --> parse{"Valid YAML, known settings,<br/>valid regular expressions,<br/>templates and keys?"}
    parse -- no --> stop
    parse -- yes --> join["Join the groups in file order.<br/>Group names are unique across all files."]
    join --> settings([config.Settings])
```

A config file holds commands that svcview runs, so svcview refuses a file that another user can write.
`Settings.GroupOf` puts a service into the first group whose `match` matches its name, and into `Other` when no
group matches.

## svcview status

`svcview status <name>` uses the same parts with no screen. It reads the config and `Domain.Services`, finds the
group of the service, runs its status command one time, and prints the template values, the command line, the
output, and the result. It exits with 1 when the result is not valid.
