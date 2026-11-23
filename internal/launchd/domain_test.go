//go:build unit

package launchd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func homeWithPlists(t *testing.T, names ...string) string {
	t.Helper()
	home := homeWith(t, nil)
	for _, name := range names {
		plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>Label</key><string>` + name + `</string></dict></plist>
`
		require.NoError(t, os.WriteFile(filepath.Join(home, "Library", "LaunchAgents", name+".plist"), []byte(plist), 0o644))
	}
	return home
}

func copyFixture(t *testing.T, path, fixture string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

func copyFixtureModifiedAt(t *testing.T, path, fixture string, modTime time.Time) {
	t.Helper()
	copyFixture(t, path, fixture)
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

var sharedFakes string

// macOS checks each new executable file at its first run, which costs about 0.3 s, so the tests share one copy of each fake.
func TestMain(m *testing.M) {
	folder, err := os.MkdirTemp("", "svcview-fakes-")
	if err != nil {
		panic(err)
	}
	scripts := map[string]string{
		"launchctl": "#!/bin/sh\n" +
			"[ \"$*\" = list ] || exit 64\n" +
			"exec cat \"$FAKE_LAUNCHCTL_LIST\"\n",
		"ps": "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >> \"$FAKE_PS_CALLS\"\n" +
			"[ $# -eq 4 ] && [ \"$1 $2 $3\" = '-o pid=,etime= -p' ] || exit 64\n" +
			"exec cat \"$FAKE_PS_OUTPUT\"\n",
	}
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(script), 0o755); err != nil {
			panic(err)
		}
	}
	sharedFakes = folder
	code := m.Run()
	_ = os.RemoveAll(folder)
	os.Exit(code)
}

func putFirstOnPath(t *testing.T, folder string) {
	t.Helper()
	t.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeProgram(t *testing.T, name, content string) {
	t.Helper()
	folder := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(folder, name), []byte(content), 0o755))
	putFirstOnPath(t, folder)
}

func fakeLaunchctl(t *testing.T, script string) {
	t.Helper()
	fakeProgram(t, "launchctl", "#!/bin/sh\n"+script)
}

func fakeLaunchctlList(t *testing.T, fixture string) string {
	t.Helper()
	output := filepath.Join(t.TempDir(), "launchctl-list.txt")
	copyFixture(t, output, fixture)
	t.Setenv("FAKE_LAUNCHCTL_LIST", output)
	putFirstOnPath(t, sharedFakes)
	return output
}

func fakePs(t *testing.T, fixture string) (calls string) {
	t.Helper()
	folder := t.TempDir()
	output := filepath.Join(folder, "ps-output.txt")
	copyFixture(t, output, fixture)
	calls = filepath.Join(folder, "ps-calls.txt")
	t.Setenv("FAKE_PS_OUTPUT", output)
	t.Setenv("FAKE_PS_CALLS", calls)
	putFirstOnPath(t, sharedFakes)
	return calls
}

func uptimeByName(services []service.Service) map[string]*time.Duration {
	uptimes := map[string]*time.Duration{}
	for _, svc := range services {
		uptimes[svc.Name] = svc.Uptime
	}
	return uptimes
}

func TestDomain(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		t.Run("an XML plist and a binary plist each give a service named by its Label", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, map[string]string{
				"com.example.xml.plist":    "xml.plist",
				"com.example.binary.plist": "binary.plist",
			})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{{Name: "com.example.xml"}, {Name: "com.example.binary"}}, services)
		})

		t.Run("the name comes from the Label, not from the file name", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, map[string]string{"another-name.plist": "xml.plist"})

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a file that is not a plist, a plist with no Label, and a file without the .plist extension give no service, and the other files still give theirs", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
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
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, map[string]string{"com.example.binary.plist": "binary.plist"})
			require.NoError(t, os.Mkdir(filepath.Join(home, "Library", "LaunchAgents", "folder.plist"), 0o755))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a symbolic link to a plist gives the service of the plist", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, nil)
			target, err := filepath.Abs(filepath.Join("testdata", "xml.plist"))
			require.NoError(t, err)
			require.NoError(t, os.Symlink(target, filepath.Join(home, "Library", "LaunchAgents", "com.example.xml.plist")))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a symbolic link to a file that does not exist gives no service, and the other files still give theirs", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, map[string]string{"com.example.binary.plist": "binary.plist"})
			require.NoError(t, os.Symlink(filepath.Join(home, "missing.plist"), filepath.Join(home, "Library", "LaunchAgents", "a-link.plist")))

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("two plists with the same Label give one service", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
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
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")

			services, err := launchd.NewDomain(t.TempDir(), 501).Services()

			require.NoError(t, err)
			require.Empty(t, services)
		})

		t.Run("a LaunchAgents folder that cannot be read gives an error that names the folder", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(home, "Library"), 0o755))
			folder := filepath.Join(home, "Library", "LaunchAgents")
			require.NoError(t, os.WriteFile(folder, []byte("a file, not a folder"), 0o644))

			_, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, folder)
		})

		t.Run("a name that launchctl list shows with a PID is running, a name with - as the PID is loaded, and a name that it does not show is not loaded", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running", "com.example.loaded", "com.example.not-loaded")
			fakeLaunchctlList(t, "launchctl-list-each-state.txt")
			fakePs(t, "ps-no-lines.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.running", State: service.Running, PID: 4122},
				{Name: "com.example.loaded", State: service.Loaded},
				{Name: "com.example.not-loaded", State: service.NotLoaded},
			}, services)
		})

		t.Run("a name that launchctl list shows, but that no plist has, gives no service", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			fakeLaunchctlList(t, "launchctl-list-each-state.txt")
			fakePs(t, "ps-no-lines.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.running", State: service.Running, PID: 4122}}, services)
		})

		t.Run("a Status of 0 is a last exit of 0, a positive Status is that exit code, and a negative Status is the signal of that number as macOS numbers signals", func(t *testing.T) {
			home := homeWithPlists(t,
				"com.example.exit-0",
				"com.example.exit-78",
				"com.example.hangup",
				"com.example.killed",
				"com.example.bus-error",
				"com.example.terminated",
				"com.example.user-2",
				"com.example.signal-32",
				"com.example.smallest-status",
			)
			fakeLaunchctlList(t, "launchctl-list-last-exits.txt")
			fakePs(t, "ps-no-lines.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.exit-0", State: service.Loaded},
				{Name: "com.example.exit-78", State: service.Loaded, LastExit: service.LastExit{Code: 78}},
				{Name: "com.example.hangup", State: service.Loaded, LastExit: service.LastExit{Signal: "SIGHUP"}},
				{Name: "com.example.killed", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}, PID: 4122},
				{Name: "com.example.bus-error", State: service.Loaded, LastExit: service.LastExit{Signal: "SIGBUS"}},
				{Name: "com.example.terminated", State: service.Loaded, LastExit: service.LastExit{Signal: "SIGTERM"}},
				{Name: "com.example.user-2", State: service.Loaded, LastExit: service.LastExit{Signal: "SIGUSR2"}},
				{Name: "com.example.signal-32", State: service.Loaded, LastExit: service.LastExit{Signal: "signal 32"}},
				{Name: "com.example.smallest-status", State: service.Loaded, LastExit: service.LastExit{Signal: "signal 2147483648"}},
			}, services)
		})

		t.Run("a service that launchctl list does not show has a last exit of 0", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.not-listed")
			fakeLaunchctlList(t, "launchctl-list-last-exits.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.not-listed", State: service.NotLoaded}}, services)
		})

		t.Run("a Status that is smaller than the smallest 32-bit int gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-status-out-of-range.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"-\t-2147483649\tcom.example.loaded"`)
			require.Empty(t, services)
		})

		t.Run("a Status that is not a number gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-status-not-a-number.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"-\texited\tcom.example.loaded"`)
			require.Empty(t, services)
		})

		t.Run("with no launchctl on PATH, gives the error that launchctl is not found, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			t.Setenv("PATH", t.TempDir())

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorIs(t, err, exec.ErrNotFound)
			require.Empty(t, services)
		})

		t.Run("the name is the rest of the row after the PID and the Status, so a tab in a name stays in it", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.with\ttab")
			fakeLaunchctlList(t, "launchctl-list-name-with-tab.txt")
			fakePs(t, "ps-no-lines.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.with\ttab", State: service.Running, PID: 4122}}, services)
		})

		t.Run("an output whose header line is not PID, Status, Label gives an error that names the header line, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-unknown-header.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"PID\tStatus\tName"`)
			require.Empty(t, services)
		})

		t.Run("a launchctl that exits with a code other than 0 gives an error with the exit code and what launchctl printed, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			fakeLaunchctl(t, "echo 'Bootstrap failed: 5: Input/output error' >&2\necho 'printed on stdout'\nexit 3\n")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, "exit status 3")
			require.ErrorContains(t, err, "Bootstrap failed: 5: Input/output error\nprinted on stdout")
			require.Empty(t, services)
		})

		t.Run("a launchctl list that does not end within its deadline gives an error that says so, and no services, also when a child of launchctl keeps the output open", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			fakeLaunchctl(t, "sleep 60\necho never\n")
			started := time.Now()

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, "launchctl list did not end within")
			require.Empty(t, services)
			require.Less(t, time.Since(started), 10*time.Second)
		})

		t.Run("a row that does not have three columns gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running", "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-row-with-two-columns.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"-\tcom.example.loaded"`)
			require.Empty(t, services)
		})

		t.Run("a running service has the PID that launchctl list shows for it and the uptime that ps reports for that PID, and a loaded or not loaded service has no PID and no uptime", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second", "com.example.loaded", "com.example.not-loaded")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			fakePs(t, "ps-two-pids.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.first", State: service.Running, PID: 4122, Uptime: new(3*time.Minute + 12*time.Second)},
				{Name: "com.example.second", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}, PID: 4123, Uptime: new(time.Hour + 3*time.Minute + 12*time.Second)},
				{Name: "com.example.loaded", State: service.Loaded},
				{Name: "com.example.not-loaded", State: service.NotLoaded},
			}, services)
		})

		t.Run("runs ps one time, with the PIDs of all running services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second", "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			calls := fakePs(t, "ps-two-pids.txt")

			_, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			data, err := os.ReadFile(calls)
			require.NoError(t, err)
			pids, found := strings.CutPrefix(string(data), "-o pid=,etime= -p ")
			require.True(t, found, string(data))
			require.ElementsMatch(t, []string{"4122", "4123"}, strings.Split(strings.TrimSuffix(pids, "\n"), ","))
		})

		t.Run("does not run ps when no service is running", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded", "com.example.not-loaded")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			calls := fakePs(t, "ps-two-pids.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.loaded", State: service.Loaded},
				{Name: "com.example.not-loaded", State: service.NotLoaded},
			}, services)
			require.NoFileExists(t, calls)
		})

		t.Run("each form of the elapsed time that ps prints, [[dd-]hh:]mm:ss, gives the uptime it reads as, and 00:00 gives an uptime of 0", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.101", "com.example.102", "com.example.103", "com.example.104", "com.example.105", "com.example.106")
			fakeLaunchctlList(t, "launchctl-list-running-101-to-107.txt")
			fakePs(t, "ps-each-elapsed-form.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, map[string]*time.Duration{
				"com.example.101": new(7 * time.Second),
				"com.example.102": new(3*time.Minute + 12*time.Second),
				"com.example.103": new(time.Hour + 3*time.Minute + 12*time.Second),
				"com.example.104": new(4*24*time.Hour + 9*time.Hour + 16*time.Minute + 32*time.Second),
				"com.example.105": new(24*time.Hour + time.Second),
				"com.example.106": new(time.Duration(0)),
			}, uptimeByName(services))
		})

		t.Run("a PID that ps does not report gives its service no uptime, and the other services keep theirs", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			fakePs(t, "ps-one-pid.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, map[string]*time.Duration{
				"com.example.first":  nil,
				"com.example.second": new(time.Hour + 3*time.Minute + 12*time.Second),
			}, uptimeByName(services))
		})

		t.Run("a line of ps that is not a PID and an elapsed time gives no uptime, and the other lines keep theirs", func(t *testing.T) {
			home := homeWithPlists(t,
				"com.example.101", "com.example.102", "com.example.103", "com.example.104",
				"com.example.105", "com.example.106", "com.example.107", "com.example.loaded")
			fakeLaunchctlList(t, "launchctl-list-running-101-to-107.txt")
			fakePs(t, "ps-lines-in-other-forms.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.Equal(t, map[string]*time.Duration{
				"com.example.101":    new(3*time.Minute + 12*time.Second),
				"com.example.102":    nil,
				"com.example.103":    nil,
				"com.example.104":    nil,
				"com.example.105":    nil,
				"com.example.106":    nil,
				"com.example.107":    nil,
				"com.example.loaded": nil,
			}, uptimeByName(services))
		})

		t.Run("when ps exits with a code other than 0, no service has an uptime, and the read still gives the services with their state and last exit", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			fakeProgram(t, "ps", "#!/bin/sh\necho ' 4122       03:12'\nexit 2\n")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.first", State: service.Running, PID: 4122},
				{Name: "com.example.second", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}, PID: 4123},
			}, services)
		})

		t.Run("when ps cannot start, no service has an uptime, and the read still gives the services with their state and last exit", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			fakeProgram(t, "ps", "#!/no/such/interpreter\n")

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.first", State: service.Running, PID: 4122},
				{Name: "com.example.second", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}, PID: 4123},
			}, services)
		})

		t.Run("when ps does not end within its deadline, no service has an uptime, and the read still gives the services, also when a child of ps keeps the output open", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.first", "com.example.second")
			fakeLaunchctlList(t, "launchctl-list-two-running.txt")
			fakeProgram(t, "ps", "#!/bin/sh\nsleep 60\necho never\n")
			started := time.Now()

			services, err := launchd.NewDomain(home, 501).Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{
				{Name: "com.example.first", State: service.Running, PID: 4122},
				{Name: "com.example.second", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}, PID: 4123},
			}, services)
			require.Less(t, time.Since(started), 10*time.Second)
		})

		t.Run("a PID that is not a number gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			fakeLaunchctlList(t, "launchctl-list-pid-not-a-number.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"x4122\t0\tcom.example.running"`)
			require.Empty(t, services)
		})

		t.Run("a PID of 0 gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			fakeLaunchctlList(t, "launchctl-list-pid-zero.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"0\t0\tcom.example.running"`)
			require.Empty(t, services)
		})

		t.Run("a PID that is too large for an int gives an error that names the row, and no services", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			fakeLaunchctlList(t, "launchctl-list-pid-too-large.txt")

			services, err := launchd.NewDomain(home, 501).Services()

			require.ErrorContains(t, err, `"99999999999999999999\t0\tcom.example.running"`)
			require.Empty(t, services)
		})
	})

	t.Run("list again", func(t *testing.T) {
		modTime := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

		t.Run("a plist that is added after a read gives its service at the next read", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, map[string]string{"com.example.xml.plist": "xml.plist"})
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			copyFixtureModifiedAt(t, filepath.Join(home, "Library", "LaunchAgents", "com.example.binary.plist"), "binary.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.ElementsMatch(t, []service.Service{{Name: "com.example.xml"}, {Name: "com.example.binary"}}, services)
		})

		t.Run("a plist that is removed after a read gives no service at the next read", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
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
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			copyFixtureModifiedAt(t, path, "xml.plist", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			copyFixtureModifiedAt(t, path, "binary.plist", modTime.Add(time.Second))

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.binary"}}, services)
		})

		t.Run("a plist whose modification time did not change is not read again, so a new Label with the same modification time keeps the old name", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			copyFixtureModifiedAt(t, path, "xml.plist", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			copyFixtureModifiedAt(t, path, "binary.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.xml"}}, services)
		})

		t.Run("a file that is not a plist is not read again while its modification time does not change", func(t *testing.T) {
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			copyFixtureModifiedAt(t, path, "not-a-plist.txt", modTime)
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.NoError(t, err)
			copyFixtureModifiedAt(t, path, "xml.plist", modTime)

			services, err := domain.Services()

			require.NoError(t, err)
			require.Empty(t, services)
		})

		t.Run("a plist that could not be read gives its service once it can be read, with the same modification time", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a file with no read permission")
			}
			fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			home := homeWith(t, nil)
			path := filepath.Join(home, "Library", "LaunchAgents", "com.example.plist")
			copyFixtureModifiedAt(t, path, "xml.plist", modTime)
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

		t.Run("each read runs launchctl list again, so a service that starts after a read is running at the next read", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.running")
			output := fakeLaunchctlList(t, "launchctl-list-no-services.txt")
			fakePs(t, "ps-no-lines.txt")
			domain := launchd.NewDomain(home, 501)
			before, err := domain.Services()
			require.NoError(t, err)
			copyFixture(t, output, "launchctl-list-each-state.txt")

			after, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.running", State: service.NotLoaded}}, before)
			require.Equal(t, []service.Service{{Name: "com.example.running", State: service.Running, PID: 4122}}, after)
		})

		t.Run("after an output with an unknown header line, the next read whose output is valid gives the services again", func(t *testing.T) {
			home := homeWithPlists(t, "com.example.loaded")
			output := fakeLaunchctlList(t, "launchctl-list-unknown-header.txt")
			domain := launchd.NewDomain(home, 501)
			_, err := domain.Services()
			require.Error(t, err)
			copyFixture(t, output, "launchctl-list-each-state.txt")

			services, err := domain.Services()

			require.NoError(t, err)
			require.Equal(t, []service.Service{{Name: "com.example.loaded", State: service.Loaded}}, services)
		})
	})

	t.Run("text", func(t *testing.T) {
		t.Run("reads as gui/ and the uid", func(t *testing.T) {
			require.Equal(t, "gui/501", launchd.NewDomain(t.TempDir(), 501).String())
			require.Equal(t, "gui/502", launchd.NewDomain(t.TempDir(), 502).String())
		})
	})
}
