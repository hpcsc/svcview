package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/hpcsc/svcview/internal/cli/version"
	"github.com/hpcsc/svcview/internal/launchd"
	"github.com/hpcsc/svcview/internal/tui"
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
		Action: showScreenOrHelp,
	}
}

func showScreenOrHelp(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		return cli.ShowCommandHelp(ctx, cmd, cmd.Args().First())
	}
	if !isTerminal(cmd.Root().Reader) || !isTerminal(cmd.Root().Writer) {
		return cli.ShowRootCommandHelp(cmd)
	}
	return showScreen(ctx, cmd.Root().Reader, cmd.Root().Writer)
}

func showScreen(ctx context.Context, input io.Reader, output io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	domain := launchd.NewDomain(home, os.Getuid())
	screen := tui.NewScreen(domain.String(), domain.Services)
	_, err = tea.NewProgram(screen, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
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
