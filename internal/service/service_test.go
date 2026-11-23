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
