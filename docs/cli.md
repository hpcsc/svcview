# The svcview CLI

## Install

The installer downloads the latest release for your platform, verifies its checksum, and puts the
binary where you can start using it. It needs `sh`, `curl`, `jq`, `tar`, and `gzip`.

```shell
sh <(curl -fsSL https://raw.githubusercontent.com/hpcsc/svcview/main/scripts/install.sh)
```

It asks for the release channel and the install directory. To skip the questions on the command line:

```shell
sh <(curl ...) --channel release --dir ~/.local/bin
```

The channel is `release` (the latest stable release) or `prerelease` (the latest build of `main`).
Without `--dir`, the binary lands in `~/.local/bin` (or `$INSTALL_DIR`). When more than one version is
available in the channel, the installer lists them for you to pick.

`GITHUB_TOKEN` or `GH_TOKEN` gives access to a private repository.

`svcview update` also installs from a release, and it replaces this very binary, so the
installer and the update command do the same kind of job; use whichever you find convenient on a fresh
machine.

## Build

```shell
task build             # build ./bin/svcview
task cli -- --help     # run the CLI from the source, with the arguments after --
```

## The screen

```shell
svcview                # open the screen
```

`svcview` with no command opens the screen when stdin and stdout are both terminals. When stdin or stdout is not a
terminal, for example in a pipe, it prints the help and opens no screen.

```
╭ svcview ──────────────────────────────────────────────────────────── gui/501 ╮
│  ● 2 running   ◐ 0 loaded   ○ 1 not loaded                                   │
╰──────────────────────────────────────────────────────────────────────────────╯
╭ Services ────────────────────╮╭ com.example.worker.01 ───────────────────────╮
│ Workers                      ││ ● running     PID 4122                       │
│ ▌● 01  busy · ENG-692 · 12m  ││ up 3h12m      last exit 0                    │
│  ● 02  idle                  ││ Status 1s ago                                │
│                              ││ mode    busy                                 │
│ Other                        ││ ticket  ENG-692                              │
│  ○ homebrew.mxcl.postgres    ││ step    review · 12m / 20m                   │
│                              ││         ▰▰▰▰▰▰▱▱▱▱                           │
│                              ││ done    14                                   │
│                              ││ failed  1                                    │
│                              ││                                              │
│                              ││ Events                                       │
│                              ││ 09:04 ● step review                          │
│                              ││ 09:00 ● claimed ENG-692                      │
│                              ││                                              │
╰──────────────────────────────╯╰──────────────────────────────────────────────╯
 ? help
```

