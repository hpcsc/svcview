package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/hpcsc/svcview/internal/cli/version"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/launchd"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
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
	uid := os.Getuid()
	settings, err := config.Read(config.Folder(home, os.Getenv("XDG_CONFIG_HOME")), uid, config.ACLEntries)
	if err != nil {
		return err
	}
	domain := launchd.NewDomain(home, uid)
	runContext, stopRuns := context.WithCancel(ctx)
	var runs sync.WaitGroup
	runStatus := func(command status.Command, svc service.Service) status.Result {
		runs.Add(1)
		defer runs.Done()
		return command.Run(runContext, svc)
	}
	screen := tui.NewScreen(domain.String(), domain.Services, runStatus, settings)
	_, err = tea.NewProgram(screen, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	stopRuns()
	// os/exec kills the program of a run in a goroutine of its own, so svcview must wait for each run to end.
	runs.Wait()
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
