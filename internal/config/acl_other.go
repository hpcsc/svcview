//go:build !darwin

package config

import "os"

func ACLEntries(*os.File) (int, error) {
	return 0, nil
}
