package launchd

import (
	"context"
	"fmt"
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

func (d *Domain) Start(ctx context.Context, svc service.Service) error {
	switch {
	case svc.State == service.Running:
		return nil
	case svc.State == service.Loaded:
		return launchctlCommand{args: []string{"kickstart", d.serviceTarget(svc)}}.run(ctx)
	case svc.PlistPath == "":
		return fmt.Errorf("%s has no plist, so svcview cannot start it", svc.Name)
	}
	return launchctlCommand{args: []string{"bootstrap", d.String(), svc.PlistPath}}.run(ctx)
}

func (d *Domain) Stop(ctx context.Context, svc service.Service) error {
	if svc.State == service.NotLoaded {
		return nil
	}
	return launchctlCommand{args: []string{"bootout", d.serviceTarget(svc)}}.run(ctx)
}

func (d *Domain) Restart(ctx context.Context, svc service.Service) error {
	if svc.State == service.NotLoaded {
		return fmt.Errorf("%s is not loaded, so svcview cannot restart it", svc.Name)
	}
	return launchctlCommand{args: []string{"kickstart", "-k", d.serviceTarget(svc)}}.run(ctx)
}

func (d *Domain) serviceTarget(svc service.Service) string {
	return d.String() + "/" + svc.Name
}
