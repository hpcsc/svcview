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

`docs/cli.md` describes each key and each setting. `docs/proposal-launchd-tui.md` gives the design.

## Quick Start

1. Build svcview. It needs [mise](https://mise.jdx.dev/):

   ```sh
   mise install
   task build
   ```

2. Open the screen:

   ```sh
   ./bin/svcview          # the services with a plist in ~/Library/LaunchAgents
   ./bin/svcview --all    # each service that launchctl list prints
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

3. Put your services in a group, and show what they report. Write `~/.config/svcview/config.yaml`:

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
   `./bin/svcview status com.example.worker.01`. [The status command](docs/cli.md#the-status-command) in
   `docs/cli.md` gives the fields and the events of a status.

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

## Development

### Requirements

[mise](https://mise.jdx.dev/) installs Go and the tools that the tasks use: Task, gotestsum, ShellCheck and
govulncheck. `mise.toml` gives their versions.

```sh
mise install
```

### Test

```sh
task test:unit              # the unit tests, with the race detector and coverage
task test:shellcheck        # the shell scripts and the hooks
task test:vulnerabilities   # the known vulnerabilities of the module and of its dependencies
```

The unit tests have the `unit` build tag, so `go test ./...` without `-tags unit` runs none of them.

### Set Up the Hook

```sh
task hook:setup
```

Before a push, the hook `hooks/pre-push` runs `task test:shellcheck` when the push changes a shell script or a
hook, and `task hook:validate-go-changes` when the push changes a Go file. For a new branch, it compares the branch
with the main branch. Set `SKIP_PREPUSH_HOOK=true` to skip the hook for one push.

### Layout

| Path | Contents |
| --- | --- |
| `cmd/svcview` | The `main` package. It runs the root command of `internal/cli`. |
| `internal/service` | The type `Service`, which the other packages use. |
| `internal/launchd` | Reads the plists in `~/Library/LaunchAgents`, runs `launchctl list` and `ps`, and returns `service.Service` values. It is the only package that knows launchd. |
| `internal/tui` | The Bubble Tea model of the screen, and its views. It does not import `internal/launchd`. |
| `internal/cli` | The commands. With no command, in a terminal, the root command opens the screen. In all other cases, it prints the help. |
| `e2e/` | The end-to-end tests. `docs/e2e-tests.md` describes them. |
| `hooks/pre-push`, `scripts/libs/git-hook-prepush.sh` | The pre-push hook. |
| `.github/workflows/ci.yml` | The CI workflow. On each push, it checks the shell scripts, builds the module, and runs the unit tests and the check for vulnerabilities. |
| `Taskfile.yml`, `Taskfile.test.yml` | The tasks. |
