package launchd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hpcsc/svcview/internal/service"
)

type list struct {
	services map[string]service.Service
}

func readList() (list, error) {
	output, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return list{}, fmt.Errorf("run launchctl list: %w", err)
	}
	return parseList(string(output))
}

func parseList(output string) (list, error) {
	l := list{services: map[string]service.Service{}}
	_, rows, _ := strings.Cut(output, "\n")
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
