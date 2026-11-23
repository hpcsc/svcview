//go:build unit

package status_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

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
}
