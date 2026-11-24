package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/hpcsc/svcview/internal/terminal"
	"github.com/urfave/cli/v3"
)

var errStatusNotValid = errors.New("the status is not valid")

func newStatusSubcommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "print the template values of a service and the command line of the status command of its group",
		ArgsUsage: "<name>",
		Action:    printStatusSteps,
	}
}

func printStatusSteps(_ context.Context, cmd *cli.Command) error {
	svc, command, err := findStatusCommand(cmd.Args().Slice())
	if err != nil {
		return err
	}
	out := cmd.Root().Writer
	if err := printStep(out, "Template values", templateValueLines(svc)); err != nil {
		return err
	}
	line, err := command.CommandLine(svc)
	if err != nil {
		if printErr := printStep(out, "\nCommand line", []string{err.Error()}); printErr != nil {
			return printErr
		}
		return errStatusNotValid
	}
	return printStep(out, "\nCommand line", []string{line})
}

func findStatusCommand(names []string) (service.Service, status.Command, error) {
	switch {
	case len(names) == 0:
		return service.Service{}, status.Command{}, errors.New("svcview status needs the name of a service, such as svcview status com.example.worker")
	case len(names) > 1:
		return service.Service{}, status.Command{}, fmt.Errorf("svcview status takes one name of a service, and it got %d: %s", len(names), strings.Join(names, " "))
	}
	name := names[0]
	settings, domain, err := readSettingsAndDomain()
	if err != nil {
		return service.Service{}, status.Command{}, err
	}
	services, err := domain.Services()
	if err != nil {
		return service.Service{}, status.Command{}, err
	}
	at := slices.IndexFunc(services, func(svc service.Service) bool { return svc.Name == name })
	if at < 0 {
		return service.Service{}, status.Command{}, fmt.Errorf(`no service has the name "%s": svcview knows only the services that have a plist in ~/Library/LaunchAgents`, name)
	}
	group := settings.GroupOf(name)
	if group.Status == nil {
		return service.Service{}, status.Command{}, fmt.Errorf(`the service %s is in the group "%s", which has no status command`, name, group.Name)
	}
	return services[at], *group.Status, nil
}

func templateValueLines(svc service.Service) []string {
	values := status.TemplateValues(svc)
	var width int
	for _, value := range values {
		width = max(width, utf8.RuneCountInString(value.Template))
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		text := value.Value
		if value.Failure != nil {
			text = value.Failure.Text
		}
		line := value.Template
		if text != "" {
			line += strings.Repeat(" ", width+2-utf8.RuneCountInString(value.Template)) + text
		}
		lines = append(lines, line)
	}
	return lines
}

func printStep(out io.Writer, heading string, lines []string) error {
	var step strings.Builder
	step.WriteString(heading + "\n")
	for _, line := range lines {
		step.WriteString("  " + terminal.ReplaceControlCharacters(line) + "\n")
	}
	_, err := io.WriteString(out, step.String())
	return err
}
