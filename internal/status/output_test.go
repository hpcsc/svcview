//go:build unit

package status_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLevel(t *testing.T) {
	t.Run("word", func(t *testing.T) {
		t.Run("the level of an output gives the word that the output wrote, and an output with no level gives info", func(t *testing.T) {
			for stdout, word := range map[string]string{
				`{"version":1,"level":"info"}`:  "info",
				`{"version":1,"level":"warn"}`:  "warn",
				`{"version":1,"level":"error"}`: "error",
				`{"version":1}`:                 "info",
			} {
				result := printing(t, stdout).Run(context.Background(), worker())

				require.NotNil(t, result.Output, stdout)
				require.Equal(t, word, result.Output.Level.String(), stdout)
			}
		})
	})
}
