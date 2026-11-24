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
 s start · x stop · ? help
```

This screen has the group `Workers`, with the match `^com\.example\.worker\.` and a
[status command](#the-status-command), in its [config file](#the-config-file).

| Part | What it shows |
| --- | --- |
| The header | The domain `gui/<uid>` of the user, and the number of services in each state. |
| The list | One row for each service that has a plist in `~/Library/LaunchAgents`, in [the groups of the config files](#groups), and then in the group `Other`. Each group starts with a line with its name, and an empty line separates two groups. In each group, the services are sorted by name. Each row starts with the glyph of the state of the service, and a row in a group shows the short name of the service. svcview cuts a name that is too long for the list with `…`. When the list has more lines than the pane, it scrolls to keep the selected row in view. When the selected row is the first row of its group, and the pane has room for more than one line, the line of the group shows too. In a group with a status command, a row shows [the summary of the status](#in-the-list-and-the-detail-pane) after the short name. The list takes 40 percent of the width. At start, the list has [the focus](#the-focus-and-the-scroll). |
| The detail pane | The selected service. Its title is the full name of the service. The pane shows the state, the PID, the uptime, and the last exit. In a group with a status command, it also shows the age of the status, and the [fields and the events](#in-the-list-and-the-detail-pane) of the last output or the [failure](#failures) of the last run. When the detail pane has [the focus](#the-focus-and-the-scroll), `↑` and `↓`, or `k` and `j`, scroll it. |
| The message line | The [confirmation](#start-and-stop-a-service) of `x`, or what launchctl printed when a start or a stop fails. The line shows above the help line only while it has a text, and the panes are one line shorter while it shows. See [The message line](#the-message-line). |
| The help line | `s start · x stop · ? help` |

The name of a service is the `Label` of its plist. svcview gets the state, the PID, and the last exit of each
service from `launchctl list`, and the uptime from `ps`.

### Glyphs and colors

| Element | Look |
| --- | --- |
| The state `running` | A green `●` |
| The state `loaded` | A yellow `◐` |
| The state `not loaded` | A gray `○` |
| A last exit other than 0 | The glyph in the list and the `last exit` value are red. |
| The selected row | A blue `▌`, and a gray background on the name, the summary, and the stale mark |
| A summary of the level `info` | The default color of the terminal |
| A summary of the level `warn` | Yellow |
| A summary of the level `error` | Red |
| A [stale status](#in-the-list-and-the-detail-pane), for example `busy  stale 3m` | The summary is dim, in the color of its level. The stale mark after it is in the default color. Dim is the faint attribute of the terminal. |
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

One of the two panes has the focus: the list at start. `tab` moves the focus to the other pane. `l` moves the focus
to the detail pane, and `h` moves it to the list, so `l` does nothing when the detail pane has the focus, and `h` does
nothing when the list has the focus. `tab`, `l`, and `h` move the focus also when the list has no rows or shows an
error. The border of the pane with the focus is blue, and the border of the other pane is dim. When the list has the
focus, `↑` and `↓` select a service. When the detail pane has the focus, `↑` and `↓` scroll it one line. `k` does
what `↑` does, and `j` does what `↓` does. The scroll stops at the first line, and at the line that puts the last
line of the text at the bottom of the pane. A text that fits in the pane does not move. The selected service stays
the same while the detail pane scrolls. While the help shows, `tab`, `↑`, `↓`, `h`, `j`, `k`, and `l` change nothing,
and the focus stays where it is.

`h`, `j`, `k`, and `l` are the keys that vim uses to move. Only the lower-case letters move: `H`, `J`, `K`, and `L`
do nothing. `←` and `→` do not move the focus.

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
| `↑` `↓`, or `k` `j` | Select a service. The keys move one row at a time in the order of the list, also from one group to the next. When the detail pane has [the focus](#the-focus-and-the-scroll), scroll it one line. |
| `tab` | Move the focus to the other pane. |
| `l` | Move the focus to the detail pane. When the detail pane has the focus, `l` does nothing. |
| `h` | Move the focus to the list. When the list has the focus, `h` does nothing. |
| `s` | Start the selected service. See [Start and stop a service](#start-and-stop-a-service). |
| `x` | Stop the selected service, after a confirmation. `y` answers yes to the confirmation, and each other key answers no. |
| `?` | Show the help, which lists all keys, or hide it. |
| `esc` | Hide the help. |
| `q` or `ctrl+c` | Quit. |

### Start and stop a service

`s` starts the selected service, and `x` stops it. svcview runs `launchctl` in the domain `gui/<uid>` of the user:

| The state of the selected service | `s` runs | `x` runs |
| --- | --- | --- |
| `not loaded` | `launchctl bootstrap gui/<uid> <plist path>` | Nothing |
| `loaded` | `launchctl kickstart gui/<uid>/<name>` | `launchctl bootout gui/<uid>/<name>`, after the confirmation |
| `running` | Nothing | `launchctl bootout gui/<uid>/<name>`, after the confirmation |

`<plist path>` is the path of the plist in `~/Library/LaunchAgents`, and `<name>` is the `Label` of the plist. svcview
gives each argument to `launchctl` as it is, with no shell.

`x` asks for a confirmation on the message line, with the full name of the service:

```
 Stop com.example.worker.01? (y/N)
