//go:build unit

package greeting_test

import (
	"testing"

	"github.com/hpcsc/svcview/internal/greeting"
	"github.com/stretchr/testify/require"
)

func TestGreeting(t *testing.T) {
	t.Run("for", func(t *testing.T) {
		t.Run("the greeting names the name", func(t *testing.T) {
			require.Equal(t, "Hello, Alice!", greeting.For("Alice"))
		})

		t.Run("the spaces around the name do not count", func(t *testing.T) {
			require.Equal(t, "Hello, Alice!", greeting.For("  Alice\n"))
		})

		t.Run("a blank name gives a greeting with no name", func(t *testing.T) {
			require.Equal(t, "Hello!", greeting.For(" \t"))
		})
	})
}
