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

func (f *plistFolder) read() ([]plist, error) {
	entries, err := os.ReadDir(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var plists []plist
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
		plists = append(plists, p)
	}
	f.kept = kept
	return plists, nil
}
