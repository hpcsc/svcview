//go:build unit

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// macOS checks each new executable file at its first run, which costs about 0.3 s, so the tests share one fake launchctl.
func TestMain(m *testing.M) {
	folder, err := os.MkdirTemp("", "svcview-fakes-")
	if err != nil {
		panic(err)
	}
	launchctl := "#!/bin/sh\n" +
		"if [ -f \"$HOME/launchctl-failure.txt\" ]; then cat \"$HOME/launchctl-failure.txt\" >&2; exit 5; fi\n" +
		"[ \"$*\" = list ] && exec cat \"$HOME/launchctl-list.txt\"\n" +
		"exit 64\n"
	if err := os.WriteFile(filepath.Join(folder, "launchctl"), []byte(launchctl), 0o755); err != nil {
		panic(err)
	}
	if err := os.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(folder)
	os.Exit(code)
}

func newHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "Library", "LaunchAgents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "launchctl-list.txt"), []byte("PID\tStatus\tLabel\n"), 0o644))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

func addPlist(t *testing.T, home, name, properties string) string {
	t.Helper()
	path := filepath.Join(home, "Library", "LaunchAgents", name+".plist")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>Label</key><string>` + name + `</string>` + properties + `</dict></plist>
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func environment(variables ...string) string {
	var dict strings.Builder
	for i := 0; i < len(variables); i += 2 {
		dict.WriteString("<key>" + variables[i] + "</key><string>" + variables[i+1] + "</string>")
	}
	return "<key>EnvironmentVariables</key><dict>" + dict.String() + "</dict>"
}

func writeConfigFile(t *testing.T, path, content string, mode os.FileMode) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	require.NoError(t, os.Chmod(path, mode))
	return path
}

func writeConfig(t *testing.T, home, content string) string {
	t.Helper()
	return writeConfigFile(t, filepath.Join(home, ".config", "svcview", "config.yaml"), content, 0o644)
}

func runSvcview(t *testing.T, arguments ...string) (string, int) {
	t.Helper()
	return runSvcviewUntil(t, context.Background(), arguments...)
}

func runSvcviewUntil(t *testing.T, ctx context.Context, arguments ...string) (string, int) {
	t.Helper()
	var output bytes.Buffer
	command := newCommand()
	command.Reader = inputThatFailsTheTest{t: t}
	command.Writer = &output
	command.ErrWriter = &output
	command.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	code := runCommand(ctx, command, append([]string{"svcview"}, arguments...))

	return output.String(), code
}

func workersWithCommand(command string) string {
	return "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: " + command + "\n"
}

type outputThatFailsAt struct {
	heading string
}

func (o outputThatFailsAt) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(o.heading+"\n")) {
		return 0, errors.New("no space left on device")
	}
	return len(p), nil
}

const workersWithPrintf = `groups:
  - name: Workers
    match: worker
    status:
      command: [printf, '{"version":1,"summary":"%s"}', '{{.Env.WORKER_MODE}}']
`

