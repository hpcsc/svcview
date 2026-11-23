package launchd

import (
	"os"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	plistformat "howett.net/plist"
)

type plist struct {
	modTime time.Time
	name    string
}

func readPlist(path string, modTime time.Time) plist {
	data, err := os.ReadFile(path)
	if err != nil {
		return plist{}
	}
	var properties struct {
		Label string `plist:"Label"`
	}
	if _, err := plistformat.Unmarshal(data, &properties); err != nil {
		return plist{modTime: modTime}
	}
	return plist{modTime: modTime, name: properties.Label}
}

func (p plist) readIfChanged(path string, modTime time.Time) plist {
	if p.modTime.Equal(modTime) {
		return p
	}
	return readPlist(path, modTime)
}

func (p plist) service() (service.Service, bool) {
	return service.Service{Name: p.name}, p.name != ""
}
