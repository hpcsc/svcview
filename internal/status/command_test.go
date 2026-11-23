//go:build unit

package status_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func newCommand(t *testing.T, timeout time.Duration, texts ...string) status.Command {
	t.Helper()
	command, err := status.NewCommand(texts, 0, timeout)
	require.NoError(t, err)
	return command
}

func printing(t *testing.T, stdout string) status.Command {
	t.Helper()
	return newCommand(t, 0, "printf", "%s", stdout)
}

func worker() service.Service {
	return service.Service{
		Name: "com.example.worker",
		Env: map[string]string{
			"WORKER_STATUS_FILE": "/Users/me/worker/status.json",
			"WORKER_QUIET":       "",
			"PRINTF":             "printf",
		},
		StdoutPath: "/Users/me/Library/Logs/worker.out.log",
		StderrPath: "/Users/me/Library/Logs/worker.err.log",
		PlistPath:  "/Users/me/Library/LaunchAgents/com.example.worker.plist",
	}
}

func summaryOf(t *testing.T, result status.Result) string {
	t.Helper()
	require.Nil(t, result.Failure)
	require.NotNil(t, result.Output)
	return result.Output.Summary
}

func TestCommand(t *testing.T) {
	t.Run("new", func(t *testing.T) {
		t.Run("a command has its every and its timeout, and an every of 0 takes the default of 2s and a timeout of 0 the default of 1s", func(t *testing.T) {
			for _, durations := range [][4]time.Duration{
				{0, 0, 2 * time.Second, time.Second},
				{0, 3 * time.Second, 2 * time.Second, 3 * time.Second},
				{5 * time.Second, 0, 5 * time.Second, time.Second},
			} {
				command, err := status.NewCommand([]string{"cat", "status.json"}, durations[0], durations[1])

				require.NoError(t, err, durations)
				require.Equal(t, [2]time.Duration{durations[2], durations[3]}, [2]time.Duration{command.Every, command.Timeout}, durations)
			}
		})

		t.Run("an every or a timeout less than 0 gives an error", func(t *testing.T) {
			for _, durations := range [][2]time.Duration{{-time.Nanosecond, 0}, {0, -time.Nanosecond}} {
				_, err := status.NewCommand([]string{"cat", "status.json"}, durations[0], durations[1])

				require.ErrorContains(t, err, "less than 0", durations)
			}
		})

		t.Run("each text can use the five template values, also the program, and a text with no template value is valid too", func(t *testing.T) {
			for _, texts := range [][]string{
				{"cat", "{{.Env.WORKER_STATUS_FILE}}"},
				{"{{.Env.WORKER_HOME}}/bin/worker", "status", "--name={{.Name}}", "--out", "{{.StdoutPath}}", "--err", "{{.StderrPath}}", "{{.PlistPath}}"},
				{"cat", ""},
				{"{{$.Name}}", "{{.Name | printf \"%s.json\"}}", "{{printf \"%s %s\" .StdoutPath .StderrPath}}"},
				{"{{if .Env.VERBOSE}}--verbose{{else}}--quiet{{end}}", "{{if .Name}}{{.PlistPath}}{{end}}", "{{$file := .Env.FILE}}{{$file}}"},
				{"{{/* the file of the worker */}}{{.Name}}"},
			} {
				_, err := status.NewCommand(texts, 0, 0)

				require.NoError(t, err, texts)
			}
		})

		t.Run("in a with, a text can use the value of the with as ., the five template values after $, and the five values as . in its else", func(t *testing.T) {
			for _, text := range []string{
				"{{with .PlistPath}}{{.}}{{end}}",
				"{{with .PlistPath}}{{$.Name}}{{end}}",
				"{{with $file := .Env.FILE}}{{$file}}{{end}}",
				"{{with .Env.FILE}}{{else}}{{.PlistPath}}{{end}}",
				"{{with .PlistPath}}{{if .}}{{.}}{{end}}{{end}}",
			} {
				_, err := status.NewCommand([]string{"cat", text}, 0, 0)

				require.NoError(t, err, text)
			}
		})

		t.Run("a command with no texts gives an error", func(t *testing.T) {
			for _, texts := range [][]string{nil, {}} {
				_, err := status.NewCommand(texts, 0, 0)

				require.ErrorContains(t, err, "the command is empty", texts)
			}
		})

		t.Run("a command whose program is empty gives an error", func(t *testing.T) {
			_, err := status.NewCommand([]string{"", "status.json"}, 0, 0)

			require.ErrorContains(t, err, "the program of the command is empty")
		})

		t.Run("a text that is not a Go template gives an error that names the text", func(t *testing.T) {
			for _, text := range []string{"{{.Env.WORKER_STATUS_FILE}", "{{.Name", "{{if .Name}}", "{{nofunction .Name}}"} {
				_, err := status.NewCommand([]string{"cat", "status.json", text}, 0, 0)

				require.ErrorContains(t, err, strconv.Quote(text), text)
				require.ErrorContains(t, err, "not a Go template", text)
			}
		})

		t.Run("a text that uses a value other than the five template values gives an error that names the text and the value", func(t *testing.T) {
			for text, value := range map[string]string{
				"{{.Label}}":                                          "{{.Label}}",
				"{{.Env}}":                                            "{{.Env}}",
				"{{.Env.WORKER.FILE}}":                                "{{.Env.WORKER.FILE}}",
				"{{.Name.First}}":                                     "{{.Name.First}}",
				"{{.}}":                                               "{{.}}",
				"{{$}}":                                               "{{$}}",
				"{{$.Label}}":                                         "{{$.Label}}",
				"{{(.Env).FILE}}":                                     "{{(.Env).FILE}}",
				"--name={{.Name}} --label={{.Label}}":                 "{{.Label}}",
				"{{.Name | printf .Label}}":                           "{{.Label}}",
				"{{printf \"%s %s\" .Name .Label}}":                   "{{.Label}}",
				"{{if .Label}}{{end}}":                                "{{.Label}}",
				"{{if .Name}}{{.Label}}{{end}}":                       "{{.Label}}",
				"{{if .Name}}{{else}}{{.Label}}{{end}}":               "{{.Label}}",
				"{{with .Label}}{{end}}":                              "{{.Label}}",
				"{{with .Name}}{{else}}{{.Label}}{{end}}":             "{{.Label}}",
				"{{with .PlistPath}}{{.Name}}{{end}}":                 "{{.Name}} in a with",
				"{{range .Env.FILES}}{{end}}":                         "{{range}}",
				"{{range .Env.FILES}}{{else}}{{.Name}}{{end}}":        "{{range}}",
				"{{if .Name}}{{range .Label}}{{end}}{{end}}":          "{{range}}",
				"{{with .Name}}{{with .PlistPath}}{{end}}{{end}}":     "{{.PlistPath}} in a with",
				"{{with .Name}}{{$.Name}}{{(.Env).FILE}}{{end}}":      "{{(.Env).FILE}}",
				"{{with .PlistPath}}{{if .}}{{.Name}}{{end}}{{end}}":  "{{.Name}} in a with",
				"{{$file := .Env.WORKER_STATUS_FILE}}{{$file.Path}}":  "{{$file.Path}}",
				"{{$ := .Name}}{{$.Name}}":                            "{{$ := .Name}}",
				"{{$ = .Name}}":                                       "{{$ = .Name}}",
				"{{with $ := .Name}}{{end}}":                          "{{$ := .Name}}",
				"{{template \"file\"}}":                               "{{template \"file\"}}",
				"{{if .Name}}{{template \"file\" .PlistPath}}{{end}}": "{{template \"file\" .PlistPath}}",
			} {
				_, err := status.NewCommand([]string{"cat", text}, 0, 0)

				require.ErrorContains(t, err, strconv.Quote(text), text)
				require.ErrorContains(t, err, "uses "+value+",", text)
			}
		})

		t.Run("a text that defines a template gives an error that names the text", func(t *testing.T) {
			for _, text := range []string{
				"{{define \"file\"}}{{.Name}}{{end}}{{template \"file\"}}",
				"{{define \"file\"}}{{end}}",
				"{{block \"file\" .}}{{.Name}}{{end}}",
			} {
				_, err := status.NewCommand([]string{"cat", text}, 0, 0)

				require.ErrorContains(t, err, strconv.Quote(text), text)
				require.ErrorContains(t, err, "defines a template", text)
			}
		})

		t.Run("a program that uses a value other than the five template values gives an error", func(t *testing.T) {
			_, err := status.NewCommand([]string{"{{.Label}}", "status.json"}, 0, 0)

			require.ErrorContains(t, err, "uses {{.Label}},")
		})
	})

	t.Run("run", func(t *testing.T) {
		t.Run("each template value gives the value of the service, and a variable with an empty value gives an empty text", func(t *testing.T) {
			command := newCommand(t, 0, "printf", `{"version":1,"summary":"%s,%s,%s,%s,%s,[%s]"}`,
				"{{.Name}}", "{{.Env.WORKER_STATUS_FILE}}", "{{.StdoutPath}}", "{{.StderrPath}}", "{{.PlistPath}}", "{{.Env.WORKER_QUIET}}")

			result := command.Run(context.Background(), worker())

			require.Equal(t, "com.example.worker,/Users/me/worker/status.json,/Users/me/Library/Logs/worker.out.log,/Users/me/Library/Logs/worker.err.log,/Users/me/Library/LaunchAgents/com.example.worker.plist,[]", summaryOf(t, result))
		})

		t.Run("the program can use a template value", func(t *testing.T) {
			command := newCommand(t, 0, "{{.Env.PRINTF}}", `{"version":1,"summary":"%s"}`, "{{.Name}}")

			result := command.Run(context.Background(), worker())

			require.Equal(t, "com.example.worker", summaryOf(t, result))
		})

		t.Run("each text that NewCommand accepts gives its value at the run", func(t *testing.T) {
			for text, expected := range map[string]string{
				"{{$.Name}}":                                              "com.example.worker",
				`{{.Name | printf "%s.json"}}`:                            "com.example.worker.json",
				"{{with .PlistPath}}{{.}}{{end}}":                         "/Users/me/Library/LaunchAgents/com.example.worker.plist",
				"{{with .Env.WORKER_QUIET}}{{else}}{{.Name}}{{end}}":      "com.example.worker",
				"{{if .Env.WORKER_QUIET}}--quiet{{else}}--verbose{{end}}": "--verbose",
				"{{$file := .Env.WORKER_STATUS_FILE}}{{$file}}":           "/Users/me/worker/status.json",
			} {
				command := newCommand(t, 0, "printf", `{"version":1,"summary":"%s"}`, text)

				result := command.Run(context.Background(), worker())

				require.Equal(t, expected, summaryOf(t, result), text)
			}
		})

		t.Run("a template value that the service does not have gives a failure that names it, and no program runs", func(t *testing.T) {
			for text, failure := range map[string]string{
				"{{.Env.WORKER_LOG}}":        "the plist has no WORKER_LOG",
				"--log={{.Env.WORKER_LOG}}":  "the plist has no WORKER_LOG",
				"{{.StdoutPath}}":            "the plist has no StandardOutPath",
				"{{.StderrPath}}":            "the plist has no StandardErrorPath",
				"{{.PlistPath}}":             "the service has no plist",
				"{{with .PlistPath}}{{end}}": "the service has no plist",
			} {
				marker := filepath.Join(t.TempDir(), "ran")
				command := newCommand(t, 0, "sh", "-c", `touch "$0"`, marker, text)

				result := command.Run(context.Background(), service.Service{Name: "com.example.worker", Env: map[string]string{"WORKER_STATUS_FILE": "/Users/me/worker/status.json"}})

				require.Equal(t, &status.Failure{Text: failure}, result.Failure, text)
				require.Nil(t, result.Output, text)
				require.NoFileExists(t, marker, text)
			}
		})

		t.Run("a text whose template stops for another reason gives a failure with the error of the template, and no program runs", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran")
			command := newCommand(t, 0, "sh", "-c", `touch "$0"`, marker, "{{slice .Name 100}}")

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Contains(t, result.Failure.Text, "slice")
			require.Contains(t, result.Failure.Text, "out of range")
			require.NoFileExists(t, marker)
		})

		t.Run("a variable of a service with no environment gives a failure that names it", func(t *testing.T) {
			command := newCommand(t, 0, "cat", "{{.Env.WORKER_STATUS_FILE}}")

			result := command.Run(context.Background(), service.Service{Name: "com.example.worker"})

			require.Equal(t, &status.Failure{Text: "the plist has no WORKER_STATUS_FILE"}, result.Failure)
		})

		t.Run("the program runs with its arguments and no shell, so $HOME, ;, |, * and a space reach the program as they are", func(t *testing.T) {
			command := newCommand(t, 0, "printf", `{"version":1,"summary":"%s"}`, "$HOME ; | * two  words")

			result := command.Run(context.Background(), worker())

			require.Equal(t, "$HOME ; | * two  words", summaryOf(t, result))
		})

		t.Run("a JSON object of version 1 on stdout gives an output with its summary, its level, its at, its fields and its events, in their order", func(t *testing.T) {
			command := printing(t, `{
				"version": 1,
				"at": "2026-11-23T09:12:00Z",
				"summary": "busy · ENG-692 · 12m",
				"level": "warn",
				"fields": [
					{"name": "mode", "value": "busy"},
					{"name": "step", "value": "review · 12m / 20m", "progress": 0.6},
					{"name": "done", "value": "14", "progress": 0},
					{"name": "failed", "value": "", "progress": 1}
				],
				"events": [
					{"at": "2026-11-23T09:04:40Z", "level": "error", "text": "step review"},
					{"at": "2026-11-23T09:00:12+02:00", "level": "info", "text": "claimed ENG-692"},
					{"at": "2026-11-23T08:59:00.5Z", "level": "warn", "text": ""}
				]
			}`)

			result := command.Run(context.Background(), worker())

			require.Nil(t, result.Failure)
			require.Equal(t, &status.Output{
				At:      new(time.Date(2026, 11, 23, 9, 12, 0, 0, time.UTC)),
				Summary: "busy · ENG-692 · 12m",
				Level:   status.LevelWarn,
				Fields: []status.Field{
					{Name: "mode", Value: "busy"},
					{Name: "step", Value: "review · 12m / 20m", Progress: new(0.6)},
					{Name: "done", Value: "14", Progress: new(0.0)},
					{Name: "failed", Value: "", Progress: new(1.0)},
				},
				Events: []status.Event{
					{At: time.Date(2026, 11, 23, 9, 4, 40, 0, time.UTC), Level: status.LevelError, Text: "step review"},
					{At: time.Date(2026, 11, 23, 7, 0, 12, 0, time.UTC), Level: status.LevelInfo, Text: "claimed ENG-692"},
					{At: time.Date(2026, 11, 23, 8, 59, 0, 500_000_000, time.UTC), Level: status.LevelWarn, Text: ""},
				},
			}, normalized(result.Output))
		})

		t.Run("a JSON object with only its version gives an output with no at, no summary, no fields, no events, and the level info, also with a new line after the object", func(t *testing.T) {
			for _, stdout := range []string{`{"version":1}`, "{\"version\":1}\n", ` {"version": 1, "at": null, "summary": null, "level": null, "fields": null, "events": null} `} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.Equal(t, &status.Output{Level: status.LevelInfo}, result.Output, stdout)
			}
		})

		t.Run("each level gives its level", func(t *testing.T) {
			for text, level := range map[string]status.Level{"info": status.LevelInfo, "warn": status.LevelWarn, "error": status.LevelError} {
				result := printing(t, `{"version":1,"level":"`+text+`"}`).Run(context.Background(), worker())

				require.Equal(t, &status.Output{Level: level}, result.Output, text)
			}
		})

		t.Run("a property that svcview does not know changes nothing", func(t *testing.T) {
			result := printing(t, `{"version":1,"summary":"busy","stale_after":"30s","Summary":"other","extra":{"a":[1,2]},"fields":[{"name":"mode","value":"busy","unit":"none"}],"events":[{"at":"2026-11-23T09:04:40Z","level":"info","text":"step review","id":7}]}`).Run(context.Background(), worker())

			require.Equal(t, &status.Output{
				Summary: "busy",
				Fields:  []status.Field{{Name: "mode", Value: "busy"}},
				Events:  []status.Event{{At: time.Date(2026, 11, 23, 9, 4, 40, 0, time.UTC), Text: "step review"}},
			}, normalized(result.Output))
		})

		t.Run("a stdout that is not a JSON object of version 1 gives a result with no output and no failure", func(t *testing.T) {
			for _, stdout := range []string{
				``,
				"\n",
				`{}`,
				`null`,
				`{"summary":"busy"}`,
				`{"version":2,"summary":"busy"}`,
				`{"version":"1","summary":"busy"}`,
				`{"version":1.5,"summary":"busy"}`,
				`{"version":null,"summary":"busy"}`,
				`{"version":1,"level":"debug"}`,
				`{"version":1,"level":"WARN"}`,
				`{"version":1,"level":2}`,
				`{"version":1,"at":"2026-11-23 09:12:00"}`,
				`{"version":1,"at":"yesterday"}`,
				`{"version":1,"at":1700000000}`,
				`{"version":1,"summary":3}`,
				`{"version":1,"summary":["busy"]}`,
				`{"version":1,"fields":{"mode":"busy"}}`,
				`{"version":1,"fields":[{"value":"busy"}]}`,
				`{"version":1,"fields":[{"name":"mode"}]}`,
				`{"version":1,"fields":[{"name":"done","value":14}]}`,
				`{"version":1,"fields":[{"name":"step","value":"review","progress":1.5}]}`,
				`{"version":1,"fields":[{"name":"step","value":"review","progress":-0.1}]}`,
				`{"version":1,"fields":[{"name":"step","value":"review","progress":"60%"}]}`,
				`{"version":1,"fields":["mode"]}`,
				`{"version":1,"events":[{"level":"info","text":"step review"}]}`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","text":"step review"}]}`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"info"}]}`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"debug","text":"step review"}]}`,
				`{"version":1,"events":[{"at":"yesterday","level":"info","text":"step review"}]}`,
				`{"version":1,"events":{"at":"2026-11-23T09:04:40Z","level":"info","text":"step review"}}`,
				`[{"version":1}]`,
				`"version 1"`,
				`1`,
				`{"version":1}{"version":1}`,
				`{"version":1} {"version":1}`,
				`{"version":1} trailing text`,
				`{"version":1`,
				`{"version":1,"summary":"busy","summary":"idle"}`,
				`busy`,
			} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.Equal(t, [2]any{(*status.Output)(nil), (*status.Failure)(nil)}, [2]any{result.Output, result.Failure}, stdout)
			}
		})

		t.Run("a program that exits with a code other than 0 gives a result with no output, also when it prints a JSON object of version 1", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; exit 3`)

			result := command.Run(context.Background(), worker())

			require.Nil(t, result.Output)
			require.Nil(t, result.Failure)
		})

		t.Run("a program that is not found gives a result with no output", func(t *testing.T) {
			command := newCommand(t, 0, "svcview-no-such-program", "status.json")

			result := command.Run(context.Background(), worker())

			require.Nil(t, result.Output)
			require.Nil(t, result.Failure)
		})

		t.Run("a program that runs longer than the timeout stops, and gives a result with no output no later than 1 s after the timeout, also when a child of the program keeps its stdout open", func(t *testing.T) {
			timeout := 200 * time.Millisecond
			marker := filepath.Join(t.TempDir(), "ended")
			command := newCommand(t, timeout, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; sleep 3; touch "$0"`, marker)
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), timeout+1500*time.Millisecond)
			time.Sleep(3 * time.Second)
			require.NoFileExists(t, marker)
		})

		t.Run("a program that stops when its context ends gives a result with no output", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			command := newCommand(t, 10*time.Second, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; sleep 60; echo never`)
			started := time.Now()
			time.AfterFunc(100*time.Millisecond, cancel)

			result := command.Run(ctx, worker())

			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), 3*time.Second)
		})

		t.Run("a stdout of 1 MiB gives an output with its summary, and a stdout of 1 byte more gives no output", func(t *testing.T) {
			wrapper := len(`{"version":1,"summary":""}`)
			for size, valid := range map[int]bool{1 << 20: true, 1<<20 + 1: false} {
				summary := strings.Repeat("a", size-wrapper)
				file := filepath.Join(t.TempDir(), "status.json")
				require.NoError(t, os.WriteFile(file, []byte(`{"version":1,"summary":"`+summary+`"}`), 0o644))

				result := newCommand(t, 0, "cat", file).Run(context.Background(), worker())

				if valid {
					require.Equal(t, summary, summaryOf(t, result))
				} else {
					require.Nil(t, result.Output)
				}
			}
		})

		t.Run("a program that prints with no end stops after 1 MiB, before its timeout, with no output", func(t *testing.T) {
			command := newCommand(t, 10*time.Second, "yes")
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), 5*time.Second)
		})

		t.Run("the result has the time at which the run ended", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `sleep 0.3; printf '{"version":1}'`)
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.WithinRange(t, result.Ended, started.Add(300*time.Millisecond), time.Now())
		})

		t.Run("the time of a result is the at of its output", func(t *testing.T) {
			result := printing(t, `{"version":1,"at":"2026-11-23T09:12:00Z"}`).Run(context.Background(), worker())

			require.Equal(t, time.Date(2026, 11, 23, 9, 12, 0, 0, time.UTC), result.Time().UTC())
		})

		t.Run("the time of a result is the time at which the run ended when the output has no at, when the run gave no output, and when it gave a failure", func(t *testing.T) {
			for _, command := range []status.Command{
				printing(t, `{"version":1}`),
				printing(t, `not a JSON object`),
				newCommand(t, 0, "cat", "{{.Env.WORKER_LOG}}"),
			} {
				result := command.Run(context.Background(), worker())

				require.Equal(t, result.Ended, result.Time())
				require.False(t, result.Ended.IsZero())
			}
		})

		t.Run("the result of a failure has the time at which the run ended", func(t *testing.T) {
			command := newCommand(t, 0, "cat", "{{.Env.WORKER_LOG}}")
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.WithinRange(t, result.Ended, started, time.Now())
		})
	})
}

func normalized(output *status.Output) *status.Output {
	if output == nil {
		return nil
	}
	if output.At != nil {
		output.At = new(output.At.UTC())
	}
	for i := range output.Events {
		output.Events[i].At = output.Events[i].At.UTC()
	}
	return output
}
