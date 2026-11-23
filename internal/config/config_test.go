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

func readSettings(t *testing.T, content string) config.Settings {
	t.Helper()
	folder, _ := folderWith(t, content)
	settings, err := config.Read(folder, os.Getuid())
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
				require.ErrorContains(t, err, `"refresh"`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a refresh that is not more than 0 gives an error that names the file and refresh, and no settings", func(t *testing.T) {
			for _, content := range []string{"refresh: 0s\n", "refresh: 0\n", "refresh: -5s\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
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

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `"status"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("groups that is not a list gives an error that names the file and the setting, and no settings", func(t *testing.T) {
			for _, content := range []string{"groups: Workers\n", "groups:\n  name: Workers\n  match: worker\n"} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, `group "Databases" is not a Go regular expression`, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("a setting in a group that svcview does not know gives an error that names the file, the group and the setting, and no settings", func(t *testing.T) {
			for content, setting := range map[string]string{
				"groups:\n  - name: Databases\n    match: redis\n    status: running\n": `"status" of group "Databases"`,
				"groups:\n  - status: running\n    name: Databases\n    match: redis\n": `"status" of group "Databases"`,
				"groups:\n  - nme: Databases\n    match: redis\n":                       `"nme" of group 1`,
			} {
				folder, path := folderWith(t, content)

				settings, err := config.Read(folder, os.Getuid())

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

				settings, err := config.Read(folder, os.Getuid())

				require.ErrorContains(t, err, path, content)
				require.ErrorContains(t, err, setting, content)
				require.Equal(t, config.Settings{}, settings, content)
			}
		})

		t.Run("two groups with the same name give an error that names the file and the group, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Databases\n    match: redis\n  - name: Workers\n    match: worker\n  - name: Databases\n    match: postgres\n")

			settings, err := config.Read(folder, os.Getuid())

			require.ErrorContains(t, err, path)
			require.ErrorContains(t, err, `group "Databases"`)
			require.Equal(t, config.Settings{}, settings)
		})

		t.Run("a group with the name Other gives an error that names the file and the group, and no settings", func(t *testing.T) {
			folder, path := folderWith(t, "groups:\n  - name: Workers\n    match: worker\n  - name: Other\n    match: redis\n")

			settings, err := config.Read(folder, os.Getuid())

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
			settings, err := config.Read(t.TempDir(), os.Getuid())
			require.NoError(t, err)

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Other",
				"homebrew.mxcl.redis":   "Other",
			}, groupOfEach(settings, "com.example.worker.01", "homebrew.mxcl.redis"))
		})
	})
}

func TestGroup(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		t.Run("a group has the name it was given, and holds each service whose name its match matches", func(t *testing.T) {
			workers, err := config.NewGroup("Workers", `^com\.example\.worker\.`)
			require.NoError(t, err)
			settings := config.Settings{Groups: []config.Group{workers}}

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Workers",
				"com.example.api":       "Other",
			}, groupOfEach(settings, "com.example.worker.01", "com.example.api"))
		})

		t.Run("a match that is not a Go regular expression gives an error that names the group", func(t *testing.T) {
			_, err := config.NewGroup("Databases", "redis(")

			require.ErrorContains(t, err, `group "Databases"`)
		})

		t.Run("a group with no name, a group with the name Other, and a group with no match give an error", func(t *testing.T) {
			for name, match := range map[string]string{"": "worker", "Other": "worker", "Workers": ""} {
				_, err := config.NewGroup(name, match)

				require.Error(t, err, "name %q, match %q", name, match)
			}
		})

		t.Run("a group with no match, as the group Other, holds no service", func(t *testing.T) {
			settings := config.Settings{Groups: []config.Group{{Name: "Workers"}, config.Other}}

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Other",
				"":                      "Other",
			}, groupOfEach(settings, "com.example.worker.01", ""))
		})
	})

	t.Run("short name", func(t *testing.T) {
		t.Run("is the part of the name after the text that the match matched, when the match starts at the first character, and else the full name", func(t *testing.T) {
			shortNames := map[[2]string]string{}
			for _, match := range []string{`^com\.example\.worker\.`, `com\.example\.`, `worker`, `^com\.example\.worker\.01$`, `.*`, `redis`, `x*`} {
				group, err := config.NewGroup("Workers", match)
				require.NoError(t, err)
				shortNames[[2]string{match, "com.example.worker.01"}] = group.ShortName("com.example.worker.01")
			}

			require.Equal(t, map[[2]string]string{
				{`^com\.example\.worker\.`, "com.example.worker.01"}:    "01",
				{`com\.example\.`, "com.example.worker.01"}:             "worker.01",
				{`worker`, "com.example.worker.01"}:                     "com.example.worker.01",
				{`^com\.example\.worker\.01$`, "com.example.worker.01"}: "com.example.worker.01",
				{`.*`, "com.example.worker.01"}:                         "com.example.worker.01",
				{`redis`, "com.example.worker.01"}:                      "com.example.worker.01",
				{`x*`, "com.example.worker.01"}:                         "com.example.worker.01",
			}, shortNames)
		})

		t.Run("in the group Other is the full name", func(t *testing.T) {
			require.Equal(t, "com.example.worker.01", config.Other.ShortName("com.example.worker.01"))
		})
	})
}
