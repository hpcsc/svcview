//go:build unit

package cli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
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

		t.Run("with --all and without a terminal, prints the commands and reads no input", func(t *testing.T) {
			output, err := run(t, "--all")

			require.NoError(t, err)
			require.Contains(t, output, "print the tag svcview was built from")
			require.Contains(t, output, "replace svcview with the latest release")
		})
	})

	t.Run("help", func(t *testing.T) {
		t.Run("lists --all under GLOBAL OPTIONS, with its usage and no default value", func(t *testing.T) {
			output, err := run(t, "--help")

			require.NoError(t, err)
			_, options, found := strings.Cut(output, "GLOBAL OPTIONS:")
			require.True(t, found, output)
			require.Regexp(t, `(?m)^\s+--all\s+show each service that launchctl list prints, also the services of macOS$`, options)
		})
	})

	t.Run("subcommand", func(t *testing.T) {
		t.Run("svcview status does not take --all: it fails with the error of a flag that it does not define, and runs no status command", func(t *testing.T) {
			home := newHome(t)
			addPlist(t, home, "com.example.worker", "")
			marker := filepath.Join(home, "ran")
			writeConfig(t, home, workersWithCommand(`[touch, '`+marker+`']`))

			_, err := run(t, "status", "--all", "com.example.worker")

			require.ErrorContains(t, err, "flag provided but not defined: -all")
			require.NoFileExists(t, marker)
		})
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Run("says that svcview has no help for it", func(t *testing.T) {
			_, err := run(t, "unknown")

			require.ErrorContains(t, err, "No help topic for 'unknown'")
		})
	})
}
