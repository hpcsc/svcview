package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/hpcsc/svcview/internal/cli/version"
	"github.com/urfave/cli/v3"
)

const releaseRepository = "hpcsc/svcview"

func Run(ctx context.Context) int {
	if err := newCommand().Run(ctx, os.Args); err != nil {
		color.Red(err.Error())
		return 1
	}

	return 0
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:                  "svcview",
		Version:               version.Current(),
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			newVersionCommand(),
			newUpdateCommand(),
		},
	}
}

func newVersionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print the tag svcview was built from, or its commit when it has no tag",
		Action: func(_ context.Context, cmd *cli.Command) error {
			_, err := fmt.Fprintln(cmd.Root().Writer, version.Current())
			return err
		},
	}
}
