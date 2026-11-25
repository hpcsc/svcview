# svcview

svcview is a terminal UI for the launchd services of the user on macOS. It shows what launchd knows about each
service: the state, the PID, the uptime, the last exit, and the properties of its plist. It also runs a status
command from its config, and shows what each service reports about itself, for example that a worker is busy with a
job, or idle.

- Lists the services that have a plist in `~/Library/LaunchAgents`, in groups from a config file. `a` or `--all`
  shows each service of the domain, also the services of macOS.
- Starts, stops, and restarts a service, and opens its log in `less`.
- Shows the summary, the fields, and the events that a status command reports, and marks a status that is stale.
- Lets another program add its group, its status command, and its own keys with one file in `conf.d/`.

`docs/cli.md` describes each key and each setting. `docs/design.md` describes how svcview works inside, with
diagrams. `docs/proposal-launchd-tui.md` gives the reasons for the design.

## Install

The install script downloads the latest release for macOS, checks its checksum, and puts `svcview` in
`~/.local/bin`. It needs `curl`, `jq`, `tar`, and `gzip`.

```sh
sh <(curl -fsSL https://raw.githubusercontent.com/hpcsc/svcview/main/scripts/install.sh) --channel release --dir ~/.local/bin
```

`~/.local/bin` must be on your `PATH`. Without `--channel` and `--dir`, the script asks for the release channel and
the install folder.

When the repository is private, set `GITHUB_TOKEN` or `GH_TOKEN` to a token that can read it, and get the script
through the GitHub API:

```sh
sh <(curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/vnd.github.raw" \
  https://api.github.com/repos/hpcsc/svcview/contents/scripts/install.sh) --channel release --dir ~/.local/bin
```

`svcview update` installs a later release. [Install](docs/cli.md#install) and [Build](docs/cli.md#build) in
`docs/cli.md` give the details, and how to build svcview from the source.

## Quick Start

1. Open the screen:

   ```sh
   svcview          # the services with a plist in ~/Library/LaunchAgents
   svcview --all    # each service that launchctl list prints
   ```

   | Key | Action |
   | --- | --- |
   | `↑` `↓`, or `k` `j` | Select a service. |
   | `/` | Filter the list by name. |
   | `a` | Show all services, or only the services with a plist. |
   | `s`, `x`, `r` | Start, stop, or restart the selected service. `x` and `r` ask for a confirmation first. |
   | `o` | Show the log of the selected service. |
   | `?` | Show all keys. |
   | `q` | Quit. |

2. Put your services in a group, and show what they report. Write `~/.config/svcview/config.yaml`:

   ```yaml
   groups:
     - name: Workers
       match: ^com\.example\.worker\.
       status:
         command: [cat, '{{.Env.WORKER_STATUS_FILE}}']
   ```

   The command gets the values of the plist of each service, here `WORKER_STATUS_FILE` from its
   `EnvironmentVariables`. It prints one JSON object, for example:

   ```json
   {"version": 1, "summary": "busy · ENG-692 · 12m", "level": "info"}
   ```

   The list then shows `busy · ENG-692 · 12m` in the row of each worker. To test the command of one service, run
   `svcview status com.example.worker.01`. [The status command](docs/cli.md#the-status-command) in `docs/cli.md`
   gives the fields and the events of a status.

   A service that writes no status file can get its status from a command that builds the JSON, for example from
   its log. svcview runs no shell, so a shell script goes in `sh -c`. Give a template value to the script as an
   argument, here `$1`, and not inside the text of the script:

   ```yaml
   status:
     command:
       - sh
       - -c
       - |
         if tail -n 1 "$1" | grep -q 'Queue empty'; then mode=idle; else mode=busy; fi
         printf '{"version":1,"summary":"%s","fields":[{"name":"mode","value":"%s"}]}\n' "$mode" "$mode"
       - sh
       - '{{.Env.WORKER_LOG}}'
   ```

   The `sh` after the script is `$0`, and the log path is `$1`.
