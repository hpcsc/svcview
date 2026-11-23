# svcview

The Go module `github.com/hpcsc/svcview`. `internal/greeting` is an example package with unit tests. Replace
it with your own packages.

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
| `internal/greeting` | The example package and its unit tests. |
| `hooks/pre-push`, `scripts/libs/git-hook-prepush.sh` | The pre-push hook. |
| `.github/workflows/ci.yml` | The CI workflow. On each push, it checks the shell scripts, builds the module, and runs the unit tests and the check for vulnerabilities. |
| `Taskfile.yml`, `Taskfile.test.yml` | The tasks. |
