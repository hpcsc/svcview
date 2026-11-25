package launchd

import (
	"maps"
	"os"
	"slices"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	plistformat "howett.net/plist"
)

type plist struct {
	modTime    time.Time
	path       string
	name       string
	env        map[string]string
	stdoutPath string
	stderrPath string
	properties []service.Property
}

func readPlist(path string, modTime time.Time) plist {
	data, err := os.ReadFile(path)
	if err != nil {
		return plist{}
	}
	var decoded struct {
		Label                string `plist:"Label"`
		Program              any    `plist:"Program"`
		ProgramArguments     any    `plist:"ProgramArguments"`
		WorkingDirectory     any    `plist:"WorkingDirectory"`
		EnvironmentVariables any    `plist:"EnvironmentVariables"`
		StandardOutPath      any    `plist:"StandardOutPath"`
		StandardErrorPath    any    `plist:"StandardErrorPath"`
	}
	if _, err := plistformat.Unmarshal(data, &decoded); err != nil {
		return plist{modTime: modTime}
	}
	p := plist{modTime: modTime, path: path, name: decoded.Label}
	p.stdoutPath, _ = decoded.StandardOutPath.(string)
	p.stderrPath, _ = decoded.StandardErrorPath.(string)
	if variables, ok := decoded.EnvironmentVariables.(map[string]any); ok {
		p.env = make(map[string]string, len(variables))
		for name, value := range variables {
			if text, ok := value.(string); ok {
				p.env[name] = text
			}
		}
	}
	p.properties = slices.Concat(
		p.programProperty(decoded.ProgramArguments, decoded.Program),
		p.textProperty("work folder", decoded.WorkingDirectory),
		p.textProperty("stdout", decoded.StandardOutPath),
		p.textProperty("stderr", decoded.StandardErrorPath),
		p.environmentProperty(),
	)
	return p
}

func (p plist) readIfChanged(path string, modTime time.Time) plist {
	if p.modTime.Equal(modTime) {
		return p
	}
	return readPlist(path, modTime)
}

func (p plist) programProperty(arguments, program any) []service.Property {
	if texts, ok := p.texts(arguments); ok {
		return []service.Property{{Name: "program", Values: texts}}
	}
	return p.textProperty("program", program)
}

func (p plist) textProperty(name string, value any) []service.Property {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return []service.Property{{Name: name, Values: []string{text}}}
}

func (p plist) environmentProperty() []service.Property {
	if p.env == nil {
		return nil
	}
	return []service.Property{{Name: "environment", Values: slices.Sorted(maps.Keys(p.env))}}
}

func (p plist) texts(value any) ([]string, bool) {
	array, ok := value.([]any)
	if !ok {
		return nil, false
	}
	var texts []string
	for _, element := range array {
		if text, ok := element.(string); ok {
			texts = append(texts, text)
		}
	}
	return texts, true
}
