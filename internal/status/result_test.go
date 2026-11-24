//go:build unit

package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func TestResult(t *testing.T) {
	ended := time.Date(2026, 11, 23, 9, 12, 30, 0, time.UTC)

	t.Run("time", func(t *testing.T) {
		t.Run("is the at of the output when the output has at", func(t *testing.T) {
			at := time.Date(2026, 11, 23, 9, 12, 0, 0, time.UTC)

			result := status.Result{Ended: ended, Output: &status.Output{At: &at}}

			require.Equal(t, at, result.Time())
		})

		t.Run("is the time at which the run ended when the output has no at, when the run gave no output, and when it gave a failure", func(t *testing.T) {
			for _, result := range []status.Result{
				{Ended: ended, Output: &status.Output{Summary: "busy"}},
				{Ended: ended},
				{Ended: ended, Failure: &status.Failure{Text: "the plist has no WORKER_STATUS_FILE"}},
			} {
				require.Equal(t, ended, result.Time())
			}
		})
	})
}

func TestFailureKind(t *testing.T) {
	t.Run("word", func(t *testing.T) {
		t.Run("the failure of each run gives the word that the row of the screen shows for its kind", func(t *testing.T) {
			for word, command := range map[string]status.Command{
				"status error":   newCommand(t, 0, "sh", "-c", "exit 3"),
				"status timeout": newCommand(t, 100*time.Millisecond, "sleep", "30"),
				"status invalid": printing(t, "busy"),
				"template value": newCommand(t, 0, "cat", "{{.Env.WORKER_LOG}}"),
			} {
				result := command.Run(context.Background(), worker())

				require.NotNil(t, result.Failure, word)
				require.Equal(t, word, result.Failure.Kind.String(), word)
			}
		})
	})
}
