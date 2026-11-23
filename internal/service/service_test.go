//go:build unit

package service_test

import (
	"testing"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/stretchr/testify/require"
)

func TestState(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		t.Run("reads as running, loaded and not loaded", func(t *testing.T) {
			require.Equal(t, "running", service.Running.String())
			require.Equal(t, "loaded", service.Loaded.String())
			require.Equal(t, "not loaded", service.NotLoaded.String())
		})
	})
}

func TestLastExit(t *testing.T) {
	t.Run("failed", func(t *testing.T) {
		t.Run("an exit code other than 0 is failed", func(t *testing.T) {
			require.True(t, service.LastExit{Code: 78}.Failed())
		})

		t.Run("any signal is failed", func(t *testing.T) {
			require.True(t, service.LastExit{Signal: "SIGTERM"}.Failed())
		})

		t.Run("an exit code of 0 with no signal is not failed", func(t *testing.T) {
			require.False(t, service.LastExit{}.Failed())
		})
	})
}
