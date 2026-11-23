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
	home string
	uid  int
}

func NewDomain(home string, uid int) *Domain {
	return &Domain{home: home, uid: uid}
}

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
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".plist" {
			continue
		}
		svc, ok := readPlist(filepath.Join(folder, entry.Name())).service()
		if !ok || named[svc.Name] {
			continue
		}
		named[svc.Name] = true
		services = append(services, svc)
	}
	return services, nil
}

func (d *Domain) String() string {
	return "gui/" + strconv.Itoa(d.uid)
}
