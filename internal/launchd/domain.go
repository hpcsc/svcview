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
	names, err := d.plists.names()
	if err != nil {
		return nil, err
	}
	listed, err := readList()
	if err != nil {
		return nil, err
	}
	services := make([]service.Service, 0, len(names))
	for _, name := range names {
		services = append(services, listed.service(name))
	}
	return services, nil
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
