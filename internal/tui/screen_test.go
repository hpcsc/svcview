//go:build unit

package tui_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/hpcsc/svcview/internal/tui"
	"github.com/stretchr/testify/require"
)

var (
	updateGolden   = flag.Bool("update", false, "write the golden files from the screen")
	refreshEvery2s = config.Settings{Refresh: 2 * time.Second}
)

func servicesNamed(names ...string) func() ([]service.Service, error) {
	return func() ([]service.Service, error) {
		var services []service.Service
		for _, name := range names {
			services = append(services, service.Service{Name: name})
		}
		return services, nil
	}
}

func readGiving(services ...service.Service) func() ([]service.Service, error) {
	return func() ([]service.Service, error) {
		return slices.Clone(services), nil
	}
}

func open(t *testing.T, domain string, read func() ([]service.Service, error), width, height int) tea.Model {
	t.Helper()
	return openIn(t, domain, refreshEvery2s, read, width, height)
}

func openWith(t *testing.T, settings config.Settings, read func() ([]service.Service, error), width, height int) tea.Model {
	t.Helper()
	return openIn(t, "gui/501", settings, read, width, height)
}

func openIn(t *testing.T, domain string, settings config.Settings, read func() ([]service.Service, error), width, height int) tea.Model {
	t.Helper()
	var screen tea.Model = tui.NewScreen(domain, read, nil, settings)
	start := screen.Init()
	require.NotNil(t, start)
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: width, Height: height})
	screen, _ = screen.Update(start())
	return screen
}

func groups(t *testing.T, nameAndMatch ...string) config.Settings {
	t.Helper()
	settings := config.Settings{Refresh: 2 * time.Second}
	for i := 0; i < len(nameAndMatch); i += 2 {
		group, err := config.NewGroup(nameAndMatch[i], nameAndMatch[i+1])
		require.NoError(t, err)
		settings.Groups = append(settings.Groups, group)
	}
	return settings
}

func requireGolden(t *testing.T, name string, screen tea.Model) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	content := screen.View().Content
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), content)
}

func press(screen tea.Model, key tea.KeyPressMsg) tea.Cmd {
	_, cmd := screen.Update(key)
	return cmd
}

var (
	up       = tea.KeyPressMsg{Code: tea.KeyUp}
	down     = tea.KeyPressMsg{Code: tea.KeyDown}
	question = tea.KeyPressMsg{Code: '?', Text: "?"}
	escape   = tea.KeyPressMsg{Code: tea.KeyEscape}
	tab      = tea.KeyPressMsg{Code: tea.KeyTab}
)

var (
	blueBorder = "\x1b[34m"
	dimBorder  = "\x1b[2m"
	styleAtEnd = regexp.MustCompile(`\x1b\[[0-9;]*m$`)
)

func borderStyles(screen tea.Model) []string {
	corners := strings.Split(strings.Split(screen.View().Content, "\n")[3], "╭")
	var styles []string
	for _, beforeCorner := range corners[:len(corners)-1] {
		styles = append(styles, styleAtEnd.FindString(beforeCorner))
	}
	return styles
}

func pressKeys(screen tea.Model, keys ...tea.KeyPressMsg) tea.Model {
	for _, key := range keys {
		screen, _ = screen.Update(key)
	}
	return screen
}

func listRows(screen tea.Model) []string {
	var rows []string
	for _, line := range strings.Split(ansi.Strip(screen.View().Content), "\n")[4:] {
		if strings.HasPrefix(line, "╰") {
			break
		}
		text, _, _ := strings.Cut(strings.TrimPrefix(line, "│"), "│")
		rows = append(rows, strings.TrimSpace(text))
	}
	return rows
}

func selectedName(screen tea.Model) string {
	for line := range strings.SplitSeq(ansi.Strip(screen.View().Content), "\n") {
		_, row, found := strings.Cut(line, "▌")
		if !found {
			continue
		}
		row, _, _ = strings.Cut(row, "│")
		if fields := strings.Fields(row); len(fields) == 2 {
			return fields[1]
		}
	}
	return ""
}

func runWithInput(t *testing.T, screen tui.Screen, input string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	program := tea.NewProgram(screen,
		tea.WithContext(ctx),
		tea.WithInput(strings.NewReader(input)),
		tea.WithOutput(io.Discard),
		tea.WithWindowSize(80, 12))
	_, err := program.Run()
	return err
}

func changingAt(at time.Duration, before, after func() ([]service.Service, error)) func() ([]service.Service, error) {
	start := time.Now()
	return func() ([]service.Service, error) {
		if time.Since(start) < at {
			return before()
		}
		return after()
	}
}

func readsInTurn(reads ...func() ([]service.Service, error)) func() ([]service.Service, error) {
	next := 0
	return func() ([]service.Service, error) {
		read := reads[min(next, len(reads)-1)]
		next++
		return read()
	}
}

func openWithTick(t *testing.T, read func() ([]service.Service, error)) (tea.Model, tea.Cmd) {
	t.Helper()
	var screen tea.Model = tui.NewScreen("gui/501", read, nil, refreshEvery2s)
	start := screen.Init()
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return screen.Update(start())
}

// readAgain must run in a synctest bubble: the tick waits 2 s on the clock of the bubble.
func readAgain(screen tea.Model, tick tea.Cmd) (tea.Model, tea.Cmd) {
	screen, read := screen.Update(tick())
	return screen.Update(read())
}

// runFor must run in a synctest bubble: it waits a minute on the clock of the bubble.
func runFor(t *testing.T, screen tui.Screen, duration time.Duration, keys ...tea.KeyPressMsg) tea.Model {
	t.Helper()
	return runSized(t, screen, 80, 12, duration, keys...)
}

// runSized must run in a synctest bubble: it waits a minute on the clock of the bubble.
func runSized(t *testing.T, screen tui.Screen, width, height int, duration time.Duration, keys ...tea.KeyPressMsg) tea.Model {
	t.Helper()
	program := tea.NewProgram(screen,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithWindowSize(width, height))
	go func() {
		time.Sleep(10 * time.Millisecond)
		for _, key := range keys {
			program.Send(key)
		}
		time.Sleep(duration)
		program.Quit()
	}()
	final, err := program.Run()
	require.NoError(t, err)
	time.Sleep(time.Minute)
	synctest.Wait()
	return final
}

func statusCommand(t *testing.T) *status.Command {
	t.Helper()
	command, err := status.NewCommand([]string{"cat", "{{.Env.WORKER_STATUS_FILE}}"}, 0, 0)
	require.NoError(t, err)
	return &command
}

func workersWithStatus(t *testing.T, otherNameAndMatch ...string) config.Settings {
	t.Helper()
	settings := groups(t, append([]string{"Workers", `^com\.example\.worker\.`}, otherNameAndMatch...)...)
	settings.Groups[0].Status = statusCommand(t)
	return settings
}

func statusEvery(t *testing.T, every time.Duration) *status.Command {
	t.Helper()
	command, err := status.NewCommand([]string{"cat", "{{.Env.WORKER_STATUS_FILE}}"}, every, 0)
	require.NoError(t, err)
	return &command
}

type runStarts struct {
	lock   sync.Mutex
	start  time.Time
	byName map[string][]time.Duration
}

func newRunStarts() *runStarts {
	return &runStarts{start: time.Now(), byName: map[string][]time.Duration{}}
}

func (r *runStarts) add(name string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.byName[name] = append(r.byName[name], time.Since(r.start))
}

func (r *runStarts) get() map[string][]time.Duration {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.byName
}

func runsGiving(results map[string]status.Result) func(status.Command, service.Service) status.Result {
	return func(_ status.Command, svc service.Service) status.Result {
		result := results[svc.Name]
		if result.Ended.IsZero() {
			result.Ended = time.Now()
		}
		return result
	}
}

func detailPane(screen tea.Model) []string {
	var detail []string
	for _, line := range strings.Split(ansi.Strip(screen.View().Content), "\n")[4:] {
		if strings.HasPrefix(line, "╰") {
			break
		}
		_, text, _ := strings.Cut(line, "││ ")
		detail = append(detail, strings.TrimRight(strings.TrimSuffix(text, "│"), " "))
	}
	return detail
}

func eventsNumbered(count int) []status.Event {
	events := make([]status.Event, 0, count)
	for i := range count {
		events = append(events, status.Event{At: time.Date(2026, 11, 23, 9, i, 0, 0, time.Local), Text: fmt.Sprintf("event %02d", i+1)})
	}
	return events
}

func summary(text string, level status.Level) status.Result {
	return status.Result{Output: &status.Output{Summary: text, Level: level}}
}

