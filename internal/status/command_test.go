//go:build unit

package status_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

				require.Equal(t, &status.Failure{Kind: status.FailureKindTemplateValue, Text: failure}, result.Failure, text)
				require.Nil(t, result.Output, text)
				require.NoFileExists(t, marker, text)
			}
		})

		t.Run("a text whose template stops for another reason gives a failure with the error of the template, and no program runs", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran")
			command := newCommand(t, 0, "sh", "-c", `touch "$0"`, marker, "{{slice .Name 100}}")

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTemplateValue, result.Failure.Kind)
			require.Contains(t, result.Failure.Text, "slice")
			require.Contains(t, result.Failure.Text, "out of range")
			require.NoFileExists(t, marker)
		})

		t.Run("a variable of a service with no environment gives a failure that names it", func(t *testing.T) {
			command := newCommand(t, 0, "cat", "{{.Env.WORKER_STATUS_FILE}}")

			result := command.Run(context.Background(), service.Service{Name: "com.example.worker"})

			require.Equal(t, &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no WORKER_STATUS_FILE"}, result.Failure)
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

		t.Run("a stdout that is not a JSON object of version 1 gives a status invalid with a parse error that says why", func(t *testing.T) {
			rfc3339 := `an RFC 3339 time in a text, such as "2026-11-23T09:12:00Z"`
			for stdout, parseError := range map[string]string{
				``:                                  "the output is empty",
				" \n\t":                             "the output is empty",
				`null`:                              "the output is a JSON null, not an object",
				`[{"version":1}]`:                   "the output is a JSON list, not an object",
				`"version 1"`:                       "the output is a JSON text, not an object",
				`1`:                                 "the output is a JSON number, not an object",
				`true`:                              "the output is a JSON boolean, not an object",
				`false`:                             "the output is a JSON boolean, not an object",
				`{"version":1}{"version":1}`:        "the output has more text after its JSON object",
				`{"version":1} {"version":1}`:       "the output has more text after its JSON object",
				`{"version":1} trailing text`:       "the output has more text after its JSON object",
				`{"version":1}x`:                    "the output has more text after its JSON object",
				`{}`:                                `"/version" is missing, and it must be 1`,
				`{"summary":"busy"}`:                `"/version" is missing, and it must be 1`,
				`{"version":null,"summary":"busy"}`: `"/version" is missing, and it must be 1`,
				`{"version":2,"summary":"busy"}`:    `"/version" is 2, and it must be 1`,
				`{"version":0}`:                     `"/version" is 0, and it must be 1`,
				`{"version":"1","summary":"busy"}`:  `"/version" must be the number 1`,
				`{"version":1.5,"summary":"busy"}`:  `"/version" must be the number 1`,
				`{"version":1,"level":"debug"}`:     `"/level" is "debug", and it must be info, warn or error`,
				`{"version":1,"level":"WARN"}`:      `"/level" is "WARN", and it must be info, warn or error`,
				`{"version":1,"level":2}`:           `"/level" must be the text info, warn or error`,
				`{"version":1,"at":"2026-11-23 09:12:00"}`:                                                 `"/at" must be ` + rfc3339,
				`{"version":1,"at":"yesterday"}`:                                                           `"/at" must be ` + rfc3339,
				`{"version":1,"at":1700000000}`:                                                            `"/at" must be ` + rfc3339,
				`{"version":1,"summary":3}`:                                                                `"/summary" must be a text`,
				`{"version":1,"summary":["busy"]}`:                                                         `"/summary" must be a text`,
				`{"version":1,"fields":{"mode":"busy"}}`:                                                   `"/fields" must be a list of objects`,
				`{"version":1,"fields":["mode"]}`:                                                          `"/fields/0" must be an object`,
				`{"version":1,"fields":[{"name":"mode","value":"busy"},{"value":"busy"}]}`:                 `"/fields/1/name" is missing`,
				`{"version":1,"fields":[{"name":"mode","value":"busy"},{"name":"step"}]}`:                  `"/fields/1/value" is missing`,
				`{"version":1,"fields":[{"name":3,"value":"14"}]}`:                                         `"/fields/0/name" must be a text`,
				`{"version":1,"fields":[{"name":"done","value":14}]}`:                                      `"/fields/0/value" must be a text`,
				`{"version":1,"fields":[{"name":"step","value":"review","progress":"60%"}]}`:               `"/fields/0/progress" must be a number`,
				`{"version":1,"events":{"at":"2026-11-23T09:04:40Z","level":"info","text":"step review"}}`: `"/events" must be a list of objects`,
				`{"version":1,"events":["step review"]}`:                                                   `"/events/0" must be an object`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"info","text":"a"},{"level":"info","text":"step review"}]}`: `"/events/1/at" is missing`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","text":"step review"}]}`:                                            `"/events/0/level" is missing`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"info"}]}`:                                                  `"/events/0/text" is missing`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"debug","text":"step review"}]}`:                            `"/events/0/level" is "debug", and it must be info, warn or error`,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":3,"text":"step review"}]}`:                                  `"/events/0/level" must be the text info, warn or error`,
				`{"version":1,"events":[{"at":"yesterday","level":"info","text":"step review"}]}`:                                        `"/events/0/at" must be ` + rfc3339,
				`{"version":1,"events":[{"at":"2026-11-23T09:04:40Z","level":"info","text":false}]}`:                                     `"/events/0/text" must be a text`,
			} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.NotNil(t, result.Failure, stdout)
				require.Equal(t, [2]any{status.FailureKindInvalid, parseError}, [2]any{result.Failure.Kind, result.Failure.Text}, stdout)
				require.Nil(t, result.Output, stdout)
			}
		})

		t.Run("a stdout that is not valid JSON gives a status invalid with the error of the JSON parser", func(t *testing.T) {
			for stdout, reason := range map[string]string{
				`busy`:         `'b'`,
				`{"version":1`: "EOF",
				`{"version":1,"summary":"busy","summary":"idle"}`: `duplicate`,
			} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.NotNil(t, result.Failure, stdout)
				require.Equal(t, status.FailureKindInvalid, result.Failure.Kind, stdout)
				require.True(t, strings.HasPrefix(result.Failure.Text, "the output is not valid JSON: "), result.Failure.Text)
				require.Contains(t, result.Failure.Text, reason, stdout)
				require.NotContains(t, result.Failure.Text, "jsontext:", stdout)
			}
		})

		t.Run("a status invalid has the lines of the output, and keeps only the first 20 lines of an output with more", func(t *testing.T) {
			var thirty, first20 []string
			for i := 1; i <= 30; i++ {
				thirty = append(thirty, "line "+strconv.Itoa(i))
			}
			first20 = thirty[:20]
			for stdout, lines := range map[string][]string{
				"busy":                            {"busy"},
				"{\n\"version\": 2\n}\n":          {"{", `"version": 2`, "}"},
				"first\n\nthird":                  {"first", "", "third"},
				strings.Join(thirty, "\n") + "\n": first20,
				strings.Join(thirty[:20], "\n"):   first20,
			} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.NotNil(t, result.Failure, stdout)
				require.Equal(t, lines, result.Failure.Lines, stdout)
			}
		})

		t.Run("a field whose progress is outside 0 to 1 gives an output with that progress", func(t *testing.T) {
			result := printing(t, `{"version":1,"fields":[{"name":"step","value":"review","progress":1.5},{"name":"done","value":"0","progress":-0.1}]}`).Run(context.Background(), worker())

			require.Nil(t, result.Failure)
			require.Equal(t, &status.Output{Fields: []status.Field{
				{Name: "step", Value: "review", Progress: new(1.5)},
				{Name: "done", Value: "0", Progress: new(-0.1)},
			}}, result.Output)
		})

		t.Run("a program that exits with a code other than 0 gives a status error with the exit code and the lines of stderr, also when it prints a JSON object of version 1", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; echo first >&2; echo second >&2; exit 3`)

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindError, Text: "exit code 3", Lines: []string{"first", "second"}}, result.Failure)
			require.Nil(t, result.Output)
		})

		t.Run("a status error keeps only the last 20 lines of stderr, also for a program that writes many more", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `i=1; while [ $i -le 5000 ]; do echo "line $i" >&2; i=$((i+1)); done; exit 1`)

			result := command.Run(context.Background(), worker())

			var last20 []string
			for i := 4981; i <= 5000; i++ {
				last20 = append(last20, "line "+strconv.Itoa(i))
			}
			require.Equal(t, &status.Failure{Kind: status.FailureKindError, Text: "exit code 1", Lines: last20}, result.Failure)
		})

		t.Run("a status error keeps only the last 20 lines of stderr when the last line has no new line", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `i=1; while [ $i -le 25 ]; do echo "line $i" >&2; i=$((i+1)); done; printf last >&2; exit 1`)

			result := command.Run(context.Background(), worker())

			var last20 []string
			for i := 7; i <= 25; i++ {
				last20 = append(last20, "line "+strconv.Itoa(i))
			}
			require.Equal(t, &status.Failure{Kind: status.FailureKindError, Text: "exit code 1", Lines: append(last20, "last")}, result.Failure)
		})

		t.Run("a status error has a last line of stderr with no new line, and keeps no empty line for the new line at the end", func(t *testing.T) {
			for stderr, lines := range map[string][]string{
				`first\nlast`:   {"first", "last"},
				`first\nx`:      {"first", "x"},
				`first\nlast\n`: {"first", "last"},
				`first\n\nlast`: {"first", "", "last"},
				`\n`:            {""},
				``:              nil,
			} {
				command := newCommand(t, 0, "sh", "-c", `printf "$0" >&2; exit 1`, stderr)

				result := command.Run(context.Background(), worker())

				require.NotNil(t, result.Failure, stderr)
				require.Equal(t, lines, result.Failure.Lines, stderr)
			}
		})

		t.Run("a status error keeps the first 4096 bytes of a longer line of stderr", func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "stderr.txt")
			require.NoError(t, os.WriteFile(file, []byte(strings.Repeat("a", 4000)+strings.Repeat("b", 1<<20)+"\nnext"), 0o644))
			command := newCommand(t, 0, "sh", "-c", `cat "$0" >&2; exit 1`, file)

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, []string{strings.Repeat("a", 4000) + strings.Repeat("b", 96), "next"}, result.Failure.Lines)
		})

		t.Run("a program that exits with 0 while a process that it started keeps its stdout open gives a status error with the lines of stderr, no later than 1 s after the exit, and that process stops", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 10*time.Second, "sh", "-c", `echo started >&2; (sleep 2; touch "$0") & printf '{"version":1,"summary":"busy"}'`, marker)
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindError, Text: exec.ErrWaitDelay.Error(), Lines: []string{"started"}}, result.Failure)
			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), 1800*time.Millisecond)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("when the program ends, each process that it started and that still runs stops, also when the process does not keep the output of the program", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 10*time.Second, "sh", "-c", `(sleep 1; touch "$0") >/dev/null 2>&1 & printf '{"version":1,"summary":"busy"}'`, marker)

			result := command.Run(context.Background(), worker())

			require.Equal(t, "busy", summaryOf(t, result))
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("a program that a signal ends gives a status error with the name of the signal", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `echo stopping >&2; kill -TERM $$`)

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindError, Text: "ended by SIGTERM", Lines: []string{"stopping"}}, result.Failure)
			require.Nil(t, result.Output)
		})

		t.Run("a program that svcview cannot start gives a status error that says why", func(t *testing.T) {
			notExecutable := filepath.Join(t.TempDir(), "status")
			require.NoError(t, os.WriteFile(notExecutable, []byte("#!/bin/sh\necho '{\"version\":1}'\n"), 0o644))
			for program, why := range map[string]string{
				"svcview-no-such-program": `"svcview-no-such-program": executable file not found in $PATH`,
				notExecutable:             "permission denied",
			} {
				result := newCommand(t, 0, program, "status.json").Run(context.Background(), worker())

				require.NotNil(t, result.Failure, program)
				require.Equal(t, status.FailureKindError, result.Failure.Kind, program)
				require.Contains(t, result.Failure.Text, why, program)
				require.Nil(t, result.Output, program)
			}
		})

		t.Run("a program that runs longer than the timeout gives a status timeout with the command line, no later than 1 s after the timeout", func(t *testing.T) {
			timeout := 200 * time.Millisecond
			command := newCommand(t, timeout, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; sleep 30`, "{{.Name}}")
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindTimeout, Text: `sh -c 'printf '\''{"version":1,"summary":"busy"}'\''; sleep 30' com.example.worker`}, result.Failure)
			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), timeout+time.Second)
		})

		t.Run("the command line of a status timeout quotes each text that a shell reads as more than one word or as other text, and leaves the other texts as they are", func(t *testing.T) {
			for text, quoted := range map[string]string{
				"--name=com.example.worker":  "--name=com.example.worker",
				"/Users/me/a_b@c%d+e:f,g.h":  "/Users/me/a_b@c%d+e:f,g.h",
				"two words":                  "'two words'",
				"":                           "''",
				"it's":                       `'it'\''s'`,
				"$HOME;*|&<>()\\\"`~#!?[]{}": `'$HOME;*|&<>()\"` + "`" + `~#!?[]{}'`,
				"é":                          "'é'",
			} {
				result := newCommand(t, 100*time.Millisecond, "sh", "-c", "sleep 30", text).Run(context.Background(), worker())

				require.Equal(t, &status.Failure{Kind: status.FailureKindTimeout, Text: "sh -c 'sleep 30' " + quoted}, result.Failure, text)
			}
		})

		t.Run("at the timeout, each process that the program started stops too", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 200*time.Millisecond, "sh", "-c", `(sleep 1; touch "$0") & sleep 30`, marker)

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTimeout, result.Failure.Kind)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("a program that exits before the timeout, while a process that it started keeps its stdout open past the timeout, gives a status timeout at the timeout, and that process stops", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 300*time.Millisecond, "sh", "-c", `(sleep 0.7; touch "$0") & printf '{"version":1,"summary":"busy"}'`, marker)
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTimeout, result.Failure.Kind)
			require.Less(t, time.Since(started), 650*time.Millisecond)
			time.Sleep(time.Second)
			require.NoFileExists(t, marker)
		})

		t.Run("at the timeout, a process that the program started and that ignores SIGTERM stops too", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 200*time.Millisecond, "sh", "-c", `trap "" TERM; (sleep 1; touch "$0") & sleep 30`, marker)

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTimeout, result.Failure.Kind)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("the stop at the timeout sends no signal to a process outside the process group of the program, such as a process in the group of svcview", func(t *testing.T) {
			outside := exec.Command("sleep", "30")
			require.NoError(t, outside.Start())
			outsideEnded := make(chan struct{})
			go func() {
				_ = outside.Wait()
				close(outsideEnded)
			}()
			t.Cleanup(func() {
				_ = outside.Process.Kill()
				<-outsideEnded
			})
			command := newCommand(t, 200*time.Millisecond, "sh", "-c", `sleep 30 & sleep 30`)

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTimeout, result.Failure.Kind)
			require.Never(t, func() bool {
				select {
				case <-outsideEnded:
					return true
				default:
					return false
				}
			}, 300*time.Millisecond, 10*time.Millisecond)
		})

		t.Run("a timeout that ends before the program starts gives a status timeout", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran")
			command := newCommand(t, time.Nanosecond, "sh", "-c", `touch "$0"`, marker)

			result := command.Run(context.Background(), worker())

			require.NotNil(t, result.Failure)
			require.Equal(t, status.FailureKindTimeout, result.Failure.Kind)
			require.Contains(t, result.Failure.Text, `sh -c 'touch "$0"' `)
			require.NoFileExists(t, marker)
		})

		t.Run("a context that ended before the start gives a result with no output and no failure", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			marker := filepath.Join(t.TempDir(), "ran")
			command := newCommand(t, 0, "sh", "-c", `touch "$0"`, marker)

			result := command.Run(ctx, worker())

			require.Equal(t, [2]any{(*status.Output)(nil), (*status.Failure)(nil)}, [2]any{result.Output, result.Failure})
			require.NoFileExists(t, marker)
		})

		t.Run("when its context ends, the program and each process that it started stop, and the result has no output and no failure", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			marker := filepath.Join(t.TempDir(), "written")
			command := newCommand(t, 10*time.Second, "sh", "-c", `printf '{"version":1,"summary":"busy"}'; (sleep 1; touch "$0") & sleep 30`, marker)
			started := time.Now()
			time.AfterFunc(100*time.Millisecond, cancel)

			result := command.Run(ctx, worker())

			require.Equal(t, [2]any{(*status.Output)(nil), (*status.Failure)(nil)}, [2]any{result.Output, result.Failure})
			require.Less(t, time.Since(started), time.Second)
			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("a stdout of 1 MiB gives an output with its summary, and a stdout of 1 byte more gives a status invalid with the first 4096 bytes of its line", func(t *testing.T) {
			wrapper := len(`{"version":1,"summary":""}`)
			for _, size := range []int{1 << 20, 1<<20 + 1} {
				summary := strings.Repeat("a", size-wrapper)
				file := filepath.Join(t.TempDir(), "status.json")
				require.NoError(t, os.WriteFile(file, []byte(`{"version":1,"summary":"`+summary+`"}`), 0o644))

				result := newCommand(t, 0, "cat", file).Run(context.Background(), worker())

				if size == 1<<20 {
					require.Equal(t, summary, summaryOf(t, result))
					continue
				}
				require.Equal(t, &status.Failure{
					Kind:  status.FailureKindInvalid,
					Text:  "the output is larger than 1 MiB",
					Lines: []string{(`{"version":1,"summary":"` + summary)[:4096]},
				}, result.Failure)
				require.Nil(t, result.Output)
			}
		})

		t.Run("a program that prints with no end stops after 1 MiB, before its timeout, with a status invalid and the first 20 lines of the output", func(t *testing.T) {
			command := newCommand(t, 10*time.Second, "yes")
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindInvalid, Text: "the output is larger than 1 MiB", Lines: slices.Repeat([]string{"y"}, 20)}, result.Failure)
			require.Nil(t, result.Output)
			require.Less(t, time.Since(started), 5*time.Second)
		})

		t.Run("a program that prints more than 1 MiB and then runs longer than the timeout gives a status timeout, not a status invalid", func(t *testing.T) {
			command := newCommand(t, 300*time.Millisecond, "sh", "-c", `head -c 2000000 /dev/zero; sleep 30`)

			result := command.Run(context.Background(), worker())

			require.Equal(t, &status.Failure{Kind: status.FailureKindTimeout, Text: "sh -c 'head -c 2000000 /dev/zero; sleep 30'"}, result.Failure)
		})

		t.Run("the result has the time at which the run ended", func(t *testing.T) {
			command := newCommand(t, 0, "sh", "-c", `sleep 0.3; printf '{"version":1}'`)
			started := time.Now()

			result := command.Run(context.Background(), worker())

			require.WithinRange(t, result.Ended, started.Add(300*time.Millisecond), time.Now())
		})

		t.Run("the result of a failure has the time at which the run ended, also when svcview cannot start the program", func(t *testing.T) {
			for _, texts := range [][]string{{"cat", "{{.Env.WORKER_LOG}}"}, {"svcview-no-such-program"}} {
				command := newCommand(t, 0, texts...)
				started := time.Now()

				result := command.Run(context.Background(), worker())

				require.NotNil(t, result.Failure, texts)
				require.WithinRange(t, result.Ended, started, time.Now(), texts)
			}
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
