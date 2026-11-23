package launchd

import (
	"os"

	"github.com/hpcsc/svcview/internal/service"
	plistformat "howett.net/plist"
)

type plist struct {
	name string
}

func readPlist(path string) plist {
	data, err := os.ReadFile(path)
	if err != nil {
		return plist{}
	}
	var properties struct {
		Label string `plist:"Label"`
	}
	if _, err := plistformat.Unmarshal(data, &properties); err != nil {
		return plist{}
	}
	return plist{name: properties.Label}
}

func (p plist) service() (service.Service, bool) {
	return service.Service{Name: p.name}, p.name != ""
}