```

`y` stops the service that the confirmation names. This is the service that was selected when you pressed `x`, also
when a refresh changed the selected row while the confirmation showed. Each other key answers no, and does nothing
else: for example, `q` does not quit, `↓` and `j` do not select another row, and `l` does not move the focus.
`ctrl+c` quits.

A stop runs `bootout`, not `launchctl kill`. `bootout` removes the service from launchd, so launchd does not start
it again, also when its plist has `KeepAlive`. The service stays `not loaded` until the next login, or until `s`
starts it. svcview does not run `launchctl disable`, so a service whose plist has `RunAtLoad` starts again at the
next login. svcview starts no service on its own.

A start and a stop run in the background, so the screen goes on with its reads, its status runs, and its keys.
svcview does not read the services when a start or a stop ends. The row shows the new state at the next refresh
after the start or the stop ends. These rules apply to the keys:

1. While the start or the stop of a service runs, `s` and `x` do nothing on that service. They still act on another
   service.
2. `s` and `x` do nothing while the help shows, and while the list shows an error.
3. When svcview quits, it does not wait for a start or a stop that still runs.

### The message line

When launchctl fails, the message line above the help line shows what launchctl printed, stderr first, for example
`Bootstrap failed: 5: Input/output error`. When launchctl prints nothing, the message line names the command and its
exit code, for example `launchctl kickstart gui/501/com.example.worker.01 exited with code 113`. svcview kills a
launchctl that does not end within 30 s, with each process that it started, and the message line says so, for
example `launchctl bootout gui/501/com.example.worker.01 did not end: svcview killed it after 30s`.

The message line shows the lines of the text on one line, with a space between them. It shows each control
character as `�`, and cuts a text that is wider than the terminal with `…`. It has no color.

The message line stays through each refresh and each status run, until the next key press. That key hides it, and
also does what it does: for example, `↓` selects the next row. The error of a later start or stop takes the place of
an earlier one. An error that ends while the confirmation shows does not hide the confirmation: the message line
shows the error after the answer. The panes, or the help, are one line shorter while the message line shows.

## The config file

Before svcview opens the screen, it reads the config files in the config folder:

| `XDG_CONFIG_HOME` | The config folder |
| --- | --- |
| An absolute path | `$XDG_CONFIG_HOME/svcview/` |
| Not set, empty, or a relative path | `~/.config/svcview/` |

svcview reads `config.yaml` first. Then it reads each file in `conf.d/` of the config folder whose name ends in
`.yaml`, in the byte order of the names. This order is the file order. Byte order compares the names character by
character, so `10-jobs.yaml` comes before `2-workers.yaml`, and `Workers.yaml` comes before `jobs.yaml`.

svcview skips each other entry of `conf.d`:

- A name that starts with a dot, for example `.worker.yaml` or the `._worker.yaml` that macOS writes on some
  volumes.
- A name that does not end in `.yaml` in lower case, for example `worker.yml`, `worker.YAML`, or `worker.yaml.off`.

A symbolic link counts as the file that it points to, and a symbolic link to no file counts as no file. svcview reads
no folder inside `conf.d`. With no `conf.d`, svcview reads only `config.yaml`. With no `config.yaml`, each setting
has its default, and svcview reads the files of `conf.d` all the same.

svcview reads the config files only at start, so a change to a config file, a new file, or a deleted file takes
effect at the next start. `svcview version`, `svcview update`, and the help do not read the files.

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
| `refresh` | The time between two reads of the services. One `refresh` after each read ends, svcview reads the plists and runs `launchctl list` and `ps` again. The value is a Go duration that is more than 0, for example `500ms`, `5s`, or `1m30s`. Only `config.yaml` can set `refresh`. | `2s` |
| `groups` | The groups of the list, in the order of the list. Each group has a `name` and a `match`, and can have a `status`. | No groups |

### A file in conf.d

The installer of a program can add a group to svcview with one file in `conf.d`, and the user does not edit
`config.yaml`. A file in `conf.d` can set only `groups`.

svcview joins the groups of all config files in file order: first the groups of `config.yaml`, and then the groups
of each file in `conf.d`. In each file, the groups keep the order of the file. Thus a service that the groups of two
files match is in the group of the earlier file. The name of a group must be unique across all config files.

This is a file that the installer of a worker writes:

```yaml
# ~/.config/svcview/conf.d/worker.yaml
groups:
  - name: Workers
    match: ^com\.example\.worker\.
    status:
      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
