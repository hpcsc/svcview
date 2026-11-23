//go:build unit

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func touchAfterOneSecond(t *testing.T, marker string) status.Command {
	t.Helper()
	command, err := status.NewCommand([]string{"sh", "-c", `touch "$0.started"; (sleep 1; touch "$0") & sleep 30`, marker}, 0, 10*time.Second)
	require.NoError(t, err)
	return command
}

func TestStatusRuns(t *testing.T) {
	t.Run("stop", func(t *testing.T) {
		t.Run("stops the program of each run that runs, and each process that the program started, so no process writes the file", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			command := touchAfterOneSecond(t, marker)
			runs := newStatusRuns(context.Background())
			ended := make(chan status.Result, 1)
			go func() { ended <- runs.run(command, service.Service{Name: "com.example.worker"}) }()
			require.Eventually(t, func() bool {
				_, err := os.Stat(marker + ".started")
				return err == nil
			}, 5*time.Second, 10*time.Millisecond)

			runs.stop()

			time.Sleep(1500 * time.Millisecond)
			require.NoFileExists(t, marker)
		})

		t.Run("returns only after each run that runs has ended", func(t *testing.T) {
			command := touchAfterOneSecond(t, filepath.Join(t.TempDir(), "written"))
			runs := newStatusRuns(context.Background())
			started := make(chan struct{})
			ended := make(chan status.Result, 1)
			go func() {
				close(started)
				ended <- runs.run(command, service.Service{Name: "com.example.worker"})
			}()
			<-started
			time.Sleep(200 * time.Millisecond)

			runs.stop()
			stopped := time.Now()

			require.False(t, (<-ended).Ended.After(stopped), "stop returned before the run ended")
		})

		t.Run("a run after stop starts no program, and gives a result with no output and no failure", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "written")
			runs := newStatusRuns(context.Background())
			runs.stop()

			before := time.Now()
			result := runs.run(touchAfterOneSecond(t, marker), service.Service{Name: "com.example.worker"})

			require.Nil(t, result.Output)
			require.Nil(t, result.Failure)
			require.WithinRange(t, result.Ended, before, time.Now())
			require.NoFileExists(t, marker+".started")
		})
	})
}
