//go:build unit

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/config"
	"github.com/stretchr/testify/require"
)

func folderWith(t *testing.T, content string) (folder, path string) {
	t.Helper()
	folder = t.TempDir()
	path = filepath.Join(folder, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return folder, path
}

func TestSettings(t *testing.T) {
	t.Run("config folder", func(t *testing.T) {
		t.Run("is svcview in XDG_CONFIG_HOME when XDG_CONFIG_HOME is an absolute path", func(t *testing.T) {
			require.Equal(t, "/Users/someone/xdg/svcview", config.Folder("/Users/someone", "/Users/someone/xdg"))
		})

		t.Run("is .config/svcview in the home folder when XDG_CONFIG_HOME is empty", func(t *testing.T) {
			require.Equal(t, "/Users/someone/.config/svcview", config.Folder("/Users/someone", ""))
		})

		t.Run("is .config/svcview in the home folder when XDG_CONFIG_HOME is a relative path, as the XDG Base Directory Specification says", func(t *testing.T) {
			require.Equal(t, "/Users/someone/.config/svcview", config.Folder("/Users/someone", "xdg"))
		})
	})

	t.Run("read", func(t *testing.T) {
		t.Run("with no config.yaml in the config folder, the refresh is 2s", func(t *testing.T) {
			settings, err := config.Read(t.TempDir(), os.Getuid())

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("with no config folder, the refresh is 2s", func(t *testing.T) {
			settings, err := config.Read(filepath.Join(t.TempDir(), "svcview"), os.Getuid())

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("an empty config.yaml, one with only comments, and one whose document is null each give a refresh of 2s", func(t *testing.T) {
			for _, content := range []string{"", "\n\n", "# refresh: 5s\n", "---\n", "~\n"} {
				folder, _ := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.NoError(t, err, content)
				require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings, content)
			}
		})

		t.Run("refresh takes a Go duration, from 1ns up", func(t *testing.T) {
			for content, refresh := range map[string]time.Duration{
				"refresh: 1ns\n":        time.Nanosecond,
				"refresh: 5s\n":         5 * time.Second,
				"refresh: 1m30s\n":      90 * time.Second,
				"refresh: 250ms\n":      250 * time.Millisecond,
				"refresh: \"10s\"\n":    10 * time.Second,
				"---\nrefresh: 3s\n":    3 * time.Second,
				"~\n---\nrefresh: 4s\n": 4 * time.Second,
			} {
				folder, _ := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.NoError(t, err, content)
				require.Equal(t, config.Settings{Refresh: refresh}, settings, content)
			}
		})

		t.Run("a refresh that is not a duration gives an error that names the file and refresh, and no settings", func(t *testing.T) {
			for _, content := range []string{"refresh: fast\n", "refresh: 5\n", "refresh: [5s]\n", "refresh:\n", "refresh: ''\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, "refresh", content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a refresh that is not more than 0 gives an error that names the file and refresh, and no settings", func(t *testing.T) {
			for _, content := range []string{"refresh: 0s\n", "refresh: 0\n", "refresh: -5s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, "refresh", content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting that svcview does not know gives an error that names the file and the setting, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\ncolour: blue\n")

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"colour"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a setting that svcview does not know in a second YAML document gives an error that names the file and the setting", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\n---\ncolour: blue\n")

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"colour"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a setting that the file sets two times gives an error that names the file and the setting, also across two YAML documents", func(t *testing.T) {
			for _, content := range []string{"refresh: 5s\nrefresh: 6s\n", "refresh: 5s\n---\nrefresh: 6s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a config.yaml that is not valid YAML gives an error that names the file, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: [5s\n")

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml whose top level is not a set of settings gives an error that names the file, and no settings", func(t *testing.T) {
			for _, content := range []string{"- refresh: 5s\n", "5s\n", "refresh: 5s\n---\n- refresh: 5s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a config.yaml that is a folder gives an error that names the file, and no settings", func(t *testing.T) {
			folder := t.TempDir()
			path := filepath.Join(folder, "config.yaml")
			require.NoError(t, os.Mkdir(path, 0o755))

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that svcview cannot read gives an error that names the file, and no settings", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			folder, path := folderWith(t, "refresh: 5s\n")
			require.NoError(t, os.Chmod(path, 0o000))

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that its group or all users can write gives an error that names the file and says that another user can write it, and no settings", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o620, 0o602, 0o666, 0o777} {
				folder, path := folderWith(t, "refresh: 5s\n")
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, mode)
				require.ErrorContains(t, err, "another user can write", mode)
				require.Equal(t, config.Settings{}, settings, mode)
			}
		})

		t.Run("a config.yaml that another user owns gives an error that names the file and says that another user can write it, also when only its owner can write it", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\n")
			require.NoError(t, os.Chmod(path, 0o600))

			settings, err := config.Read(folder, os.Getuid()+1)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, "another user can write")
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that the user of svcview owns and that only its owner can write gives its settings", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o644, 0o600, 0o400, 0o755} {
				folder, path := folderWith(t, "refresh: 5s\n")
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid())

				require.NoError(t, err, mode)
				require.Equal(t, config.Settings{Refresh: 5 * time.Second}, settings, mode)
			}
		})

		t.Run("a config.yaml that is a symbolic link counts with the mode of the file that it links to", func(t *testing.T) {
			for mode, refused := range map[os.FileMode]bool{0o644: false, 0o666: true} {
				target := filepath.Join(t.TempDir(), "settings.yaml")
				require.NoError(t, os.WriteFile(target, []byte("refresh: 5s\n"), 0o600))
				require.NoError(t, os.Chmod(target, mode))
				folder := t.TempDir()
				require.NoError(t, os.Symlink(target, filepath.Join(folder, "config.yaml")))

				settings, err := config.Read(folder, os.Getuid())

				if refused {
					require.ErrorContains(t, err, "another user can write", mode)
					require.Equal(t, config.Settings{}, settings, mode)
				} else {
					require.NoError(t, err, mode)
					require.Equal(t, config.Settings{Refresh: 5 * time.Second}, settings, mode)
				}
			}
		})

		t.Run("with no config.yaml, the settings are the defaults for any user", func(t *testing.T) {
			settings, err := config.Read(t.TempDir(), os.Getuid()+1)

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})
	})
}
