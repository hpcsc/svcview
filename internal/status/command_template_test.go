//go:build unit

package status_test

import (
	"strconv"
	"testing"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func newCommandTemplate(t *testing.T, texts ...string) status.CommandTemplate {
	t.Helper()
	commandTemplate, err := status.NewCommandTemplate(texts)
	require.NoError(t, err)
	return commandTemplate
}

func TestCommandTemplate(t *testing.T) {
	t.Run("new", func(t *testing.T) {
		t.Run("a command with no texts gives an error", func(t *testing.T) {
			for _, texts := range [][]string{nil, {}} {
				_, err := status.NewCommandTemplate(texts)

				require.ErrorContains(t, err, "the command is empty", texts)
			}
		})

		t.Run("a command whose program is empty gives an error", func(t *testing.T) {
			_, err := status.NewCommandTemplate([]string{"", "{{.Env.WORKER_LOG}}"})

			require.ErrorContains(t, err, "the program of the command is empty")
		})

		t.Run("a text that is not a Go template gives an error that names the text", func(t *testing.T) {
			text := "{{.Env.WORKER_LOG}"

			_, err := status.NewCommandTemplate([]string{"less", "+F", text})

			require.ErrorContains(t, err, strconv.Quote(text))
			require.ErrorContains(t, err, "not a Go template")
		})

		t.Run("a text that defines a template gives an error that says that a text of a command cannot define one", func(t *testing.T) {
			text := "{{define \"log\"}}{{.Env.WORKER_LOG}}{{end}}"

			_, err := status.NewCommandTemplate([]string{"less", text})

			require.ErrorContains(t, err, strconv.Quote(text))
			require.ErrorContains(t, err, "defines a template, and a text of a command cannot define one")
		})

		t.Run("a text that uses a value other than the five template values gives an error that names the text and the value, also in the program", func(t *testing.T) {
			for _, texts := range [][]string{
				{"less", "{{.Label}}"},
				{"{{.Label}}", "{{.Env.WORKER_LOG}}"},
			} {
				_, err := status.NewCommandTemplate(texts)

				require.ErrorContains(t, err, strconv.Quote("{{.Label}}"), texts)
				require.ErrorContains(t, err, "uses {{.Label}},", texts)
			}
		})
	})

	t.Run("apply", func(t *testing.T) {
		t.Run("gives the command line with each template value of the service, also in the program", func(t *testing.T) {
			commandTemplate := newCommandTemplate(t, "{{.Env.PRINTF}}", "--name={{.Name}}", "{{.Env.WORKER_STATUS_FILE}}", "{{.StdoutPath}}", "{{.StderrPath}}", "{{.PlistPath}}", "+F")

			line, failure := commandTemplate.Apply(worker())

			require.Nil(t, failure)
			require.Equal(t, []string{
				"printf",
				"--name=com.example.worker",
				"/Users/me/worker/status.json",
				"/Users/me/Library/Logs/worker.out.log",
				"/Users/me/Library/Logs/worker.err.log",
				"/Users/me/Library/LaunchAgents/com.example.worker.plist",
				"+F",
			}, line)
		})

		t.Run("a variable that the plist does not have gives the failure of the kind template value that names it", func(t *testing.T) {
			commandTemplate := newCommandTemplate(t, "less", "+F", "{{.Env.WORKER_LOG}}")

			line, failure := commandTemplate.Apply(worker())

			require.Nil(t, line)
			require.Equal(t, &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no WORKER_LOG"}, failure)
		})

		t.Run("a path that the service does not have gives the failure of the kind template value that names it", func(t *testing.T) {
			for text, failure := range map[string]string{
				"{{.StdoutPath}}": "the plist has no StandardOutPath",
				"{{.StderrPath}}": "the plist has no StandardErrorPath",
				"{{.PlistPath}}":  "the service has no plist",
			} {
				commandTemplate := newCommandTemplate(t, "less", text)

				line, got := commandTemplate.Apply(service.Service{Name: "com.example.worker"})

				require.Nil(t, line, text)
				require.Equal(t, &status.Failure{Kind: status.FailureKindTemplateValue, Text: failure}, got, text)
			}
		})
	})
}
