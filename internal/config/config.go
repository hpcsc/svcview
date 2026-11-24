package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	fileName           = "config.yaml"
	confD              = "conf.d"
	writeByAnotherUser = fs.FileMode(0o022)
)

type Settings struct {
	Refresh time.Duration
	Groups  []Group
}

func Read(folder string, uid int, aclEntries func(*os.File) (int, error)) (Settings, error) {
	settings := Settings{Refresh: 2 * time.Second}
	if err := readConfigFile(filepath.Join(folder, fileName), uid, aclEntries, settings.set); err != nil {
		return Settings{}, err
	}
	paths, err := filesOfConfD(filepath.Join(folder, confD))
	if err != nil {
		return Settings{}, err
	}
	for _, path := range paths {
		if err := readConfigFile(path, uid, aclEntries, settings.setInConfD); err != nil {
			return Settings{}, err
		}
	}
	return settings, nil
}

func (s *Settings) set(path string, name, value *yaml.Node) error {
	switch name.Value {
	case "refresh":
		return s.setRefresh(name, value)
	case "groups":
		return s.setGroups(path, name, value)
	}
	return fmt.Errorf("line %d: svcview does not know the setting %q", name.Line, name.Value)
}

func (s *Settings) setInConfD(path string, name, value *yaml.Node) error {
	if name.Value == "refresh" {
		return fmt.Errorf("line %d: only %s can set the setting %q", name.Line, fileName, name.Value)
	}
	return s.set(path, name, value)
}

func (s *Settings) setRefresh(name, value *yaml.Node) error {
	refresh, err := readDuration(value, strconv.Quote(name.Value))
	if err != nil {
		return err
	}
	s.Refresh = refresh
	return nil
}

func (s *Settings) setGroups(path string, name, value *yaml.Node) error {
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
		group.configFile = path
		if err := s.join(group, entry.Line); err != nil {
			return err
		}
	}
	return nil
}

func (s *Settings) join(group Group, line int) error {
	for _, earlier := range s.Groups {
		if earlier.Name == group.Name {
			return fmt.Errorf("line %d: the name of group %q is already the name of a group in the config file %s, and the name of a group must be unique across all config files", line, group.Name, earlier.configFile)
		}
	}
	s.Groups = append(s.Groups, group)
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

func readConfigFile(path string, uid int, aclEntries func(*os.File) (int, error), set func(path string, name, value *yaml.Node) error) error {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil && !info.Mode().IsRegular() {
			return notRegularFile(path, info.Mode())
		}
		return fmt.Errorf("read the config file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("read the config file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return notRegularFile(path, info.Mode())
	}
	owner := info.Sys().(*syscall.Stat_t).Uid
	if owner != uint32(uid) || info.Mode().Perm()&writeByAnotherUser != 0 {
		return fmt.Errorf("another user can write the config file %s, so svcview does not read it: the file belongs to user %d and has the mode %s, and it must belong to user %d with no write permission for its group or for other users", path, owner, info.Mode().Perm(), uid)
	}
	entries, err := aclEntries(file)
	if err != nil {
		return fmt.Errorf("svcview cannot read the ACL of the config file %s, so it does not read the file: %w", path, err)
	}
	if entries > 0 {
		return fmt.Errorf("the config file %s has an ACL, which can let another user write it, so svcview does not read it: /bin/chmod -N %s removes the ACL", path, path)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read the config file: %w", err)
	}
	if err := readContent(path, content, set); err != nil {
		return fmt.Errorf("the config file %s: %w", path, err)
	}
	return nil
}

func readContent(path string, content []byte, set func(path string, name, value *yaml.Node) error) error {
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
			if err := set(path, name, value); err != nil {
				return err
			}
		}
	}
}

func notRegularFile(path string, mode fs.FileMode) error {
	return fmt.Errorf("the config file %s is not a regular file, so svcview does not read it: it has the mode %s", path, mode)
}

func filesOfConfD(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("svcview cannot list the config files in %s: %w", folder, err)
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || filepath.Ext(name) != ".yaml" {
			continue
		}
		paths = append(paths, filepath.Join(folder, name))
	}
	return paths, nil
}

func readDuration(value *yaml.Node, setting string) (time.Duration, error) {
	duration, err := time.ParseDuration(value.Value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("line %d: the setting %s must be a duration more than 0, such as 5s or 1m30s", value.Line, setting)
	}
	return duration, nil
}
