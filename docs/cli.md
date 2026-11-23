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
│ ▌● 01                        ││ up 3h12m      last exit 0                    │
│  ● 02                        ││                                              │
│                              ││                                              │
│ Other                        ││                                              │
│  ○ homebrew.mxcl.postgres    ││                                              │
╰──────────────────────────────╯╰──────────────────────────────────────────────╯
 ? help
```

This screen has the group `Workers`, with the match `^com\.example\.worker\.`, in its
[config file](#the-config-file).

| Part | What it shows |
| --- | --- |
| The header | The domain `gui/<uid>` of the user, and the number of services in each state. |
| The list | One row for each service that has a plist in `~/Library/LaunchAgents`, in [the groups of the config file](#groups), and then in the group `Other`. Each group starts with a line with its name, and an empty line separates two groups. In each group, the services are sorted by name. Each row starts with the glyph of the state of the service, and a row in a group shows the short name of the service. svcview cuts a name that is too long for the list with `…`. When the list has more lines than the pane, it scrolls to keep the selected row in view. When the selected row is the first row of its group, and the pane has room for more than one line, the line of the group shows too. The list takes 40 percent of the width. |
| The detail pane | The selected service. Its title is the full name of the service. The pane shows the state, the PID, the uptime, and the last exit. |
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
| The selected row | A blue `▌`, and a gray background on the name |

svcview uses the 16 colors of the ANSI palette, so the theme of the terminal sets the real colors. Each state has a
glyph of a different shape, so the screen shows the state without color too.

### The detail pane

| Value | What it shows |
| --- | --- |
| `PID` | The PID of the process of a `running` service. A service that does not run shows `-`. |
| `up` | The time since the process of the service started, in its two largest units: for example `up 45s`, `up 3m12s`, `up 3h12m`, or `up 4d9h`. A service that does not run shows `-`. A service whose PID `ps` does not report shows `-` too. |
| `last exit` | The exit code of the last run, or the name of the signal that ended it, for example `SIGKILL`. |

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
| `↑` `↓` | Select a service. The keys move one row at a time in the order of the list, also from one group to the next. |
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
  - name: Apple
    match: ^com\.apple\.
```

| Setting | What it does | Default |
| --- | --- | --- |
| `refresh` | The time between two reads of the services. One `refresh` after each read ends, svcview reads the plists and runs `launchctl list` and `ps` again. The value is a Go duration that is more than 0, for example `500ms`, `5s`, or `1m30s`. | `2s` |
| `groups` | The groups of the list, in the order of the list. Each group has a `name` and a `match`. | No groups |

### Groups

Each entry in `groups` is a group of the list, with two settings:

| Setting | What it does |
| --- | --- |
| `name` | The name of the group, which the list shows on the line of the group. Two names that differ in case, for example `Workers` and `workers`, are two names. |
| `match` | A [Go regular expression](https://pkg.go.dev/regexp/syntax). The group holds each service whose name the expression matches. A match in any part of the name counts, unless the expression has `^` or `$`. |

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

### Errors

These errors stop svcview before the screen opens. svcview prints the error, which names the file, and exits with 1.

- A setting that svcview does not know, or a setting that the file sets two times.
- A `refresh` that is not a duration, or that is not more than 0.
- A file that is not valid YAML, or whose top level is not a set of settings such as `refresh: 5s`.
- A file that svcview cannot read.
- A file that another user can write.

These errors in `groups` stop svcview too. Each error names the file, and an error in one group also names the
group. A group with no name has the name of its place in `groups`, for example `group 2`.

- A `groups` that is not a list, or an entry that is not a set of settings such as `name: Workers`.
- A group with no `name`, or with no `match`.
- A `match` that is not a Go regular expression.
- A setting in a group that svcview does not know, or a setting that a group sets two times.
- Two groups with the same name.
- A group with the name `Other`, because `Other` is the group of the services that no group matches.

### A config file that another user can write

svcview runs as your user and does what its config file says. A user who can write the file can then control what
svcview does with your rights. So svcview refuses `config.yaml` when its group or other users can write it, or when
a user other than you owns it.

svcview checks the file that it reads, so for a symbolic link it checks the file that the link points to. It does
not check the folders that hold the file.

svcview checks the mode and the owner of the file, not its ACL. On macOS, an ACL can let another user write a file
whose mode is `0644`, and svcview still reads that file. `ls -le` shows the ACL of a file, and `chmod -N` removes it.

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
