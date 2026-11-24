//go:build unit

package status_test

import (
	"context"
	"testing"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func TestTemplateValue(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		t.Run("lists the name, each variable of the environment in the order of the names, and the three paths, each with the value of the service", func(t *testing.T) {
			svc := service.Service{
				Name: "com.example.worker",
				Env: map[string]string{
					"WORKER_STATUS_FILE": "/Users/me/worker/status.json",
					"HOME_DIR":           "/Users/me",
					"WORKER_QUIET":       "",
				},
				StdoutPath: "/Users/me/Library/Logs/worker.out.log",
				StderrPath: "/Users/me/Library/Logs/worker.err.log",
				PlistPath:  "/Users/me/Library/LaunchAgents/com.example.worker.plist",
			}

			values := status.TemplateValues(svc)

			require.Equal(t, []status.TemplateValue{
				{Template: "{{.Name}}", Value: "com.example.worker"},
				{Template: "{{.Env.HOME_DIR}}", Value: "/Users/me"},
				{Template: "{{.Env.WORKER_QUIET}}", Value: ""},
				{Template: "{{.Env.WORKER_STATUS_FILE}}", Value: "/Users/me/worker/status.json"},
				{Template: "{{.StdoutPath}}", Value: "/Users/me/Library/Logs/worker.out.log"},
				{Template: "{{.StderrPath}}", Value: "/Users/me/Library/Logs/worker.err.log"},
				{Template: "{{.PlistPath}}", Value: "/Users/me/Library/LaunchAgents/com.example.worker.plist"},
			}, values)
		})

		t.Run("a service with no environment lists no variable", func(t *testing.T) {
			svc := service.Service{Name: "com.example.worker", StdoutPath: "/tmp/out.log", StderrPath: "/tmp/err.log", PlistPath: "/tmp/worker.plist"}

			values := status.TemplateValues(svc)

			require.Equal(t, []status.TemplateValue{
				{Template: "{{.Name}}", Value: "com.example.worker"},
				{Template: "{{.StdoutPath}}", Value: "/tmp/out.log"},
				{Template: "{{.StderrPath}}", Value: "/tmp/err.log"},
				{Template: "{{.PlistPath}}", Value: "/tmp/worker.plist"},
			}, values)
		})

		t.Run("a variable whose name a template cannot write after .Env is listed too", func(t *testing.T) {
			svc := service.Service{Name: "com.example.worker", Env: map[string]string{"WORKER-HOME": "/Users/me/worker"}, StdoutPath: "/tmp/out.log", StderrPath: "/tmp/err.log", PlistPath: "/tmp/worker.plist"}

			values := status.TemplateValues(svc)

			require.Contains(t, values, status.TemplateValue{Template: "{{.Env.WORKER-HOME}}", Value: "/Users/me/worker"})
		})

		t.Run("a service with no stdout path, no stderr path and no plist gives for each of these values a failure of the kind template value and no value", func(t *testing.T) {
			svc := service.Service{Name: "com.example.worker"}

			values := status.TemplateValues(svc)

			require.Equal(t, []status.TemplateValue{
				{Template: "{{.Name}}", Value: "com.example.worker"},
				{Template: "{{.StdoutPath}}", Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no StandardOutPath"}},
				{Template: "{{.StderrPath}}", Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no StandardErrorPath"}},
				{Template: "{{.PlistPath}}", Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the service has no plist"}},
			}, values)
		})

		t.Run("each failure is the failure that a run of a text with that value gives", func(t *testing.T) {
			svc := service.Service{Name: "com.example.worker"}

			for _, value := range status.TemplateValues(svc)[1:] {
				result := newCommand(t, 0, "cat", value.Template).Run(context.Background(), svc)

				require.Equal(t, result.Failure, value.Failure, value.Template)
			}
		})
	})
}
