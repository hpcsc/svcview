//go:build unit

package tui_test

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/service"
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
	var screen tea.Model = tui.NewScreen(domain, read, settings)
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
)

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
	var screen tea.Model = tui.NewScreen("gui/501", read, refreshEvery2s)
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
	program := tea.NewProgram(screen,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithWindowSize(80, 12))
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s)
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s)
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

			lines := strings.Split(screen.View().Content, "\n")
			require.Contains(t, lines[4], "│ launchctl list: exit status 3: Bootstrap failed ")
			require.Contains(t, lines[5], "│ printed �[2J on stdout ")
		})
	})

	t.Run("refresh", func(t *testing.T) {
		t.Run("shows a service that is added, no later than 2 s after the change, with no key press", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("does not read again before 2 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 1900*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("removes the row of a service that is removed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker", "com.example.removed"), servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.removed")
			})
		})

		t.Run("shows the glyph of the new state of a service at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● com.example.worker")
			})
		})

		t.Run("changes the counts at the read that changes a state", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● 1 running   ◐ 0 loaded   ○ 0 not loaded")
			})
		})

		t.Run("shows the new PID, uptime and last exit of the selected service in the detail pane at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: new(time.Hour)}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4123, Uptime: new(5 * time.Second), LastExit: service.LastExit{Signal: "SIGKILL"}}))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "permission denied")
			})
		})

		t.Run("with a refresh of 5 s, shows a service that is added, no later than 5 s after the read before it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, config.Settings{Refresh: 5 * time.Second}), 5100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("with a refresh of 5 s, does not read again before 5 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, config.Settings{Refresh: 5 * time.Second}), 4900*time.Millisecond)

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

				runFor(t, tui.NewScreen("gui/501", slow, refreshEvery2s), 20*time.Second)

				require.Equal(t, int32(1), mostAtOnce.Load())
				require.GreaterOrEqual(t, reads.Load(), int32(3))
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s)
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

				screen := runFor(t, tui.NewScreen("gui/501", read, settings), 2100*time.Millisecond, down)

				require.Equal(t, "com.apple.a", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row comes before it in another group", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := groups(t, "Workers", "worker")
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.worker.01", "com.apple.a", "homebrew.mxcl.redis"),
					servicesNamed("com.example.worker.01", "com.example.worker.02", "com.apple.a", "homebrew.mxcl.redis"))

				screen := runFor(t, tui.NewScreen("gui/501", read, settings), 2100*time.Millisecond, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down)

				require.Contains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row before it goes", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.b", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down)

				require.NotContains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})

		t.Run("when the selected service is gone, the row that takes its place is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down)

				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("when the selected service was the last row and is gone, the last row is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.b"))

				screen := runFor(t, tui.NewScreen("gui/501", read, refreshEvery2s), 2100*time.Millisecond, down, down)

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

	t.Run("keys", func(t *testing.T) {
		t.Run("q ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s), "q"))
		})

		t.Run("ctrl+c ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s), "\x03"))
		})

		t.Run("? shows each key of the screen with its action, in place of the panes", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

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
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), refreshEvery2s), "?q"))
		})

		t.Run("the help line at the bottom of the screen shows ? help, under the panes and under the help", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)
			panes := strings.Split(ansi.Strip(screen.View().Content), "\n")
			help := strings.Split(ansi.Strip(pressKeys(screen, question).View().Content), "\n")

			require.Equal(t, []string{"╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯", " ? help"}, panes[10:])
			require.Equal(t, []string{"╰" + strings.Repeat("─", 78) + "╯", " ? help"}, help[10:])
		})

		t.Run("another key does not end the screen", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.Nil(t, press(screen, tea.KeyPressMsg{Code: 'x', Text: "x"}))
		})
	})
}
