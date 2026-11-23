package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	fileName           = "config.yaml"
	writeByAnotherUser = fs.FileMode(0o022)
)

type Settings struct {
	Refresh time.Duration
	Groups  []Group
}

func Read(folder string, uid int) (Settings, error) {
	settings := Settings{Refresh: 2 * time.Second}
	path := filepath.Join(folder, fileName)
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read the config file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Settings{}, fmt.Errorf("read the config file: %w", err)
	}
	owner := info.Sys().(*syscall.Stat_t).Uid
	if owner != uint32(uid) || info.Mode().Perm()&writeByAnotherUser != 0 {
		return Settings{}, fmt.Errorf("another user can write the config file %s, so svcview does not read it: the file belongs to user %d and has the mode %s, and it must belong to user %d with no write permission for its group or for other users", path, owner, info.Mode().Perm(), uid)
	}
	content, err := io.ReadAll(file)
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
	switch name.Value {
	case "refresh":
		return s.setRefresh(name, value)
	case "groups":
		return s.setGroups(name, value)
	}
	return fmt.Errorf("line %d: svcview does not know the setting %q", name.Line, name.Value)
}

func (s *Settings) setRefresh(name, value *yaml.Node) error {
	refresh, err := time.ParseDuration(value.Value)
	if err != nil || refresh <= 0 {
		return fmt.Errorf("line %d: the setting %q must be a duration more than 0, such as 5s or 1m30s", value.Line, name.Value)
	}
	s.Refresh = refresh
	return nil
}

func (s *Settings) setGroups(name, value *yaml.Node) error {
	if value.Tag == "!!null" {
		return nil
	}
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: the setting %q must be a list of groups, each with a name and a match", value.Line, name.Value)
	}
	lineOf := map[string]int{}
	for i, entry := range value.Content {
		group, err := readGroup(entry, i+1)
		if err != nil {
			return err
		}
		if line, ok := lineOf[group.Name]; ok {
			return fmt.Errorf("line %d: the name of group %q is already the name of the group on line %d", entry.Line, group.Name, line)
		}
		lineOf[group.Name] = entry.Line
		s.Groups = append(s.Groups, group)
	}
	return nil
}

func (s *Settings) GroupOf(serviceName string) Group {
	for _, group := range s.Groups {
		if group.matches(serviceName) {
			return group
		}
	}
	return Other
}

func Folder(home, xdgConfigHome string) string {
	if filepath.IsAbs(xdgConfigHome) {
		return filepath.Join(xdgConfigHome, "svcview")
	}
	return filepath.Join(home, ".config", "svcview")
}