This screen has the group `Workers`, with the match `^com\.example\.worker\.` and a
[status command](#the-status-command), in its [config file](#the-config-file).

| Part | What it shows |
| --- | --- |
| The header | The domain `gui/<uid>` of the user, and the number of services in each state. |
| The list | One row for each service that has a plist in `~/Library/LaunchAgents`, in [the groups of the config file](#groups), and then in the group `Other`. Each group starts with a line with its name, and an empty line separates two groups. In each group, the services are sorted by name. Each row starts with the glyph of the state of the service, and a row in a group shows the short name of the service. svcview cuts a name that is too long for the list with `…`. When the list has more lines than the pane, it scrolls to keep the selected row in view. When the selected row is the first row of its group, and the pane has room for more than one line, the line of the group shows too. In a group with a status command, a row shows [the summary of the status](#in-the-list-and-the-detail-pane) after the short name. The list takes 40 percent of the width. At start, the list has [the focus](#the-focus-and-the-scroll). |
| The detail pane | The selected service. Its title is the full name of the service. The pane shows the state, the PID, the uptime, and the last exit. In a group with a status command, it also shows the age of the status, and the [fields and the events](#in-the-list-and-the-detail-pane) of the last output or the [failure](#failures) of the last run. When the detail pane has [the focus](#the-focus-and-the-scroll), `↑` and `↓` scroll it. |
| The help line | `? help` |

The name of a service is the `Label` of its plist. svcview gets the state, the PID, and the last exit of each
service from `launchctl list`, and the uptime from `ps`.

### Glyphs and colors

| Element | Look |
| --- | --- |
| The state `running` | A green `●` |
| The state `loaded` | A yellow `◐` |
| The state `not loaded` | A gray `○` |
| A last exit other than 0 | The glyph in the list and the `last exit` value are red. |
| The selected row | A blue `▌`, and a gray background on the name and the summary |
| A summary of the level `info` | The default color of the terminal |
| A summary of the level `warn` | Yellow |
| A summary of the level `error` | Red |
| A template value that the plist does not have, for example `the plist has no WORKER_STATUS_FILE` | Red, in place of the summary |
| A [failure](#failures) of the status command: `status error`, `status timeout`, or `status invalid` | Red, in place of the summary |
| The bar of a field with a `progress` | A blue `▰` for each done cell, and a dim `▱` for each other cell. Dim is the faint attribute of the terminal, in its default color. |
| The `●` of an event of the level `info`, `warn`, or `error` | The default color, yellow, or red, as the summary of the same level |
| The border of the pane with [the focus](#the-focus-and-the-scroll) | Blue lines and corners. The title keeps the default color. |
| The border of the other pane | Dim lines and corners. The title keeps the default color. |

svcview uses the 16 colors of the ANSI palette, so the theme of the terminal sets the real colors. Each state has a
glyph of a different shape, so the screen shows the state without color too.

### The detail pane

| Value | What it shows |
| --- | --- |
| `PID` | The PID of the process of a `running` service. A service that does not run shows `-`. |
| `up` | The time since the process of the service started, in its two largest units: for example `up 45s`, `up 3m12s`, `up 3h12m`, or `up 4d9h`. A service that does not run shows `-`. A service whose PID `ps` does not report shows `-` too. |
| `last exit` | The exit code of the last run, or the name of the signal that ended it, for example `SIGKILL`. |
| `Status` | The age of the status, for example `Status 1s ago`, in a group with a [status command](#the-status-command). The line shows after the first run of the status command of the service ends. After a valid output, the lines under `Status` show [each field](#in-the-list-and-the-detail-pane) of the output with its name and its value, and a bar of 10 cells under the value of a field with a `progress`. After a [failure](#failures), the lines under `Status` tell why the run failed. |
| `Events` | The [events](#in-the-list-and-the-detail-pane) of the last output, after the fields: for each event, the hours and minutes of its `at`, a `●` in the color of its `level`, and its `text`. The line `Events` shows only when the output has events. |

### The focus and the scroll

One of the two panes has the focus: the list at start. `tab` moves the focus to the other pane. The border of the
pane with the focus is blue, and the border of the other pane is dim. When the list has the focus, `↑` and `↓` select
a service. When the detail pane has the focus, `↑` and `↓` scroll it one line. The scroll stops at the first line, and
at the line that puts the last line of the text at the bottom of the pane. A text that fits in the pane does not move.
The selected service stays the same while the detail pane scrolls. While the help shows, `tab`, `↑`, and `↓` change
nothing, and the focus stays where it is.

A service that becomes selected shows its detail pane from the first line. svcview does not keep the scroll of a
service, so a service that is selected again shows from the first line too. When the text of the detail pane gets
shorter, or the terminal changes its size, the scroll moves back so that the last line of the text is at the bottom of
the pane or above it. A refresh, and a run that gives a text of the same length, keep the scroll where it is. A read
that fails shows no service in the detail pane, so the next read that succeeds shows the selected service from the
first line.

### Refresh

One `refresh` after each read ends, svcview reads the plists and runs `launchctl list` and `ps` again, with no key
press. The setting `refresh` of [the config file](#the-config-file) sets the time, and the default is 2 s. svcview
reads a plist again only when the modification time of the file changes. The same service stays selected. When the
selected service is gone, the row after it in the list is selected, or the last row when it was the last.

When `launchctl list` fails, or prints a header line other than `PID`, `Status`, `Label`, the list shows the error
and no rows. When `ps` fails, no service shows an uptime.

### Keys

| Key | Action |
| --- | --- |
| `↑` `↓` | Select a service. The keys move one row at a time in the order of the list, also from one group to the next. When the detail pane has [the focus](#the-focus-and-the-scroll), scroll it one line. |
| `tab` | Move the focus to the other pane. |
| `?` | Show the help, which lists all keys, or hide it. |
| `esc` | Hide the help. |
| `q` or `ctrl+c` | Quit. |

## The config file

Before svcview opens the screen, it reads `config.yaml` in the config folder:

| `XDG_CONFIG_HOME` | The config folder |
| --- | --- |
| An absolute path | `$XDG_CONFIG_HOME/svcview/` |
| Not set, empty, or a relative path | `~/.config/svcview/` |

svcview reads the file only at start, so a change to the file takes effect at the next start. With no
`config.yaml`, each setting has its default. `svcview version`, `svcview update`, and the help do not read the
file.

```yaml
# ~/.config/svcview/config.yaml
refresh: 5s
groups:
  - name: Workers
    match: ^com\.example\.worker\.
    status:
      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
  - name: Apple
    match: ^com\.apple\.
```

| Setting | What it does | Default |
| --- | --- | --- |
| `refresh` | The time between two reads of the services. One `refresh` after each read ends, svcview reads the plists and runs `launchctl list` and `ps` again. The value is a Go duration that is more than 0, for example `500ms`, `5s`, or `1m30s`. | `2s` |
| `groups` | The groups of the list, in the order of the list. Each group has a `name` and a `match`, and can have a `status`. | No groups |

### Groups

Each entry in `groups` is a group of the list, with these settings:

| Setting | What it does |
| --- | --- |
| `name` | The name of the group, which the list shows on the line of the group. Two names that differ in case, for example `Workers` and `workers`, are two names. |
| `match` | A [Go regular expression](https://pkg.go.dev/regexp/syntax). The group holds each service whose name the expression matches. A match in any part of the name counts, unless the expression has `^` or `$`. |
| `status` | Optional. The [status command](#the-status-command) of the services of the group. |

These rules put each service in one group:

1. svcview tries the groups in the order of the config file. A service is in the first group whose `match` matches
   its name, also when the `match` of a later group matches the name too.
2. A service that no group matches is in the group `Other`. The list shows `Other` after the groups of the config
   file.
3. In each group, the services are sorted by name.
4. A group with no service shows no line. When each service is in a group of the config file, the list shows no
   line `Other`.

With no `groups`, each service is in the group `Other`.

A row in a group shows the short name of its service: the part of the name after the text that `match` matched,
when the match starts at the first character of the name. For example, `^com\.example\.worker\.` gives `01` for
`com.example.worker.01`. The row shows the full name when the match starts after the first character, when the
match is the whole name, and in the group `Other`. The title of the detail pane always shows the full name.

Write a `match` in single quotes, for example `'[0-9]+\.worker'`. With no quotes, a `match` that starts with a
character such as `[` or `*` is not valid YAML. In double quotes, YAML reads each `\` as the start of an escape, so
`"^com\.example"` is not valid YAML.

### The status command

A group can have a status command. svcview runs the command for each service of the group, reads one JSON object
from its stdout, and shows the summary of that object in the row of the service.

```yaml
groups:
  - name: Workers
    match: ^com\.example\.worker\.
    status:
      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
      every: 2s
      timeout: 1s
```

| Setting | What it does | Default |
| --- | --- | --- |
| `status.command` | The program and its arguments, as a list of texts with the program first. Each text can use the [template values](#template-values). | Required |
| `status.every` | The time between two runs of the command for one service. The value is a Go duration that is more than 0. | `2s` |
| `status.timeout` | The time after which svcview stops a run, with each process that the command started. The value is a Go duration that is more than 0. | `1s` |

svcview runs the program and its arguments with no shell. Thus `$HOME`, `;`, `|`, `*` and a space in a text reach
the program as they are. For a shell, write `[sh, -c, '...']`. The command runs with the environment and the working
folder of svcview. The `EnvironmentVariables` of the plist reach the command only through `{{.Env.X}}`.

When the first read of the services ends, svcview runs the command of each service of the group. After that, it
runs the command of each service of the group every `every`. The command runs in each state of the service, also
`not loaded`. These rules apply to the runs:

1. svcview does not start a second run for a service until the run of that service ends. The next run starts at
   the first `every` after the run ends.
2. A service that a read adds gets its first run at the next `every` of its group.
3. svcview stops a run at `timeout`. It reads a maximum of 1 MiB from stdout, and it keeps the last 20 lines of
   stderr.
4. When svcview quits, it stops each command that still runs, with each process that the command started.

#### Template values

svcview applies Go `text/template` to each text of the command, also to the program. The values come from the plist
of the service.

| Value | Content |
| --- | --- |
| `{{.Name}}` | The name of the service, that is the `Label` of its plist. |
| `{{.Env.X}}` | The value of `X` in `EnvironmentVariables`. A variable with an empty value gives an empty text. |
| `{{.StdoutPath}}` | `StandardOutPath` |
| `{{.StderrPath}}` | `StandardErrorPath` |
| `{{.PlistPath}}` | The path of the plist file in `~/Library/LaunchAgents`. For a symbolic link, the path of the link. |

A value that the plist does not have is not an empty text. svcview runs no program, and the row of the service
shows the reason in red:

| Value | The row shows |
| --- | --- |
| `{{.Env.X}}` | `the plist has no X`, for example `the plist has no WORKER_STATUS_FILE` |
| `{{.StdoutPath}}` | `the plist has no StandardOutPath` |
| `{{.StderrPath}}` | `the plist has no StandardErrorPath` |
| `{{.PlistPath}}` | `the service has no plist` |

A text can use the functions of Go templates and variables, for example `{{.Name | printf "%s.json"}}` or
`{{$file := .Env.WORKER_STATUS_FILE}}{{$file}}`. In the body of a `with`, `.` is the value of the `with`, so a
template value needs `$` there, for example `{{with .PlistPath}}{{$.Name}}{{end}}`. A text that uses a value in an
`if` or a `with` also needs that value in the plist.

These texts stop svcview at start, with an error that names the file, the group and the text:

- A template value other than the five above, for example `{{.Label}}` or `{{.Env}}`.
- `range`, `define`, `block`, and `{{template ...}}`.
- A field without `$` in the body of a `with`, for example `{{with .PlistPath}}{{.Name}}{{end}}`.
- A field of a variable that the text declares, for example `{{$file := .Env.WORKER_STATUS_FILE}}{{$file.Path}}`.
- A new value for `$`, for example `{{$ := .Name}}`.

#### Output

The output of a status command is one JSON object of version 1 on stdout:

```json
{
  "version": 1,
  "at": "2026-11-23T09:12:00Z",
  "summary": "busy · ENG-692 · 12m",
  "level": "info",
  "fields": [
    {"name": "mode", "value": "busy"},
    {"name": "step", "value": "review · 12m / 20m", "progress": 0.6}
  ],
  "events": [
    {"at": "2026-11-23T09:04:40Z", "level": "info", "text": "step review"}
  ]
}
```

| Property | Need | Shape | What the screen shows |
| --- | --- | --- | --- |
| `version` | Required | The number `1`. svcview accepts no other version. | Nothing |
| `at` | Optional | The time of the data, as an RFC 3339 text | The age of `at` in the detail pane |
| `summary` | Optional | Text | The summary in the row of the service |
| `level` | Optional | `info`, `warn`, or `error`. The default is `info`. | The color of the summary |
| `fields` | Optional | A list of objects. Each has the texts `name` and `value`, and can have `progress`, a number from 0 to 1. A `progress` outside 0 to 1 makes the output invalid. | One line for each field under `Status` in the detail pane, with its name and its value, and a bar under the value of a field with `progress` |
| `events` | Optional | A list of objects. Each has `at`, an RFC 3339 text, `level`, and the text `text`. | One line for each event under `Events` in the detail pane, with the time of `at`, a `●` in the color of `level`, and `text` |

svcview ignores a property that it does not know, so a program can add a property before svcview uses it. The names
of the properties are case-sensitive, and an object must not have a name two times. A new line after the object is
valid. A second JSON value after the object is not valid.

svcview shows the summary only when the command exits with 0 and prints a valid object. Any other run is a
[failure](#failures), and the row shows the failure in place of the summary.

#### In the list and the detail pane

The row of a service shows the `summary` of the last run that ended, after the short name. The summaries of the
rows of one group start in one column. `level` sets the color of the summary: the default color for `info`, yellow
for `warn`, and red for `error`. A template value that the plist does not have shows in red in place of the summary,
for example `the plist has no WORKER_STATUS_FILE`, and so does a [failure](#failures). The row shows nothing after
the short name before the first run ends, and after a valid output with no `summary`. svcview cuts a summary that is
too long for the list with `…`.

The detail pane shows the line `Status` under the data from launchd, with the age of the status, for example
`Status 1s ago`. The age is the time since `at` when the output has `at`. In each other case, it is the time since
the last run ended. The age has the units of `up`, and an `at` after the current time shows `0s`. The age grows
each time the screen draws, also with no new run. Before the first run of the service ends, the detail pane shows
no line `Status`. After a failure, the detail pane shows under `Status` the text of the failure, and then its lines,
and no fields and no events.

Under `Status`, the detail pane shows each field of the last valid output on a line of its own, in the order of the
output: the `name` of the field, and then its `value`. The values of one output start in one column, two spaces
after the widest name of that output. svcview shows each field in the same way, whatever its name, so a service can
add a field with no change to svcview. A field with an empty `value` shows only its name.

Under the value of a field with `progress`, a bar of 10 cells shows how far the job is. The bar is on a line of its
own, and starts in the column of the values. The number of `▰` is the progress times 10, rounded down, and the other
cells are `▱`. For example, `0.6` shows 6 `▰` and 4 `▱`, and `0.99` shows 9 `▰` and 1 `▱`. Thus only a progress of 1
fills the bar.

```
step    review · 12m / 20m
        ▰▰▰▰▰▰▱▱▱▱
```

After the fields, an output with events shows an empty line, the line `Events`, and one line for each event, in the
order of the output. svcview does not sort, filter, or count the events, and it shows an event that is older than one
hour too. Each line shows the hours and minutes of `at` in the local time zone, for example `09:04`, then a `●` in
the color of `level`, and then `text`. The time has no date, so an event of another day shows only its hours and
minutes. svcview takes the local time zone from `TZ`, or from the system when `TZ` is not set. An output with no
events shows no line `Events`.

```
Events
09:04 ● step review
09:00 ● claimed ENG-692
```

Each control character in the name or the value of a field, or in the text of an event, shows as `�`. Thus an output
cannot send an escape sequence to the terminal. When the detail pane has [the focus](#the-focus-and-the-scroll), `↑`
and `↓` scroll it to the lines that do not fit in the pane. It cuts each line at the width of the pane with `…`.

#### Failures

A run that gives no valid output is a failure. The row shows the failure in red in place of the summary, and the
detail pane shows under `Status` why the run failed:

| Failure | The row shows | The detail pane shows under `Status` |
| --- | --- | --- |
| The program exits with a code other than 0, a signal ends it, or svcview cannot start it | `status error` | `exit code 3`, `ended by SIGTERM`, or why svcview cannot start the program, for example `exec: "worker": executable file not found in $PATH`. Then the last 20 lines of stderr. |
| The run lasts longer than `timeout` | `status timeout` | The command line: the program and its arguments after svcview applies the template values, for example `sh -c 'sleep 30'`. A text that a shell reads in another way is in single quotes. |
| The output is not a JSON object of version 1, or is larger than 1 MiB | `status invalid` | The parse error, and then the first 20 lines of the output |
| A template value that the plist does not have | The reason, for example `the plist has no WORKER_STATUS_FILE` | The same reason. svcview runs no program. |

These rules decide which failure a run shows:

1. A run that lasts longer than `timeout` is a `status timeout`, whatever the program printed before.
2. An output larger than 1 MiB is a `status invalid`. svcview stops the read of stdout at 1 MiB, so the program can
   end with `SIGPIPE` or with an exit code other than 0. The row still shows `status invalid`.
3. A program that exits with a code other than 0 is a `status error`, also when it prints a valid output.
4. A program that exits with 0, while a process that it started keeps stdout or stderr open for more than 1 s after
   the exit, is a `status error` with the text `exec: WaitDelay expired before I/O complete`. For example,
   `sh -c 'worker-daemon & cat status.json'` gives this failure, because `worker-daemon` keeps the stdout of the
   shell. svcview then stops `worker-daemon` with the process group. Rule 1 comes first: when `timeout` ends before
   that 1 s, the run is a `status timeout`.

The parse error names the place in the output as a JSON pointer, and says what version 1 needs there:

| Output | Parse error |
| --- | --- |
| No text, or only white space | `the output is empty` |
| More than 1 MiB | `the output is larger than 1 MiB` |
| Text that is not valid JSON, for example `busy` | `the output is not valid JSON: invalid character 'b' at start of value` |
| An object with a name two times | `the output is not valid JSON: duplicate object member name "summary"` |
| A JSON value that is not an object, for example `[{"version": 1}]` | `the output is a JSON list, not an object` |
| A second JSON value or other text after the object | `the output has more text after its JSON object` |
| No `version`, or a `version` other than 1 | `"/version" is missing, and it must be 1`, or `"/version" is 2, and it must be 1` |
| A `level` other than `info`, `warn`, or `error` | `"/level" is "debug", and it must be info, warn or error` |
| A property with a value of another shape | `"/at" must be an RFC 3339 time in a text, such as "2026-11-23T09:12:00Z"`, or `"/fields/0/value" must be a text` |
| A field or an event with a property missing | `"/fields/1/name" is missing`, or `"/events/0/text" is missing` |
| A field with a `progress` less than 0 or more than 1. The error names the first such field. | `"/fields/2/progress" is 1.5, and it must be from 0 to 1` |

svcview starts each status command in a process group of its own. It stops that process group with `SIGKILL` at
`timeout`, when the program ends, and when svcview quits. svcview quits on `q` and `ctrl+c`, when its terminal
closes, and on `SIGTERM`. Thus no process that the command starts runs longer than the run, for example the `sleep`
of `sh -c 'sleep 30 &'`. A process that leaves the process group, for example with `setsid`, does not stop.

svcview keeps the last 20 lines of stderr and the first 20 lines of an output that is not valid, and a maximum of
4096 bytes of each line. When the detail pane has [the focus](#the-focus-and-the-scroll), `↑` and `↓` scroll it to
the lines that do not fit in the pane. It cuts each line at the width of the pane with `…`. Each control character in
a line shows as `�`.

#### A fast status command

svcview runs the status command of each service every few seconds, so the command must be fast. A program that
starts a large runtime for each run, for example a BEAM VM or a JVM, is too slow. In that case:

1. Make the service write its status to a file at a fixed interval.
2. Put the path of the file in the `EnvironmentVariables` of the plist, for example as `WORKER_STATUS_FILE`.
3. Use `cat` as the status command: `command: [cat, '{{.Env.WORKER_STATUS_FILE}}']`.

The service must write the file in one step. It writes a temporary file in the same folder, and then renames the
temporary file to the name of the status file. A rename in one folder replaces the file in one step, so `cat` never
reads half a file.

### Errors

These errors stop svcview before the screen opens. svcview prints the error, which names the file, and exits with 1.

- A setting that svcview does not know, or a setting that the file sets two times.
- A `refresh` that is not a duration, or that is not more than 0.
- A file that is not valid YAML, or whose top level is not a set of settings such as `refresh: 5s`.
- A file that svcview cannot read.
- A file that another user can write.
- A file that has an ACL with one or more entries, or whose ACL svcview cannot read.

These errors in `groups` stop svcview too. Each error names the file, and an error in one group also names the
group. A group with no name has the name of its place in `groups`, for example `group 2`.

- A `groups` that is not a list, or an entry that is not a set of settings such as `name: Workers`.
- A group with no `name`, or with no `match`.
- A `match` that is not a Go regular expression.
- A setting in a group that svcview does not know, or a setting that a group sets two times.
- Two groups with the same name.
- A group with the name `Other`, because `Other` is the group of the services that no group matches.

These errors in `status` stop svcview too. Each error names the file and the group.

- A `status` that is not a set of settings, or a `status` with no `command`.
- A setting in `status` that svcview does not know, or a setting that `status` sets two times.
- A `command` that is not a list of texts, an empty `command`, or a `command` whose program is an empty text.
- An `every` or a `timeout` that is not a duration, or that is not more than 0.
- A text of `command` that is not a Go template, or that uses a value or an action that a text cannot use. The
  section [Template values](#template-values) lists them.

### A config file that another user can write

svcview runs as your user and does what its config file says. A user who can write the file can then control what
svcview does with your rights. So svcview refuses `config.yaml` when its group or other users can write it, when
a user other than you owns it, or when it has an ACL with one or more entries.

svcview checks the file that it reads, so for a symbolic link it checks the file that the link points to. It does
not check the folders that hold the file.

On macOS, an ACL can let another user write a file whose mode is `0644`. Each entry of an ACL names a user or a
group by a GUID, and svcview cannot tell an entry for you from an entry for another user. So svcview refuses
`config.yaml` when its ACL has one or more entries, whatever the entries allow or deny. It also refuses the file when
it cannot read the ACL. An ACL with no entries gives no rights to anyone, so svcview reads that file.

`/bin/ls -le` shows the ACL of a file, and `/bin/chmod -N` removes it:

```shell
/bin/ls -le ~/.config/svcview/config.yaml
/bin/chmod -N ~/.config/svcview/config.yaml
```

Use `/bin/ls` and `/bin/chmod`. The GNU `ls` and `chmod`, for example those of Homebrew, can come first on your
`PATH`, and they have no `-e` and no `-N`.

On Linux, svcview reads no ACL. A POSIX ACL entry that lets another user write the file sets the group write bit of
the mode, so the check of the mode refuses that file.

To give the file a mode that svcview accepts, remove the write permission of the group and of other users:

```shell
chmod go-w ~/.config/svcview/config.yaml
```

When another user owns the file, make it yours first:

```shell
sudo chown "$USER" ~/.config/svcview/config.yaml
```

## Version and update

```shell
svcview version                # the tag of a release or a prerelease, or the commit of any other build
svcview update                 # install the latest release
svcview update --prerelease    # install the latest prerelease, a build of main
svcview update --check         # only tell you whether this build is the latest
```

`internal/cli/version` reports the version. A release build gets its tag from goreleaser, through
`-ldflags -X .../internal/cli/version.releaseTag`. Any other build reports the short commit sha that Go
keeps in the build information, with `-dirty` after it when the working tree had changes.

`svcview update` downloads the archive for your platform from a GitHub release, checks it
against the `checksums.txt` of that release, and then replaces the binary. It names each step on
stderr, and in a terminal it shows how much of the download has arrived. `GITHUB_TOKEN` or `GH_TOKEN`
gives access to a private repository.

Releases and prereleases are two channels. Each command installs the latest build of its channel when
this build is a different one, so `svcview update` on a prerelease goes back to the latest
release. A build from a commit is not a release or a prerelease, so `svcview update` does not
replace it unless you add `--force`.

## Release

GoReleaser builds the releases from `.goreleaser.yaml`. The CI workflow, `.github/workflows/ci.yml`, publishes
them after its jobs `test` and `e2e` pass:

- A push of a tag `v<major>.<minor>.<patch>` publishes a release.
- Each push to `main` publishes a prerelease. The job `prerelease` tags the commit with the next patch after the
  latest release, the run number and the commit, for example `v0.2.1-42.g4829f92`. It keeps only the 5 newest
  prereleases. A run whose commit is no longer the newest commit of `main` publishes nothing, so an older build
  never replaces a newer prerelease.
- A re-run of a `prerelease` job that pushed its tag fails at the tag. The next push to `main` publishes a new
  prerelease.
- When three pushes to `main` come close together, the newest commit can get no prerelease: the `prerelease` job
  of an older commit can cancel the pending `prerelease` job of the newest commit. The next push to `main`
  publishes a new prerelease.

```shell
git tag vX.X.X
git push origin vX.X.X
```

`task release:local` makes a snapshot build in `dist/`, and publishes nothing.

## E2E Test

The end-to-end tests run the built binary in a terminal, send keys to it, and read the screen. They are
in `e2e/`, and [e2e-tests.md](e2e-tests.md) tells how they work.

```shell
task test:e2e          # in Docker
task test:e2e:local    # on this machine, with Node
```