```

The installer does these steps:

1. It finds the config folder as svcview does: `$XDG_CONFIG_HOME/svcview` when `XDG_CONFIG_HOME` is an absolute
   path, and `~/.config/svcview` in each other case.
2. It writes the file as the user who runs svcview, not as root. The file must have no write permission for its
   group and for other users, for example the mode `0644`. svcview refuses a file that
   [another user can write](#a-config-file-that-another-user-can-write).
3. It writes the file under a name that svcview skips, for example `.worker.yaml`, and then renames the file to
   `worker.yaml`. A rename in one folder replaces the file in one step, so svcview never reads half a file.

```shell
case "$XDG_CONFIG_HOME" in
  /*) folder="$XDG_CONFIG_HOME/svcview/conf.d" ;;
  *) folder="$HOME/.config/svcview/conf.d" ;;
esac
mkdir -p "$folder"
cat > "$folder/.worker.yaml" <<'EOF'
groups:
  - name: Workers
    match: ^com\.example\.worker\.
    status:
      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
EOF
chmod 0644 "$folder/.worker.yaml"
mv -f "$folder/.worker.yaml" "$folder/worker.yaml"
```

The uninstaller deletes the file. At the next start, svcview shows no group of that file, and the services of the
group are in `Other`.

### Groups

Each entry in `groups` is a group of the list, with these settings:

| Setting | What it does |
| --- | --- |
| `name` | The name of the group, which the list shows on the line of the group. Two names that differ in case, for example `Workers` and `workers`, are two names. |
| `match` | A [Go regular expression](https://pkg.go.dev/regexp/syntax). The group holds each service whose name the expression matches. A match in any part of the name counts, unless the expression has `^` or `$`. |
| `status` | Optional. The [status command](#the-status-command) of the services of the group. |

These rules put each service in one group:

1. svcview tries the groups in file order, and the groups of one file in the order of that file. A service is in the
   first group whose `match` matches its name, also when the `match` of a later group matches the name too.
2. A service that no group matches is in the group `Other`. The list shows `Other` after the groups of the config
   files.
3. In each group, the services are sorted by name.
4. A group with no service shows no line. When each service is in a group of a config file, the list shows no
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
      stale_after: 30s
```

| Setting | What it does | Default |
| --- | --- | --- |
| `status.command` | The program and its arguments, as a list of texts with the program first. Each text can use the [template values](#template-values). | Required |
| `status.every` | The time between two runs of the command for one service. The value is a Go duration that is more than 0. | `2s` |
| `status.timeout` | The time after which svcview stops a run, with each process that the command started. The value is a Go duration that is more than 0. | `1s` |
| `status.stale_after` | The age of `at` after which svcview marks the status of a service as [stale](#in-the-list-and-the-detail-pane). The value is a Go duration that is more than 0. | No check |

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

[`svcview status <name>`](#test-a-status-command-with-svcview-status) shows each template value of a service, and the
command line that the values make.

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
| `at` | Optional | The time of the data, as an RFC 3339 text | The age of `at` in the detail pane. svcview compares the age of `at` with `status.stale_after` to find a [stale status](#in-the-list-and-the-detail-pane). |
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

A service that crashes or does not respond stops the writes to its status file, but `cat` still prints the last
file. Thus the row can show `busy` for a service that does nothing. When the group sets `status.stale_after`, svcview
marks such a status as stale.

A status is stale when the `at` of the last output is older than `stale_after`. These statuses are never stale:

- An output with no `at`, whatever the time at which the run ended.
- An `at` that is exactly as old as `stale_after`, or newer, or after the current time.
- A [failure](#failures). The row shows the failure in red in place of the summary.
- A status of a group with no `stale_after`. svcview has no default `stale_after`.

The row of a stale status shows the summary dim, in the color of its level, and then two spaces and the stale mark
in the default color. The stale mark is `stale` and the age of `at` in its largest whole unit, rounded down: `45s`,
`3m` for 3 minutes and 12 seconds, `2h`, or `1d`. An output with no `summary` shows only the stale mark, in the column
of the summaries.

```
Workers
○ 01  busy · ENG-692  stale 3m
○ 02  idle
```

svcview runs the status command in each state of the service, so a `not loaded` service shows the stale mark too
when its `at` is old. svcview checks the age of `at` each time the screen draws. Thus the stale mark shows with no new
run when `at` gets older than `stale_after`, and the age in the mark grows. The stale mark goes away at the end of the
first run whose output has an `at` that is not older than `stale_after`. A stale status does not change the runs: the
command still runs every `every`. [`svcview status <name>`](#test-a-status-command-with-svcview-status) does not mark
a stale status.

When the summary, the two spaces, and the stale mark do not fit in the list, svcview cuts the summary with `…` and
shows the whole stale mark. When the list has no room for one cell of the summary beside the short name and the stale
mark, svcview cuts the row at its end with `…`, as for each other row.

The detail pane shows the line `Status` under the data from launchd, with the age of the status, for example
`Status 1s ago`. The age is the time since `at` when the output has `at`. In each other case, it is the time since
the last run ended. The age has the units of `up`, and an `at` after the current time shows `0s`. The age grows
each time the screen draws, also with no new run. For a stale status, the line `Status` shows the age of `at`, for
example `Status 3m12s ago`, with no stale mark and no dim. Before the first run of the service ends, the detail pane
shows no line `Status`. After a failure, the detail pane shows under `Status` the text of the failure, and then its
lines, and no fields and no events.

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
and `↓`, or `k` and `j`, scroll it to the lines that do not fit in the pane. It cuts each line at the width of the
pane with `…`.

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
4096 bytes of each line. When the detail pane has [the focus](#the-focus-and-the-scroll), `↑` and `↓`, or `k` and
`j`, scroll it to the lines that do not fit in the pane. It cuts each line at the width of the pane with `…`. Each
control character in a line shows as `�`.

[`svcview status <name>`](#test-a-status-command-with-svcview-status) runs the status command of one service one time,
shows its failure with the same words, and exits with 1.

#### A fast status command

svcview runs the status command of each service every few seconds, so the command must be fast. A program that
starts a large runtime for each run, for example a BEAM VM or a JVM, is too slow. In that case:

1. Make the service write its status to a file at a fixed interval.
2. Put the path of the file in the `EnvironmentVariables` of the plist, for example as `WORKER_STATUS_FILE`.
3. Use `cat` as the status command: `command: [cat, '{{.Env.WORKER_STATUS_FILE}}']`.

The service must write the file in one step. It writes a temporary file in the same folder, and then renames the
temporary file to the name of the status file. A rename in one folder replaces the file in one step, so `cat` never
reads half a file.

#### Test a status command with `svcview status`

`svcview status <name>` runs the status command of one service one time, and prints each step. Use it to test a status
command before you ship it, for example in the install script of a program.

```shell
svcview status com.example.worker.01
```

svcview reads the config files and the services as the screen does: `config.yaml`, each file of `conf.d`, and the
plists in `~/Library/LaunchAgents`. It finds the service with that name and the group of the service. Then it runs the
status command of the group one time, with the `timeout` of the group, and prints four parts in this order:

| Part | What it shows |
| --- | --- |
| `Template values` | Each [template value](#template-values) of the service, as a text writes it, and the value that it gives. A value that the plist does not have shows why, for example `the plist has no StandardErrorPath`. The list has one `{{.Env.X}}` for each variable of `EnvironmentVariables`, in the order of the names, also when the command does not use it. |
| `Command line` | The program and its arguments after svcview applies the template values. A text that a shell reads in another way is in single quotes. A `status timeout` shows the same text. |
| `Output` | What the program printed on stdout, line by line, a maximum of 1 MiB. When the program printed nothing on stdout, this part says so. |
| `Result` | `valid` and what svcview read from the output, or the [failure](#failures) of the run. |

For a valid output, `Result` shows `at` as an RFC 3339 time, or `the output has no at`. Then it shows `summary`,
`level` (`info` when the output has none), each field with its `name`, its `value` and its `progress`, and each event
with its `at`, its `level` and its `text`, in the order of the output:

```
Template values
  {{.Name}}                    com.example.worker.01
  {{.Env.WORKER_STATUS_FILE}}  /Users/me/worker/status.json
  {{.Env.WORKER_TOKEN}}        tk_7f3a9c
  {{.StdoutPath}}              /Users/me/Library/Logs/worker.out.log
  {{.StderrPath}}              the plist has no StandardErrorPath
  {{.PlistPath}}               /Users/me/Library/LaunchAgents/com.example.worker.01.plist

Command line
  cat /Users/me/worker/status.json

Output
  {"version": 1, "at": "2026-11-23T09:12:00Z", "summary": "busy · ENG-692 · 12m",
   "fields": [{"name": "mode", "value": "busy"}, {"name": "step", "value": "review · 12m / 20m", "progress": 0.6}],
   "events": [{"at": "2026-11-23T09:04:40Z", "level": "info", "text": "step review"}]}

Result
  valid
  at       2026-11-23T09:12:00Z
  summary  busy · ENG-692 · 12m
  level    info
  fields
    mode  busy
    step  review · 12m / 20m (progress 0.6)
  events
    2026-11-23T09:04:40Z  info   step review
```

For a failure, `Result` shows the word that the row shows, then the text that the detail pane shows, and then the
lines of the failure:

```
Output
  busy

Result
  status invalid
  the output is not valid JSON: invalid character 'b' at start of value
    busy
```

A template value that the plist does not have stops svcview before the run. `Command line` then shows why, for example
`the plist has no WORKER_STATUS_FILE`. svcview runs no program, and prints no `Output` and no `Result`.

svcview exits with 0 only when the status is valid. It exits with 1 in each other case:

- a `status error`, a `status timeout` or a `status invalid`
- a template value that the plist does not have
- a name that no service has. svcview knows only the services that have a plist in `~/Library/LaunchAgents`.
- a service whose group has no status command, also a service in the group `Other`
- no name, or more than one name
- an [error](#errors) in a config file, also in the group of another service, and an error of `launchctl list`
- `SIGINT`, `SIGTERM` or `SIGHUP` during the run. svcview stops the program with each process that it started, and
  prints no `valid`.

When svcview stops before `Template values`, it prints the cause and no part: no name or more than one name, a name
that no service has, a group with no status command, and an error. It prints the failure of the status one time, in the part where the
failure happens, and does not print it again as an error. `Template values` also shows why a path has no value. Thus
a command that uses `{{.StderrPath}}` of a plist with no `StandardErrorPath` shows `the plist has no
StandardErrorPath` in `Template values`, and again under `Command line`, which names the value that stops the
command line.

`svcview status` reads no input and needs no terminal. It prints the same text when stdout is a file or a pipe, so a
script or an installer can run it and read its exit code. The text is for a person: the parts have no color, and
their layout is not a contract. A script must read only the exit code. In a terminal, a cause that stops svcview
shows in red, as each other error of svcview does.

`Template values` shows each variable of `EnvironmentVariables`, also a value that holds a token or a password, as
`WORKER_TOKEN` above shows. Do not put the text where other people can read it. Each control character in the text,
other than the new line of `Output`, shows as `�`. Thus a plist, a config file or an output cannot send an escape
sequence to the terminal.

`svcview status` does not start, stop, or change a service.

### Errors

These errors in a config file stop svcview before the screen opens. svcview prints the error, which names the file,
and exits with 1.

- A setting that svcview does not know, or a setting that the file sets two times.
- A `refresh` that is not a duration, or that is not more than 0.
- A `refresh` in a file in `conf.d`, because only `config.yaml` can set `refresh`.
- A file that is not valid YAML, or whose top level is not a set of settings such as `refresh: 5s`.
- A config file that is not a regular file, for example a folder, a named pipe, or a device. For a symbolic link,
  svcview checks the file that the link points to.
- A file that svcview cannot read.
- A file that another user can write.
- A file that has an ACL with one or more entries, or whose ACL svcview cannot read.

A `conf.d` that is not a folder, or that svcview cannot list, stops svcview too. The error names `conf.d`.

These errors in `groups` stop svcview too. Each error names the file, and an error in one group also names the
group. A group with no name has the name of its place in `groups`, for example `group 2`.

- A `groups` that is not a list, or an entry that is not a set of settings such as `name: Workers`.
- A group with no `name`, or with no `match`.
- A `match` that is not a Go regular expression.
- A setting in a group that svcview does not know, or a setting that a group sets two times.
- Two groups with the same name, in one file or in two files. When the two groups are in two files, the error names
  the file of the second group, and also the file of the first group.
- A group with the name `Other`, because `Other` is the group of the services that no group matches.

These errors in `status` stop svcview too. Each error names the file and the group.

- A `status` that is not a set of settings, or a `status` with no `command`.
- A setting in `status` that svcview does not know, or a setting that `status` sets two times.
- A `command` that is not a list of texts, an empty `command`, or a `command` whose program is an empty text.
- An `every`, a `timeout`, or a `stale_after` that is not a duration, or that is not more than 0.
- A text of `command` that is not a Go template, or that uses a value or an action that a text cannot use. The
  section [Template values](#template-values) lists them.

### A config file that another user can write

svcview runs as your user and does what its config files say. A user who can write a config file can then control
what svcview does with your rights. So svcview refuses a config file, `config.yaml` or a file in `conf.d`, when its
group or other users can write it, when a user other than you owns it, or when it has an ACL with one or more
entries. svcview does the same checks on each config file.

svcview checks each file that it reads, so for a symbolic link it checks the file that the link points to. It does
not check `conf.d`, the config folder, or the other folders that hold the files. A user who can write `conf.d` can
delete or rename your files in it, but svcview refuses a file that such a user writes.

On macOS, an ACL can let another user write a file whose mode is `0644`. Each entry of an ACL names a user or a
group by a GUID, and svcview cannot tell an entry for you from an entry for another user. So svcview refuses a
config file when its ACL has one or more entries, whatever the entries allow or deny. It also refuses the file when
it cannot read the ACL. An ACL with no entries gives no rights to anyone, so svcview reads that file.

`/bin/ls -le` shows the ACL of a file, and `/bin/chmod -N` removes it:

```shell
/bin/ls -le ~/.config/svcview/config.yaml ~/.config/svcview/conf.d
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
