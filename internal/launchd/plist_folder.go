package launchd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type plistFolder struct {
	path string
	kept map[string]plist
}

func (f *plistFolder) names() ([]string, error) {
	entries, err := os.ReadDir(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	seen := map[string]bool{}
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
		if p.name == "" || seen[p.name] {
			continue
		}
		seen[p.name] = true
		names = append(names, p.name)
	}
	f.kept = kept
	return names, nil
}
