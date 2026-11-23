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
	named, err := d.plists.services()
	if err != nil {
		return nil, err
	}
	listed, err := readList()
	if err != nil {
		return nil, err
	}
	services := make([]service.Service, 0, len(named))
	for _, svc := range named {
		services = append(services, listed.service(svc.Name))
	}
	return services, nil
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
