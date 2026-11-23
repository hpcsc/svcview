//go:build unit

package service_test

import (
	"testing"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSignalName(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		t.Run("a signal of macOS has its name, and another number reads as signal and the number", func(t *testing.T) {
			names := map[int]string{}
			for _, signal := range []int{1, 9, 15, 31, 0, 32, 2147483648} {
				names[signal] = service.SignalName(signal)
			}

			require.Equal(t, map[int]string{
				1:          "SIGHUP",
				9:          "SIGKILL",
				15:         "SIGTERM",
				31:         "SIGUSR2",
				0:          "signal 0",
				32:         "signal 32",
				2147483648: "signal 2147483648",
			}, names)
		})
	})
}
