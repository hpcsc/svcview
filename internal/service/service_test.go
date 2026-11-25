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

func TestService(t *testing.T) {
	t.Run("log path", func(t *testing.T) {
		t.Run("is the StandardOutPath when the plist has both paths", func(t *testing.T) {
			svc := service.Service{StdoutPath: "/Users/me/Library/Logs/a.out.log", StderrPath: "/Users/me/Library/Logs/a.err.log"}

			require.Equal(t, "/Users/me/Library/Logs/a.out.log", svc.LogPath())
		})

		t.Run("is the StandardErrorPath when the plist has no StandardOutPath", func(t *testing.T) {
			svc := service.Service{StderrPath: "/Users/me/Library/Logs/a.err.log"}

			require.Equal(t, "/Users/me/Library/Logs/a.err.log", svc.LogPath())
		})

		t.Run("is empty when the plist has neither path", func(t *testing.T) {
			require.Empty(t, service.Service{Name: "com.example.a"}.LogPath())
		})
	})
}
