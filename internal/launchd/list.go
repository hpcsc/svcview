package launchd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
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
		pid, status, name := columns[0], columns[1], columns[2]
		state := service.Running
		if pid == "-" {
			state = service.Loaded
		}
		code, err := strconv.ParseInt(status, 10, 32)
		if err != nil {
			return list{}, fmt.Errorf("launchctl list printed the row %q, whose Status is not an exit code or the negative number of a signal", row)
		}
		l.services[name] = service.Service{Name: name, State: state, LastExit: lastExit(int(code))}
	}
	return l, nil
}

func (l list) service(name string) service.Service {
	if svc, ok := l.services[name]; ok {
		return svc
	}
	return service.Service{Name: name, State: service.NotLoaded}
}

var macOSSignals = [...]string{
	1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 5: "SIGTRAP", 6: "SIGABRT", 7: "SIGEMT", 8: "SIGFPE",
	9: "SIGKILL", 10: "SIGBUS", 11: "SIGSEGV", 12: "SIGSYS", 13: "SIGPIPE", 14: "SIGALRM", 15: "SIGTERM",
	16: "SIGURG", 17: "SIGSTOP", 18: "SIGTSTP", 19: "SIGCONT", 20: "SIGCHLD", 21: "SIGTTIN", 22: "SIGTTOU",
	23: "SIGIO", 24: "SIGXCPU", 25: "SIGXFSZ", 26: "SIGVTALRM", 27: "SIGPROF", 28: "SIGWINCH", 29: "SIGINFO",
	30: "SIGUSR1", 31: "SIGUSR2",
}

func lastExit(status int) service.LastExit {
	if status >= 0 {
		return service.LastExit{Code: status}
	}
	signal := -status
	if signal < len(macOSSignals) {
		return service.LastExit{Signal: macOSSignals[signal]}
	}
	return service.LastExit{Signal: fmt.Sprintf("signal %d", signal)}
}
