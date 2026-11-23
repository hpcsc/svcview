//go:build unit

package launchd_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/launchd"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/stretchr/testify/require"
)

func homeWith(t *testing.T, fixtureByFileName map[string]string) string {
	t.Helper()
	home := t.TempDir()
	folder := filepath.Join(home, "Library", "LaunchAgents")
	require.NoError(t, os.MkdirAll(folder, 0o755))
	for name, fixture := range fixtureByFileName {
		data, err := os.ReadFile(filepath.Join("testdata", fixture))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(folder, name), data, 0o644))
	}
	return home
}

func writeFixture(t *testing.T, path, fixture string, modTime time.Time) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

func TestDomain(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		t.Run("an XML plist and a binary plist each give a service named by its Label", func(t *testing.T) {
			home := homeWith(t, map[string]string{
				"com.example.xml.plist":    "xml.plist",
				"com.example.binary.plist": "binary.plist",
			})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{{Name: "com.example.xml"}, {Name: "com.example.binary"}}, services)
		})

		t.Run("the name comes from the Label, not from the file name", func(t *testing.T) {
			home := homeWith(t, map[string]string{"another-name.plist": "xml.plist"})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a file that is not a plist, a plist with no Label, and a file without the .plist extension give no service, and the other files still give theirs", func(t *testing.T) {
			home := homeWith(t, map[string]string{
				"a-text-file.plist":        "not-a-plist.txt",
				"b-no-label.plist":         "no-label.plist",
				"c-xml.plist.bak":          "xml.plist",
				"d-binary.plist":           "binary.plist",
				"com.example.xml.disabled": "xml.plist",
			})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a folder with the .plist extension gives no service", func(t *testing.T) {
			home := homeWith(t, map[string]string{"com.example.binary.plist": "binary.plist"})
			require.NoError(t, os.Mkdir(filepath.Join(home, "Library", "LaunchAgents", "folder.plist"), 0o755))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a symbolic link to a plist gives the service of the plist", func(t *testing.T) {
			home := homeWith(t, nil)
			target, err := filepath.Abs(filepath.Join("testdata", "xml.plist"))
			require.NoError(t, err)
			require.NoError(t, os.Symlink(target, filepath.Join(home, "Library", "LaunchAgents", "com.example.xml.plist")))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a symbolic link to a file that does not exist gives no service, and the other files still give theirs", func(t *testing.T) {
			home := homeWith(t, map[string]string{"com.example.binary.plist": "binary.plist"})
			require.NoError(t, os.Symlink(filepath.Join(home, "missing.plist"), filepath.Join(home, "Library", "LaunchAgents", "a-link.plist")))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("two plists with the same Label give one service", func(t *testing.T) {
			home := homeWith(t, map[string]string{
				"com.example.xml.plist":    "xml.plist",
				"a-copy.plist":             "xml.plist",
				"com.example.binary.plist": "binary.plist",
			})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{{Name: "com.example.xml"}, {Name: "com.example.binary"}}, services)
		})

		t.Run("a home folder with no Library/LaunchAgents folder gives no services and no error", func(t *testing.T) {
			services, err := launchd.NewDomain(t.TempDir(), 501).Services()

			require.NoError(t, err)
			require.Empty(t, services)
		})

		t.Run("a LaunchAgents folder that cannot be read gives an error that names the folder", func(t *testing.T) {
			home := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(home, "Library"), 0o755))
			folder := filepath.Join(home, "Library", "LaunchAgents")
			require.NoError(t, os.WriteFile(folder, []byte("a file, not a folder"), 0o644))

			_, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, folder)
		})
	})

	t.Run("list again", func(t *testing.T) {
		modTime := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

		t.Run("a plist that is added after a read gives its service at the next read", func(t *testing.T) {
			home := homeWith(t, map[string]string{"com.example.xml.plist": "xml.plist"})
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			writeFixture(t, filepath.Join(home, "Library", "LaunchAgents", "com.example.binary.plist"), "binary.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{{Name: "com.example.xml"}, {Name: "com.example.binary"}}, services)
		})

		t.Run("a plist that is removed after a read gives no service at the next read", func(t *testing.T) {
			home := homeWith(t, map[string]string{"com.example.xml.plist": "xml.plist", "com.example.binary.plist": "binary.plist"})
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			require.NoError(t, os.Remove(filepath.Join(home, "Library", "LaunchAgents", "com.example.xml.plist")))

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a plist whose Label changes with its modification time gives the new name at the next read", func(t *testing.T) {
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			writeFixture(t, path, "xml.plist", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			writeFixture(t, path, "binary.plist", modTime.Add(time.Second))

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a plist whose modification time did not change is not read again, so a new Label with the same modification time keeps the old name", func(t *testing.T) {
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			writeFixture(t, path, "xml.plist", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			writeFixture(t, path, "binary.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a file that is not a plist is not read again while its modification time does not change", func(t *testing.T) {
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			writeFixture(t, path, "not-a-plist.txt", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			writeFixture(t, path, "xml.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.Empty(t, services)
		})

		t.Run("a plist that could not be read gives its service once it can be read, with the same modification time", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			writeFixture(t, path, "xml.plist", modTime)
			require.NoError(t, os.Chmod(path, 0o000))
			domain := launchd.NewDomain(home, 501)
			unreadable, err := domain.Services()
			require.NoError(t, err)
			require.NoError(t, os.Chmod(path, 0o644))

			services, err := domain.Services()

			require.NoError(t, err)
			require.Empty(t, unreadable)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})
	})

	t.Run("text", func(t *testing.T) {
		t.Run("reads as gui/ and the uid", func(t *testing.T) {
			require.Equal(t, "gui/501", launchd.NewDomain(t.TempDir(), 501).String())
			require.Equal(t, "gui/502", launchd.NewDomain(t.TempDir(), 502).String())
		})
	})
}
