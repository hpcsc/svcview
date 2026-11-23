package launchd

import (
	"path/filepath"
	"strconv"

	"github.com/hpcsc/svcview/internal/service"
)

type Domain struct {
	uid    int
	plists *plistFolder
}

func NewDomain(home string, uid int) *Domain {
	return &Domain{uid: uid, plists: &plistFolder{path: filepath.Join(home, "Library", "LaunchAgents")}}
}

// Callers must not call Services from two goroutines at the same time.
func (d *Domain) Services() ([]service.Service, error) {
	plists, err := d.plists.read()
	if err != nil {
		return nil, err
	}
	listed, err := readLaunchctlList()
	if err != nil {
		return nil, err
	}
	services := make([]service.Service, 0, len(plists))
	var pids []int
	for _, p := range plists {
		svc := listed.service(p.name)
		svc.Env, svc.StdoutPath, svc.StderrPath, svc.PlistPath = p.env, p.stdoutPath, p.stderrPath, p.path
		services = append(services, svc)
		if svc.State == service.Running {
			pids = append(pids, svc.PID)
		}
	}
	uptimes := readUptimes(pids)
	for i := range services {
		if uptime, ok := uptimes[services[i].PID]; ok {
			services[i].Uptime = &uptime
		}
	}
	return services, nil
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
