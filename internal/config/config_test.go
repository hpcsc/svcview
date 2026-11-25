//go:build unit

package config_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/stretchr/testify/require"
)

func noACL(*os.File) (int, error) {
	return 0, nil
}

func aclWithEntries(count int) func(*os.File) (int, error) {
	return func(*os.File) (int, error) {
		return count, nil
	}
}

func folderWith(t *testing.T, content string) (folder, path string) {
	t.Helper()
	folder = t.TempDir()
	path = filepath.Join(folder, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return folder, path
}

func readSettings(t *testing.T, content string) config.Settings {
	t.Helper()
	folder, _ := folderWith(t, content)
	settings, err := config.Read(folder, os.Getuid(), noACL)
	require.NoError(t, err)
	return settings
}

func folderWithConfD(t *testing.T, content string) (folder, path string) {
	t.Helper()
	folder = t.TempDir()
	return folder, writeConfD(t, folder, "worker.yaml", content)
}

func writeConfD(t *testing.T, folder, name, content string) string {
	t.Helper()
	confD := filepath.Join(folder, "conf.d")
	require.NoError(t, os.MkdirAll(confD, 0o755))
	path := filepath.Join(confD, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func groupsYAML(nameAndMatch ...string) string {
	content := "groups:\n"
	for i := 0; i < len(nameAndMatch); i += 2 {
		content += "  - name: " + nameAndMatch[i] + "\n    match: " + nameAndMatch[i+1] + "\n"
	}
	return content
}

var notRegularFiles = map[string]func(t *testing.T, path string){
	"a folder": func(t *testing.T, path string) {
		require.NoError(t, os.Mkdir(path, 0o755))
	},
	"a named pipe with no writer": func(t *testing.T, path string) {
		require.NoError(t, syscall.Mkfifo(path, 0o644))
	},
	"a symbolic link to a folder": func(t *testing.T, path string) {
		require.NoError(t, os.Symlink(t.TempDir(), path))
	},
	"a symbolic link to a named pipe with no writer": func(t *testing.T, path string) {
		pipe := filepath.Join(t.TempDir(), "pipe")
		require.NoError(t, syscall.Mkfifo(pipe, 0o644))
		require.NoError(t, os.Symlink(pipe, path))
	},
	"a symbolic link to a device": func(t *testing.T, path string) {
		require.NoError(t, os.Symlink("/dev/null", path))
	},
	"a symbolic link to a socket": func(t *testing.T, path string) {
		folder, err := os.MkdirTemp("/tmp", "svcview-")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(folder) })
		socket := filepath.Join(folder, "socket")
		listener, err := net.Listen("unix", socket)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		require.NoError(t, os.Symlink(socket, path))
	},
}

func readWithin(t *testing.T, deadline time.Duration, folder string) (config.Settings, error) {
	t.Helper()
	type result struct {
		settings config.Settings
		err      error
	}
	done := make(chan result, 1)
	go func() {
		settings, err := config.Read(folder, os.Getuid(), noACL)
		done <- result{settings, err}
	}()
	select {
	case read := <-done:
		return read.settings, read.err
	case <-time.After(deadline):
		require.FailNow(t, "config.Read did not return", "it still waits after %s", deadline)
		return config.Settings{}, nil
	}
}

func groupNames(settings config.Settings) []string {
	var names []string
	for _, group := range settings.Groups {
		names = append(names, group.Name)
	}
	return names
}

func groupOfEach(settings config.Settings, names ...string) map[string]string {
	groups := map[string]string{}
	for _, name := range names {
		groups[name] = settings.GroupOf(name).Name
	}
	return groups
}

type actionRead struct {
	Key     string
	Name    string
	Line    []string
	Confirm bool
}

func actionsRead(group config.Group, svc service.Service) []actionRead {
	var read []actionRead
	for _, action := range group.Actions {
		line, _ := action.Command.Apply(svc)
		read = append(read, actionRead{Key: action.Key, Name: action.Name, Line: line, Confirm: action.Confirm})
	}
	return read
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
			settings, err := config.Read(t.TempDir(), os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("with no config folder, the refresh is 2s", func(t *testing.T) {
			settings, err := config.Read(filepath.Join(t.TempDir(), "svcview"), os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("an empty config.yaml, one with only comments, and one whose document is null each give a refresh of 2s", func(t *testing.T) {
			for _, content := range []string{"", "\n\n", "# refresh: 5s\n", "---\n", "~\n"} {
				folder, _ := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

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

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.NoError(t, err, content)
				require.Equal(t, config.Settings{Refresh: refresh}, settings, content)
			}
		})

		t.Run("a refresh that is not a duration gives an error that names the file and refresh, and no settings", func(t *testing.T) {
			for _, content := range []string{"refresh: fast\n", "refresh: 5\n", "refresh: [5s]\n", "refresh:\n", "refresh: ''\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a refresh that is not more than 0 gives an error that names the file and refresh, and no settings", func(t *testing.T) {
			for _, content := range []string{"refresh: 0s\n", "refresh: 0\n", "refresh: -5s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting that svcview does not know gives an error that names the file and the setting, and no settings, also when its value is a duration", func(t *testing.T) {
			for content, setting := range map[string]string{
				"refresh: 5s\ncolour: blue\n": `"colour"`,
				"interval: 5s\n":              `"interval"`,
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting that svcview does not know in a second YAML document gives an error that names the file and the setting", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\n---\ncolour: blue\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"colour"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a setting that the file sets two times gives an error that names the file and the setting, also across two YAML documents", func(t *testing.T) {
			for _, content := range []string{"refresh: 5s\nrefresh: 6s\n", "refresh: 5s\n---\nrefresh: 6s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a config.yaml that is not valid YAML gives an error that names the file, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: [5s\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml whose top level is not a set of settings gives an error that names the file, and no settings", func(t *testing.T) {
			for _, content := range []string{"- refresh: 5s\n", "5s\n", "refresh: 5s\n---\n- refresh: 5s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a config.yaml that is not a regular file gives an error at once that names the file and says that it is not a regular file, and no settings, also through a symbolic link", func(t *testing.T) {
			for kind, create := range notRegularFiles {
				folder := t.TempDir()
				path := filepath.Join(folder, "config.yaml")
				create(t, path)

				settings, err := readWithin(t, 2*time.Second, folder)

				require.ErrorContains(t, err, path, kind)
				require.ErrorContains(t, err, "is not a regular file", kind)
				require.Equal(t, config.Settings{}, settings, kind)
			}
		})

		t.Run("a config.yaml that svcview cannot read gives an error that names the file, and no settings", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			folder, path := folderWith(t, "refresh: 5s\n")
			require.NoError(t, os.Chmod(path, 0o000))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that its group or all users can write gives an error that names the file and says that another user can write it, and no settings", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o620, 0o602, 0o666, 0o777} {
				folder, path := folderWith(t, "refresh: 5s\n")
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, mode)
				require.ErrorContains(t, err, "another user can write", mode)
				require.Equal(t, config.Settings{}, settings, mode)
			}
		})

		t.Run("a config.yaml that another user owns gives an error that names the file and says that another user can write it, also when only its owner can write it", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\n")
			require.NoError(t, os.Chmod(path, 0o600))

			settings, err := config.Read(folder, os.Getuid()+1, noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, "another user can write")
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that the user of svcview owns and that only its owner can write gives its settings", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o644, 0o600, 0o400, 0o755} {
				folder, path := folderWith(t, "refresh: 5s\n")
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid(), noACL)

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

				settings, err := config.Read(folder, os.Getuid(), noACL)

				if refused {
					require.ErrorContains(t, err, "another user can write", mode)
					require.Equal(t, config.Settings{}, settings, mode)
				} else {
					require.NoError(t, err, mode)
					require.Equal(t, config.Settings{Refresh: 5 * time.Second}, settings, mode)
				}
			}
		})

		t.Run("a config.yaml with one or more ACL entries gives an error that names the file and chmod -N, and no settings, also when the owner and the mode pass", func(t *testing.T) {
			for _, count := range []int{1, 2} {
				folder, path := folderWith(t, "refresh: 5s\n")

				settings, err := config.Read(folder, os.Getuid(), aclWithEntries(count))

				require.ErrorContains(t, err, path, count)
				require.ErrorContains(t, err, "ACL", count)
				require.ErrorContains(t, err, "chmod -N", count)
				require.Equal(t, config.Settings{}, settings, count)
			}
		})

		t.Run("a config.yaml whose ACL svcview cannot count gives an error that names the file and the cause, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "refresh: 5s\n")
			unreadable := func(*os.File) (int, error) {
				return 0, errors.New("fgetattrlist: bad file descriptor")
			}

			settings, err := config.Read(folder, os.Getuid(), unreadable)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, "fgetattrlist: bad file descriptor")
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("with no config.yaml, svcview counts no ACL, and the settings are the defaults", func(t *testing.T) {
			settings, err := config.Read(t.TempDir(), os.Getuid(), aclWithEntries(1))

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("on macOS, a config.yaml with the mode 0644 and an ACL entry gives an error, whatever the entry allows or denies", func(t *testing.T) {
			skipUnlessMacOS(t)
			for _, entry := range []string{"everyone allow write", "user:nobody allow append", "everyone deny delete"} {
				path := configFile(t, entry)

				settings, err := config.Read(filepath.Dir(path), os.Getuid(), config.ACLEntries)

				require.ErrorContains(t, err, path, entry)
				require.ErrorContains(t, err, "chmod -N", entry)
				require.Equal(t, config.Settings{}, settings, entry)
			}
		})

		t.Run("on macOS, a config.yaml that is a symbolic link counts with the ACL of the file that it links to, not of the link", func(t *testing.T) {
			skipUnlessMacOS(t)
			withACL := configFile(t, "everyone allow write")
			withoutACL := configFile(t)
			for target, refused := range map[string]bool{withACL: true, withoutACL: false} {
				folder := t.TempDir()
				link := filepath.Join(folder, "config.yaml")
				require.NoError(t, os.Symlink(target, link))
				if !refused {
					chmod(t, "-h", "+a", "everyone allow write", link)
				}

				settings, err := config.Read(folder, os.Getuid(), config.ACLEntries)

				if refused {
					require.ErrorContains(t, err, "chmod -N", target)
					require.Equal(t, config.Settings{}, settings, target)
				} else {
					require.NoError(t, err, target)
					require.Equal(t, config.Settings{Refresh: 5 * time.Second}, settings, target)
				}
			}
		})

		t.Run("with no config.yaml, the settings are the defaults for any user", func(t *testing.T) {
			settings, err := config.Read(t.TempDir(), os.Getuid()+1, noACL)

			require.NoError(t, err)
			require.Equal(t, config.Settings{Refresh: 2 * time.Second}, settings)
		})

		t.Run("each entry of groups gives a group with its name, in the order of the file", func(t *testing.T) {
			settings := readSettings(t, "refresh: 5s\ngroups:\n  - name: Workers\n    match: worker\n  - name: Databases\n    match: redis|postgres\n  - name: Apple\n    match: ^com\\.apple\\.\n")

			require.Equal(t, []string{"Workers", "Databases", "Apple"}, groupNames(settings))
			require.Equal(t, 5*time.Second, settings.Refresh)
		})

		t.Run("with no groups, with an empty list of groups, or with groups that has no entries, the settings have no groups", func(t *testing.T) {
			for _, content := range []string{"refresh: 5s\n", "refresh: 5s\ngroups: []\n", "refresh: 5s\ngroups:\n", "groups: ~\nrefresh: 5s\n"} {
				settings := readSettings(t, content)

				require.Equal(t, config.Settings{Refresh: 5 * time.Second}, settings, content)
			}
		})

		t.Run("a setting other than refresh and groups at the top of a file with groups gives an error that names the file and the setting", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\nstatus: running\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"status"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("groups that is not a list gives an error that names the file and the setting, and no settings", func(t *testing.T) {
			for _, content := range []string{"groups: Workers\n", "groups:\n  name: Workers\n  match: worker\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"groups"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("an entry of groups that is not a set of settings gives an error that names the file and the group by its place, and no settings", func(t *testing.T) {
			for _, content := range []string{
				"groups:\n  - name: Workers\n    match: worker\n  - Databases\n",
				"groups:\n  - name: Workers\n    match: worker\n  -\n",
				"groups:\n  - name: Workers\n    match: worker\n  - [Databases, redis]\n",
				"groups:\n  - name: Workers\n    match: worker\n  - [Databases]\n",
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, "group 2 ", content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a group with no name gives an error that names the file and the group by its place, and no settings", func(t *testing.T) {
			for _, content := range []string{
				"groups:\n  - name: Workers\n    match: worker\n  - match: redis\n",
				"groups:\n  - name: Workers\n    match: worker\n  - name:\n    match: redis\n",
				"groups:\n  - name: Workers\n    match: worker\n  - name: ''\n    match: redis\n",
				"groups:\n  - name: Workers\n    match: worker\n  - name: ~\n    match: redis\n",
				"groups:\n  - name: Workers\n    match: worker\n  - name: [Databases]\n    match: redis\n",
				"groups:\n  - name: &first Workers\n    match: worker\n  - name: *first\n    match: redis\n",
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, "group 2 has no name", content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a group with no match gives an error that names the file and the group, and no settings", func(t *testing.T) {
			for _, content := range []string{
				"groups:\n  - name: Databases\n",
				"groups:\n  - name: Databases\n    match:\n",
				"groups:\n  - name: Databases\n    match: ''\n",
				"groups:\n  - name: Databases\n    match: ~\n",
				"groups:\n  - name: Databases\n    match: [redis]\n",
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `group "Databases" has no match`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a match that is not a Go regular expression gives an error that names the file and the group, and no settings", func(t *testing.T) {
			for _, content := range []string{
				"groups:\n  - name: Databases\n    match: '['\n",
				"groups:\n  - name: Databases\n    match: redis(\n",
				"groups:\n  - name: Databases\n    match: (?<=redis)\n",
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `group "Databases" is not a Go regular expression`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting in a group that svcview does not know gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"groups:\n  - name: Databases\n    match: redis\n    hooks: [restart]\n": `"hooks" of group "Databases"`,
				"groups:\n  - hooks: [restart]\n    name: Databases\n    match: redis\n": `"hooks" of group "Databases"`,
				"groups:\n  - nme: Databases\n    match: redis\n":                        `"nme" of group 1`,
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting that a group sets two times gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"groups:\n  - name: Databases\n    match: redis\n    match: postgres\n": `"match" of group "Databases"`,
				"groups:\n  - match: redis\n    match: postgres\n":                      `"match" of group 1`,
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a group with status gives a status command with its program, its arguments, its every and its timeout", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, '{{.Env.WORKER_STATUS_FILE}}', --all]\n      every: 5s\n      timeout: 3s\n")
			command, err := status.NewCommand([]string{"cat", "{{.Env.WORKER_STATUS_FILE}}", "--all"}, 5*time.Second, 3*time.Second)
			require.NoError(t, err)
			withOtherTexts, err := status.NewCommand([]string{"cat", "{{.Env.WORKER_STATUS_FILE}}", "--last"}, 5*time.Second, 3*time.Second)
			require.NoError(t, err)

			require.Equal(t, &command, settings.Groups[0].Status)
			require.NotEqual(t, &withOtherTexts, settings.Groups[0].Status)
		})

		t.Run("a group whose status sets stale_after gives a status command with that stale after, with its program, its arguments, its every and its timeout", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, '{{.Env.WORKER_STATUS_FILE}}', --all]\n      every: 5s\n      timeout: 3s\n      stale_after: 30s\n")
			command, err := status.NewCommand([]string{"cat", "{{.Env.WORKER_STATUS_FILE}}", "--all"}, 5*time.Second, 3*time.Second)
			require.NoError(t, err)
			command.StaleAfter = 30 * time.Second

			require.Equal(t, &command, settings.Groups[0].Status)
		})

		t.Run("a group whose status has no stale_after has a stale after of 0, also with every and timeout", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n  - name: Jobs\n    match: job\n    status:\n      command: [cat, job.json]\n      every: 5s\n      timeout: 3s\n")

			require.Equal(t, [2]time.Duration{0, 0}, [2]time.Duration{settings.Groups[0].Status.StaleAfter, settings.Groups[1].Status.StaleAfter})
		})

		t.Run("a group with status and no every or timeout has an every of 2s and a timeout of 1s", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command:\n        - cat\n        - '{{.Env.WORKER_STATUS_FILE}}'\n")

			require.Equal(t, [2]time.Duration{2 * time.Second, time.Second}, [2]time.Duration{settings.Groups[0].Status.Every, settings.Groups[0].Status.Timeout})
		})

		t.Run("a group with no status has no status command, also after a group with one, and so does the group Other", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n  - name: Databases\n    match: redis\n")

			require.Equal(t, []*status.Command{nil, nil}, []*status.Command{settings.GroupOf("homebrew.mxcl.redis").Status, settings.GroupOf("com.apple.a").Status})
		})

		t.Run("a text of command that is not a Go template, or that uses a value other than the five template values, gives an error that names the file, the group and the text, and no settings", func(t *testing.T) {
			for content, text := range map[string]string{
				"command: [cat, '{{.Env.WORKER_STATUS_FILE}']": `"{{.Env.WORKER_STATUS_FILE}"`,
				"command: [cat, '{{.Label}}']":                 `"{{.Label}}"`,
				"command: [cat, '{{.Env}}']":                   `"{{.Env}}"`,
				"command: ['{{.Label}}', status.json]":         `"{{.Label}}"`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `status of group "Workers"`, content)
				require.ErrorContains(t, err, text, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a command that is not a list of texts, that is empty, or whose program is empty gives an error that names the file, the group and the fault, and no settings", func(t *testing.T) {
			for content, fault := range map[string]string{
				"command: cat status.json":      "must be a list of texts",
				"command:":                      "must be a list of texts",
				"command: {cat: status.json}":   "must be a list of texts",
				"command: [cat, [status.json]]": "must be a list of texts",
				"command: [cat, ~]":             "must be a list of texts",
				"command: []":                   "the command is empty",
				"command: ['', status.json]":    "the program of the command is empty",
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `status of group "Workers"`, content)
				require.ErrorContains(t, err, fault, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a status with no command gives an error that names the file and the group, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      every: 5s\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `status of group "Workers" has no command`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("an every, a timeout or a stale_after that is not a duration more than 0 gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"every: fast":        `"every"`,
				"every: 0s":          `"every"`,
				"every: -5s":         `"every"`,
				"every:":             `"every"`,
				"timeout: 5":         `"timeout"`,
				"timeout: 0":         `"timeout"`,
				"timeout: [1s]":      `"timeout"`,
				"timeout: -1ms":      `"timeout"`,
				"timeout: ''":        `"timeout"`,
				"every: 1m30s x":     `"every"`,
				"stale_after: soon":  `"stale_after"`,
				"stale_after: 0s":    `"stale_after"`,
				"stale_after: -5s":   `"stale_after"`,
				"stale_after:":       `"stale_after"`,
				"stale_after: ''":    `"stale_after"`,
				"stale_after: [30s]": `"stale_after"`,
				"stale_after: 30":    `"stale_after"`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting+` of the status of group "Workers" must be a duration more than 0`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a status that is not a set of settings gives an error that names the file and the group, and no settings", func(t *testing.T) {
			for _, content := range []string{"status: running", "status: [cat, status.json]", "status:", "status: ~"} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `"status" of group "Workers"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting in status that svcview does not know gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for _, setting := range []string{"actions", "interval", "max_age"} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n      "+setting+": 1m\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, setting)
				require.ErrorContains(t, err, `svcview does not know the setting "`+setting+`" of the status of group "Workers"`, setting)
				require.Equal(t, config.Settings{}, settings, setting)
			}
		})

		t.Run("a setting that status sets two times gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"command: [cat, status.json]\n      every: 5s\n      every: 6s":              `"every" of the status of group "Workers" is already on line 6`,
				"command: [cat, a.json]\n      command: [cat, b.json]":                       `"command" of the status of group "Workers" is already on line 5`,
				"command: [cat, status.json]\n      timeout: 1s\n      timeout: 1s":          `"timeout" of the status of group "Workers" is already on line 6`,
				"command: [cat, status.json]\n      stale_after: 30s\n      stale_after: 1m": `"stale_after" of the status of group "Workers" is already on line 6`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a group with actions gives each action with its key, its name, its command and its confirm, in the order of the file, and an action with no confirm does not confirm", func(t *testing.T) {
			settings := readSettings(t, `groups:
  - name: Workers
    match: worker
    actions:
      - key: L
        name: worker log
        command: [less, +F, '{{.Env.WORKER_LOG}}']
        confirm: true
      - key: R
        name: reset
        command: ['{{.Env.WORKER_HOME}}/bin/reset', --name, '{{.Name}}']
        confirm: false
      - key: "1"
        name: open plist
        command: [open, '{{.PlistPath}}']
      - key: é
        name: édit
        command: [open, -e, '{{.PlistPath}}']
`)
			worker := service.Service{
				Name:      "com.example.worker.01",
				Env:       map[string]string{"WORKER_LOG": "/Users/me/worker/worker.log", "WORKER_HOME": "/Users/me/worker"},
				PlistPath: "/Users/me/Library/LaunchAgents/com.example.worker.01.plist",
			}

			require.Equal(t, []actionRead{
				{Key: "L", Name: "worker log", Line: []string{"less", "+F", "/Users/me/worker/worker.log"}, Confirm: true},
				{Key: "R", Name: "reset", Line: []string{"/Users/me/worker/bin/reset", "--name", "com.example.worker.01"}, Confirm: false},
				{Key: "1", Name: "open plist", Line: []string{"open", "/Users/me/Library/LaunchAgents/com.example.worker.01.plist"}, Confirm: false},
				{Key: "é", Name: "édit", Line: []string{"open", "-e", "/Users/me/Library/LaunchAgents/com.example.worker.01.plist"}, Confirm: false},
			}, actionsRead(settings.Groups[0], worker))
		})

		t.Run("a key, a name or a command of an action that YAML reads as a number or a boolean is its text", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: 7\n        name: 42\n        command: [true, 1.5]\n")

			require.Equal(t, []actionRead{{Key: "7", Name: "42", Line: []string{"true", "1.5"}}}, actionsRead(settings.Groups[0], service.Service{Name: "com.example.worker.01"}))
		})

		t.Run("a group with no actions, a group with an empty list of actions, and the group Other have no actions", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - {key: L, name: worker log, command: [less]}\n  - name: Databases\n    match: redis\n  - name: Jobs\n    match: job\n    actions: []\n")

			require.Len(t, settings.GroupOf("com.example.worker").Actions, 1)
			require.Empty(t, settings.GroupOf("homebrew.mxcl.redis").Actions)
			require.Empty(t, settings.GroupOf("com.example.job").Actions)
			require.Empty(t, settings.GroupOf("com.apple.a").Actions)
		})

		t.Run("an actions that is not a list gives an error that names the file and the group, and no settings", func(t *testing.T) {
			for _, content := range []string{"actions: L", "actions:", "actions: ~", "actions: {key: L, name: worker log, command: [less]}"} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `the setting "actions" of group "Workers" must be a list of actions`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a fault in an action gives an error that names the file, the group, the action by its place, and the fault, and no settings", func(t *testing.T) {
			for action, fault := range map[string]string{
				"L":        `action 2 of group "Workers" is not a set of settings`,
				"[L, log]": `action 2 of group "Workers" is not a set of settings`,
				"{key: R, name: reset, command: [reset], args: [x]}":       `svcview does not know the setting "args" of action 2 of group "Workers"`,
				"{key: R, name: reset, command: [reset], key: S}":          `the setting "key" of action 2 of group "Workers" is already on line`,
				"{name: reset, command: [reset]}":                          `action 2 of group "Workers" has no key`,
				"{key: R, command: [reset]}":                               `action 2 of group "Workers" has no name`,
				"{key: R, name: reset}":                                    `action 2 of group "Workers" has no command`,
				"{key: ctrl+l, name: reset, command: [reset]}":             `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: LL, name: reset, command: [reset]}":                 `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: \"\\t\", name: reset, command: [reset]}":            `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: ' ', name: reset, command: [reset]}":                `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: '', name: reset, command: [reset]}":                 `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: ~, name: reset, command: [reset]}":                  `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: [R], name: reset, command: [reset]}":                `the setting "key" of action 2 of group "Workers" must be one character that prints and is not a space`,
				"{key: R, name: '', command: [reset]}":                     `the setting "name" of action 2 of group "Workers" must be text that is not empty`,
				"{key: R, name: ~, command: [reset]}":                      `the setting "name" of action 2 of group "Workers" must be text that is not empty`,
				"{key: R, name: [reset], command: [reset]}":                `the setting "name" of action 2 of group "Workers" must be text that is not empty`,
				"{key: R, name: reset, command: reset}":                    `the setting "command" of action 2 of group "Workers" must be a list of texts`,
				"{key: R, name: reset, command: [reset, [x]]}":             `the setting "command" of action 2 of group "Workers" must be a list of texts`,
				"{key: R, name: reset, command: []}":                       `action 2 of group "Workers": the command is empty`,
				"{key: R, name: reset, command: ['', x]}":                  `action 2 of group "Workers": the program of the command is empty`,
				"{key: R, name: reset, command: [reset, '{{.Label}}']}":    `action 2 of group "Workers": the text "{{.Label}}" of the command uses {{.Label}},`,
				"{key: R, name: reset, command: [reset, '{{.Name']}":       `action 2 of group "Workers": the text "{{.Name" of the command is not a Go template`,
				"{key: R, name: reset, command: [reset], confirm: yes}":    `the setting "confirm" of action 2 of group "Workers" must be true or false`,
				"{key: R, name: reset, command: [reset], confirm: 'true'}": `the setting "confirm" of action 2 of group "Workers" must be true or false`,
				"{key: R, name: reset, command: [reset], confirm: 1}":      `the setting "confirm" of action 2 of group "Workers" must be true or false`,
				"{key: R, name: reset, command: [reset], confirm: ~}":      `the setting "confirm" of action 2 of group "Workers" must be true or false`,
				"{key: R, name: reset, command: [reset], confirm: [true]}": `the setting "confirm" of action 2 of group "Workers" must be true or false`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - {key: L, name: worker log, command: [less]}\n      - "+action+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, action)
				require.ErrorContains(t, err, fault, action)
				require.Equal(t, config.Settings{}, settings, action)
			}
		})

		t.Run("an action whose key is a key of svcview gives an error that names the file, the group, the action and the key, and no settings", func(t *testing.T) {
			for _, key := range []string{"?", "/", "h", "j", "k", "l", "o", "q", "r", "s", "x"} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - {key: R, name: reset, command: [reset]}\n      - {key: '"+key+"', name: worker log, command: [less]}\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, key)
				require.ErrorContains(t, err, `the key "`+key+`" of action 2 of group "Workers" is a key of svcview`, key)
				require.Equal(t, config.Settings{}, settings, key)
			}
		})

		t.Run("two actions of one group with the same key give an error that names the file, the group, the second action, the key and the line of the first action, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - {key: L, name: worker log, command: [less]}\n      - {key: R, name: reset, command: [reset]}\n      - {key: L, name: worker errors, command: [less]}\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `line 7: the key "L" of action 3 of group "Workers" is already the key of the action on line 5`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a key of svcview in upper case, two keys that differ only in case, and one key in two groups give their actions", func(t *testing.T) {
			settings := readSettings(t, `groups:
  - name: Workers
    match: worker
    actions:
      - {key: L, name: worker log, command: [less]}
      - {key: S, name: worker status, command: [cat]}
      - {key: X, name: worker exit, command: [kill]}
      - {key: a, name: archive, command: [tar]}
      - {key: A, name: archive all, command: [tar]}
  - name: Jobs
    match: job
    actions:
      - {key: L, name: job log, command: [less]}
`)
			keysAndNames := map[string][]string{}
			for _, group := range settings.Groups {
				for _, action := range group.Actions {
					keysAndNames[group.Name] = append(keysAndNames[group.Name], action.Key+" "+action.Name)
				}
			}

			require.Equal(t, map[string][]string{
				"Workers": {"L worker log", "S worker status", "X worker exit", "a archive", "A archive all"},
				"Jobs":    {"L job log"},
			}, keysAndNames)
		})

		t.Run("an action in a group of a file in conf.d gives an error that names that file", func(t *testing.T) {
			folder, path := folderWithConfD(t, "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - {key: ctrl+l, name: worker log, command: [less]}\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `action 1 of group "Workers"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a group with the name Other gives an error that names the file and the group, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n  - name: Other\n    match: redis\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"Other"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("two names that differ only in case are two groups", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n  - name: workers\n    match: job\n  - name: other\n    match: redis\n")

			require.Equal(t, []string{"Workers", "workers", "other"}, groupNames(settings))
		})

		t.Run("the groups of config.yaml come first, then the groups of each file of conf.d in the byte order of the names, and the groups of each file keep the order of that file", func(t *testing.T) {
			folder, _ := folderWith(t, "refresh: 5s\n"+groupsYAML("Main", "main", "Apple", "apple"))
			writeConfD(t, folder, "a.yaml", groupsYAML("LowerA", "lower"))
			writeConfD(t, folder, "B.yaml", groupsYAML("UpperB", "upper"))
			writeConfD(t, folder, "2-a.yaml", groupsYAML("Two", "two", "AlsoTwo", "also"))
			writeConfD(t, folder, "10-b.yaml", groupsYAML("Ten", "ten"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, []string{"Main", "Apple", "Ten", "Two", "AlsoTwo", "UpperB", "LowerA"}, groupNames(settings))
			require.Equal(t, 5*time.Second, settings.Refresh)
		})

		t.Run("a stale_after in the status of a group of a file of conf.d gives the status command that stale after", func(t *testing.T) {
			folder, _ := folderWithConfD(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n      stale_after: 1m\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, time.Minute, settings.Groups[0].Status.StaleAfter)
		})

		t.Run("a file of conf.d gives its groups when no config.yaml exists, and the refresh is 2s", func(t *testing.T) {
			folder, _ := folderWithConfD(t, groupsYAML("Workers", "worker"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, []string{"Workers"}, groupNames(settings))
			require.Equal(t, 2*time.Second, settings.Refresh)
		})

		t.Run("after the file of conf.d is deleted, a new read has no group of that file, and the services of the group are in Other", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Databases", "redis"))
			path := writeConfD(t, folder, "worker.yaml", groupsYAML("Workers", "worker"))
			before, err := config.Read(folder, os.Getuid(), noACL)
			require.NoError(t, err)
			require.NoError(t, os.Remove(path))

			after, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, map[string]string{"com.example.worker": "Workers", "homebrew.mxcl.redis": "Databases"}, groupOfEach(before, "com.example.worker", "homebrew.mxcl.redis"))
			require.Equal(t, map[string]string{"com.example.worker": "Other", "homebrew.mxcl.redis": "Databases"}, groupOfEach(after, "com.example.worker", "homebrew.mxcl.redis"))
		})

		t.Run("an entry of conf.d whose name does not end in .yaml, or starts with a dot, is skipped, whatever the entry is", func(t *testing.T) {
			folder, _ := folderWithConfD(t, groupsYAML("Workers", "worker"))
			confD := filepath.Join(folder, "conf.d")
			writeConfD(t, folder, "worker.yml", "groups: [\n")
			writeConfD(t, folder, "jobs.YAML", "colour: blue\n")
			writeConfD(t, folder, ".worker.yaml", "groups: [\n")
			writeConfD(t, folder, "._worker.yaml", "\x00\x05\x16\x07\x00\x02\x00\x00")
			require.NoError(t, os.Chmod(writeConfD(t, folder, "worker.yaml.off", groupsYAML("Off", "off")), 0o666))
			require.NoError(t, os.Mkdir(filepath.Join(confD, "old"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(confD, "old", "worker.yaml"), []byte("groups: [\n"), 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(confD, ".old.yaml"), 0o755))
			require.NoError(t, syscall.Mkfifo(filepath.Join(confD, "pipe.yml"), 0o644))

			settings, err := readWithin(t, 2*time.Second, folder)

			require.NoError(t, err)
			require.Equal(t, []string{"Workers"}, groupNames(settings))
		})

		t.Run("an entry of conf.d whose name ends in .yaml and that is not a regular file gives an error at once that names the entry and says that it is not a regular file, and no settings", func(t *testing.T) {
			for kind, create := range notRegularFiles {
				folder, _ := folderWith(t, groupsYAML("Databases", "redis"))
				path := filepath.Join(folder, "conf.d", "worker.yaml")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				create(t, path)

				settings, err := readWithin(t, 2*time.Second, folder)

				require.ErrorContains(t, err, path, kind)
				require.ErrorContains(t, err, "is not a regular file", kind)
				require.Equal(t, config.Settings{}, settings, kind)
			}
		})

		t.Run("an entry of conf.d that is a symbolic link to no file is skipped, and one that is a symbolic link to a file gives the groups of that file", func(t *testing.T) {
			folder, _ := folderWithConfD(t, groupsYAML("Workers", "worker"))
			confD := filepath.Join(folder, "conf.d")
			target := filepath.Join(t.TempDir(), "settings.yaml")
			require.NoError(t, os.WriteFile(target, []byte(groupsYAML("Jobs", "job")), 0o644))
			require.NoError(t, os.Symlink(target, filepath.Join(confD, "jobs.yaml")))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "gone.yaml"), filepath.Join(confD, "gone.yaml")))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, []string{"Jobs", "Workers"}, groupNames(settings))
		})

		t.Run("with no conf.d, or with a conf.d that is a symbolic link to no folder, the settings are those of config.yaml", func(t *testing.T) {
			noConfD, _ := folderWith(t, "refresh: 5s\n"+groupsYAML("Databases", "redis"))
			linkToNoFolder, _ := folderWith(t, "refresh: 5s\n"+groupsYAML("Databases", "redis"))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(linkToNoFolder, "conf.d")))

			for _, folder := range []string{noConfD, linkToNoFolder} {
				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.NoError(t, err, folder)
				require.Equal(t, []string{"Databases"}, groupNames(settings), folder)
				require.Equal(t, 5*time.Second, settings.Refresh, folder)
			}
		})

		t.Run("a conf.d that is a symbolic link to a folder gives the files of that folder", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Databases", "redis"))
			other := t.TempDir()
			writeConfD(t, other, "worker.yaml", groupsYAML("Workers", "worker"))
			require.NoError(t, os.Symlink(filepath.Join(other, "conf.d"), filepath.Join(folder, "conf.d")))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, []string{"Databases", "Workers"}, groupNames(settings))
		})

		t.Run("a conf.d that is a file or a named pipe gives an error at once that names conf.d, and no settings", func(t *testing.T) {
			for kind, create := range map[string]func(t *testing.T, path string){
				"a file": func(t *testing.T, path string) {
					require.NoError(t, os.WriteFile(path, []byte(groupsYAML("Workers", "worker")), 0o644))
				},
				"a named pipe with no writer": func(t *testing.T, path string) {
					require.NoError(t, syscall.Mkfifo(path, 0o644))
				},
			} {
				folder, _ := folderWith(t, groupsYAML("Databases", "redis"))
				confD := filepath.Join(folder, "conf.d")
				create(t, confD)

				settings, err := readWithin(t, 2*time.Second, folder)

				require.ErrorContains(t, err, confD, kind)
				require.Equal(t, config.Settings{}, settings, kind)
			}
		})

		t.Run("a conf.d that svcview cannot list gives an error that names conf.d, and no settings", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root lists a folder with no read permission")
			}
			folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
			confD := filepath.Dir(path)
			require.NoError(t, os.Chmod(confD, 0o000))
			t.Cleanup(func() { _ = os.Chmod(confD, 0o755) })

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, confD)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a file of conf.d that svcview cannot read gives an error that names the file, and no settings", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
			require.NoError(t, os.Chmod(path, 0o000))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.True(t, errors.Is(err, os.ErrPermission), err)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config.yaml that is a symbolic link to a file that svcview cannot read gives the error of the open, not that it is not a regular file, and no settings", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			target := filepath.Join(t.TempDir(), "settings.yaml")
			require.NoError(t, os.WriteFile(target, []byte("refresh: 5s\n"), 0o600))
			require.NoError(t, os.Chmod(target, 0o000))
			folder := t.TempDir()
			path := filepath.Join(folder, "config.yaml")
			require.NoError(t, os.Symlink(target, path))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.True(t, errors.Is(err, os.ErrPermission), err)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a config file that is a symbolic link to itself gives an error that names the file, and no settings", func(t *testing.T) {
			inConfigYAML := t.TempDir()
			configYAML := filepath.Join(inConfigYAML, "config.yaml")
			require.NoError(t, os.Symlink(configYAML, configYAML))
			inConfD, _ := folderWith(t, groupsYAML("Databases", "redis"))
			fileOfConfD := filepath.Join(inConfD, "conf.d", "worker.yaml")
			require.NoError(t, os.Mkdir(filepath.Dir(fileOfConfD), 0o755))
			require.NoError(t, os.Symlink(fileOfConfD, fileOfConfD))

			for folder, path := range map[string]string{inConfigYAML: configYAML, inConfD: fileOfConfD} {
				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, path)
				require.Equal(t, config.Settings{}, settings, path)
			}
		})

		t.Run("a file of conf.d that its group or all users can write gives an error that names the file and says that another user can write it, and no settings", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o620, 0o602, 0o666, 0o777} {
				folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, mode)
				require.ErrorContains(t, err, "another user can write", mode)
				require.Equal(t, config.Settings{}, settings, mode)
			}
		})

		t.Run("a file of conf.d that another user owns gives an error that names the file and says that another user can write it, also when only its owner can write it", func(t *testing.T) {
			folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
			require.NoError(t, os.Chmod(path, 0o600))

			settings, err := config.Read(folder, os.Getuid()+1, noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, "another user can write")
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a file of conf.d that the user of svcview owns and that only its owner can write gives its groups", func(t *testing.T) {
			for _, mode := range []os.FileMode{0o644, 0o600, 0o400, 0o755} {
				folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
				require.NoError(t, os.Chmod(path, mode))

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.NoError(t, err, mode)
				require.Equal(t, []string{"Workers"}, groupNames(settings), mode)
			}
		})

		t.Run("a file of conf.d that is a symbolic link counts with the mode of the file that it links to", func(t *testing.T) {
			for mode, refused := range map[os.FileMode]bool{0o644: false, 0o666: true} {
				target := filepath.Join(t.TempDir(), "settings.yaml")
				require.NoError(t, os.WriteFile(target, []byte(groupsYAML("Workers", "worker")), 0o600))
				require.NoError(t, os.Chmod(target, mode))
				folder := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(folder, "conf.d"), 0o755))
				require.NoError(t, os.Symlink(target, filepath.Join(folder, "conf.d", "worker.yaml")))

				settings, err := config.Read(folder, os.Getuid(), noACL)

				if refused {
					require.ErrorContains(t, err, "another user can write", mode)
					require.Equal(t, config.Settings{}, settings, mode)
				} else {
					require.NoError(t, err, mode)
					require.Equal(t, []string{"Workers"}, groupNames(settings), mode)
				}
			}
		})

		t.Run("a file of conf.d with one or more ACL entries gives an error that names the file and chmod -N, and no settings, also when the owner and the mode pass", func(t *testing.T) {
			for _, count := range []int{1, 2} {
				folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))

				settings, err := config.Read(folder, os.Getuid(), aclWithEntries(count))

				require.ErrorContains(t, err, path, count)
				require.ErrorContains(t, err, "ACL", count)
				require.ErrorContains(t, err, "chmod -N", count)
				require.Equal(t, config.Settings{}, settings, count)
			}
		})

		t.Run("a file of conf.d whose ACL svcview cannot count gives an error that names the file and the cause, and no settings", func(t *testing.T) {
			folder, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
			unreadable := func(*os.File) (int, error) {
				return 0, errors.New("fgetattrlist: bad file descriptor")
			}

			settings, err := config.Read(folder, os.Getuid(), unreadable)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, "fgetattrlist: bad file descriptor")
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("on macOS, a file of conf.d with the mode 0644 and an ACL entry gives an error that names the file and chmod -N, and so does a symbolic link in conf.d to a file with an ACL entry", func(t *testing.T) {
			skipUnlessMacOS(t)
			inConfD, path := folderWithConfD(t, groupsYAML("Workers", "worker"))
			chmod(t, "+a", "everyone allow write", path)
			target := configFile(t, "everyone allow write")
			throughLink := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(throughLink, "conf.d"), 0o755))
			link := filepath.Join(throughLink, "conf.d", "worker.yaml")
			require.NoError(t, os.Symlink(target, link))

			for folder, named := range map[string]string{inConfD: path, throughLink: link} {
				settings, err := config.Read(folder, os.Getuid(), config.ACLEntries)

				require.ErrorContains(t, err, named, folder)
				require.ErrorContains(t, err, "chmod -N", folder)
				require.Equal(t, config.Settings{}, settings, folder)
			}
		})

		t.Run("a match in a file of conf.d that is not a Go regular expression gives an error that names that file and the group, and no settings", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Jobs", "job"))
			path := writeConfD(t, folder, "worker.yaml", groupsYAML("Databases", "'['"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `group "Databases" is not a Go regular expression`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a refresh in a file of conf.d gives an error that names the file and says that only config.yaml can set it, and no settings, with or without a refresh in config.yaml, and with no config.yaml", func(t *testing.T) {
			withRefresh, _ := folderWith(t, "refresh: 5s\n")
			withoutRefresh, _ := folderWith(t, groupsYAML("Databases", "redis"))
			noConfigYAML := t.TempDir()
			for _, folder := range []string{withRefresh, withoutRefresh, noConfigYAML} {
				path := writeConfD(t, folder, "worker.yaml", "refresh: 10s\n"+groupsYAML("Workers", "worker"))

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, folder)
				require.ErrorContains(t, err, `only config.yaml can set the setting "refresh"`, folder)
				require.Equal(t, config.Settings{}, settings, folder)
			}
		})

		t.Run("a refresh in a later YAML document of a file of conf.d, or a refresh that is not a duration there, gives the same error", func(t *testing.T) {
			for _, content := range []string{
				groupsYAML("Workers", "worker") + "---\nrefresh: 10s\n",
				"refresh: fast\n",
				"refresh: [5s]\n",
			} {
				folder, path := folderWithConfD(t, content)

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `only config.yaml can set the setting "refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting that svcview does not know in a file of conf.d gives the error that it gives in config.yaml, with the name of the file of conf.d", func(t *testing.T) {
			folder, path := folderWithConfD(t, groupsYAML("Workers", "worker")+"colour: blue\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `svcview does not know the setting "colour"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a group of a file of conf.d whose name a group of config.yaml has gives an error that names the file of conf.d, the group, and config.yaml, and no settings", func(t *testing.T) {
			folder, configYAML := folderWith(t, groupsYAML("Databases", "redis", "Workers", "worker"))
			path := writeConfD(t, folder, "worker.yaml", groupsYAML("Jobs", "job", "Workers", "worker"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, "the config file "+path+": line 4:")
			require.ErrorContains(t, err, `group "Workers"`)
			require.ErrorContains(t, err, "the config file "+configYAML)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a group of a file of conf.d whose name a group of an earlier file of conf.d has gives an error that names the later file, the group, and the earlier file, and no settings", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Databases", "redis"))
			later := writeConfD(t, folder, "b.yaml", groupsYAML("Workers", "job"))
			earlier := writeConfD(t, folder, "a.yaml", groupsYAML("Workers", "worker"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, "the config file "+later+": line 2:")
			require.ErrorContains(t, err, `group "Workers"`)
			require.ErrorContains(t, err, "the config file "+earlier)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("two names that differ only in case, in two config files, are two groups", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Workers", "worker"))
			writeConfD(t, folder, "a.yaml", groupsYAML("workers", "job"))
			writeConfD(t, folder, "b.yaml", groupsYAML("WORKERS", "task"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, []string{"Workers", "workers", "WORKERS"}, groupNames(settings))
		})

		t.Run("two groups with the same name in one config file give an error that names the file, the group and the line of each, also in a file of conf.d", func(t *testing.T) {
			inConfigYAML, configYAML := folderWith(t, groupsYAML("Workers", "worker", "Workers", "job"))
			inConfD, _ := folderWith(t, groupsYAML("Databases", "redis"))
			fileOfConfD := writeConfD(t, inConfD, "worker.yaml", groupsYAML("Workers", "worker", "Workers", "job"))

			for folder, path := range map[string]string{inConfigYAML: configYAML, inConfD: fileOfConfD} {
				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, "the config file "+path+": line 4: the name of group \"Workers\" is already the name of the group on line 2", path)
				require.Equal(t, config.Settings{}, settings, path)
			}
		})
	})

	t.Run("group of a service", func(t *testing.T) {
		t.Run("a service is in a group when the match of the group matches any part of its name", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n  - name: Databases\n    match: redis|postgres\n")

			require.Equal(t, map[string]string{
				"com.example.worker.01":  "Workers",
				"worker":                 "Workers",
				"homebrew.mxcl.postgres": "Databases",
				"homebrew.mxcl.redis":    "Databases",
			}, groupOfEach(settings, "com.example.worker.01", "worker", "homebrew.mxcl.postgres", "homebrew.mxcl.redis"))
		})

		t.Run("^ and $ in a match tie it to the start and the end of the name", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Starts\n    match: ^com\\.example\\.\n  - name: Ends\n    match: \\.01$\n")

			require.Equal(t, map[string]string{
				"com.example.api":       "Starts",
				"org.com.example.api":   "Other",
				"org.example.worker.01": "Ends",
				"org.example.01.worker": "Other",
			}, groupOfEach(settings, "com.example.api", "org.com.example.api", "org.example.worker.01", "org.example.01.worker"))
		})

		t.Run("a service that the matches of two groups match is in the first of them", func(t *testing.T) {
			workersFirst := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n  - name: Examples\n    match: example\n")
			examplesFirst := readSettings(t, "groups:\n  - name: Examples\n    match: example\n  - name: Workers\n    match: worker\n")

			require.Equal(t, "Workers", workersFirst.GroupOf("com.example.worker.01").Name)
			require.Equal(t, "Examples", examplesFirst.GroupOf("com.example.worker.01").Name)
		})

		t.Run("a service that the groups of two config files match is in the group of the earlier file in file order", func(t *testing.T) {
			folder, _ := folderWith(t, groupsYAML("Examples", "example"))
			writeConfD(t, folder, "b.yaml", groupsYAML("Jobs", "job"))
			writeConfD(t, folder, "a.yaml", groupsYAML("Workers", "worker"))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.NoError(t, err)
			require.Equal(t, map[string]string{
				"com.example.worker": "Examples",
				"org.worker.job":     "Workers",
				"org.job":            "Jobs",
			}, groupOfEach(settings, "com.example.worker", "org.worker.job", "org.job"))
		})

		t.Run("a service that no group matches is in the group Other", func(t *testing.T) {
			settings := readSettings(t, "groups:\n  - name: Workers\n    match: worker\n")

			require.Equal(t, "Other", settings.GroupOf("homebrew.mxcl.redis").Name)
		})

		t.Run("with no groups, each service is in the group Other", func(t *testing.T) {
			settings, err := config.Read(t.TempDir(), os.Getuid(), noACL)
			require.NoError(t, err)

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Other",
				"homebrew.mxcl.redis":   "Other",
			}, groupOfEach(settings, "com.example.worker.01", "homebrew.mxcl.redis"))
		})
	})
}
