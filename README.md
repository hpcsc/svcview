# svcview

svcview is a terminal UI for the launchd services of the user on macOS. With no command, in a terminal, it opens a
screen that lists each service that has a plist in `~/Library/LaunchAgents`. `docs/proposal-launchd-tui.md` gives
the design.

## Requirements

[mise](https://mise.jdx.dev/) installs Go and the tools that the tasks use: Task, gotestsum, ShellCheck and
govulncheck. `mise.toml` gives their versions.

```sh
mise install
```

## Test

```sh
task test:unit              # the unit tests, with the race detector and coverage
task test:shellcheck        # the shell scripts and the hooks
task test:vulnerabilities   # the known vulnerabilities of the module and of its dependencies
```

The unit tests have the `unit` build tag, so `go test ./...` without `-tags unit` runs none of them.

## Set Up the Hook

```sh
task hook:setup
```

Before a push, the hook `hooks/pre-push` runs `task test:shellcheck` when the push changes a shell script or a
hook, and `task hook:validate-go-changes` when the push changes a Go file. For a new branch, it compares the branch
with the main branch. Set `SKIP_PREPUSH_HOOK=true` to skip the hook for one push.

## Layout

| Path | Contents |
| --- | --- |
| `cmd/svcview` | The `main` package. It runs the root command of `internal/cli`. |
| `internal/service` | The type `Service`, which the other packages use. |
| `internal/launchd` | Reads the plists in `~/Library/LaunchAgents`, and returns `service.Service` values. It is the only package that knows launchd. |
| `internal/tui` | The Bubble Tea model of the screen, and its views. It does not import `internal/launchd`. |
| `internal/cli` | The commands. With no command, in a terminal, the root command opens the screen. In all other cases, it prints the help. |
| `e2e/` | The end-to-end tests. `docs/e2e-tests.md` describes them. |
| `hooks/pre-push`, `scripts/libs/git-hook-prepush.sh` | The pre-push hook. |
| `.github/workflows/ci.yml` | The CI workflow. On each push, it checks the shell scripts, builds the module, and runs the unit tests and the check for vulnerabilities. |
| `Taskfile.yml`, `Taskfile.test.yml` | The tasks. |
