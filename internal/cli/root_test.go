//go:build unit

package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

type inputThatFailsTheTest struct {
	t *testing.T
}

func (i inputThatFailsTheTest) Read([]byte) (int, error) {
	i.t.Error("svcview read its input")
	return 0, io.EOF
}

func TestRootCommand(t *testing.T) {
	run := func(t *testing.T, arguments ...string) (string, error) {
		t.Helper()
		var output bytes.Buffer
		command := newCommand()
		command.Reader = inputThatFailsTheTest{t: t}
		command.Writer = &output
		command.ErrWriter = io.Discard
		command.ExitErrHandler = func(context.Context, *cli.Command, error) {}

		err := command.Run(context.Background(), append([]string{"svcview"}, arguments...))

		return output.String(), err
	}

	t.Run("no command", func(t *testing.T) {
		t.Run("without a terminal, prints the commands and reads no input", func(t *testing.T) {
			output, err := run(t)

			require.NoError(t, err)
			require.Contains(t, output, "print the tag svcview was built from")
			require.Contains(t, output, "replace svcview with the latest release")
		})
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Run("says that svcview has no help for it", func(t *testing.T) {
			_, err := run(t, "unknown")

			require.ErrorContains(t, err, "No help topic for 'unknown'")
		})
	})
}
