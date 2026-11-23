//go:build unit

package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hpcsc/svcview/internal/config"
	"github.com/stretchr/testify/require"
)

func configFile(t *testing.T, aclEntries ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("refresh: 5s\n"), 0o644))
	for _, entry := range aclEntries {
		chmod(t, "+a", entry, path)
	}
	return path
}

func chmod(t *testing.T, arguments ...string) {
	t.Helper()
	path := arguments[len(arguments)-1]
	t.Cleanup(func() {
		_ = exec.Command("/bin/chmod", "-h", "-N", path).Run()
	})
	output, err := exec.Command("/bin/chmod", arguments...).CombinedOutput()
	require.NoError(t, err, string(output))
}

func aclEntriesOf(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	entries, err := config.ACLEntries(file)
	require.NoError(t, err)
	return entries
}

func skipUnlessMacOS(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS has the ACLs of /bin/chmod +a")
	}
}

func TestACLEntries(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		t.Run("on macOS, a file with no ACL has 0 entries", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t)

			require.Equal(t, 0, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file whose ACL chmod -N removed has 0 entries", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t, "everyone allow write", "user:nobody allow append")
			chmod(t, "-N", path)

			require.Equal(t, 0, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file with one entry that allows a right has 1 entry", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t, "everyone allow write")

			require.Equal(t, 1, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file with one entry that only denies a right has 1 entry", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t, "everyone deny delete")

			require.Equal(t, 1, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file with two entries has 2 entries", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t, "everyone allow write", "user:nobody allow append")

			require.Equal(t, 2, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file whose ACL lost its only entry has 0 entries", func(t *testing.T) {
			skipUnlessMacOS(t)
			path := configFile(t, "everyone allow write")
			chmod(t, "-a#", "0", path)

			require.Equal(t, 0, aclEntriesOf(t, path))
		})

		t.Run("on macOS, a file opened through a symbolic link has the entries of the file that the link points to, not of the link", func(t *testing.T) {
			skipUnlessMacOS(t)
			target := configFile(t, "everyone allow write", "user:nobody allow append")
			link := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.Symlink(target, link))
			chmod(t, "-h", "+a", "everyone deny delete", link)

			require.Equal(t, 2, aclEntriesOf(t, link))
		})

		t.Run("on macOS, a file that is closed gives an error, not a count", func(t *testing.T) {
			skipUnlessMacOS(t)
			file, err := os.Open(configFile(t, "everyone allow write"))
			require.NoError(t, err)
			require.NoError(t, file.Close())

			entries, err := config.ACLEntries(file)

			require.Error(t, err)
			require.Zero(t, entries)
		})

		t.Run("on other systems, a file has 0 entries", func(t *testing.T) {
			if runtime.GOOS == "darwin" {
				t.Skip("macOS reads the ACL of the file")
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte("refresh: 5s\n"), 0o644))

			require.Equal(t, 0, aclEntriesOf(t, path))
		})
	})
}
