package launchd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hpcsc/svcview/internal/service"
)

type plistFolder struct {
	path string
	kept map[string]plist
}

func (f *plistFolder) services() ([]service.Service, error) {
	entries, err := os.ReadDir(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var services []service.Service
	named := map[string]bool{}
	kept := map[string]plist{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".plist" {
			continue
		}
		path := filepath.Join(f.path, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		p := f.kept[path].readIfChanged(path, info.ModTime())
		kept[path] = p
		svc, ok := p.service()
		if !ok || named[svc.Name] {
			continue
		}
		named[svc.Name] = true
		services = append(services, svc)
	}
	f.kept = kept
	return services, nil
}
