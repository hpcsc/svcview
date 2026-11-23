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
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/tui"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update", false, "write the golden files from the screen")

func servicesNamed(names ...string) func() ([]service.Service, error) {
	return func() ([]service.Service, error) {
		var services []service.Service
		for _, name := range names {
			services = append(services, service.Service{Name: name})
		}
		return services, nil
	}
}

func servicesRead(services ...service.Service) func() ([]service.Service, error) {
	return func() ([]service.Service, error) {
		return slices.Clone(services), nil
	}
}

func open(t *testing.T, domain string, read func() ([]service.Service, error), width, height int) tea.Model {
	t.Helper()
	var screen tea.Model = tui.NewScreen(domain, read)
	start := screen.Init()
	require.NotNil(t, start)
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: width, Height: height})
	screen, _ = screen.Update(start())
	return screen
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

// runFor must run in a synctest bubble: it waits a minute on the clock of the bubble.
func runFor(t *testing.T, screen tui.Screen, duration time.Duration) tea.Model {
	t.Helper()
	program := tea.NewProgram(screen,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithWindowSize(80, 12))
	go func() {
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"))
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
	})

	t.Run("list", func(t *testing.T) {
		t.Run("shows the group Other with one row for each service, sorted by name", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("homebrew.mxcl.redis", "com.example.worker", "com.apple.a"), 80, 12)

			requireGolden(t, "sorted", screen)
		})

		t.Run("takes 40 percent of the width, rounded down", func(t *testing.T) {
			widths := map[int]int{}
			for _, width := range []int{100, 99} {
				lines := strings.Split(open(t, "gui/501", servicesNamed(), width, 12).View().Content, "\n")
				widths[width] = ansi.StringWidth(lines[2])
			}

			require.Equal(t, map[int]int{100: 40, 99: 39}, widths)
		})

		t.Run("keeps the top border as wide as the pane when the title fills it", func(t *testing.T) {
			lines := strings.Split(open(t, "gui/501", servicesNamed("com.example.worker"), 30, 6).View().Content, "\n")

			var widths []int
			for _, line := range lines[2:] {
				widths = append(widths, ansi.StringWidth(line))
			}
			require.Equal(t, []int{12, 12, 12, 12}, widths)
		})

		t.Run("shows each control character in a name as �, so the name cannot add a line or send a sequence to the terminal", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.\x1b[2Jworker\nnext"), 100, 12)

			require.Contains(t, screen.View().Content, "com.example.�[2Jworker�next")
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("starts each row with the glyph of its state: a green ● for running, a yellow ◐ for loaded, and a gray ○ for not loaded", func(t *testing.T) {
			screen := open(t, "gui/501", servicesRead(
				service.Service{Name: "com.example.a-running", State: service.Running},
				service.Service{Name: "com.example.b-loaded", State: service.Loaded},
				service.Service{Name: "com.example.c-not-loaded", State: service.NotLoaded},
			), 80, 12)

			requireGolden(t, "each-state", screen)
		})

		t.Run("cuts a name that is longer than the list pane and ends it with …, and leaves a name that fits with its glyph", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a-name-that-is-too-long", "com.example.exact", "com.example.fits"), 60, 12)

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

				screen := runFor(t, tui.NewScreen("gui/501", read), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("does not read again before 2 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read), 1900*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("removes the row of a service that is removed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker", "com.example.removed"), servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.removed")
			})
		})

		t.Run("shows the glyph of the new state of a service at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesRead(service.Service{Name: "com.example.worker", State: service.Loaded}),
					servicesRead(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● com.example.worker")
			})
		})

		t.Run("after a read that gives an error, the next read that succeeds shows the rows again", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				failing := func() ([]service.Service, error) {
					return nil, errors.New("open /Users/someone/Library/LaunchAgents: permission denied")
				}
				read := changingAt(100*time.Millisecond, failing, servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "permission denied")
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

				runFor(t, tui.NewScreen("gui/501", slow), 20*time.Second)

				require.Equal(t, int32(1), mostAtOnce.Load())
				require.GreaterOrEqual(t, reads.Load(), int32(3))
			})
		})
	})

	t.Run("keys", func(t *testing.T) {
		t.Run("q ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker")), "q"))
		})

		t.Run("ctrl+c ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker")), "\x03"))
		})

		t.Run("another key does not end the screen", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.Nil(t, press(screen, tea.KeyPressMsg{Code: 'x', Text: "x"}))
		})
	})
}
