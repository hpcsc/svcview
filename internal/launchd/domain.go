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
	return d.plists.services()
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
