//go:build unit

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpcsc/svcview/internal/config"
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

		t.Run("a config.yaml that is a folder gives an error that names the file, and no settings", func(t *testing.T) {
			folder := t.TempDir()
			path := filepath.Join(folder, "config.yaml")
			require.NoError(t, os.Mkdir(path, 0o755))

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.Equal(t, config.Settings{}, settings)
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
				"groups:\n  - name: Databases\n    match: redis\n    actions: [restart]\n": `"actions" of group "Databases"`,
				"groups:\n  - actions: [restart]\n    name: Databases\n    match: redis\n": `"actions" of group "Databases"`,
				"groups:\n  - nme: Databases\n    match: redis\n":                          `"nme" of group 1`,
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

		t.Run("an every or a timeout that is not a duration more than 0 gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"every: fast":    `"every"`,
				"every: 0s":      `"every"`,
				"every: -5s":     `"every"`,
				"every:":         `"every"`,
				"timeout: 5":     `"timeout"`,
				"timeout: 0":     `"timeout"`,
				"timeout: [1s]":  `"timeout"`,
				"timeout: -1ms":  `"timeout"`,
				"timeout: ''":    `"timeout"`,
				"every: 1m30s x": `"every"`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting+` of the status of group "Workers"`, content)
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
			for _, setting := range []string{"stale_after", "actions", "interval"} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, status.json]\n      "+setting+": 1m\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, setting)
				require.ErrorContains(t, err, `svcview does not know the setting "`+setting+`" of the status of group "Workers"`, setting)
				require.Equal(t, config.Settings{}, settings, setting)
			}
		})

		t.Run("a setting that status sets two times gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"command: [cat, status.json]\n      every: 5s\n      every: 6s":     `"every" of the status of group "Workers"`,
				"command: [cat, a.json]\n      command: [cat, b.json]":              `"command" of the status of group "Workers"`,
				"command: [cat, status.json]\n      timeout: 1s\n      timeout: 1s": `"timeout" of the status of group "Workers"`,
			} {
				folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n    status:\n      "+content+"\n")

				settings, err := config.Read(folder, os.Getuid(), noACL)

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting+" is already on line", content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("two groups with the same name give an error that names the file and the group, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Databases\n    match: redis\n  - name: Workers\n    match: worker\n  - name: Databases\n    match: postgres\n")

			settings, err := config.Read(folder, os.Getuid(), noACL)

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `group "Databases"`)
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