func TestScreen(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		t.Run("reads the services with no key press and shows them", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.Contains(t, screen.View().Content, "com.example.worker")
		})

		t.Run("takes the whole terminal, so the shell gets its screen back at the end", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.True(t, screen.View().AltScreen)
		})

		t.Run("a terminal that is smaller than the panes does not stop the screen", func(t *testing.T) {
			for width := range 8 {
				for height := range 6 {
					screen := open(t, "gui/501", servicesNamed("com.example.worker"), width, height)

					require.NotPanics(t, func() { screen.View() }, "width %d, height %d", width, height)
				}
			}
		})

		t.Run("shows no rows before the size of the terminal is known", func(t *testing.T) {
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s)
			screen, _ = screen.Update(screen.Init()())

			require.Empty(t, screen.View().Content)
		})
	})

	t.Run("header", func(t *testing.T) {
		t.Run("shows svcview and the domain", func(t *testing.T) {
			screen := open(t, "gui/502", servicesNamed(), 80, 12)

			header, _, _ := strings.Cut(screen.View().Content, "\n")
			require.Contains(t, header, "svcview")
			require.Contains(t, header, "gui/502")
		})

		t.Run("counts the services in each state under the domain, each count after the glyph of its state in its color, with 0 for a state with no service", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.a-running", State: service.Running},
				service.Service{Name: "com.example.b-running", State: service.Running},
				service.Service{Name: "com.example.c-not-loaded", State: service.NotLoaded},
			), 80, 12)

			requireGolden(t, "counts", screen)
		})

		t.Run("when the read gives an error, shows no counts", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return []service.Service{{Name: "com.example.worker", State: service.Running}}, errors.New("launchctl list: exit status 3")
			}

			screen := open(t, "gui/501", failing, 80, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, "│"+strings.Repeat(" ", 78)+"│", lines[1])
		})

		t.Run("keeps its top border as wide as the terminal, and cuts svcview with … when the domain leaves too little room for it", func(t *testing.T) {
			widths := map[int]int{}
			for width := 13; width <= 30; width++ {
				header, _, _ := strings.Cut(open(t, "gui/501", servicesNamed(), width, 12).View().Content, "\n")
				widths[width] = ansi.StringWidth(header)
			}
			header, _, _ := strings.Cut(open(t, "gui/501", servicesNamed(), 19, 12).View().Content, "\n")

			for width := 13; width <= 30; width++ {
				require.Equal(t, width, widths[width], "terminal of %d columns", width)
			}
			require.Equal(t, "╭ svcvi…  gui/501 ╮", header)
		})

		t.Run("shows no counts before the first read ends", func(t *testing.T) {
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s)
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, "│"+strings.Repeat(" ", 78)+"│", lines[1])
		})
	})

	t.Run("list", func(t *testing.T) {
		t.Run("shows the group Other with one row for each service, sorted by name", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("homebrew.mxcl.redis", "com.example.worker", "com.apple.a"), 80, 12)

			requireGolden(t, "sorted", screen)
		})

		t.Run("shows the groups of the settings in their order, then the group Other, each under its group line, with an empty line between two groups, and the services of each group sorted by name", func(t *testing.T) {
			settings := groups(t, "Workers", "worker", "Databases", "redis|postgres")

			screen := openWith(t, settings, servicesNamed("homebrew.mxcl.redis", "com.example.worker.02", "com.apple.a", "homebrew.mxcl.postgres", "com.example.worker.01"), 80, 16)

			requireGolden(t, "groups", screen)
			require.Equal(t, []string{
				"Workers",
				"▌○ com.example.worker.01",
				"○ com.example.worker.02",
				"",
				"Databases",
				"○ homebrew.mxcl.postgres",
				"○ homebrew.mxcl.redis",
				"",
				"Other",
				"○ com.apple.a",
			}, listRows(screen))
		})

		t.Run("a group with no service shows no line, and with each service in a group of the settings, shows no line Other", func(t *testing.T) {
			settings := groups(t, "Workers", "worker", "Empty", "nothing", "Databases", "redis")

			screen := openWith(t, settings, servicesNamed("homebrew.mxcl.redis", "com.example.worker.01"), 80, 14)

			require.Equal(t, []string{
				"Workers",
				"▌○ com.example.worker.01",
				"",
				"Databases",
				"○ homebrew.mxcl.redis",
				"",
				"",
				"",
			}, listRows(screen))
		})

		t.Run("shows each control character in the name of a group as �", func(t *testing.T) {
			settings := groups(t, "Work\x1b[2Jers", "worker")

			screen := openWith(t, settings, servicesNamed("com.example.worker.01"), 80, 12)

			require.Equal(t, "Work�[2Jers", listRows(screen)[0])
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("a row in a group shows the short name of its service, and a row in Other shows the full name", func(t *testing.T) {
			settings := groups(t, "Workers", `^com\.example\.worker\.`)

			screen := openWith(t, settings, servicesNamed("com.example.worker.02", "com.apple.a", "com.example.worker.01"), 80, 12)

			requireGolden(t, "short-names", screen)
			require.Equal(t, []string{
				"Workers",
				"▌○ 01",
				"○ 02",
				"",
				"Other",
				"○ com.apple.a",
			}, listRows(screen))
		})

		t.Run("cuts a short name that is longer than the list pane and ends it with …, and shows each control character in a short name as �", func(t *testing.T) {
			settings := groups(t, "Examples", `^com\.example\.`)

			screen := openWith(t, settings, servicesNamed("com.example.a-very-long-name-for-the-list", "com.example.b\x1b[2Jworker"), 60, 12)

			require.Equal(t, []string{"Examples", "▌○ a-very-long-name…", "○ b�[2Jworker"}, listRows(screen)[:3])
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("takes 40 percent of the width, rounded down, and the detail pane beside it takes the rest", func(t *testing.T) {
			widths := map[int][2]int{}
			for _, width := range []int{100, 99} {
				lines := strings.Split(open(t, "gui/501", servicesNamed(), width, 12).View().Content, "\n")
				list, detail, _ := strings.Cut(lines[3], "╮")
				widths[width] = [2]int{ansi.StringWidth(list + "╮"), ansi.StringWidth(detail)}
			}

			require.Equal(t, map[int][2]int{100: {40, 60}, 99: {39, 60}}, widths)
		})

		t.Run("keeps the top border as wide as the pane when the title fills it", func(t *testing.T) {
			lines := strings.Split(open(t, "gui/501", servicesNamed("com.example.worker"), 30, 7).View().Content, "\n")

			var widths []int
			for _, line := range lines[3 : len(lines)-1] {
				widths = append(widths, ansi.StringWidth(line))
			}
			require.Equal(t, []int{30, 30, 30}, widths)
		})

		t.Run("shows each control character in a name as �, so the name cannot add a line or send a sequence to the terminal", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.\x1b[2Jworker\nnext"), 100, 12)

			require.Contains(t, screen.View().Content, "com.example.�[2Jworker�next")
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("shows each control character in a name of a row that is not selected as �", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b\x1b[2Jworker\nnext"), 100, 12)

			require.Contains(t, ansi.Strip(screen.View().Content), "  ○ com.example.b�[2Jworker�next")
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("starts each row with the glyph of its state: a green ● for running, a yellow ◐ for loaded, and a gray ○ for not loaded", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.a-running", State: service.Running},
				service.Service{Name: "com.example.b-loaded", State: service.Loaded},
				service.Service{Name: "com.example.c-not-loaded", State: service.NotLoaded},
			), 80, 12)

			requireGolden(t, "each-state", screen)
		})

		t.Run("draws the glyph red when the last exit is other than 0, whatever the state, and keeps the color of the state in the header", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.a-killed", State: service.Running, LastExit: service.LastExit{Signal: "SIGKILL"}},
				service.Service{Name: "com.example.b-exit-78", State: service.Loaded, LastExit: service.LastExit{Code: 78}},
				service.Service{Name: "com.example.c-exit-1", State: service.NotLoaded, LastExit: service.LastExit{Code: 1}},
				service.Service{Name: "com.example.d-running", State: service.Running},
				service.Service{Name: "com.example.e-loaded", State: service.Loaded},
			), 80, 12)

			requireGolden(t, "last-exit", screen)
		})

		t.Run("cuts a name that is longer than the list pane and ends it with …, and leaves a name that fits with its glyph", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.z-name-that-is-too-long", "com.example.exact", "com.example.fits"), 60, 12)

			requireGolden(t, "long-name", screen)
		})

		t.Run("with no services, shows no rows", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed(), 80, 12)

			requireGolden(t, "no-services", screen)
		})

		t.Run("when the read gives an error, shows the error and no rows", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return []service.Service{{Name: "com.example.worker"}}, errors.New("open /Users/someone/Library/LaunchAgents: permission denied")
			}

			screen := open(t, "gui/501", failing, 80, 12)

			requireGolden(t, "read-error", screen)
		})

		t.Run("shows each line of an error on a row of its own, and each control character in a line as �", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return nil, errors.New("launchctl list: exit status 3: Bootstrap failed\nprinted \x1b[2J on stdout")
			}

			screen := open(t, "gui/501", failing, 200, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Contains(t, lines[4], "│ launchctl list: exit status 3: Bootstrap failed ")
			require.Contains(t, lines[5], "│ printed �[2J on stdout ")
		})
	})

	t.Run("refresh", func(t *testing.T) {
		t.Run("shows a service that is added, no later than 2 s after the change, with no key press", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("does not read again before 2 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 1900*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("removes the row of a service that is removed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker", "com.example.removed"), servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.removed")
			})
		})

		t.Run("shows the glyph of the new state of a service at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● com.example.worker")
			})
		})

		t.Run("changes the counts at the read that changes a state", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● 1 running   ◐ 0 loaded   ○ 0 not loaded")
			})
		})

		t.Run("shows the new PID, uptime and last exit of the selected service in the detail pane at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: new(time.Hour)}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4123, Uptime: new(5 * time.Second), LastExit: service.LastExit{Signal: "SIGKILL"}}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				content := ansi.Strip(screen.View().Content)
				require.Contains(t, content, "● running     PID 4123")
				require.Contains(t, content, "up 5s         last exit SIGKILL")
			})
		})

		t.Run("after a read that gives an error, the next read that succeeds shows the rows again", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				failing := func() ([]service.Service, error) {
					return nil, errors.New("open /Users/someone/Library/LaunchAgents: permission denied")
				}
				read := changingAt(100*time.Millisecond, failing, servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "permission denied")
			})
		})

		t.Run("with a refresh of 5 s, shows a service that is added, no later than 5 s after the read before it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, config.Settings{Refresh: 5 * time.Second}), 5100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("with a refresh of 5 s, does not read again before 5 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, config.Settings{Refresh: 5 * time.Second}), 4900*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("does not start a read while the previous read runs", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var running, mostAtOnce, reads atomic.Int32
				slow := func() ([]service.Service, error) {
					reads.Add(1)
					now := running.Add(1)
					if now > mostAtOnce.Load() {
						mostAtOnce.Store(now)
					}
					time.Sleep(3 * time.Second)
					running.Add(-1)
					return []service.Service{{Name: "com.example.worker"}}, nil
				}

				runFor(t, tui.NewScreen("gui/501", slow, nil, refreshEvery2s), 20*time.Second)

				require.Equal(t, int32(1), mostAtOnce.Load())
				require.GreaterOrEqual(t, reads.Load(), int32(3))
			})
		})
	})

	t.Run("status", func(t *testing.T) {
		t.Run("after the first read, each service of a group with a status command gets one run with its service, whatever its state, and a service of a group with no status command gets none", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				services := []service.Service{
					{Name: "com.example.worker.01", State: service.Running, PID: 4122, Env: map[string]string{"WORKER_STATUS_FILE": "/tmp/01.json"}},
					{Name: "com.example.worker.02", State: service.Loaded, Env: map[string]string{"WORKER_STATUS_FILE": "/tmp/02.json"}},
					{Name: "com.example.worker.03", State: service.NotLoaded},
					{Name: "homebrew.mxcl.redis", State: service.Running, PID: 4123},
					{Name: "com.apple.a", State: service.Running, PID: 4124},
				}
				var lock sync.Mutex
				var ran []service.Service
				run := func(_ status.Command, svc service.Service) status.Result {
					lock.Lock()
					defer lock.Unlock()
					ran = append(ran, svc)
					return status.Result{}
				}

				runFor(t, tui.NewScreen("gui/501", readGiving(services...), run, workersWithStatus(t, "Databases", "redis")), 100*time.Millisecond)

				require.ElementsMatch(t, services[:3], ran)
			})
		})

		t.Run("with an every of 2 s, each service gets a run every 2 s, also when it is not loaded, and the row shows a new summary no later than 2 s after the output changes", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				starts := newRunStarts()
				read := readGiving(
					service.Service{Name: "com.example.worker.a", State: service.Running, PID: 4122},
					service.Service{Name: "com.example.worker.b", State: service.NotLoaded},
				)
				run := func(_ status.Command, svc service.Service) status.Result {
					starts.add(svc.Name)
					if time.Since(starts.start) < 3*time.Second {
						return summary("idle", status.LevelInfo)
					}
					return summary("busy", status.LevelInfo)
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 5*time.Second)

				require.Equal(t, []string{"Workers", "▌● a  busy", "○ b  busy"}, listRows(screen)[:3])
				require.Equal(t, map[string][]time.Duration{
					"com.example.worker.a": {0, 2 * time.Second, 4 * time.Second},
					"com.example.worker.b": {0, 2 * time.Second, 4 * time.Second},
				}, starts.get())
			})
		})

		t.Run("two groups with different every run their commands each at its own every", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				starts := newRunStarts()
				settings := groups(t, "Workers", `^com\.example\.worker\.`, "Databases", "redis")
				settings.Groups[0].Status = statusEvery(t, 2*time.Second)
				settings.Groups[1].Status = statusEvery(t, 3*time.Second)
				var lock sync.Mutex
				everyOf := map[string]time.Duration{}
				run := func(command status.Command, svc service.Service) status.Result {
					starts.add(svc.Name)
					lock.Lock()
					defer lock.Unlock()
					everyOf[svc.Name] = command.Every
					return status.Result{}
				}

				runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "homebrew.mxcl.redis"), run, settings), 6500*time.Millisecond)

				require.Equal(t, map[string][]time.Duration{
					"com.example.worker.a": {0, 2 * time.Second, 4 * time.Second, 6 * time.Second},
					"homebrew.mxcl.redis":  {0, 3 * time.Second, 6 * time.Second},
				}, starts.get())
				require.Equal(t, map[string]time.Duration{"com.example.worker.a": 2 * time.Second, "homebrew.mxcl.redis": 3 * time.Second}, everyOf)
			})
		})

		t.Run("a run that lasts longer than every gets no second run of the same service while it runs, and the next run starts at the first every after it ends", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				starts := newRunStarts()
				var running, mostAtOnce atomic.Int32
				run := func(_ status.Command, svc service.Service) status.Result {
					starts.add(svc.Name)
					now := running.Add(1)
					if now > mostAtOnce.Load() {
						mostAtOnce.Store(now)
					}
					time.Sleep(3 * time.Second)
					running.Add(-1)
					return status.Result{}
				}

				runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, workersWithStatus(t)), 9*time.Second)

				require.Equal(t, map[string][]time.Duration{"com.example.worker.a": {0, 4 * time.Second, 8 * time.Second}}, starts.get())
				require.Equal(t, int32(1), mostAtOnce.Load())
			})
		})

		t.Run("a service that a read adds gets runs at the next every of its group, and a service that a read removes gets no more runs", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				starts := newRunStarts()
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, 3*time.Second)
				read := changingAt(3500*time.Millisecond, servicesNamed("com.example.worker.a", "com.example.worker.b"), servicesNamed("com.example.worker.a", "com.example.worker.c"))
				run := func(_ status.Command, svc service.Service) status.Result {
					starts.add(svc.Name)
					return status.Result{}
				}

				runFor(t, tui.NewScreen("gui/501", read, run, settings), 7*time.Second)

				require.Equal(t, map[string][]time.Duration{
					"com.example.worker.a": {0, 3 * time.Second, 6 * time.Second},
					"com.example.worker.b": {0, 3 * time.Second},
					"com.example.worker.c": {6 * time.Second},
				}, starts.get())
			})
		})

		t.Run("the row keeps the summary and the detail pane keeps the line Status after a read of the services", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, 3*time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summary("busy", status.LevelInfo)})

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, settings), 2500*time.Millisecond)

				require.Equal(t, []string{"Workers", "▌○ a  busy"}, listRows(screen)[:2])
				require.Equal(t, "Status 1m2s ago", detailPane(screen)[2])
			})
		})

		t.Run("a read that gives an error starts no runs, and the next read that succeeds gets runs at the next every", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				starts := newRunStarts()
				failing := func() ([]service.Service, error) {
					return nil, errors.New("launchctl list: exit status 3")
				}
				read := changingAt(time.Second, failing, servicesNamed("com.example.worker.a"))
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, 3*time.Second)
				run := func(_ status.Command, svc service.Service) status.Result {
					starts.add(svc.Name)
					return status.Result{}
				}

				runFor(t, tui.NewScreen("gui/501", read, run, settings), 4500*time.Millisecond)

				require.Equal(t, map[string][]time.Duration{"com.example.worker.a": {3 * time.Second}}, starts.get())
			})
		})

		t.Run("the row shows the summary of the last run after the short name, the summaries of one group start in one column, info in the default color, warn in yellow, error in red, and the text of a failure in red", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.bb", "com.example.worker.ccc", "com.example.worker.dddd", "com.apple.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a":    summary("busy · ENG-692", status.LevelInfo),
					"com.example.worker.bb":   summary("idle", status.LevelWarn),
					"com.example.worker.ccc":  summary("3 failed", status.LevelError),
					"com.example.worker.dddd": {Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no WORKER_STATUS_FILE"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				requireGolden(t, "status-summaries", screen)
				require.Equal(t, []string{
					"Workers",
					"▌○ a     busy · ENG-692",
					"○ bb    idle",
					"○ ccc   3 failed",
					"○ dddd  the plist has no W…",
					"",
					"Other",
					"○ com.apple.a",
				}, listRows(screen))
			})
		})

		t.Run("the selected row shows the summary on its background, in the color of its level", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.bb")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a":  summary("busy", status.LevelInfo),
					"com.example.worker.bb": summary("idle", status.LevelWarn),
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond, down)

				requireGolden(t, "status-selected", screen)
			})
		})

		t.Run("a row shows nothing after the short name before its first run ends, when the last run gave no output and no failure, when the output has no summary, and in a group with no status command", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c", "com.example.worker.d", "homebrew.mxcl.redis")
				run := func(_ status.Command, svc service.Service) status.Result {
					switch svc.Name {
					case "com.example.worker.a":
						return summary("busy", status.LevelInfo)
					case "com.example.worker.b":
						return status.Result{Output: &status.Output{Level: status.LevelError}}
					case "com.example.worker.d":
						time.Sleep(time.Minute)
					}
					return status.Result{}
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t, "Databases", "redis")), 80, 14, 100*time.Millisecond)

				require.Equal(t, []string{
					"Workers",
					"▌○ a  busy",
					"○ b",
					"○ c",
					"○ d",
					"",
					"Databases",
					"○ homebrew.mxcl.redis",
				}, listRows(screen))
			})
		})

		t.Run("the row shows status error in red in place of the summary for a status error, also on the background of the selected row", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit code 3", Lines: []string{"broken"}}},
					"com.example.worker.b": {Failure: &status.Failure{Kind: status.FailureKindError, Text: "ended by SIGTERM"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				requireGolden(t, "status-error", screen)
				require.Equal(t, []string{"Workers", "▌○ a  status error", "○ b  status error"}, listRows(screen)[:3])
			})
		})

		t.Run("the row shows status timeout in red in place of the summary for a status timeout, and the detail pane shows the command line under Status", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Failure: &status.Failure{Kind: status.FailureKindTimeout, Text: "sleep 30"}},
					"com.example.worker.b": {Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit code 3"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				requireGolden(t, "status-timeout", screen)
				require.Equal(t, []string{"Workers", "▌○ a  status timeout", "○ b  status error"}, listRows(screen)[:3])
				require.Equal(t, "sleep 30", detailPane(screen)[3])
			})
		})

		t.Run("the row shows status invalid in red in place of the summary for a status invalid, and the detail pane shows the parse error and the lines of the output under Status", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Failure: &status.Failure{Kind: status.FailureKindInvalid, Text: `"/version" is 2, and it must be 1`, Lines: []string{"{", `"version": 2`, "}"}}},
					"com.example.worker.b": {Failure: &status.Failure{Kind: status.FailureKindTimeout, Text: "sleep 30"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 13, 100*time.Millisecond)

				requireGolden(t, "status-invalid", screen)
				require.Equal(t, []string{"Workers", "▌○ a  status invalid", "○ b  status timeout"}, listRows(screen)[:3])
				require.Equal(t, []string{`"/version" is 2, and it must be 1`, "{", `"version": 2`, "}"}, detailPane(screen)[3:7])
			})
		})

		t.Run("cuts a summary that is longer than the list pane and ends it with …, and shows each control character in a summary or a failure as �", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": summary("busy\x1b[2J\nnext · a summary that is longer than the pane", status.LevelWarn),
					"com.example.worker.b": {Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "no\x1b[2Jfile"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"Workers", "▌○ a  busy�[2J�next · a sum…", "○ b  no�[2Jfile"}, listRows(screen)[:3])
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})
	})

	t.Run("detail pane", func(t *testing.T) {
		t.Run("shows the state with its glyph, the PID, the uptime and the last exit of the selected service, under its name", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: new(3*time.Hour + 12*time.Minute + 7*time.Second)},
			), 80, 12)

			requireGolden(t, "detail", screen)
		})

		t.Run("the title is the full name of the selected service, also when the list cuts the name with …", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a-name-that-is-too-long-for-the-list"), 100, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")

			require.Contains(t, lines[3], "╭ com.example.a-name-that-is-too-long-for-the-list ─")
			require.Contains(t, lines[5], "▌○ com.example.a-name-that-is-too-l… │")
		})

		t.Run("the title is the full name of the selected service, also when its row shows the short name", func(t *testing.T) {
			settings := groups(t, "Workers", `^com\.example\.worker\.`)

			screen := openWith(t, settings, servicesNamed("com.example.worker.01", "com.example.worker.02"), 80, 12)
			screen = pressKeys(screen, down)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Contains(t, lines[3], "╭ com.example.worker.02 ─")
			require.Equal(t, "▌○ 02", listRows(screen)[2])
		})

		t.Run("cuts a title that is longer than the pane and ends it with …", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a-name-that-is-too-long-for-the-list"), 60, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")

			require.Equal(t, "╭ Services ────────────╮╭ com.example.a-name-that-is-too-… ╮", lines[3])
		})

		t.Run("shows the service that ↓ selects", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.a", State: service.Running, PID: 4122},
				service.Service{Name: "com.example.b", State: service.Running, PID: 4123},
			), 80, 12)

			screen = pressKeys(screen, down)

			content := ansi.Strip(screen.View().Content)
			require.Contains(t, content, "╭ com.example.b ─")
			require.Contains(t, content, "PID 4123")
			require.NotContains(t, content, "PID 4122")
		})

		t.Run("shows the uptime in its two largest units, and an uptime of 0 as 0s", func(t *testing.T) {
			for uptime, text := range map[time.Duration]string{
				0:                               "up 0s ",
				7 * time.Second:                 "up 7s ",
				59 * time.Second:                "up 59s ",
				time.Minute:                     "up 1m0s ",
				3*time.Minute + 12*time.Second:  "up 3m12s ",
				59*time.Minute + 59*time.Second: "up 59m59s ",
				time.Hour:                       "up 1h0m ",
				3*time.Hour + 12*time.Minute + 7*time.Second:   "up 3h12m ",
				23*time.Hour + 59*time.Minute + 59*time.Second: "up 23h59m ",
				24 * time.Hour: "up 1d0h ",
				4*24*time.Hour + 9*time.Hour + 16*time.Minute:    "up 4d9h ",
				400*24*time.Hour + 23*time.Hour + 59*time.Minute: "up 400d23h ",
			} {
				screen := open(t, "gui/501", readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: &uptime}), 100, 12)

				require.Contains(t, ansi.Strip(screen.View().Content), "│ "+text, uptime)
			}
		})

		t.Run("a service with no PID and no uptime shows - for each", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}), 80, 12)

			content := ansi.Strip(screen.View().Content)
			require.Contains(t, content, "◐ loaded      PID -")
			require.Contains(t, content, "up -          last exit 0")
		})

		t.Run("shows the name of the signal that ended the last run, not a number", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, LastExit: service.LastExit{Signal: "SIGKILL"}},
			), 80, 12)

			require.Contains(t, ansi.Strip(screen.View().Content), "last exit SIGKILL")
		})

		t.Run("draws a last exit other than 0 red, and keeps the glyph in the list red", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.worker", State: service.Loaded, LastExit: service.LastExit{Code: 78}},
			), 80, 12)

			requireGolden(t, "detail-last-exit", screen)
		})

		t.Run("shows the age of at under the data from launchd, in the units of the uptime", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Ended: start, Output: &status.Output{At: new(start.Add(-3 * time.Minute)), Summary: "busy"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				time.Sleep(time.Until(start.Add(2 * time.Hour)))

				requireGolden(t, "status-detail", screen)
				require.Equal(t, []string{"○ not loaded  PID -", "up -          last exit 0", "Status 2h3m ago", "", "", ""}, detailPane(screen))
			})
		})

		t.Run("shows the age of the last run when the output has no at, when the run gave no output, and when it gave a failure", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Ended: start.Add(-12 * time.Second), Output: &status.Output{Summary: "busy"}},
					"com.example.worker.b": {Ended: start.Add(-13 * time.Second)},
					"com.example.worker.c": {Ended: start.Add(-14 * time.Second), Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no WORKER_STATUS_FILE"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				time.Sleep(time.Until(start.Add(3 * time.Minute)))

				var ages []string
				for _, selected := range []tea.Model{screen, pressKeys(screen, down), pressKeys(screen, down, down)} {
					ages = append(ages, detailPane(selected)[2])
				}
				require.Equal(t, []string{"Status 3m12s ago", "Status 3m13s ago", "Status 3m14s ago"}, ages)
			})
		})

		t.Run("shows the age in the units of the uptime, and an at after the current time as 0s", func(t *testing.T) {
			for at, text := range map[time.Duration]string{
				-time.Second:                      "Status 1s ago",
				-(3*time.Minute + 12*time.Second): "Status 3m12s ago",
				-(3*time.Hour + 12*time.Minute):   "Status 3h12m ago",
				-(26 * time.Hour):                 "Status 1d2h ago",
				0:                                 "Status 0s ago",
				10 * time.Hour:                    "Status 0s ago",
			} {
				synctest.Test(t, func(t *testing.T) {
					start := time.Now()
					read := servicesNamed("com.example.worker.a")
					run := runsGiving(map[string]status.Result{
						"com.example.worker.a": {Ended: start, Output: &status.Output{At: new(start.Add(time.Minute + at))}},
					})

					screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
					time.Sleep(time.Until(start.Add(time.Minute)))

					require.Equal(t, text, detailPane(screen)[2], at)
				})
			}
		})

		t.Run("the age grows as the screen draws again, with no new run", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				var runs atomic.Int32
				run := func(status.Command, service.Service) status.Result {
					runs.Add(1)
					return status.Result{Ended: start}
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				time.Sleep(time.Until(start.Add(5 * time.Minute)))
				before := detailPane(screen)[2]
				time.Sleep(7 * time.Second)

				require.Equal(t, []string{"Status 5m0s ago", "Status 5m7s ago"}, []string{before, detailPane(screen)[2]})
				require.Equal(t, int32(1), runs.Load())
			})
		})

		t.Run("under the line Status, shows the text of a failure and then each of its lines, with each control character as �", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit\x1b[2J code 3", Lines: []string{"first", "", "third\x1b[2J line"}}},
					"com.example.worker.b": {Failure: &status.Failure{Kind: status.FailureKindTemplateValue, Text: "the plist has no WORKER_STATUS_FILE"}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				require.Equal(t, [][]string{
					{"exit�[2J code 3", "first", "", "third�[2J line", ""},
					{"the plist has no WORKER_STATUS_FILE", "", "", "", ""},
				}, [][]string{detailPane(screen)[3:], detailPane(pressKeys(screen, down))[3:]})
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})

		t.Run("under the line Status, shows each field as its name and its value, in the order of the output, with the values of one output in one column two spaces after its widest name", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "com.example.worker.b")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "mode", Value: "busy"},
						{Name: "ticket", Value: "ENG-692"},
						{Name: "done", Value: "14"},
						{Name: "failed", Value: "1"},
					}}},
					"com.example.worker.b": {Output: &status.Output{Fields: []status.Field{
						{Name: "id", Value: "7"},
						{Name: "queue", Value: "high"},
						{Name: "a-long-name", Value: "x"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				require.Equal(t, [][]string{
					{"mode    busy", "ticket  ENG-692", "done    14", "failed  1", ""},
					{"id           7", "queue        high", "a-long-name  x", "", ""},
				}, [][]string{detailPane(screen)[3:], detailPane(pressKeys(screen, down))[3:]})
			})
		})

		t.Run("shows a field whose name svcview does not know, and a field named Status, Events, summary or progress, as each other field", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "Status", Value: "s"},
						{Name: "Events", Value: "e"},
						{Name: "summary", Value: "busy"},
						{Name: "progress", Value: "0.5"},
						{Name: "zz-unknown", Value: "u"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				require.Equal(t, []string{
					"Status      s",
					"Events      e",
					"summary     busy",
					"progress    0.5",
					"zz-unknown  u",
				}, detailPane(screen)[3:])
			})
		})

		t.Run("a field with an empty value shows its name and nothing after it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "failed", Value: ""},
						{Name: "mode", Value: "busy"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"failed", "mode    busy", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("an output whose fields each have an empty name shows each value two spaces from the left of the text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{{Name: "", Value: "busy"}, {Name: "", Value: "ENG-692"}}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"  busy", "  ENG-692", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("shows each control character in the name or the value of a field as �, and puts the values in the column after the widest name as it shows", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "mo\x1b[2Jde", Value: "busy\x1b[2J\nnext"},
						{Name: "ticket", Value: "ENG-692"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"mo�[2Jde  busy�[2J�next", "ticket    ENG-692", ""}, detailPane(screen)[3:])
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})

		t.Run("an output with no fields shows no line under Status", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Ended: start, Output: &status.Output{Summary: "busy"}}})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				time.Sleep(time.Until(start.Add(5 * time.Minute)))

				require.Equal(t, []string{"Status 5m0s ago", "", "", ""}, detailPane(screen)[2:])
			})
		})

		t.Run("shows the fields of the last run, and the fields of the next run when its output has other fields", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				run := func(status.Command, service.Service) status.Result {
					if time.Since(start) < time.Second {
						return status.Result{Ended: time.Now(), Output: &status.Output{Fields: []status.Field{{Name: "mode", Value: "idle"}}}}
					}
					return status.Result{Ended: time.Now(), Output: &status.Output{Fields: []status.Field{{Name: "ticket", Value: "ENG-692"}, {Name: "step", Value: "review"}}}}
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 2500*time.Millisecond)

				require.Equal(t, []string{"ticket  ENG-692", "step    review", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("under the value of a field with a progress, shows a bar of 10 cells on a line of its own, in the column of the values", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "ticket", Value: "ENG-692"},
						{Name: "step", Value: "review · 12m / 20m", Progress: new(0.6)},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"ticket  ENG-692", "step    review · 12m / 20m", "        ▰▰▰▰▰▰▱▱▱▱"}, detailPane(screen)[3:])
			})
		})

		t.Run("a field with no progress has no line for a bar, and the field after it follows its value", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{
						{Name: "mode", Value: "busy"},
						{Name: "done", Value: "14", Progress: new(1.0)},
						{Name: "failed", Value: "1"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 13, 100*time.Millisecond)

				require.Equal(t, []string{"mode    busy", "done    14", "        ▰▰▰▰▰▰▰▰▰▰", "failed  1"}, detailPane(screen)[3:])
			})
		})

		t.Run("the bar shows the progress times 10, rounded down, as ▰, and the rest of its 10 cells as ▱, so only a progress of 1 fills it", func(t *testing.T) {
			bars := map[float64]string{
				0.65:     "▰▰▰▰▰▰▱▱▱▱",
				0.99:     "▰▰▰▰▰▰▰▰▰▱",
				0.999999: "▰▰▰▰▰▰▰▰▰▱",
				0.05:     "▱▱▱▱▱▱▱▱▱▱",
			}
			for tenths := range 11 {
				bars[float64(tenths)/10] = strings.Repeat("▰", tenths) + strings.Repeat("▱", 10-tenths)
			}
			for progress, bar := range bars {
				synctest.Test(t, func(t *testing.T) {
					read := servicesNamed("com.example.worker.a")
					run := runsGiving(map[string]status.Result{
						"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{{Name: "step", Value: "review", Progress: &progress}}}},
					})

					screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

					require.Equal(t, "      "+bar, detailPane(screen)[4], progress)
				})
			}
		})

		t.Run("draws each ▰ of a bar in the accent color, blue, and each ▱ dim, in the default color", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{{Name: "step", Value: "review", Progress: new(0.6)}}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Contains(t, screen.View().Content, "      \x1b[34m▰▰▰▰▰▰\x1b[m\x1b[2m▱▱▱▱\x1b[m")
			})
		})

		t.Run("after the fields, shows an empty line, the line Events, and each event in the order of the output, with the hours and minutes of its at in the local time zone, a ●, and its text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{
						Fields: []status.Field{{Name: "mode", Value: "busy"}},
						Events: []status.Event{
							{At: time.Date(2026, 11, 23, 9, 4, 40, 0, time.Local), Text: "step review"},
							{At: time.Date(2026, 11, 23, 9, 0, 12, 0, time.Local), Text: "claimed ENG-692"},
						},
					}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				require.Equal(t, []string{"mode  busy", "", "Events", "09:04 ● step review", "09:00 ● claimed ENG-692"}, detailPane(screen)[3:])
			})
		})

		t.Run("shows the time of an event with two digits for the hours, from 00 to 23, and two digits for the minutes, and no date, also for an event of another day", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: []status.Event{
						{At: time.Date(2026, 11, 23, 0, 5, 0, 0, time.Local), Text: "a"},
						{At: time.Date(2026, 11, 23, 7, 9, 59, 0, time.Local), Text: "b"},
						{At: time.Date(2026, 11, 23, 13, 45, 0, 0, time.Local), Text: "c"},
						{At: time.Date(2026, 11, 23, 23, 59, 59, 0, time.Local), Text: "d"},
						{At: time.Date(2020, 1, 2, 14, 30, 0, 0, time.Local), Text: "e"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 16, 100*time.Millisecond)

				require.Equal(t, []string{"", "Events", "00:05 ● a", "07:09 ● b", "13:45 ● c", "23:59 ● d", "14:30 ● e"}, detailPane(screen)[3:])
			})
		})

		t.Run("shows an at in another time zone as the hours and minutes of the same time in the local time zone", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				machineZone := time.Local
				t.Cleanup(func() { time.Local = machineZone })
				time.Local = time.FixedZone("UTC+5:30", 5*60*60+30*60)
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: []status.Event{
						{At: time.Date(2026, 11, 23, 3, 34, 0, 0, time.UTC), Text: "step review"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"", "Events", "09:04 ● step review"}, detailPane(screen)[3:])
			})
		})

		t.Run("draws the ● of an event in the default color for info, yellow for warn, and red for error", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: []status.Event{
						{At: time.Date(2026, 11, 23, 9, 0, 0, 0, time.Local), Level: status.LevelInfo, Text: "a"},
						{At: time.Date(2026, 11, 23, 9, 1, 0, 0, time.Local), Level: status.LevelWarn, Text: "b"},
						{At: time.Date(2026, 11, 23, 9, 2, 0, 0, time.Local), Level: status.LevelError, Text: "c"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

				content := screen.View().Content
				require.Equal(t, []bool{true, true, true}, []bool{
					strings.Contains(content, "09:00 ● a "),
					strings.Contains(content, "09:01 \x1b[33m●\x1b[m b "),
					strings.Contains(content, "09:02 \x1b[31m●\x1b[m c "),
				})
			})
		})

		t.Run("shows each control character in the text of an event as �", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: []status.Event{
						{At: time.Date(2026, 11, 23, 9, 4, 0, 0, time.Local), Text: "step\x1b[2J\nreview"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"", "Events", "09:04 ● step�[2J�review"}, detailPane(screen)[3:])
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})

		t.Run("an output with fields and no events shows no line Events and no empty line for it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{{Name: "mode", Value: "busy"}}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"mode  busy", "", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("after a run with fields and events, a failure of the next run shows the failure and its lines under Status, and no field and no event", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				run := func(status.Command, service.Service) status.Result {
					if time.Since(start) < time.Second {
						return status.Result{Ended: time.Now(), Output: &status.Output{
							Fields: []status.Field{{Name: "mode", Value: "busy"}},
							Events: []status.Event{{At: time.Date(2026, 11, 23, 9, 4, 0, 0, time.Local), Text: "step review"}},
						}}
					}
					return status.Result{Ended: time.Now(), Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit code 3", Lines: []string{"broken"}}}
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 14, 2500*time.Millisecond)

				require.Equal(t, []string{"exit code 3", "broken", "", "", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("draws the fields under Status, and under the value of a field with a progress, its bar of blue ▰ and dim ▱", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := readGiving(service.Service{Name: "com.example.worker.01", State: service.Running, PID: 4122, Uptime: new(3*time.Hour + 12*time.Minute)})
				run := runsGiving(map[string]status.Result{
					"com.example.worker.01": {Output: &status.Output{Summary: "busy · ENG-692 · 12m", Fields: []status.Field{
						{Name: "mode", Value: "busy"},
						{Name: "ticket", Value: "ENG-692"},
						{Name: "step", Value: "review · 12m / 20m", Progress: new(0.6)},
						{Name: "done", Value: "14"},
						{Name: "failed", Value: "1"},
					}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 16, 100*time.Millisecond)

				requireGolden(t, "status-fields", screen)
			})
		})

		t.Run("draws the fields under Status, then an empty line, the line Events, and each event with its time, its ● in the color of its level, and its text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := readGiving(service.Service{Name: "com.example.worker.01", State: service.Running, PID: 4122, Uptime: new(3*time.Hour + 12*time.Minute)})
				run := runsGiving(map[string]status.Result{
					"com.example.worker.01": {Output: &status.Output{
						Summary: "busy · ENG-692 · 12m",
						Fields: []status.Field{
							{Name: "mode", Value: "busy"},
							{Name: "ticket", Value: "ENG-692"},
							{Name: "step", Value: "review · 12m / 20m", Progress: new(0.6)},
							{Name: "done", Value: "14"},
							{Name: "failed", Value: "1"},
						},
						Events: []status.Event{
							{At: time.Date(2026, 11, 23, 9, 4, 40, 0, time.Local), Level: status.LevelInfo, Text: "step review"},
							{At: time.Date(2026, 11, 23, 9, 0, 12, 0, time.Local), Level: status.LevelWarn, Text: "claimed ENG-692"},
						},
					}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 19, 100*time.Millisecond)

				requireGolden(t, "status-fields-events", screen)
			})
		})

		t.Run("shows no Status line for a service of a group with no status command, and before the first run of the service ends", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a", "homebrew.mxcl.redis")
				run := func(status.Command, service.Service) status.Result {
					time.Sleep(30 * time.Second)
					return status.Result{Ended: time.Now()}
				}

				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t, "Databases", "redis")), 80, 14, 100*time.Millisecond)

				require.Equal(t, [][]string{
					{"○ not loaded  PID -", "up -          last exit 0", ""},
					{"○ not loaded  PID -", "up -          last exit 0", ""},
				}, [][]string{detailPane(screen)[:3], detailPane(pressKeys(screen, down))[:3]})
			})
		})

		t.Run("with no rows, shows no service", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed(), 80, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			_, detail, _ := strings.Cut(lines[3], "╮")
			require.Equal(t, "╭"+strings.Repeat("─", 46)+"╮", detail)
			require.NotContains(t, lines[4], "PID")
		})

		t.Run("when the read gives an error, shows no service", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return []service.Service{{Name: "com.example.worker", State: service.Running, PID: 4122}}, errors.New("exit status 3")
			}

			screen := open(t, "gui/501", failing, 80, 12)

			content := screen.View().Content
			require.Contains(t, content, "exit status 3")
			require.NotContains(t, content, "com.example.worker")
			require.NotContains(t, content, "PID")
		})

		t.Run("shows no service before the first read ends", func(t *testing.T) {
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s)
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			_, detail, _ := strings.Cut(lines[3], "╮")
			require.Equal(t, "╭"+strings.Repeat("─", 46)+"╮", detail)
		})
	})

	t.Run("select", func(t *testing.T) {
		t.Run("at start, the first row is selected", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.b", "com.example.a"), 80, 12)

			require.Equal(t, "com.example.a", selectedName(screen))
		})

		t.Run("at start, the first row of the first group is selected", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")

			screen := openWith(t, settings, servicesNamed("com.apple.a", "com.example.worker.01"), 80, 12)

			require.Equal(t, "com.example.worker.01", selectedName(screen))
		})

		t.Run("↓ selects the next row in the order of the list, also in the next group, and ↑ the row before it", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")
			screen := openWith(t, settings, servicesNamed("com.apple.a", "com.example.worker.02", "com.example.worker.01"), 80, 12)

			second := pressKeys(screen, down)
			third := pressKeys(second, down)
			back := pressKeys(third, up)

			require.Equal(t, []string{"com.example.worker.02", "com.apple.a", "com.example.worker.02"}, []string{selectedName(second), selectedName(third), selectedName(back)})
		})

		t.Run("↓ past the last row that the list pane shows scrolls the list over the group lines and the empty lines, so the selected row stays in view", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")
			screen := openWith(t, settings, servicesNamed("com.example.worker.1", "com.example.worker.2", "com.example.worker.3", "com.example.worker.4", "com.apple.a", "com.apple.b", "com.apple.c"), 80, 12)

			firstOfOther := pressKeys(screen, down, down, down, down)
			last := pressKeys(firstOfOther, down, down)

			require.Equal(t, []string{
				"○ com.example.worker.2",
				"○ com.example.worker.3",
				"○ com.example.worker.4",
				"",
				"Other",
				"▌○ com.apple.a",
			}, listRows(firstOfOther))
			require.Equal(t, []string{
				"○ com.example.worker.4",
				"",
				"Other",
				"○ com.apple.a",
				"○ com.apple.b",
				"▌○ com.apple.c",
			}, listRows(last))
		})

		t.Run("↑ to the first row of a group above the list pane shows the group line of the group too", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")
			screen := openWith(t, settings, servicesNamed("com.example.worker.1", "com.apple.a", "com.apple.b", "com.apple.c", "com.apple.d", "com.apple.e", "com.apple.f", "com.apple.g", "com.apple.h"), 80, 12)
			screen = pressKeys(screen, down, down, down, down, down, down, down, down)

			screen = pressKeys(screen, up, up, up, up, up, up, up)

			require.Equal(t, []string{
				"Other",
				"▌○ com.apple.a",
				"○ com.apple.b",
				"○ com.apple.c",
				"○ com.apple.d",
				"○ com.apple.e",
			}, listRows(screen))
		})

		t.Run("a list pane with room for two lines shows the group line above the first row of a group", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")
			screen := openWith(t, settings, servicesNamed("com.example.worker.1", "com.apple.a", "com.apple.b"), 80, 8)

			screen = pressKeys(screen, down, down, up)

			require.Equal(t, []string{"Other", "▌○ com.apple.a"}, listRows(screen))
		})

		t.Run("when the selected service is gone, the row that takes its place in the order of the list is selected, also in the next group", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := groups(t, "Workers", "worker")
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.worker.01", "com.example.worker.02", "com.apple.a", "homebrew.mxcl.redis"),
					servicesNamed("com.example.worker.01", "com.apple.a", "homebrew.mxcl.redis"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, settings), 2100*time.Millisecond, down)

				require.Equal(t, "com.apple.a", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row comes before it in another group", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := groups(t, "Workers", "worker")
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.worker.01", "com.apple.a", "homebrew.mxcl.redis"),
					servicesNamed("com.example.worker.01", "com.example.worker.02", "com.apple.a", "homebrew.mxcl.redis"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, settings), 2100*time.Millisecond, down)

				require.Contains(t, screen.View().Content, "com.example.worker.02")
				require.Equal(t, "com.apple.a", selectedName(screen))
			})
		})

		t.Run("↓ selects the next row, and ↑ the row before it", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)

			screen = pressKeys(screen, down, down, up)

			require.Equal(t, "com.example.b", selectedName(screen))
		})

		t.Run("↓ at the last row keeps the last row selected", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b"), 80, 12)

			screen = pressKeys(screen, down, down)

			require.Equal(t, "com.example.b", selectedName(screen))
		})

		t.Run("↑ at the first row keeps the first row selected", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b"), 80, 12)

			screen = pressKeys(screen, up)

			require.Equal(t, "com.example.a", selectedName(screen))
		})

		t.Run("the selected row starts with a blue ▌ and has a background color, and the other rows have no ▌", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.a-running", State: service.Running},
				service.Service{Name: "com.example.b-loaded", State: service.Loaded},
				service.Service{Name: "com.example.c-not-loaded", State: service.NotLoaded},
			), 80, 12)

			screen = pressKeys(screen, down)

			requireGolden(t, "selected", screen)
			require.Equal(t, 1, strings.Count(screen.View().Content, "▌"))
		})

		t.Run("cuts a selected name that is longer than the list pane and ends it with …", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a-name-that-is-too-long"), 60, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")

			require.Contains(t, lines[5], "│ ▌○ com.example.a-na… ││")
		})

		t.Run("↓ past the last row that the list pane shows scrolls the list, so the selected row stays in view", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7"), 80, 12)

			screen = pressKeys(screen, down, down, down, down, down)

			require.Equal(t, []string{
				"○ com.example.1",
				"○ com.example.2",
				"○ com.example.3",
				"○ com.example.4",
				"○ com.example.5",
				"▌○ com.example.6",
			}, listRows(screen))
		})

		t.Run("↑ keeps the list where it is while the selected row is in view, and scrolls it back when the row is above it", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7", "com.example.8", "com.example.9"), 80, 12)
			screen = pressKeys(screen, down, down, down, down, down, down, down, down)

			inView := pressKeys(screen, up, up, up, up, up)
			above := pressKeys(inView, up)
			second := pressKeys(above, up)

			require.Equal(t, []string{
				"▌○ com.example.4",
				"○ com.example.5",
				"○ com.example.6",
				"○ com.example.7",
				"○ com.example.8",
				"○ com.example.9",
			}, listRows(inView))
			require.Equal(t, []string{
				"▌○ com.example.3",
				"○ com.example.4",
				"○ com.example.5",
				"○ com.example.6",
				"○ com.example.7",
				"○ com.example.8",
			}, listRows(above))
			require.Equal(t, []string{
				"▌○ com.example.2",
				"○ com.example.3",
				"○ com.example.4",
				"○ com.example.5",
				"○ com.example.6",
				"○ com.example.7",
			}, listRows(second))
		})

		t.Run("a list pane with room for one row shows the selected row", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.1", "com.example.2", "com.example.3"), 80, 7)

			second := pressKeys(screen, down)
			first := pressKeys(second, up)

			require.Equal(t, []string{"▌○ com.example.1"}, listRows(screen))
			require.Equal(t, []string{"▌○ com.example.2"}, listRows(second))
			require.Equal(t, []string{"▌○ com.example.1"}, listRows(first))
		})

		t.Run("a smaller terminal scrolls the list, so the selected row stays in view", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7"), 80, 20)
			screen = pressKeys(screen, down, down, down, down, down, down)

			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

			require.Equal(t, []string{
				"○ com.example.2",
				"○ com.example.3",
				"○ com.example.4",
				"○ com.example.5",
				"○ com.example.6",
				"▌○ com.example.7",
			}, listRows(screen))
		})

		t.Run("a read that removes the rows at the end of a scrolled list keeps the list pane full", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7", "com.example.8", "com.example.9"),
					servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down, down, down)

				require.Equal(t, []string{
					"○ com.example.2",
					"○ com.example.3",
					"○ com.example.4",
					"○ com.example.5",
					"○ com.example.6",
					"▌○ com.example.7",
				}, listRows(screen))
			})
		})

		t.Run("the group line shows again when the first row is selected", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7"), 80, 12)

			screen = pressKeys(screen, down, down, down, down, down, down, up, up, up, up, up, up)

			require.Equal(t, []string{
				"Other",
				"▌○ com.example.1",
				"○ com.example.2",
				"○ com.example.3",
				"○ com.example.4",
				"○ com.example.5",
			}, listRows(screen))
		})

		t.Run("a read that removes rows fills the list pane again from the top", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.1", "com.example.2", "com.example.3", "com.example.4", "com.example.5", "com.example.6", "com.example.7"),
					servicesNamed("com.example.1", "com.example.2", "com.example.7"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down)

				require.Equal(t, []string{
					"Other",
					"○ com.example.1",
					"○ com.example.2",
					"▌○ com.example.7",
					"",
					"",
				}, listRows(screen))
			})
		})

		t.Run("with no rows, no row is selected, and ↑ and ↓ do nothing", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed(), 80, 12)
			before := screen.View().Content

			screen = pressKeys(screen, down, up)

			require.Equal(t, before, screen.View().Content)
			require.NotContains(t, before, "▌")
		})

		t.Run("a refresh keeps the same name selected when a row comes before it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.b", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.Contains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row before it goes", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.b", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.NotContains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})

		t.Run("when the selected service is gone, the row that takes its place is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("when the selected service was the last row and is gone, the last row is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.b"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, down, down)

				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})

		t.Run("a read that gives an error, and ↑ and ↓ while the error shows, keep the selected name for the next read that succeeds", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				failing := func() ([]service.Service, error) {
					return []service.Service{{Name: "com.example.x"}}, errors.New("exit status 3")
				}
				rows := servicesNamed("com.example.a", "com.example.b", "com.example.c")
				screen, tick := openWithTick(t, readsInTurn(rows, failing, rows))
				screen = pressKeys(screen, down)
				screen, tick = readAgain(screen, tick)
				require.Contains(t, screen.View().Content, "exit status 3")
				screen = pressKeys(screen, down)

				screen, _ = readAgain(screen, tick)

				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})
	})

	t.Run("focus", func(t *testing.T) {
		t.Run("at start, the list has the focus: the border of the list is blue, and the border of the detail pane is dim", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)

			require.Equal(t, []string{blueBorder, dimBorder}, borderStyles(screen))
		})

		t.Run("tab moves the focus to the detail pane: its lines and corners are blue, those of the list are dim, and each title keeps the default color", func(t *testing.T) {
			screen := open(t, "gui/501", readGiving(
				service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: new(3*time.Hour + 12*time.Minute)},
			), 80, 12)

			screen = pressKeys(screen, tab)

			requireGolden(t, "focus-detail", screen)
			require.Equal(t, []string{dimBorder, blueBorder}, borderStyles(screen))
		})

		t.Run("tab again moves the focus back to the list", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)

			screen = pressKeys(screen, tab, tab)

			require.Equal(t, []string{blueBorder, dimBorder}, borderStyles(screen))
		})

		t.Run("a refresh keeps the focus on the detail pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.a"), servicesNamed("com.example.a", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, refreshEvery2s), 2100*time.Millisecond, tab)

				require.Contains(t, screen.View().Content, "com.example.added")
				require.Equal(t, []string{dimBorder, blueBorder}, borderStyles(screen))
			})
		})

		t.Run("the end of a run keeps the focus on the detail pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summary("busy", status.LevelInfo)})

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, settings), 1500*time.Millisecond, tab)

				require.Equal(t, "Status 1m0s ago", detailPane(screen)[2])
				require.Equal(t, []string{dimBorder, blueBorder}, borderStyles(screen))
			})
		})

		t.Run("? and then ? or esc show and hide the help, and the focus stays on the detail pane", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)
			screen = pressKeys(screen, tab)

			require.Equal(t, [][]string{{dimBorder, blueBorder}, {dimBorder, blueBorder}}, [][]string{
				borderStyles(pressKeys(screen, question, question)),
				borderStyles(pressKeys(screen, question, escape)),
			})
		})

		t.Run("while the help shows, tab changes nothing, and the border of the help keeps the default color", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)
			help := pressKeys(screen, question)

			afterTab := pressKeys(help, tab)

			require.Equal(t, help.View().Content, afterTab.View().Content)
			require.Equal(t, []string{""}, borderStyles(afterTab))
			require.Equal(t, []string{blueBorder, dimBorder}, borderStyles(pressKeys(afterTab, escape)))
		})

		t.Run("tab moves the focus also when the list has no rows, or shows an error", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return nil, errors.New("launchctl list: exit status 3")
			}

			noRows := pressKeys(open(t, "gui/501", servicesNamed(), 80, 12), tab)
			readError := pressKeys(open(t, "gui/501", failing, 80, 12), tab)

			require.Equal(t, [][]string{{dimBorder, blueBorder}, {dimBorder, blueBorder}}, [][]string{borderStyles(noRows), borderStyles(readError)})
		})
	})

	t.Run("scroll", func(t *testing.T) {
		t.Run("with the focus on the detail pane, ↓ moves the text up one line, so the next line shows at the bottom, and ↑ moves it back down one line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				scrolled := pressKeys(screen, tab, down)
				back := pressKeys(scrolled, up)

				require.Equal(t, [][]string{
					{"up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02"},
					{"○ not loaded  PID -", "up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01"},
				}, [][]string{detailPane(scrolled), detailPane(back)})
			})
		})

		t.Run("↓ stops when the last line of the text is at the bottom of the pane, and ↑ stops at the first line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				bottom := pressKeys(screen, tab, down, down, down, down, down, down, down, down, down, down, down, down)
				top := pressKeys(bottom, up, up, up, up, up, up, up, up, up, up, up, up, down)

				require.Equal(t, [][]string{
					{"09:04 ● event 05", "09:05 ● event 06", "09:06 ● event 07", "09:07 ● event 08", "09:08 ● event 09", "09:09 ● event 10"},
					{"up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02"},
				}, [][]string{detailPane(bottom), detailPane(top)})
			})
		})

		t.Run("with no rows, or an error in the list, ↑ and ↓ with the focus on the detail pane change nothing", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return nil, errors.New("launchctl list: exit status 3")
			}
			noRows := pressKeys(open(t, "gui/501", servicesNamed(), 80, 12), tab)
			readError := pressKeys(open(t, "gui/501", failing, 80, 12), tab)

			require.Equal(t,
				[]string{noRows.View().Content, readError.View().Content},
				[]string{pressKeys(noRows, down, down).View().Content, pressKeys(readError, down, down).View().Content})
		})

		t.Run("↓ while a read error shows does not scroll the detail pane of the service that the next read shows", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				failing := func() ([]service.Service, error) {
					return nil, errors.New("exit status 3")
				}
				rows := servicesNamed("com.example.a")
				var screen tea.Model = tui.NewScreen("gui/501", readsInTurn(rows, failing, rows), nil, refreshEvery2s)
				start := screen.Init()
				screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 7})
				screen, tick := screen.Update(start())
				screen = pressKeys(screen, tab)
				screen, tick = readAgain(screen, tick)
				require.Contains(t, screen.View().Content, "exit status 3")
				screen = pressKeys(screen, down, down)

				screen, _ = readAgain(screen, tick)

				require.Equal(t, [][]string{{"○ not loaded  PID -"}, {"up -          last exit 0"}}, [][]string{detailPane(screen), detailPane(pressKeys(screen, down))})
			})
		})

		t.Run("a text that fits in the detail pane does not move", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)
			before := screen.View().Content

			screen = pressKeys(screen, tab, down, down, up, tab)

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("only the lines inside the detail pane move, and its title and its border stay", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{
					Fields: []status.Field{{Name: "mode", Value: "busy"}, {Name: "step", Value: "review", Progress: new(0.6)}},
					Events: eventsNumbered(4),
				}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				screen = pressKeys(screen, tab, down, down, down)

				requireGolden(t, "scrolled-detail", screen)
			})
		})

		t.Run("with the focus on the detail pane, ↑ and ↓ do not change the selected service, and the list does not move", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.1", "com.example.worker.2", "com.example.worker.3", "com.example.worker.4", "com.example.worker.5",
					"com.example.worker.6", "com.example.worker.7", "com.example.worker.8", "com.example.worker.9")
				run := runsGiving(map[string]status.Result{"com.example.worker.4": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, down, down, down)
				list := listRows(screen)

				screen = pressKeys(screen, tab, down, down, up, down)

				require.Equal(t, "4", selectedName(screen))
				require.Equal(t, list, listRows(screen))
				require.Equal(t, "Status 1m0s ago", detailPane(screen)[0])
			})
		})

		t.Run("with the focus on the list, ↑ and ↓ select a service, and the detail pane does not scroll", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}},
					"com.example.worker.b": {Output: &status.Output{Events: eventsNumbered(10)}},
				})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				screen = pressKeys(screen, down, down, down)

				require.Equal(t, "b", selectedName(screen))
				require.Equal(t, "○ not loaded  PID -", detailPane(screen)[0])
			})
		})

		t.Run("after a scroll, tab and ↓ select the next service, whose detail pane shows from its first line, and ↑ then shows the first service from its first line too", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := readGiving(
					service.Service{Name: "com.example.worker.a", State: service.Running, PID: 4122},
					service.Service{Name: "com.example.worker.b", State: service.Running, PID: 4123},
				)
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}},
					"com.example.worker.b": {Output: &status.Output{Events: eventsNumbered(10)}},
				})
				screen := runSized(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				next := pressKeys(screen, tab, down, down, tab, down)
				first := pressKeys(next, up)

				require.Equal(t, [][]string{{"b", "● running     PID 4123"}, {"a", "● running     PID 4122"}}, [][]string{
					{selectedName(next), detailPane(next)[0]},
					{selectedName(first), detailPane(first)[0]},
				})
			})
		})

		t.Run("a read that removes the selected service while its detail pane is scrolled selects the service that takes its place, and shows it from its first line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				a := service.Service{Name: "com.example.worker.a", State: service.Running, PID: 4122}
				b := service.Service{Name: "com.example.worker.b", State: service.Running, PID: 4123}
				c := service.Service{Name: "com.example.worker.c", State: service.Running, PID: 4124}
				read := changingAt(100*time.Millisecond, readGiving(a, b, c), readGiving(a, c))
				run := runsGiving(map[string]status.Result{
					"com.example.worker.b": {Output: &status.Output{Events: eventsNumbered(10)}},
					"com.example.worker.c": {Output: &status.Output{Events: eventsNumbered(10)}},
				})

				screen := runFor(t, tui.NewScreen("gui/501", read, run, workersWithStatus(t)), 2100*time.Millisecond, down, tab, down, down)

				require.Equal(t, []string{"c", "● running     PID 4124"}, []string{selectedName(screen), detailPane(screen)[0]})
			})
		})

		t.Run("a run that gives a shorter text that is still longer than the pane shows the last line of the new text at the bottom of the pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, time.Second)
				run := func(status.Command, service.Service) status.Result {
					if time.Since(start) < time.Second {
						return status.Result{Ended: time.Now(), Output: &status.Output{Events: eventsNumbered(10)}}
					}
					return status.Result{Ended: time.Now(), Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit code 3", Lines: []string{
						"line 1", "line 2", "line 3", "line 4", "line 5", "line 6", "line 7", "line 8",
					}}}
				}

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, settings), 1500*time.Millisecond, tab, down, down, down, down, down, down, down, down, down)

				require.Equal(t, []string{"line 3", "line 4", "line 5", "line 6", "line 7", "line 8"}, detailPane(screen))
			})
		})

		t.Run("a run that gives a text that fits in the pane shows it from its first line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, time.Second)
				run := func(status.Command, service.Service) status.Result {
					if time.Since(start) < time.Second {
						return status.Result{Ended: time.Now(), Output: &status.Output{Events: eventsNumbered(10)}}
					}
					return status.Result{Ended: time.Now(), Failure: &status.Failure{Kind: status.FailureKindError, Text: "exit code 3"}}
				}

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, settings), 1500*time.Millisecond, tab, down, down, down, down, down)

				require.Equal(t, []string{"○ not loaded  PID -", "up -          last exit 0", "Status 1m0s ago", "exit code 3", "", ""}, detailPane(screen))
			})
		})

		t.Run("a terminal that gets higher shows more lines with no empty line under the last line, and a terminal that gets lower keeps the scroll where it is", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, tab, down, down, down, down, down, down, down, down, down)

				higher, _ := screen.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
				lower, _ := higher.Update(tea.WindowSizeMsg{Width: 80, Height: 10})

				require.Equal(t, [][]string{
					{"09:02 ● event 03", "09:03 ● event 04", "09:04 ● event 05", "09:05 ● event 06", "09:06 ● event 07", "09:07 ● event 08", "09:08 ● event 09", "09:09 ● event 10"},
					{"09:02 ● event 03", "09:03 ● event 04", "09:04 ● event 05", "09:05 ● event 06"},
				}, [][]string{detailPane(higher), detailPane(lower)})
			})
		})

		t.Run("a refresh, and a run that gives a text of the same length, keep the scroll where it is, and the age in the line Status grows with no change of the scroll", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, settings), 2500*time.Millisecond, tab, down, down)
				before := detailPane(screen)
				time.Sleep(7 * time.Second)

				require.Equal(t, [][]string{
					{"Status 1m0s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02", "09:02 ● event 03"},
					{"Status 1m7s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02", "09:02 ● event 03"},
				}, [][]string{before, detailPane(screen)})
			})
		})

		t.Run("a terminal with no room for a line of the detail pane does not stop the screen after ↓", func(t *testing.T) {
			for height := range 8 {
				screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, height)

				screen = pressKeys(screen, tab, down, down, down)

				require.NotPanics(t, func() { screen.View() }, "height %d", height)
			}
		})

		t.Run("while the help shows, ↑ and ↓ do not scroll the detail pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				screen = pressKeys(screen, tab, question, down, down, escape)

				require.Equal(t, "○ not loaded  PID -", detailPane(screen)[0])
			})
		})
	})

	t.Run("keys", func(t *testing.T) {
		t.Run("q ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s), "q"))
		})

		t.Run("ctrl+c ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s), "\x03"))
		})

		t.Run("? shows each key of the screen with its action, in place of the panes", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 13)

			screen = pressKeys(screen, question)

			requireGolden(t, "keys", screen)
		})

		t.Run("? again hides the help, and the screen shows the same service selected as before", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)
			screen = pressKeys(screen, down)
			before := screen.View().Content

			screen = pressKeys(screen, question, question)

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("esc hides the help, and the screen shows the same service selected as before", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)
			screen = pressKeys(screen, down)
			before := screen.View().Content

			screen = pressKeys(screen, question, escape)

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("esc while the panes show changes nothing", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b"), 80, 12)
			before := screen.View().Content

			screen = pressKeys(screen, escape)

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("↑ and ↓ do not change the selected service while the help shows", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)

			screen = pressKeys(screen, down, question, down, up, up, escape)

			require.Equal(t, "com.example.b", selectedName(screen))
		})

		t.Run("q ends the screen while the help shows", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, refreshEvery2s), "?q"))
		})

		t.Run("the help line at the bottom of the screen shows s start, x stop and ? help, under the panes and under the help", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)
			panes := strings.Split(ansi.Strip(screen.View().Content), "\n")
			help := strings.Split(ansi.Strip(pressKeys(screen, question).View().Content), "\n")

			require.Equal(t, []string{"╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯", " s start · x stop · ? help"}, panes[10:])
			require.Equal(t, []string{"╰" + strings.Repeat("─", 78) + "╯", " s start · x stop · ? help"}, help[10:])
		})

		t.Run("the help line is cut at the width of the terminal", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 12, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, " s start · x", lines[len(lines)-1])
		})

		t.Run("another key does not end the screen", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.Nil(t, press(screen, tea.KeyPressMsg{Code: 'x', Text: "x"}))
		})
	})
}
