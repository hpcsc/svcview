# Proposal: A launchd TUI That Shows What Each Service Reports About Itself

svcview is a terminal UI for the launchd services of the user on macOS. It shows what launchd knows about each
service. It also runs a status command from its config to show what the service reports about itself.

## The Problem

launchd knows about the process of a service. It does not know what the program in that process does.

| Question | launchd can answer |
| --- | --- |
| Is the service loaded? Is its process alive? What is its PID? | Yes |
| What was the exit code of the last run? | Yes |
| Is the program busy, idle, or paused? | No |
| Which job does it do now, and for how long? | No |
| What did it do in the last hour? | No |

For example, a worker claims jobs from a queue. launchd starts the worker at login, and starts it again after
each exit. When `launchctl list` shows a PID, the worker can be busy, idle, or paused until a usage limit
resets. To see which, a person reads the log file of the worker.

These launchd TUIs exist:

- [na2mene/lazylaunchd](https://github.com/na2mene/lazylaunchd) (Go)
- [JeremieAlcaraz/launch-tui-macos](https://github.com/JeremieAlcaraz/launch-tui-macos) (Go)
- [wecraftforfun/launch-tui](https://github.com/wecraftforfun/launch-tui) (Go)
- [macournoyer/launchtui](https://github.com/macournoyer/launchtui) (Rust)
- [biquando/ltui](https://github.com/biquando/ltui) (Rust)
- [mach-kernel/launchk](https://github.com/mach-kernel/launchk) (Rust)
- [cassiomc1/lazylaunchd](https://github.com/cassiomc1/lazylaunchd)

From their descriptions, each one shows only the data of launchd. None of them runs a command to get the status
that a service reports. On 2026-11-23, the three Go tools had 0, 0, and 24 stars on GitHub. Thus svcview starts as
a new tool, and not as a change to one of them.

## Goals

- **G1:** List the launchd services of the user, with the state, the PID, and the last exit of each service.
- **G2:** Show the status that each service reports, through a status command from the config.
- **G3:** Start, stop, and restart a service, and show its log.
- **G4:** Let another program add its services to svcview with one config file, and no change to svcview.
- **G5:** Install svcview on a Mac with the install script, and stop with a clear message on other systems.

## Not in Scope

- systemd and the other service managers. The names in the config and in the status contract do not say launchd,
  so a later version can add them. See [Names That Do Not Say launchd](#names-that-do-not-say-launchd).
- The system domain and the services in `/Library/LaunchDaemons`. They need root.
- Install, edit, or delete a plist. The program that owns the service does that.
- Run history. launchd keeps no history, and svcview keeps none.
- Releases for Linux and Windows.

## Words in This Document

| Word | Meaning |
| --- | --- |
| service | A launchd job in the `gui/<uid>` domain of the user. |
| name | The launchd label of a service, for example `com.example.worker.01`. |
| state | What launchd says about the service: `running`, `loaded`, or `not loaded`. |
| last exit | The exit code of the last run of the service, or the signal that ended it. |
| uptime | The time since the process of the service started. |
| status | What a service reports about itself, through its status command. |
| group | A set of services from the config that share a status command and actions. |
| key | A key of the keyboard. |
| setting | An entry in a config file. |
| property | An entry in a plist or in the JSON output of a status command. |

## The Screen

```
╭ svcview ─────────────────────────────────────────── gui/501 ╮
│  ● 2 running   ◐ 0 loaded   ○ 1 not loaded                  │
╰─────────────────────────────────────────────────────────────╯
╭ Services ────────────────────╮╭ com.example.worker.01 ──────╮
│ Workers                      ││ ● running     PID 4122      │
│ ▌● 01   busy · ENG-692 · 12m ││ up 3h12m      last exit 0   │
│  ● 02   idle · poll in 21s   ││                             │
│                              ││ Status               1s ago │
│ Other                        ││ mode    busy                │
│  ○ homebrew.mxcl.postgres…   ││ ticket  ENG-692             │
│                              ││ step    review · 12m / 20m  │
│                              ││         ▰▰▰▰▰▰▱▱▱▱          │
│                              ││ done    14                  │
│                              ││ failed  1                   │
│                              ││                             │
│                              ││ Events                      │
│                              ││ 09:04 ● step review         │
│                              ││ 09:00 ● claimed ENG-692     │
╰──────────────────────────────╯╰─────────────────────────────╯
 s start · x stop · r restart · o log · L worker log · ? help
```

The screen has three parts:

- **The header:** the domain, and the number of services in each state.
- **The list:** one row for each service, in groups. The groups come in the order of the config. The group `Other`
  comes last and holds each service that no group matches. In a group, svcview sorts the services by name.
- **The detail pane:** the selected service, with the data from launchd, the fields of its status, and its events.

A row shows the glyph of the state, the short name, and the `summary` of the status. The short name is the part of
the name after the text that `match` matched, when the match starts at the first character. For example,
`^com\.example\.worker\.` gives `01` for `com.example.worker.01`. The group `Other` shows the full name.
svcview cuts a long name with `…`. The title of the detail pane always shows the full name.

The list takes 40 percent of the width. When the terminal has fewer than 60 columns, svcview shows one pane at a
time. `enter` or `l` opens the detail pane, and `esc` or `h` goes back to the list.

When `launchctl` exits with an error, a message line above the help line shows its output.

### Colors and Glyphs

| Element | Look |
| --- | --- |
| The state `running` | A green `●` |
| The state `loaded` | A yellow `◐` |
| The state `not loaded` | A gray `○` |
| A last exit other than 0 | The glyph of the state and the `last exit` value are red. Thus a service that launchd starts again after each crash is red in the list. |
| The `level` values `info`, `warn`, and `error` | The default color, yellow, and red, for the summary and for the `●` of an event |
| A stale status | The summary is dim, with `stale 3m` after it. |
| A field with `progress` | A bar of 10 cells under the value: `▰` in the accent color for the done part, and a dim `▱` for the rest |
| The selected row | A `▌` in the accent color, and a background color |
| The focused pane | The border has the accent color. The border of the other pane is dim. |

svcview uses the 16 colors of the ANSI palette, so the theme of the terminal sets the real colors. The accent color
is blue. When `NO_COLOR` is set, svcview uses no color. Each state has a glyph of a different shape, so the screen
shows the state without color too.

### Keys

| Key | Action |
| --- | --- |
| `↑` `↓`, or `k` `j` | Select a service. When the detail pane has the focus, scroll it. |
| `tab` | Move the focus to the other pane. |
| `l` | Move the focus to the detail pane. When svcview shows one pane at a time, open the detail pane. |
| `h` | Move the focus to the list. When svcview shows one pane at a time, go back to the list. |
| `enter` | Open the detail pane, when svcview shows one pane at a time. |
| `/` | Filter the list by name. |
| `s` | Start the service. |
| `x` | Stop the service. svcview asks for a confirmation. |
| `r` | Restart the service. svcview asks for a confirmation. |
| `o` | Show the log of the service. |
| `?` | Show all keys. |
| `q` | Quit. |

`h`, `j`, `k`, and `l` move as in vim, so a person who uses vim does not move a hand to the arrow keys.

The help line shows the keys of the actions. A group can add more keys. See
[Actions of a Group](#actions-of-a-group).

## Data From launchd

svcview runs `launchctl` and reads the plist files. It does not use the XPC API of launchd. Therefore the code
builds on Linux, and the e2e tests can use a fake `launchctl`.

| Data | Source |
| --- | --- |
| The names, the PIDs, and the last exits | `launchctl list`, which prints three columns: `PID`, `Status`, and `Label` |
| The plist path for each name | svcview reads each `*.plist` in `~/Library/LaunchAgents` and gets its `Label` property |
| The program, the environment, and the log paths | The properties `ProgramArguments`, `EnvironmentVariables`, `StandardOutPath`, and `StandardErrorPath` of the plist |
| The uptime of each process | `ps -o pid=,etime= -p <PID>,<PID>`, one call for all the PIDs at each refresh |

svcview gets the state of a service from these two sources:

| State | Rule |
| --- | --- |
| `running` | `launchctl list` shows the name with a PID. |
| `loaded` | `launchctl list` shows the name without a PID. |
| `not loaded` | A plist in `~/Library/LaunchAgents` has the name, but `launchctl list` does not show it. |

svcview runs `launchctl list` at each refresh, by default every 2 s. It reads a plist again only when the
modification time of the file changes.

By default, svcview shows only the services that have a plist in `~/Library/LaunchAgents`. With `--all`, it shows
each name that `launchctl list` prints, which includes the services of macOS. A service without a plist in that
folder has no environment and no log paths.

svcview does not parse `launchctl print`. The man page of `launchctl` says that the output of `print` is not an
API. svcview checks the header line of `launchctl list`. When the header is not `PID`, `Status`, `Label`,
svcview shows an error and does not try to read the rows.

### Actions on launchd

| Action | Command | Note |
| --- | --- | --- |
| Start a service that is `not loaded` | `launchctl bootstrap gui/<uid> <plist path>` | |
| Start a service that is `loaded` | `launchctl kickstart gui/<uid>/<name>` | |
| Stop | `launchctl bootout gui/<uid>/<name>` | The service stays stopped until the next login, or until a start in svcview. |
| Restart | `launchctl kickstart -k gui/<uid>/<name>` | Only for a service that is `loaded` or `running`. |
| Log | `less +F <StandardOutPath>` | svcview gives the terminal to `less`, and shows the screen again when `less` ends. When the plist has no `StandardOutPath`, svcview uses `StandardErrorPath`. |

Stop uses `bootout`, not `launchctl kill`. launchd starts a service with `KeepAlive` again after a kill, so a
kill does not stop it. `bootout` removes the service from launchd until the next login.

## The Status Command

A group in the config can have a status command. svcview runs the command for each service of the group, and
reads one JSON object from its stdout.

### Settings of a Group

```yaml
# ~/.config/svcview/conf.d/worker.yaml
groups:
  - name: Workers
    match: '^com\.example\.worker\.'
    status:
      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
      every: 2s
      timeout: 1s
      stale_after: 30s
    actions:
      - key: L
        name: worker log
        command: [less, +F, '{{.Env.WORKER_LOG}}']
```

| Setting | Meaning | Default |
| --- | --- | --- |
| `name` | The name of the group on the screen. It must be unique across all config files. | Required |
| `match` | A Go regular expression. A service is in the group when its name matches it. | Required |
| `status.command` | The program and its arguments. svcview does not use a shell. For a shell, write `[sh, -c, '...']`. | No status command |
| `status.every` | The time between two runs of the command. | `2s` |
| `status.timeout` | The time after which svcview kills the command. | `1s` |
| `status.stale_after` | The age of `at` after which svcview marks the status as stale. | No check |
| `actions` | More keys for the services of the group. | None |

### Template Values

svcview applies Go `text/template` to each argument of a command. The values come from the plist of the service.

| Value | Content |
| --- | --- |
| `{{.Name}}` | The name of the service, that is its launchd label. |
| `{{.Env.X}}` | The value of `X` in `EnvironmentVariables`. |
| `{{.StdoutPath}}` | `StandardOutPath` |
| `{{.StderrPath}}` | `StandardErrorPath` |
| `{{.PlistPath}}` | The path of the plist file. |

A value that the plist does not have is an error, not an empty string. For example, when the plist has no
`WORKER_STATUS_FILE`, the status of the service shows `the plist has no WORKER_STATUS_FILE`.

### Output

```json
{
  "version": 1,
  "at": "2026-11-23T09:12:00Z",
  "summary": "busy · ENG-692 · 12m",
  "level": "info",
  "fields": [
    {"name": "mode", "value": "busy"},
    {"name": "ticket", "value": "ENG-692"},
    {"name": "step", "value": "review · 12m / 20m", "progress": 0.6},
    {"name": "done", "value": "14"},
    {"name": "failed", "value": "1"}
  ],
  "events": [
    {"at": "2026-11-23T09:04:40Z", "level": "info", "text": "step review"},
    {"at": "2026-11-23T09:00:12Z", "level": "info", "text": "claimed ENG-692"}
  ]
}
```

| Property | Need | Meaning |
| --- | --- | --- |
| `version` | Required | The version of the contract. This version of svcview accepts only `1`. |
| `at` | Optional | The time of the data, in RFC 3339. The detail pane shows its age, and svcview compares it with `stale_after`. Without `at`, the detail pane shows the age of the last run. |
| `summary` | Optional | One line for the row of the service in the list. |
| `level` | Optional | `info`, `warn`, or `error`. It sets the color of `summary`. Default: `info`. |
| `fields` | Optional | Pairs of `name` and `value` for the detail pane, in this order. A field can also have `progress`, a number from 0 to 1, which svcview shows as a bar. A `progress` outside 0 to 1 makes the output invalid. |
| `events` | Optional | Lines for the detail pane, in this order. Each event has `at`, `level`, and `text`. |

svcview ignores a property that it does not know. Therefore a service can add a property before svcview uses it.

svcview does not know what a field means. When a service adds a field, svcview shows it with no change to svcview.

### Failures

| Failure | The row in the list shows | The detail pane shows |
| --- | --- | --- |
| The command exits with a code other than 0. | `status error`, in red | The exit code and the last 20 lines of stderr |
| The command runs longer than `timeout`. | `status timeout`, in red | The command line |
| The output is not a valid JSON object of version 1. | `status invalid`, in red | The parse error and the first 20 lines of the output |
| `at` is older than `stale_after`. | The summary, dim, with `stale 3m` after it | The age of `at` |

svcview runs one status command at a time for each service. When a run does not end before the next `every`,
svcview skips that run. On a timeout, svcview kills the process group of the command. svcview reads a maximum
of 1 MiB from stdout.

svcview runs the status command in each state of the service. The status of a service that is `not loaded` can be
old, and `stale_after` shows that.

### A Fast Status Command

svcview runs the status command every few seconds for each service, so the command must be fast. A program that
starts a large runtime for each run, for example a BEAM VM or a JVM, is too slow. In that case:

1. Make the service write its status to a file at a fixed interval.
2. Use `cat` as the status command.

The service must write the file in one step. It writes to a temporary file in the same folder, and then renames
it. Thus `cat` never reads half a file.

### Actions of a Group

A group can add keys to the screen.

| Setting | Meaning |
| --- | --- |
| `key` | One key. It must not be a key of svcview, and it must be unique in the group. |
| `name` | The text in the help line. |
| `command` | The program and its arguments, with the template values. |
| `confirm` | When `true`, svcview asks for a confirmation first. Default: `false`. |

svcview gives the terminal to the command, as for the log action. It shows the screen again when the command ends.

### `svcview status <name>`

This subcommand runs the status command of one service one time. It prints the template values, the command line,
the output, and the parse result. It exits with 1 when the status is not valid. Use it to test a new status
command.

## Config Files

svcview reads `config.yaml` first. Then it reads each `*.yaml` file in `conf.d/`, in name order. Both are in
`$XDG_CONFIG_HOME/svcview/`, or in `~/.config/svcview/` when `XDG_CONFIG_HOME` is not set. svcview does not use
`~/Library/Application Support`, because most terminal tools keep their config in `~/.config`.

| Rule | Detail |
| --- | --- |
| Top-level settings | `refresh` (the time between two runs of `launchctl list`, default `2s`) and `groups`. Only `config.yaml` can set `refresh`. |
| Merge | svcview joins the `groups` of all files, in file order. |
| Two matches | A service that matches two groups goes to the first one. |
| Errors | An unknown setting, an invalid regular expression, an invalid template, a duplicate group name, or a key conflict stops svcview at start. The error names the file and the group. |
| No config | svcview shows each service that has a plist in `~/Library/LaunchAgents`, in the group `Other`. |
| File mode | svcview refuses a config file that another user can write, because the file holds commands that svcview runs. |

With `conf.d/`, the installer of a program can add its group in one file, and not edit the main config of the
user. For example, the installer of the worker writes `conf.d/worker.yaml`, and its uninstaller deletes it.

## Names That Do Not Say launchd

This version supports only launchd. The names in the config and in the contract are neutral, so a later version
can add systemd and the config of the users stays valid:

- The config and the template values use `name`, not `label`.
- The status contract has no launchd words.
- All launchd code is in `internal/launchd`. The other packages use the type `service.Service`, and do not import
  launchd types.

This version has no Go interface for a service manager. The work for a second manager adds the interface, when the
needs of the two managers are known.

## Code Layout

| Package | Job |
| --- | --- |
| `internal/service` | The types `Service` and `State`. |
| `internal/launchd` | Runs `launchctl`, reads the plists, and returns `service.Service` values. It is the only package that knows launchd. |
| `internal/config` | Reads and checks `config.yaml` and `conf.d/`. |
| `internal/status` | Runs the status commands and parses their output. |
| `internal/tui` | The Bubble Tea model and the views. |
| `internal/cli` | Starts the TUI from the root command, and adds `svcview status`. |

The work deletes the example package `internal/greeting`.

The root command starts the TUI when stdin and stdout are terminals. Otherwise it prints the help, as it does now.
Thus the e2e test "with no command, prints its commands and reads no input" stays valid.

| Library | Use |
| --- | --- |
| Bubble Tea, Lip Gloss, and Bubbles | The TUI |
| `howett.net/plist` | XML and binary plists |
| `go.yaml.in/yaml/v3` | The config. The module has it already, as an indirect dependency. |
| `text/template` | The template values |

## Tests

| Level | What the tests check | How |
| --- | --- | --- |
| Unit | The parse of `launchctl list` | Fixture files in `testdata/` |
| Unit | The read of XML and binary plists | Fixture plists |
| Unit | The merge of config files, and each config error | Fixture folders |
| Unit | The status contract, and each failure | Fixture outputs, and small shell scripts as status commands |
| Unit | The TUI model: keys, selection, focus, and confirmation | A fake list of services |
| Unit | The views at fixed widths, with and without `NO_COLOR` | Golden files of the screen |
| e2e | The screen and the actions | A fake `launchctl`, in Docker and on a Mac |

The e2e tests run in Docker on Linux, which has no launchd. A fake `launchctl` script comes first on `PATH`:

- `launchctl list` prints the content of a fixture file.
- Each other call adds its arguments to a file. A test reads this file after a key press.
- `HOME` points to a scratch folder with plists in `Library/LaunchAgents`.

`task test:e2e:local` on a Mac uses the fake `launchctl` too, so the tests do not change the real services.

## Release

- `.goreleaser.yaml` builds only `darwin`, for `amd64` and `arm64`.
- On Linux, `scripts/install.sh` stops with the message `svcview supports only macOS`.
- The e2e Dockerfile builds a Linux binary for the tests. The code has no `darwin` build tag, so this build works.

## Plan

| Step | Result |
| --- | --- |
| 1. The list | The config, the groups, the data from launchd and `ps`, the two panes with the launchd data, and `--all`. It is useful without a status command. |
| 2. Status | The status command, the summary in the list, the fields with their progress bars, the events, and `svcview status <name>`. |
| 3. Actions | Start, stop, restart, log, and the actions of a group. |
| 4. Release | The darwin-only release, the install script, and the docs in `README.md` and `docs/cli.md`. |

The first user of the status command is a worker repo. Its part of the work is outside svcview:

1. The worker writes its status to the file in `WORKER_STATUS_FILE` every 2 s.
2. The install script of the worker adds `WORKER_STATUS_FILE` to the plist and writes `conf.d/worker.yaml`.

## Open Questions

1. Does a GitHub macOS runner have a `gui/<uid>` domain? If it has one, a CI job can load a real test service and
   run one e2e test against launchd.
2. The `gui/<uid>` domain needs a login session at the console of the Mac. What must svcview show over SSH when
   no console session exists?
3. Must stop also run `launchctl disable`, so that the service stays stopped after the next login? This version
   does not.
