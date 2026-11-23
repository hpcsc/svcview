package launchd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hpcsc/svcview/internal/service"
)

type Domain struct {
	home   string
	uid    int
	plists map[string]plist
}

func NewDomain(home string, uid int) *Domain {
	return &Domain{home: home, uid: uid}
}

// Callers must not call Services from two goroutines at the same time.
func (d *Domain) Services() ([]service.Service, error) {
	folder := filepath.Join(d.home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var services []service.Service
	named := map[string]bool{}
	plists := map[string]plist{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".plist" {
			continue
		}
		path := filepath.Join(folder, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		p := d.plists[path].readIfChanged(path, info.ModTime())
		plists[path] = p
		svc, ok := p.service()
		if !ok || named[svc.Name] {
			continue
		}
		named[svc.Name] = true
		services = append(services, svc)
	}
	d.plists = plists
	return services, nil
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
