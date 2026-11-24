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

	t.Run("stale", func(t *testing.T) {
		at := time.Date(2026, 11, 23, 9, 12, 0, 0, time.UTC)

		t.Run("is stale when the at of the output is older than stale after", func(t *testing.T) {
			result := status.Result{Ended: ended, Output: &status.Output{Summary: "busy", At: &at}}

			require.True(t, result.Stale(30*time.Second, at.Add(3*time.Minute)))
			require.True(t, result.Stale(30*time.Second, at.Add(30*time.Second+time.Nanosecond)))
		})

		t.Run("is not stale when the at is exactly as old as stale after, newer, or after the given time", func(t *testing.T) {
			result := status.Result{Ended: ended, Output: &status.Output{Summary: "busy", At: &at}}

			require.False(t, result.Stale(30*time.Second, at.Add(30*time.Second)))
			require.False(t, result.Stale(30*time.Second, at.Add(10*time.Second)))
			require.False(t, result.Stale(30*time.Second, at.Add(-time.Minute)))
		})

		t.Run("is never stale when the output has no at, whatever the time at which the run ended", func(t *testing.T) {
			result := status.Result{Ended: ended, Output: &status.Output{Summary: "busy"}}

			require.False(t, result.Stale(30*time.Second, ended.Add(48*time.Hour)))
		})

		t.Run("is never stale when the run gave a failure or no output", func(t *testing.T) {
			for _, result := range []status.Result{
				{Ended: ended, Failure: &status.Failure{Kind: status.FailureKindTimeout, Text: "cat /tmp/worker.json"}},
				{Ended: ended, Output: &status.Output{At: &at}, Failure: &status.Failure{Kind: status.FailureKindInvalid, Text: "the output has more text after its JSON object"}},
				{Ended: ended},
			} {
				require.False(t, result.Stale(30*time.Second, ended.Add(48*time.Hour)))
			}
		})

		t.Run("is never stale with a stale after of 0, also when the at is days old", func(t *testing.T) {
			result := status.Result{Ended: ended, Output: &status.Output{Summary: "busy", At: &at}}

			require.False(t, result.Stale(0, at.Add(72*time.Hour)))
		})

		t.Run("checks the at with the smallest stale after more than 0", func(t *testing.T) {
			result := status.Result{Ended: ended, Output: &status.Output{Summary: "busy", At: &at}}

			require.True(t, result.Stale(time.Nanosecond, at.Add(time.Second)))
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
