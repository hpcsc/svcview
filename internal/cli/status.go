package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
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
		Usage:     "run the status command of a service one time, and print its template values, its command line, its output and its result",
		ArgsUsage: "<name>",
		Action:    printStatusSteps,
	}
}

func printStatusSteps(ctx context.Context, cmd *cli.Command) error {
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
	if err := printStep(out, "\nCommand line", []string{line}); err != nil {
		return err
	}
	result := command.Run(ctx, svc)
	if result.Output == nil && result.Failure == nil {
		return errors.New("svcview got a signal and stopped the status command before the command ended")
	}
	if err := printStep(out, "\nOutput", stdoutLines(result.Stdout)); err != nil {
		return err
	}
	if err := printStep(out, "\nResult", resultLines(result)); err != nil {
		return err
	}
	if result.Failure != nil {
		return errStatusNotValid
	}
	return nil
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
		lines = append(lines, inColumn(value.Template, text, width))
	}
	return lines
}

func stdoutLines(stdout string) []string {
	if stdout == "" {
		return []string{"the program printed nothing on stdout"}
	}
	return strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
}

func resultLines(result status.Result) []string {
	if failure := result.Failure; failure != nil {
		lines := []string{failure.Kind.String(), failure.Text}
		for _, line := range failure.Lines {
			lines = append(lines, "  "+line)
		}
		return lines
	}
	output := result.Output
	at := "the output has no at"
	if output.At != nil {
		at = output.At.Format(time.RFC3339Nano)
	}
	labelWidth := len("summary")
	lines := []string{
		"valid",
		inColumn("at", at, labelWidth),
		inColumn("summary", output.Summary, labelWidth),
		inColumn("level", output.Level.String(), labelWidth),
	}
	if len(output.Fields) > 0 {
		lines = append(lines, "fields")
		lines = append(lines, fieldLines(output.Fields)...)
	}
	if len(output.Events) > 0 {
		lines = append(lines, "events")
		for _, event := range output.Events {
			lines = append(lines, "  "+event.At.Format(time.RFC3339Nano)+"  "+inColumn(event.Level.String(), event.Text, len("error")))
		}
	}
	return lines
}

func fieldLines(fields []status.Field) []string {
	var width int
	for _, field := range fields {
		width = max(width, utf8.RuneCountInString(field.Name))
	}
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		text := field.Value
		if field.Progress != nil && text != "" {
			text += " "
		}
		if field.Progress != nil {
			text += "(progress " + strconv.FormatFloat(*field.Progress, 'g', -1, 64) + ")"
		}
		lines = append(lines, "  "+inColumn(field.Name, text, width))
	}
	return lines
}

func inColumn(label, text string, width int) string {
	if text == "" {
		return label
	}
	return label + strings.Repeat(" ", width+2-utf8.RuneCountInString(label)) + text
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
