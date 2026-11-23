package launchd

import (
	"os"
	"time"

	plistformat "howett.net/plist"
)

type plist struct {
	modTime    time.Time
	path       string
	name       string
	env        map[string]string
	stdoutPath string
	stderrPath string
}

func readPlist(path string, modTime time.Time) plist {
	data, err := os.ReadFile(path)
	if err != nil {
		return plist{}
	}
	var properties struct {
		Label                string `plist:"Label"`
		EnvironmentVariables any    `plist:"EnvironmentVariables"`
		StandardOutPath      any    `plist:"StandardOutPath"`
		StandardErrorPath    any    `plist:"StandardErrorPath"`
	}
	if _, err := plistformat.Unmarshal(data, &properties); err != nil {
		return plist{modTime: modTime}
	}
	p := plist{modTime: modTime, path: path, name: properties.Label}
	p.stdoutPath, _ = properties.StandardOutPath.(string)
	p.stderrPath, _ = properties.StandardErrorPath.(string)
	if variables, ok := properties.EnvironmentVariables.(map[string]any); ok {
		p.env = make(map[string]string, len(variables))
		for name, value := range variables {
			if text, ok := value.(string); ok {
				p.env[name] = text
			}
		}
	}
	return p
}

func (p plist) readIfChanged(path string, modTime time.Time) plist {
	if p.modTime.Equal(modTime) {
		return p
	}
	return readPlist(path, modTime)
}