func TestStatusSubcommand(t *testing.T) {
	t.Run("steps", func(t *testing.T) {
		t.Run("prints the template values of the service, the command line of the status command of its group, the output and the result, and exits with 0 for a valid status", func(t *testing.T) {
			home := newHome(t)
			plist := addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy", "HOME_DIR", "/Users/me", "WORKER_QUIET", "")+
				"<key>StandardOutPath</key><string>/Users/me/Library/Logs/worker.out.log</string>")
			writeConfig(t, home, workersWithPrintf)

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Equal(t, "Template values\n"+
				"  {{.Name}}"+strings.Repeat(" ", 14)+"com.example.worker\n"+
				"  {{.Env.HOME_DIR}}"+strings.Repeat(" ", 6)+"/Users/me\n"+
				"  {{.Env.WORKER_MODE}}"+strings.Repeat(" ", 3)+"busy\n"+
				"  {{.Env.WORKER_QUIET}}\n"+
				"  {{.StdoutPath}}"+strings.Repeat(" ", 8)+"/Users/me/Library/Logs/worker.out.log\n"+
				"  {{.StderrPath}}"+strings.Repeat(" ", 8)+"the plist has no StandardErrorPath\n"+
				"  {{.PlistPath}}"+strings.Repeat(" ", 9)+plist+"\n"+
				"\n"+
				"Command line\n"+
				`  printf '{"version":1,"summary":"%s"}' busy`+"\n"+
				"\n"+
				"Output\n"+
				`  {"version":1,"summary":"busy"}`+"\n"+
				"\n"+
				"Result\n"+
				"  valid\n"+
				"  at       the output has no at\n"+
				"  summary  busy\n"+
				"  level    info\n", output)
			require.Equal(t, 0, code)
		})

		t.Run("a template value that the plist does not have shows its failure one time under Command line, runs no program, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			marker := filepath.Join(home, "ran")
			writeConfig(t, home, `groups:
  - name: Workers
    match: worker
    status:
      command: [sh, -c, 'touch "$0"', '`+marker+`', '{{.Env.WORKER_STATUS_FILE}}']
`)

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "  {{.Env.WORKER_MODE}}  busy\n")
			require.True(t, strings.HasSuffix(output, "\nCommand line\n  the plist has no WORKER_STATUS_FILE\n"), output)
			require.Equal(t, 1, strings.Count(output, "the plist has no WORKER_STATUS_FILE"), output)
			require.Equal(t, 1, code)
			require.NoFileExists(t, marker)
		})

		t.Run("a valid status exits with 1 when svcview cannot write one of its steps", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf)
			for _, heading := range []string{"Template values", "Command line", "Output", "Result"} {
				command := newCommand()
				command.Reader = inputThatFailsTheTest{t: t}
				command.Writer = outputThatFailsAt{heading: heading}
				command.ErrWriter = outputThatFailsAt{heading: heading}
				command.ExitErrHandler = func(context.Context, *cli.Command, error) {}

				code := runCommand(context.Background(), command, []string{"svcview", "status", "com.example.worker"})

				require.Equal(t, 1, code, heading)
			}
		})

		t.Run("each control character in a variable, a value or the command line prints as �", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER\tMODE", "busy\tidle"))
			writeConfig(t, home, `groups:
  - name: Workers
    match: worker
    status:
      command: [printf, "%s\t", '{{.Name}}']
`)

			output, _ := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "  {{.Env.WORKER�MODE}}  busy�idle\n")
			require.Contains(t, output, "  printf '%s�' com.example.worker\n")
			require.NotContains(t, output, "\t")
		})
	})

	t.Run("run", func(t *testing.T) {
		t.Run("a valid output prints the stdout line by line, then valid with the at, the summary, the level, each field and each event in the order of the output", func(t *testing.T) {
			home := newHome(t)
			statusFile := filepath.Join(home, "status.json")
			require.NoError(t, os.WriteFile(statusFile, []byte(`{
  "version": 1,
  "at": "2026-11-23T09:12:00Z",
  "summary": "busy · ENG-692",
  "level": "warn",
  "fields": [
    {"name": "mode", "value": "busy"},
    {"name": "step", "value": "review", "progress": 0.625},
    {"name": "failed", "value": "", "progress": 0}
  ],
  "events": [
    {"at": "2026-11-23T09:04:40Z", "level": "error", "text": "step review"},
    {"at": "2026-11-23T09:00:12+02:00", "level": "info", "text": "claimed ENG-692"}
  ]
}
`), 0o644))
			addPlist(t, home, "com.example.worker", environment("WORKER_STATUS_FILE", statusFile))
			writeConfig(t, home, workersWithCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`))

			output, code := runSvcview(t, "status", "com.example.worker")

			_, printed, found := strings.Cut(output, "\nOutput\n")
			require.True(t, found, output)
			require.Equal(t, `  {
    "version": 1,
    "at": "2026-11-23T09:12:00Z",
    "summary": "busy · ENG-692",
    "level": "warn",
    "fields": [
      {"name": "mode", "value": "busy"},
      {"name": "step", "value": "review", "progress": 0.625},
      {"name": "failed", "value": "", "progress": 0}
    ],
    "events": [
      {"at": "2026-11-23T09:04:40Z", "level": "error", "text": "step review"},
      {"at": "2026-11-23T09:00:12+02:00", "level": "info", "text": "claimed ENG-692"}
    ]
  }

Result
  valid
  at       2026-11-23T09:12:00Z
  summary  busy · ENG-692
  level    warn
  fields
    mode    busy
    step    review (progress 0.625)
    failed  (progress 0)
  events
    2026-11-23T09:04:40Z  error  step review
    2026-11-23T09:00:12+02:00  info   claimed ENG-692
`, printed)
			require.Equal(t, 0, code)
		})

		t.Run("an output with one field and one event prints the field and the event", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			writeConfig(t, home, workersWithCommand(`[printf, '%s', '{"version":1,"fields":[{"name":"mode","value":"busy"}],"events":[{"at":"2026-11-23T09:04:40Z","level":"warn","text":"step review"}]}']`))

			output, code := runSvcview(t, "status", "com.example.worker")

			require.True(t, strings.HasSuffix(output, "  level    info\n  fields\n    mode  busy\n  events\n    2026-11-23T09:04:40Z  warn   step review\n"), output)
			require.Equal(t, 0, code)
		})

		t.Run("a program that exits with a code other than 0 prints its stdout, then status error, the exit code and the lines of stderr, one time, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			writeConfig(t, home, workersWithCommand(`[sh, -c, 'printf partial; echo first >&2; echo broken >&2; exit 3']`))

			output, code := runSvcview(t, "status", "com.example.worker")

			require.True(t, strings.HasSuffix(output, "\nOutput\n  partial\n\nResult\n  status error\n  exit code 3\n    first\n    broken\n"), output)
			require.Equal(t, 1, strings.Count(output, "exit code 3"), output)
			require.Equal(t, 1, code)
		})

		t.Run("a program that a signal ends, or that svcview cannot start, prints status error with the signal or the reason, and exits with 1", func(t *testing.T) {
			for command, reason := range map[string]string{
				`[sh, -c, 'echo stopping >&2; kill -TERM $$']`: "  ended by SIGTERM\n    stopping\n",
				`[svcview-no-such-program]`:                    "executable file not found",
			} {
				home := newHome(t)
				addPlist(t, home, "com.example.worker", "")
				writeConfig(t, home, workersWithCommand(command))

				output, code := runSvcview(t, "status", "com.example.worker")

				require.Contains(t, output, "\nResult\n  status error\n", command)
				require.Contains(t, output, reason, command)
				require.Equal(t, 1, code, command)
			}
		})

		t.Run("a program that prints nothing on stdout prints that it printed nothing under Output", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			writeConfig(t, home, workersWithCommand(`[sh, -c, 'exit 0']`))

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "\nOutput\n  the program printed nothing on stdout\n\nResult\n  status invalid\n  the output is empty\n")
			require.Equal(t, 1, code)
		})

		t.Run("a program that runs longer than the timeout of its group prints status timeout with the command line no later than 1 s after the timeout, stops each process that it started, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			marker := filepath.Join(home, "written")
			writeConfig(t, home, `groups:
  - name: Workers
    match: worker
    status:
      timeout: 300ms
      command: [sh, -c, '(sleep 1; touch "$0") & sleep 30', '`+marker+`']
`)
			started := time.Now()

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Less(t, time.Since(started), 1300*time.Millisecond)
			require.Contains(t, output, "\nResult\n  status timeout\n  sh -c '(sleep 1; touch \"$0\") & sleep 30' ")
			require.Equal(t, 1, code)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("a program that runs longer than 1 s and less than the timeout of its group gives its result, and exits with 0", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			writeConfig(t, home, `groups:
  - name: Workers
    match: worker
    status:
      timeout: 3s
      command: [sh, -c, 'sleep 1.2 && printf "%s" "$0"', '{"version":1,"summary":"late"}']
`)

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "\nResult\n  valid\n")
			require.Contains(t, output, "  summary  late\n")
			require.Equal(t, 0, code)
		})

		t.Run("an output that is not a JSON object of version 1, that is larger than 1 MiB, or that has a progress outside 0 to 1 prints status invalid with the parse error, and exits with 1", func(t *testing.T) {
			for command, parseError := range map[string]string{
				`[printf, busy]`: "  the output is not valid JSON: ",
				`[printf, '%s', '{"version":1,"fields":[{"name":"step","value":"review","progress":1.5}]}']`: `  "/fields/0/progress" is 1.5, and it must be from 0 to 1` + "\n",
				`[yes]`: "  the output is larger than 1 MiB\n    y\n",
			} {
				home := newHome(t)
				addPlist(t, home, "com.example.worker", "")
				writeConfig(t, home, workersWithCommand(command))

				output, code := runSvcview(t, "status", "com.example.worker")

				require.Contains(t, output, "\nResult\n  status invalid\n"+parseError, command)
				require.Equal(t, 1, code, command)
			}
		})

		t.Run("each control character in the stdout other than the new line, and in the result, prints as �", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			for command, printed := range map[string]string{
				`[printf, "a\e[2Jb\tc\nd"]`:                            "\nOutput\n  a�[2Jb�c\n  d\n",
				`[printf, '%s', '{"version":1,"summary":"a\u001bb"}']`: "  summary  a�b\n",
			} {
				writeConfig(t, home, workersWithCommand(command))

				output, _ := runSvcview(t, "status", "com.example.worker")

				require.Contains(t, output, printed, command)
				require.NotContains(t, output, "\x1b", command)
			}
		})

		t.Run("when the context of svcview ends during the run, the program and each process that it started stop, and svcview exits with 1 and does not print valid", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			marker := filepath.Join(home, "written")
			writeConfig(t, home, `groups:
  - name: Workers
    match: worker
    status:
      timeout: 10s
      command: [sh, -c, '(sleep 1; touch "$0") & sleep 30', '`+marker+`']
`)
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(300*time.Millisecond, cancel)
			started := time.Now()

			output, code := runSvcviewUntil(t, ctx, "status", "com.example.worker")

			require.Less(t, time.Since(started), 2*time.Second)
			require.Contains(t, output, "signal")
			require.NotContains(t, output, "valid")
			require.Equal(t, 1, code)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})
	})

	t.Run("arguments", func(t *testing.T) {
		t.Run("with no name, says that it needs the name of a service, and exits with 1", func(t *testing.T) {
			newHome(t)

			output, code := runSvcview(t, "status")

			require.Contains(t, output, "needs the name of a service")
			require.Equal(t, 1, code)
		})

		t.Run("with more than one name, says that it takes one name, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf)

			output, code := runSvcview(t, "status", "com.example.worker", "com.example.other")

			require.Contains(t, output, "takes one name of a service")
			require.NotContains(t, output, "Template values")
			require.Equal(t, 1, code)
		})
	})

	t.Run("find the service", func(t *testing.T) {
		t.Run("a name that no service has gives a cause that names the name and the folder of the plists, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf)

			output, code := runSvcview(t, "status", "com.example.workers")

			require.Contains(t, output, `"com.example.workers"`)
			require.Contains(t, output, "~/Library/LaunchAgents")
			require.NotContains(t, output, "Template values")
			require.Equal(t, 1, code)
		})

		t.Run("a service whose group has no status command gives a cause that names the service and the group, also for the group Other, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			addPlist(t, home, "com.example.job", "")
			writeConfig(t, home, "groups:\n  - name: Workers\n    match: worker\n")

			for name, group := range map[string]string{"com.example.worker": `"Workers"`, "com.example.job": `"Other"`} {
				output, code := runSvcview(t, "status", name)

				require.Contains(t, output, name, name)
				require.Contains(t, output, group, name)
				require.Contains(t, output, "has no status command", name)
				require.NotContains(t, output, "Template values", name)
				require.Equal(t, 1, code, name)
			}
		})

		t.Run("each control character in a name or a group prints as �", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, "groups:\n  - name: \"Work\\eers\"\n    match: worker\n")

			for name, printed := range map[string]string{"com.example.worker": `"Work�ers"`, "com.\x1b[2Jexample": "com.�[2Jexample"} {
				output, code := runSvcview(t, "status", name)

				require.Contains(t, output, printed, name)
				require.NotContains(t, output, "\x1b", name)
				require.Equal(t, 1, code, name)
			}
		})
	})

	t.Run("config", func(t *testing.T) {
		t.Run("a config.yaml or a file of conf.d that another user can write stops it with the error of the screen before it prints a step, and exits with 1", func(t *testing.T) {
			for _, file := range []string{"config.yaml", "conf.d/worker.yaml"} {
				home := newHome(t)
				addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
				path := writeConfigFile(t, filepath.Join(home, ".config", "svcview", file), workersWithPrintf, 0o666)

				output, code := runSvcview(t, "status", "com.example.worker")

				require.Contains(t, output, "another user can write the config file "+path, file)
				require.NotContains(t, output, "Template values", file)
				require.Equal(t, 1, code, file)
			}
		})

		t.Run("an error in the group of another service stops it before it prints a step, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf+"  - name: Jobs\n    match: job\n    colour: blue\n")

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, `"colour"`)
			require.NotContains(t, output, "Template values")
			require.Equal(t, 1, code)
		})

		t.Run("with no HOME, says that $HOME is not defined before it prints a step, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf)
			t.Setenv("HOME", "")

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "$HOME is not defined")
			require.NotContains(t, output, "Template values")
			require.Equal(t, 1, code)
		})

		t.Run("reads the config folder of XDG_CONFIG_HOME when it is set", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			xdgConfigHome := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
			writeConfigFile(t, filepath.Join(xdgConfigHome, "svcview", "config.yaml"), workersWithPrintf, 0o644)

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, `printf '{"version":1,"summary":"%s"}' busy`)
			require.Equal(t, 0, code)
		})
	})

	t.Run("read the services", func(t *testing.T) {
		t.Run("an error of launchctl list stops it with that error before it prints a step, and exits with 1", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", environment("WORKER_MODE", "busy"))
			writeConfig(t, home, workersWithPrintf)
			require.NoError(t, os.WriteFile(filepath.Join(home, "launchctl-failure.txt"), []byte("Could not connect to launchd"), 0o644))

			output, code := runSvcview(t, "status", "com.example.worker")

			require.Contains(t, output, "launchctl list")
			require.Contains(t, output, "Could not connect to launchd")
			require.NotContains(t, output, "Template values")
			require.Equal(t, 1, code)
		})
	})

	t.Run("help", func(t *testing.T) {
		t.Run("svcview --help, and svcview with no command and no terminal, list status with its usage", func(t *testing.T) {
			newHome(t)

			for _, arguments := range [][]string{{"--help"}, {}} {
				output, code := runSvcview(t, arguments...)

				require.Contains(t, output, "status", arguments)
				require.Contains(t, output, "run the status command of a service one time", arguments)
				require.Equal(t, 0, code, arguments)
			}
		})

		t.Run("svcview status --help shows that it takes a <name>, and reads no config file", func(t *testing.T) {
			home := newHome(t)
			writeConfigFile(t, filepath.Join(home, ".config", "svcview", "config.yaml"), workersWithPrintf, 0o666)

			output, code := runSvcview(t, "status", "--help")

			require.Contains(t, output, "<name>")
			require.NotContains(t, output, "another user can write")
			require.Equal(t, 0, code)
		})
	})
}
