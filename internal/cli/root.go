package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/hpcsc/svcview/internal/cli/version"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/launchd"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/terminal"
	"github.com/hpcsc/svcview/internal/tui"
	"github.com/urfave/cli/v3"
)

const releaseRepository = "hpcsc/svcview"

func Run(ctx context.Context) int {
	return runCommand(ctx, newCommand(), os.Args)
}

func runCommand(ctx context.Context, command *cli.Command, arguments []string) int {
	err := command.Run(ctx, arguments)
	switch {
	case err == nil:
		return 0
	case !errors.Is(err, errStatusNotValid):
		color.New(color.FgRed).Fprintln(command.Writer, terminal.ReplaceControlCharacters(err.Error()))
	}
	return 1
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:                  "svcview",
		Version:               version.Current(),
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			newVersionCommand(),
			newUpdateCommand(),
			newStatusSubcommand(),
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
	settings, domain, err := readSettingsAndDomain()
	if err != nil {
		return err
	}
	runs := newStatusRuns(ctx)
	start := func(svc service.Service) error { return domain.Start(ctx, svc) }
	stop := func(svc service.Service) error { return domain.Stop(ctx, svc) }
	screen := tui.NewScreen(domain.String(), domain.Services, runs.run, start, stop, settings)
	_, err = tea.NewProgram(screen, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	runs.stop()
	return err
}

func readSettingsAndDomain() (config.Settings, *launchd.Domain, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config.Settings{}, nil, err
	}
	uid := os.Getuid()
	settings, err := config.Read(config.Folder(home, os.Getenv("XDG_CONFIG_HOME")), uid, config.ACLEntries)
	if err != nil {
		return config.Settings{}, nil, err
	}
	return settings, launchd.NewDomain(home, uid), nil
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
