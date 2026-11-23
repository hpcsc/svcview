package launchd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hpcsc/svcview/internal/service"
)

const (
	listHeader  = "PID\tStatus\tLabel"
	listTimeout = 5 * time.Second
)

type list struct {
	services map[string]service.Service
}

func readList() (list, error) {
	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "launchctl", "list")
	command.WaitDelay = time.Second
	stdout, err := command.Output()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return list{}, fmt.Errorf("launchctl list did not end within %s", listTimeout)
	}
	if exited, ok := errors.AsType[*exec.ExitError](err); ok {
		printed := strings.TrimSpace(strings.TrimSpace(string(exited.Stderr)) + "\n" + strings.TrimSpace(string(stdout)))
		return list{}, fmt.Errorf("launchctl list: %w: %s", err, printed)
	}
	if err != nil {
		return list{}, fmt.Errorf("run launchctl list: %w", err)
	}
	return parseList(string(stdout))
}

func parseList(output string) (list, error) {
	header, rows, _ := strings.Cut(output, "\n")
	if header != listHeader {
		return list{}, fmt.Errorf("launchctl list printed the header line %q, not %q, so svcview does not read its rows", header, listHeader)
	}
	l := list{services: map[string]service.Service{}}
	for line := range strings.Lines(rows) {
		row := strings.TrimSuffix(line, "\n")
		columns := strings.SplitN(row, "\t", 3)
		if len(columns) != 3 {
			return list{}, fmt.Errorf("launchctl list printed the row %q, which does not have the 3 columns PID, Status and Label", row)
		}
		pid, name := columns[0], columns[2]
		state := service.Running
		if pid == "-" {
			state = service.Loaded
		}
		l.services[name] = service.Service{Name: name, State: state}
	}
	return l, nil
}

func (l list) service(name string) service.Service {
	if svc, ok := l.services[name]; ok {
		return svc
	}
	return service.Service{Name: name, State: service.NotLoaded}
}
