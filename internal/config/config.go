package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
)

const fileName = "config.yaml"

type Settings struct {
	Refresh time.Duration
}

func Folder(home, xdgConfigHome string) string {
	if filepath.IsAbs(xdgConfigHome) {
		return filepath.Join(xdgConfigHome, "svcview")
	}
	return filepath.Join(home, ".config", "svcview")
}

func Read(folder string) (Settings, error) {
	settings := Settings{Refresh: 2 * time.Second}
	path := filepath.Join(folder, fileName)
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read the config file: %w", err)
	}
	if err := settings.read(content); err != nil {
		return Settings{}, fmt.Errorf("the config file %s: %w", path, err)
	}
	return settings, nil
}

func (s *Settings) read(content []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	lineOf := map[string]int{}
	for {
		var document yaml.Node
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("not valid YAML: %w", err)
		}
		top := document.Content[0]
		if top.Tag == "!!null" {
			continue
		}
		if top.Kind != yaml.MappingNode {
			return fmt.Errorf("line %d: the top level is not a set of settings, such as refresh: 5s", top.Line)
		}
		for i := 0; i < len(top.Content); i += 2 {
			name, value := top.Content[i], top.Content[i+1]
			if line, ok := lineOf[name.Value]; ok {
				return fmt.Errorf("line %d: the setting %q is already on line %d", name.Line, name.Value, line)
			}
			lineOf[name.Value] = name.Line
			if err := s.set(name, value); err != nil {
				return err
			}
		}
	}
}

func (s *Settings) set(name, value *yaml.Node) error {
	if name.Value != "refresh" {
		return fmt.Errorf("line %d: svcview does not know the setting %q", name.Line, name.Value)
	}
	refresh, err := time.ParseDuration(value.Value)
	if err != nil || refresh <= 0 {
		return fmt.Errorf("line %d: refresh must be a duration more than 0, such as 5s or 1m30s", value.Line)
	}
	s.Refresh = refresh
	return nil
}
