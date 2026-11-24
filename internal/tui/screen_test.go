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
	var screen tea.Model = tui.NewScreen(domain, read, nil, nil, nil, nil, settings)
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
	keyS     = tea.KeyPressMsg{Code: 's', Text: "s"}
	keyX     = tea.KeyPressMsg{Code: 'x', Text: "x"}
	keyR     = tea.KeyPressMsg{Code: 'r', Text: "r"}
	keyY     = tea.KeyPressMsg{Code: 'y', Text: "y"}
	keyN     = tea.KeyPressMsg{Code: 'n', Text: "n"}
	keyQ     = tea.KeyPressMsg{Code: 'q', Text: "q"}
	keyJ     = tea.KeyPressMsg{Code: 'j', Text: "j"}
	keyK     = tea.KeyPressMsg{Code: 'k', Text: "k"}
	keyL     = tea.KeyPressMsg{Code: 'l', Text: "l"}
	keyH     = tea.KeyPressMsg{Code: 'h', Text: "h"}
	ctrlC    = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
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
	var screen tea.Model = tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s)
	start := screen.Init()
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return screen.Update(start())
}

// readAgain must run in a synctest bubble: the tick waits 2 s on the clock of the bubble.
func readAgain(screen tea.Model, tick tea.Cmd) (tea.Model, tea.Cmd) {
	screen, read := screen.Update(tick())
	return screen.Update(read())
}

func readNow(screen tea.Model) tea.Model {
	screen, _ = screen.Update(screen.Init()())
	return screen
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

type readTimes struct {
	lock  sync.Mutex
	start time.Time
	times []time.Duration
}

func newReadTimes() *readTimes {
	return &readTimes{start: time.Now()}
}

func (r *readTimes) recording(read func() ([]service.Service, error)) func() ([]service.Service, error) {
	return func() ([]service.Service, error) {
		r.lock.Lock()
		r.times = append(r.times, time.Since(r.start))
		r.lock.Unlock()
		return read()
	}
}

func (r *readTimes) get() []time.Duration {
	r.lock.Lock()
	defer r.lock.Unlock()
	return slices.Clone(r.times)
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

type serviceCalls struct {
	lock     sync.Mutex
	services []service.Service
}

func (c *serviceCalls) record(svc service.Service) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.services = append(c.services, svc)
	return nil
}

func (c *serviceCalls) get() []service.Service {
	c.lock.Lock()
	defer c.lock.Unlock()
	return slices.Clone(c.services)
}

func openStarting(t *testing.T, read func() ([]service.Service, error), start func(service.Service) error) tea.Model {
	t.Helper()
	return openActing(t, read, start, nil, nil)
}

func openActing(t *testing.T, read func() ([]service.Service, error), start, stop, restart func(service.Service) error) tea.Model {
	t.Helper()
	var screen tea.Model = tui.NewScreen("gui/501", read, nil, start, stop, restart, refreshEvery2s)
	first := screen.Init()
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	screen, _ = screen.Update(first())
	return screen
}

func failingWith(text string) func(service.Service) error {
	return func(service.Service) error {
		return errors.New(text)
	}
}

func bottomLines(screen tea.Model, count int) []string {
	lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
	return lines[len(lines)-count:]
}

func runAll(cmds ...tea.Cmd) {
	for _, cmd := range cmds {
		if cmd != nil {
			cmd()
		}
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

func summaryAt(text string, level status.Level, at time.Time) status.Result {
	return status.Result{Output: &status.Output{Summary: text, Level: level, At: &at}}
}

func workersStaleAfter(t *testing.T, staleAfter time.Duration, otherNameAndMatch ...string) config.Settings {
	t.Helper()
	settings := workersWithStatus(t, otherNameAndMatch...)
	settings.Groups[0].Status.StaleAfter = staleAfter
	return settings
}

type drawNow struct{}

type drawn struct {
	view tea.View
}

func (d drawn) Init() tea.Cmd {
	return nil
}

func (d drawn) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return d, nil
}

func (d drawn) View() tea.View {
	return d.view
}

type drawRecorder struct {
	screen tea.Model
	draws  *[]tea.Model
}

func (r drawRecorder) Init() tea.Cmd {
	return r.screen.Init()
}

func (r drawRecorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(drawNow); ok {
		*r.draws = append(*r.draws, drawn{view: r.screen.View()})
		return r, nil
	}
	var cmd tea.Cmd
	r.screen, cmd = r.screen.Update(msg)
	return r, cmd
}

func (r drawRecorder) View() tea.View {
	return r.screen.View()
}

// drawsAt must run in a synctest bubble: it waits a minute on the clock of the bubble.
func drawsAt(t *testing.T, screen tui.Screen, width, height int, times ...time.Duration) []tea.Model {
	t.Helper()
	var draws []tea.Model
	program := tea.NewProgram(drawRecorder{screen: screen, draws: &draws},
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithWindowSize(width, height))
	start := time.Now()
	go func() {
		for _, at := range times {
			time.Sleep(at - time.Since(start))
			program.Send(drawNow{})
		}
		program.Quit()
	}()
	_, err := program.Run()
	require.NoError(t, err)
	time.Sleep(time.Minute)
	synctest.Wait()
	return draws
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s)
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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s)
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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("does not read again before 2 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 1900*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("removes the row of a service that is removed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker", "com.example.removed"), servicesNamed("com.example.worker"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "com.example.removed")
			})
		})

		t.Run("shows the glyph of the new state of a service at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● com.example.worker")
			})
		})

		t.Run("changes the counts at the read that changes a state", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Loaded}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, ansi.Strip(screen.View().Content), "● 1 running   ◐ 0 loaded   ○ 0 not loaded")
			})
		})

		t.Run("shows the new PID, uptime and last exit of the selected service in the detail pane at the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4122, Uptime: new(time.Hour)}),
					readGiving(service.Service{Name: "com.example.worker", State: service.Running, PID: 4123, Uptime: new(5 * time.Second), LastExit: service.LastExit{Signal: "SIGKILL"}}))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.worker")
				require.NotContains(t, screen.View().Content, "permission denied")
			})
		})

		t.Run("with a refresh of 5 s, shows a service that is added, no later than 5 s after the read before it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, config.Settings{Refresh: 5 * time.Second}), 5100*time.Millisecond)

				require.Contains(t, screen.View().Content, "com.example.added")
			})
		})

		t.Run("with a refresh of 5 s, does not read again before 5 s have passed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond, servicesNamed("com.example.worker"), servicesNamed("com.example.worker", "com.example.added"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, config.Settings{Refresh: 5 * time.Second}), 4900*time.Millisecond)

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

				runFor(t, tui.NewScreen("gui/501", slow, nil, nil, nil, nil, refreshEvery2s), 20*time.Second)

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

				runFor(t, tui.NewScreen("gui/501", readGiving(services...), run, nil, nil, nil, workersWithStatus(t, "Databases", "redis")), 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 5*time.Second)

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

				runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "homebrew.mxcl.redis"), run, nil, nil, nil, settings), 6500*time.Millisecond)

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

				runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 9*time.Second)

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

				runFor(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, settings), 7*time.Second)

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

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, settings), 2500*time.Millisecond)

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

				runFor(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, settings), 4500*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond, down)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t, "Databases", "redis")), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 13, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"Workers", "▌○ a  busy�[2J�next · a sum…", "○ b  no�[2Jfile"}, listRows(screen)[:3])
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})

		t.Run("with a stale_after of 30 s, the row of a service whose output has an at 3 minutes old shows the summary, two spaces, then stale and the age of at, and the detail pane keeps the age in two units", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				at := time.Now().Add(-3*time.Minute - 12*time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summaryAt("busy", status.LevelInfo, at)})

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, 500*time.Millisecond)

				require.Equal(t, []string{"Workers", "▌○ a  busy  stale 3m"}, listRows(draws[0])[:2])
				require.Equal(t, "Status 3m12s ago", detailPane(draws[0])[2])
			})
		})

		t.Run("the age in the stale mark is the age of at in its largest whole unit, rounded down", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": summaryAt("busy", status.LevelInfo, drawTime.Add(-45*time.Second)),
					"com.example.worker.b": summaryAt("busy", status.LevelInfo, drawTime.Add(-time.Minute)),
					"com.example.worker.c": summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute-59*time.Second)),
					"com.example.worker.d": summaryAt("busy", status.LevelInfo, drawTime.Add(-time.Hour)),
					"com.example.worker.e": summaryAt("busy", status.LevelInfo, drawTime.Add(-2*time.Hour-59*time.Minute)),
					"com.example.worker.f": summaryAt("busy", status.LevelInfo, drawTime.Add(-24*time.Hour)),
					"com.example.worker.g": summaryAt("busy", status.LevelInfo, drawTime.Add(-47*time.Hour)),
					"com.example.worker.h": summaryAt("busy", status.LevelInfo, drawTime.Add(-23*time.Hour-59*time.Minute)),
				})
				read := servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c", "com.example.worker.d", "com.example.worker.e", "com.example.worker.f", "com.example.worker.g", "com.example.worker.h")

				draws := drawsAt(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 15, time.Second)

				require.Equal(t, []string{
					"Workers",
					"▌○ a  busy  stale 45s",
					"○ b  busy  stale 1m",
					"○ c  busy  stale 3m",
					"○ d  busy  stale 1h",
					"○ e  busy  stale 2h",
					"○ f  busy  stale 1d",
					"○ g  busy  stale 1d",
					"○ h  busy  stale 23h",
				}, listRows(draws[0]))
			})
		})

		t.Run("a stale summary is dim in the color of its level, the stale mark is in the default color, both are on the background of the selected row, and an output with no summary shows only the mark in the column of the summaries", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				read := servicesNamed("com.example.worker.a", "com.example.worker.bb", "com.example.worker.ccc", "com.example.worker.dddd", "com.example.worker.eeeee")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a":     summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute)),
					"com.example.worker.bb":    summaryAt("idle", status.LevelWarn, drawTime.Add(-2*time.Hour)),
					"com.example.worker.ccc":   summaryAt("failed", status.LevelError, drawTime.Add(-45*time.Second)),
					"com.example.worker.dddd":  summaryAt("busy", status.LevelInfo, drawTime.Add(-10*time.Second)),
					"com.example.worker.eeeee": summaryAt("", status.LevelInfo, drawTime.Add(-5*time.Minute)),
				})

				draws := drawsAt(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, time.Second)

				requireGolden(t, "status-stale", draws[0])
				require.Equal(t, []string{
					"Workers",
					"▌○ a      busy  stale 3m",
					"○ bb     idle  stale 2h",
					"○ ccc    failed  stale 45s",
					"○ dddd   busy",
					"○ eeeee  stale 5m",
				}, listRows(draws[0]))
			})
		})

		t.Run("shows no stale mark for an output with no at, an at not older than stale_after, a failure, a group whose status command has no stale_after, and a group with no status command", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				settings := workersStaleAfter(t, 30*time.Second, "Jobs", `^com\.example\.job\.`, "Databases", "redis")
				settings.Groups[1].Status = statusCommand(t)
				read := servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c", "com.example.job.d", "homebrew.mxcl.redis")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Ended: drawTime.Add(-48 * time.Hour), Output: &status.Output{Summary: "busy"}},
					"com.example.worker.b": summaryAt("idle", status.LevelInfo, drawTime.Add(-30*time.Second)),
					"com.example.worker.c": {Failure: &status.Failure{Kind: status.FailureKindTimeout, Text: "cat /tmp/c.json"}},
					"com.example.job.d":    summaryAt("done", status.LevelInfo, drawTime.Add(-72*time.Hour)),
				})

				draws := drawsAt(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, settings), 80, 16, time.Second)

				require.Equal(t, []string{
					"Workers",
					"▌○ a  busy",
					"○ b  idle",
					"○ c  status timeout",
					"",
					"Jobs",
					"○ d  done",
					"",
					"Databases",
					"○ homebrew.mxcl.redis",
				}, listRows(draws[0]))
			})
		})

		t.Run("a running service and a not loaded service whose outputs have an at older than stale_after each show the stale mark", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				read := readGiving(
					service.Service{Name: "com.example.worker.a", State: service.Running, PID: 4122},
					service.Service{Name: "com.example.worker.b", State: service.NotLoaded},
				)
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute)),
					"com.example.worker.b": summaryAt("idle", status.LevelInfo, drawTime.Add(-5*time.Minute)),
				})

				draws := drawsAt(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, time.Second)

				require.Equal(t, []string{"Workers", "▌● a  busy  stale 3m", "○ b  idle  stale 5m"}, listRows(draws[0])[:3])
			})
		})

		t.Run("with an at 20 s old and a stale_after of 30 s, the row shows no stale mark at first, the mark once at is more than 30 s old while the runs give the same at, and an age that grows at each draw", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				at := time.Now().Add(-20 * time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summaryAt("busy", status.LevelInfo, at)})

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, 9500*time.Millisecond, 10500*time.Millisecond, 45*time.Second)

				require.Equal(t, []string{"▌○ a  busy", "▌○ a  busy  stale 30s", "▌○ a  busy  stale 1m"}, []string{listRows(draws[0])[1], listRows(draws[1])[1], listRows(draws[2])[1]})
			})
		})

		t.Run("after a run that gives a stale at, the next run whose at is newer than stale_after shows its summary with no stale mark at the end of that run", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				run := func(_ status.Command, svc service.Service) status.Result {
					if time.Since(start) < 3*time.Second {
						return summaryAt("busy", status.LevelInfo, start.Add(-3*time.Minute))
					}
					return summaryAt("idle", status.LevelInfo, time.Now())
				}

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, 3*time.Second, 4500*time.Millisecond)

				require.Equal(t, []string{"▌○ a  busy  stale 3m", "▌○ a  idle"}, []string{listRows(draws[0])[1], listRows(draws[1])[1]})
			})
		})

		t.Run("in a terminal of 80 columns, a stale row whose summary is too long cuts the summary with … and keeps it dim in the color of its level, then shows the whole stale mark, and a row that is not stale or whose summary fits is cut as before", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				read := servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c", "com.example.worker.d")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": summaryAt("busy · ENG-692 · 12m", status.LevelInfo, drawTime.Add(-3*time.Minute)),
					"com.example.worker.b": summaryAt("busy · ENG-692 · 12m", status.LevelWarn, drawTime.Add(-2*time.Hour)),
					"com.example.worker.c": summaryAt("busy · ENG-692 · review · 12m", status.LevelInfo, drawTime.Add(-10*time.Second)),
					"com.example.worker.d": summaryAt("idle", status.LevelInfo, drawTime.Add(-3*time.Minute)),
				})

				draws := drawsAt(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, time.Second)

				require.Equal(t, []string{
					"Workers",
					"▌○ a  busy · ENG-…  stale 3m",
					"○ b  busy · ENG-…  stale 2h",
					"○ c  busy · ENG-692 · revi…",
					"○ d  idle  stale 3m",
				}, listRows(draws[0])[:5])
				require.Contains(t, draws[0].View().Content, "\x1b[100;2mbusy · ENG-…\x1b[m\x1b[100m  stale 3m\x1b[m")
				require.Contains(t, draws[0].View().Content, "\x1b[33;2mbusy · ENG-…\x1b[m  stale 2h ")
			})
		})

		t.Run("shows each control character of a cut stale summary as �, and sends no control character to the terminal", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summaryAt("busy\x1b[2J\nnext · ENG-692 · 12m", status.LevelInfo, drawTime.Add(-3*time.Minute))})

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, time.Second)

				require.Equal(t, "▌○ a  busy�[2J�ne…  stale 3m", listRows(draws[0])[1])
				require.NotContains(t, draws[0].View().Content, "\x1b[2J")
			})
		})

		t.Run("with no room for one cell of a stale summary beside the short name and the stale mark, cuts the line at its end with …, as any row", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				settings := workersStaleAfter(t, 30*time.Second, "Jobs", `^com\.example\.job\.`)
				settings.Groups[1].Status = statusCommand(t)
				settings.Groups[1].Status.StaleAfter = 30 * time.Second
				run := runsGiving(map[string]status.Result{
					"com.example.worker.report-runner": summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute)),
					"com.example.job.report-runners":   summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute)),
				})

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.report-runner", "com.example.job.report-runners"), run, nil, nil, nil, settings), 80, 12, time.Second)

				require.Equal(t, []string{
					"Workers",
					"▌○ report-runner  busy  sta…",
					"",
					"Jobs",
					"○ report-runners  busy  st…",
				}, listRows(draws[0])[:5])
			})
		})

		t.Run("in a list pane with room for one cell of a stale summary, shows … in place of the summary and the whole stale mark", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				drawTime := time.Now().Add(time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.nightly-sync": summaryAt("busy", status.LevelInfo, drawTime.Add(-3*time.Minute))})

				draws := drawsAt(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.nightly-sync"), run, nil, nil, nil, workersStaleAfter(t, 30*time.Second)), 80, 12, time.Second)

				require.Equal(t, "▌○ nightly-sync  …  stale 3m", listRows(draws[0])[1])
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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

					screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"failed", "mode    busy", ""}, detailPane(screen)[3:])
			})
		})

		t.Run("an output whose fields each have an empty name shows each value two spaces from the left of the text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Fields: []status.Field{{Name: "", Value: "busy"}, {Name: "", Value: "ENG-692"}}}},
				})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				require.Equal(t, []string{"mo�[2Jde  busy�[2J�next", "ticket    ENG-692", ""}, detailPane(screen)[3:])
				require.NotContains(t, screen.View().Content, "\x1b[2J")
			})
		})

		t.Run("an output with no fields shows no line under Status", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				read := servicesNamed("com.example.worker.a")
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Ended: start, Output: &status.Output{Summary: "busy"}}})

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 2500*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 13, 100*time.Millisecond)

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

					screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 16, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 14, 2500*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 16, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 19, 100*time.Millisecond)

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

				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t, "Databases", "redis")), 80, 14, 100*time.Millisecond)

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
			var screen tea.Model = tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s)
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

		t.Run("j selects the next row in the order of the list, also in the next group, and k the row before it", func(t *testing.T) {
			settings := groups(t, "Workers", "worker")
			screen := openWith(t, settings, servicesNamed("com.apple.a", "com.example.worker.02", "com.example.worker.01"), 80, 12)

			second := pressKeys(screen, keyJ)
			third := pressKeys(second, keyJ)
			back := pressKeys(third, keyK)

			require.Equal(t, []string{"com.example.worker.02", "com.apple.a", "com.example.worker.02"}, []string{selectedName(second), selectedName(third), selectedName(back)})
		})

		t.Run("j at the last row keeps the last row selected, and k at the first row keeps the first row selected", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b"), 80, 12)

			last := pressKeys(screen, keyJ, keyJ)
			first := pressKeys(last, keyK, keyK)

			require.Equal(t, []string{"com.example.b", "com.example.a"}, []string{selectedName(last), selectedName(first)})
		})

		t.Run("J and K in upper case do not change the selected service", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)
			screen = pressKeys(screen, down)
			before := screen.View().Content

			afterJ := pressKeys(screen, tea.KeyPressMsg{Code: 'J', Text: "J"})
			afterK := pressKeys(screen, tea.KeyPressMsg{Code: 'K', Text: "K"})

			require.Equal(t, []string{before, before}, []string{afterJ.View().Content, afterK.View().Content})
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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, settings), 2100*time.Millisecond, down)

				require.Equal(t, "com.apple.a", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row comes before it in another group", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := groups(t, "Workers", "worker")
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.worker.01", "com.apple.a", "homebrew.mxcl.redis"),
					servicesNamed("com.example.worker.01", "com.example.worker.02", "com.apple.a", "homebrew.mxcl.redis"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, settings), 2100*time.Millisecond, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down, down, down, down, down, down)

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

		t.Run("with no rows, j and k change nothing", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed(), 80, 12)
			before := screen.View().Content

			screen = pressKeys(screen, keyJ, keyK)

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("a refresh keeps the same name selected when a row comes before it", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.b", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.Contains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("a refresh keeps the same name selected when a row before it goes", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.b", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.NotContains(t, screen.View().Content, "com.example.a")
				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})

		t.Run("when the selected service is gone, the row that takes its place is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.c"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down)

				require.Equal(t, "com.example.c", selectedName(screen))
			})
		})

		t.Run("when the selected service was the last row and is gone, the last row is selected", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := changingAt(100*time.Millisecond,
					servicesNamed("com.example.a", "com.example.b", "com.example.c"),
					servicesNamed("com.example.a", "com.example.b"))

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, nil, refreshEvery2s), 2100*time.Millisecond, tab)

				require.Contains(t, screen.View().Content, "com.example.added")
				require.Equal(t, []string{dimBorder, blueBorder}, borderStyles(screen))
			})
		})

		t.Run("the end of a run keeps the focus on the detail pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := workersWithStatus(t)
				settings.Groups[0].Status = statusEvery(t, time.Second)
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summary("busy", status.LevelInfo)})

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, settings), 1500*time.Millisecond, tab)

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

		t.Run("l moves the focus to the detail pane: its border is blue, and the border of the list is dim, and h then moves the focus back to the list", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)

			detail := pressKeys(screen, keyL)
			list := pressKeys(detail, keyH)

			require.Equal(t, [][]string{{dimBorder, blueBorder}, {blueBorder, dimBorder}}, [][]string{borderStyles(detail), borderStyles(list)})
		})

		t.Run("l with the focus on the detail pane, and h with the focus on the list, change nothing", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b"), 80, 12)
			onList := screen.View().Content
			detail := pressKeys(screen, keyL)
			onDetail := detail.View().Content

			require.Equal(t, []string{onList, onDetail}, []string{pressKeys(screen, keyH).View().Content, pressKeys(detail, keyL).View().Content})
		})

		t.Run("after l, ↓ and j scroll the detail pane with the same service selected, and after h, they each select the next service", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{
					"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}},
					"com.example.worker.b": {Output: &status.Output{Events: eventsNumbered(10)}},
					"com.example.worker.c": {Output: &status.Output{Events: eventsNumbered(10)}},
				})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b", "com.example.worker.c"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				scrolled := pressKeys(screen, keyL, down, keyJ)
				selected := pressKeys(scrolled, keyH, down, keyJ)

				require.Equal(t, [][]string{{"a", "Status 1m0s ago"}, {"c", "○ not loaded  PID -"}}, [][]string{
					{selectedName(scrolled), detailPane(scrolled)[0]},
					{selectedName(selected), detailPane(selected)[0]},
				})
			})
		})

		t.Run("l and h move the focus also when the list has no rows, or shows an error", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return nil, errors.New("launchctl list: exit status 3")
			}

			noRows := pressKeys(open(t, "gui/501", servicesNamed(), 80, 12), keyL)
			readError := pressKeys(open(t, "gui/501", failing, 80, 12), keyL)

			require.Equal(t, [][]string{{dimBorder, blueBorder}, {dimBorder, blueBorder}, {blueBorder, dimBorder}, {blueBorder, dimBorder}}, [][]string{
				borderStyles(noRows),
				borderStyles(readError),
				borderStyles(pressKeys(noRows, keyH)),
				borderStyles(pressKeys(readError, keyH)),
			})
		})

		t.Run("while the help shows, l and h change nothing, and after the help hides, the focus is where it was before the help showed", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)
			helpOverList := pressKeys(screen, question)
			helpOverDetail := pressKeys(screen, keyL, question)

			afterL := pressKeys(helpOverList, keyL)
			afterH := pressKeys(helpOverDetail, keyH)

			require.Equal(t, []string{helpOverList.View().Content, helpOverDetail.View().Content}, []string{afterL.View().Content, afterH.View().Content})
			require.Equal(t, [][]string{{blueBorder, dimBorder}, {dimBorder, blueBorder}}, [][]string{
				borderStyles(pressKeys(afterL, escape)),
				borderStyles(pressKeys(afterH, escape)),
			})
		})

		t.Run("H and L in upper case do not move the focus", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a"), 80, 12)

			onList := pressKeys(screen, tea.KeyPressMsg{Code: 'L', Text: "L"})
			onDetail := pressKeys(screen, tab, tea.KeyPressMsg{Code: 'H', Text: "H"})

			require.Equal(t, [][]string{{blueBorder, dimBorder}, {dimBorder, blueBorder}}, [][]string{borderStyles(onList), borderStyles(onDetail)})
		})
	})

	t.Run("scroll", func(t *testing.T) {
		t.Run("with the focus on the detail pane, ↓ moves the text up one line, so the next line shows at the bottom, and ↑ moves it back down one line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				bottom := pressKeys(screen, tab, down, down, down, down, down, down, down, down, down, down, down, down)
				top := pressKeys(bottom, up, up, up, up, up, up, up, up, up, up, up, up, down)

				require.Equal(t, [][]string{
					{"09:04 ● event 05", "09:05 ● event 06", "09:06 ● event 07", "09:07 ● event 08", "09:08 ● event 09", "09:09 ● event 10"},
					{"up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02"},
				}, [][]string{detailPane(bottom), detailPane(top)})
			})
		})

		t.Run("with the focus on the detail pane, j moves the text up one line, so the next line shows at the bottom, and k moves it back down one line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				scrolled := pressKeys(screen, tab, keyJ)
				back := pressKeys(scrolled, keyK)

				require.Equal(t, [][]string{
					{"up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01", "09:01 ● event 02"},
					{"○ not loaded  PID -", "up -          last exit 0", "Status 1m0s ago", "", "Events", "09:00 ● event 01"},
				}, [][]string{detailPane(scrolled), detailPane(back)})
			})
		})

		t.Run("j stops when the last line of the text is at the bottom of the pane, and k stops at the first line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				bottom := pressKeys(screen, tab, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ, keyJ)
				top := pressKeys(bottom, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyK, keyJ)

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
				var screen tea.Model = tui.NewScreen("gui/501", readsInTurn(rows, failing, rows), nil, nil, nil, nil, refreshEvery2s)
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
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				screen = pressKeys(screen, tab, down, down, down)

				requireGolden(t, "scrolled-detail", screen)
			})
		})

		t.Run("with the focus on the detail pane, ↑ and ↓ do not change the selected service, and the list does not move", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.1", "com.example.worker.2", "com.example.worker.3", "com.example.worker.4", "com.example.worker.5",
					"com.example.worker.6", "com.example.worker.7", "com.example.worker.8", "com.example.worker.9")
				run := runsGiving(map[string]status.Result{"com.example.worker.4": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, down, down, down)
				list := listRows(screen)

				screen = pressKeys(screen, tab, down, down, up, down)

				require.Equal(t, "4", selectedName(screen))
				require.Equal(t, list, listRows(screen))
				require.Equal(t, "Status 1m0s ago", detailPane(screen)[0])
			})
		})

		t.Run("with the focus on the detail pane, j and k do not change the selected service, and the list does not move", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				read := servicesNamed("com.example.worker.1", "com.example.worker.2", "com.example.worker.3", "com.example.worker.4", "com.example.worker.5",
					"com.example.worker.6", "com.example.worker.7", "com.example.worker.8", "com.example.worker.9")
				run := runsGiving(map[string]status.Result{"com.example.worker.4": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, down, down, down)
				list := listRows(screen)

				screen = pressKeys(screen, tab, keyJ, keyJ, keyK, keyJ)

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
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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
				screen := runSized(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

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

				screen := runFor(t, tui.NewScreen("gui/501", read, run, nil, nil, nil, workersWithStatus(t)), 2100*time.Millisecond, down, tab, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, settings), 1500*time.Millisecond, tab, down, down, down, down, down, down, down, down, down)

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

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, settings), 1500*time.Millisecond, tab, down, down, down, down, down)

				require.Equal(t, []string{"○ not loaded  PID -", "up -          last exit 0", "Status 1m0s ago", "exit code 3", "", ""}, detailPane(screen))
			})
		})

		t.Run("a terminal that gets higher shows more lines with no empty line under the last line, and a terminal that gets lower keeps the scroll where it is", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
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

				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, settings), 2500*time.Millisecond, tab, down, down)
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
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)

				screen = pressKeys(screen, tab, question, down, down, escape)

				require.Equal(t, "○ not loaded  PID -", detailPane(screen)[0])
			})
		})

		t.Run("while the help shows, j and k do not scroll the detail pane", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, nil, nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, tab, keyJ, keyJ)

				screen = pressKeys(screen, question, keyJ, keyJ, keyK, escape)

				require.Equal(t, "Status 1m0s ago", detailPane(screen)[0])
			})
		})
	})

	t.Run("start a service", func(t *testing.T) {
		notLoaded := service.Service{Name: "com.example.a", State: service.NotLoaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.a.plist"}
		loaded := service.Service{Name: "com.example.b", State: service.Loaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.b.plist"}
		running := service.Service{Name: "com.example.c", State: service.Running, PID: 4122, PlistPath: "/Users/me/Library/LaunchAgents/com.example.c.plist"}

		t.Run("s on a not loaded service and on a loaded service calls the start one time with the service of the selected row", func(t *testing.T) {
			starts := &serviceCalls{}
			screen := openStarting(t, readGiving(notLoaded, loaded), starts.record)

			screen, startA := screen.Update(keyS)
			screen = pressKeys(screen, down)
			_, startB := screen.Update(keyS)
			runAll(startA, startB)

			require.Equal(t, []service.Service{notLoaded, loaded}, starts.get())
		})

		t.Run("s on a running service calls nothing", func(t *testing.T) {
			starts := &serviceCalls{}
			screen := openStarting(t, readGiving(running), starts.record)

			_, cmd := screen.Update(keyS)
			runAll(cmd)

			require.Empty(t, starts.get())
		})

		t.Run("while the start of a service has not ended, s on that service calls nothing, and s on another service still calls the start", func(t *testing.T) {
			starts := &serviceCalls{}
			screen := openStarting(t, readGiving(notLoaded, loaded), starts.record)

			screen, first := screen.Update(keyS)
			screen, again := screen.Update(keyS)
			screen = pressKeys(screen, down)
			_, other := screen.Update(keyS)
			runAll(first, again, other)

			require.Equal(t, []service.Service{notLoaded, loaded}, starts.get())
		})

		t.Run("after the start of a service ends, s on that service calls the start again", func(t *testing.T) {
			starts := &serviceCalls{}
			screen := openStarting(t, readGiving(notLoaded), starts.record)

			screen, first := screen.Update(keyS)
			screen, _ = screen.Update(first())
			_, again := screen.Update(keyS)
			runAll(again)

			require.Equal(t, []service.Service{notLoaded, notLoaded}, starts.get())
		})

		t.Run("s calls nothing while the help shows, with no rows, and while the list shows a read error", func(t *testing.T) {
			starts := &serviceCalls{}
			failing := func() ([]service.Service, error) {
				return []service.Service{notLoaded}, errors.New("launchctl list: exit status 3")
			}

			_, inHelp := pressKeys(openStarting(t, readGiving(notLoaded), starts.record), question).Update(keyS)
			_, noRows := openStarting(t, readGiving(), starts.record).Update(keyS)
			_, readError := readNow(openStarting(t, readsInTurn(readGiving(notLoaded), failing), starts.record)).Update(keyS)
			runAll(inHelp, noRows, readError)

			require.Empty(t, starts.get())
		})

		t.Run("the screen reads every 2 s and takes keys while a start runs, and a start that lasts longer than the refresh does not move the next read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reads := newReadTimes()
				read := reads.recording(readGiving(notLoaded, loaded))
				starts := &serviceCalls{}
				start := func(svc service.Service) error {
					_ = starts.record(svc)
					time.Sleep(5 * time.Second)
					return nil
				}

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, start, nil, nil, refreshEvery2s), 6500*time.Millisecond, keyS, down)

				require.Equal(t, []service.Service{notLoaded}, starts.get())
				require.Equal(t, []time.Duration{0, 2 * time.Second, 4 * time.Second, 6 * time.Second}, reads.get())
				require.Equal(t, "com.example.b", selectedName(screen))
			})
		})

		t.Run("after a start ends, the row shows the state that the next refresh reads, and the end of the start starts no read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reads := newReadTimes()
				read := reads.recording(changingAt(time.Second, readGiving(loaded), readGiving(service.Service{Name: "com.example.b", State: service.Running, PID: 4123})))
				starts := &serviceCalls{}
				start := func(svc service.Service) error {
					_ = starts.record(svc)
					time.Sleep(time.Second)
					return nil
				}

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, start, nil, nil, refreshEvery2s), 2500*time.Millisecond, keyS)

				require.Equal(t, []service.Service{loaded}, starts.get())
				require.Equal(t, []time.Duration{0, 2 * time.Second}, reads.get())
				require.Contains(t, ansi.Strip(screen.View().Content), "▌● com.example.b")
			})
		})
	})

	t.Run("message line", func(t *testing.T) {
		listBottom := "╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯"
		helpLine := " s start · x stop · r restart · ? help"

		t.Run("when the start gives an error, the line above the help line shows its text, and the panes end one line higher", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a"), failingWith("Bootstrap failed: 5: Input/output error"))

			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())

			requireGolden(t, "message-line", screen)
			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, []string{listBottom, " Bootstrap failed: 5: Input/output error", helpLine}, lines[9:])
		})

		t.Run("when the start gives an error while the help shows, the help box ends one line higher", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a"), failingWith("Bootstrap failed: 5: Input/output error"))

			screen, start := screen.Update(keyS)
			screen = pressKeys(screen, question)
			screen, _ = screen.Update(start())

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, []string{"╰" + strings.Repeat("─", 78) + "╯", " Bootstrap failed: 5: Input/output error", helpLine}, lines[9:])
		})

		t.Run("shows the lines of an error on one line, each line after the line before it with a space between them, and each control character as �", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a"), failingWith("Bootstrap failed: 5: Input/output error\nTry re-running the command as root\x1b[2J for richer errors."))
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 120, Height: 12})

			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())

			require.Equal(t, " Bootstrap failed: 5: Input/output error Try re-running the command as root�[2J for richer errors.", bottomLines(screen, 2)[0])
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("cuts a text that is wider than the terminal, and ends it with …", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a"), failingWith("Bootstrap failed: 5: Input/output error"))
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 30, Height: 12})

			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())

			require.Equal(t, " Bootstrap failed: 5: Input/o…", bottomLines(screen, 2)[0])
		})

		t.Run("stays through each refresh and each status run until the next key press, and that key also does what it does without a message line", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": summary("busy", status.LevelInfo)})
				screen := runFor(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a", "com.example.worker.b"), run, failingWith("Bootstrap failed: 5: Input/output error"), nil, nil, workersWithStatus(t)), 5*time.Second, keyS)
				withMessage := bottomLines(screen, 2)

				screen = pressKeys(screen, down)

				require.Equal(t, []string{" Bootstrap failed: 5: Input/output error", helpLine}, withMessage)
				require.Equal(t, []string{listBottom, helpLine}, bottomLines(screen, 2))
				require.Equal(t, "b", selectedName(screen))
			})
		})

		t.Run("a start that ends with no error shows no message line", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a"), func(service.Service) error { return nil })
			before := screen.View().Content

			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())

			require.Equal(t, before, screen.View().Content)
		})

		t.Run("the error of a later start takes the place of an earlier one", func(t *testing.T) {
			start := func(svc service.Service) error {
				return fmt.Errorf("%s failed", svc.Name)
			}
			screen := openStarting(t, servicesNamed("com.example.a", "com.example.b"), start)

			screen, startA := screen.Update(keyS)
			screen = pressKeys(screen, down)
			screen, startB := screen.Update(keyS)
			screen, _ = screen.Update(startA())
			screen, _ = screen.Update(startB())

			require.Equal(t, []string{listBottom, " com.example.b failed", helpLine}, bottomLines(screen, 3))
		})

		t.Run("after a key hides the message line, the screen shows what it showed before the error", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a", "com.example.b"), failingWith("Bootstrap failed: 5: Input/output error"))
			before := screen.View().Content
			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())
			withMessage := screen.View().Content

			screen = pressKeys(screen, escape)

			require.NotEqual(t, before, withMessage)
			require.Equal(t, before, screen.View().Content)
		})

		t.Run("when the message line shows and hides, the selected row stays in view in the list", func(t *testing.T) {
			screen := openStarting(t, servicesNamed("com.example.a", "com.example.b", "com.example.c", "com.example.d", "com.example.e", "com.example.f", "com.example.g", "com.example.h", "com.example.i", "com.example.j"), failingWith("Bootstrap failed"))
			screen = pressKeys(screen, down, down, down, down, down, down, down, down, down)

			screen, start := screen.Update(keyS)
			screen, _ = screen.Update(start())
			withMessage := listRows(screen)
			screen = pressKeys(screen, escape)

			require.Equal(t, []string{"○ com.example.f", "○ com.example.g", "○ com.example.h", "○ com.example.i", "▌○ com.example.j"}, withMessage)
			require.Equal(t, []string{"○ com.example.e", "○ com.example.f", "○ com.example.g", "○ com.example.h", "○ com.example.i", "▌○ com.example.j"}, listRows(screen))
		})

		t.Run("when the message line shows and hides, the scroll of the detail pane stays inside its text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := runsGiving(map[string]status.Result{"com.example.worker.a": {Output: &status.Output{Events: eventsNumbered(10)}}})
				screen := runSized(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker.a"), run, failingWith("Bootstrap failed"), nil, nil, workersWithStatus(t)), 80, 12, 100*time.Millisecond)
				screen = pressKeys(screen, tab, down, down, down, down, down, down, down, down, down, down, down, down)

				screen, start := screen.Update(keyS)
				screen, _ = screen.Update(start())
				withMessage := detailPane(screen)
				screen = pressKeys(screen, escape)

				require.Equal(t, []string{"09:04 ● event 05", "09:05 ● event 06", "09:06 ● event 07", "09:07 ● event 08", "09:08 ● event 09"}, withMessage)
				require.Equal(t, []string{"09:04 ● event 05", "09:05 ● event 06", "09:06 ● event 07", "09:07 ● event 08", "09:08 ● event 09", "09:09 ● event 10"}, detailPane(screen))
			})
		})
	})

	t.Run("stop a service", func(t *testing.T) {
		notLoaded := service.Service{Name: "com.example.a", State: service.NotLoaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.a.plist"}
		loaded := service.Service{Name: "com.example.b", State: service.Loaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.b.plist"}
		running := service.Service{Name: "com.example.c", State: service.Running, PID: 4122, PlistPath: "/Users/me/Library/LaunchAgents/com.example.c.plist"}
		listBottom := "╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯"
		helpLine := " s start · x stop · r restart · ? help"

		t.Run("x on a loaded service and on a running service shows the confirmation with the full name of the service on the message line, and calls nothing yet", func(t *testing.T) {
			stops := &serviceCalls{}
			screen := openActing(t, readGiving(loaded, running), nil, stops.record, nil)

			screen, onLoaded := screen.Update(keyX)
			loadedLines := bottomLines(screen, 3)
			screen = pressKeys(screen, keyN, down)
			screen, onRunning := screen.Update(keyX)
			runAll(onLoaded, onRunning)

			requireGolden(t, "confirmation", screen)
			require.Equal(t, []string{listBottom, " Stop com.example.b? (y/N)", helpLine}, loadedLines)
			require.Equal(t, []string{listBottom, " Stop com.example.c? (y/N)", helpLine}, bottomLines(screen, 3))
			require.Empty(t, stops.get())
		})

		t.Run("y hides the confirmation and calls the stop one time with the service that the confirmation names", func(t *testing.T) {
			stops := &serviceCalls{}
			screen := openActing(t, readGiving(loaded, running), nil, stops.record, nil)

			screen = pressKeys(screen, down, keyX)
			screen, stop := screen.Update(keyY)
			runAll(stop)

			require.Equal(t, []service.Service{running}, stops.get())
			require.Equal(t, []string{listBottom, helpLine}, bottomLines(screen, 2))
		})

		t.Run("y stops the service that the confirmation names, also when a refresh removed it from the list and changed the selected row while the confirmation showed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				stops := &serviceCalls{}
				other := service.Service{Name: "com.example.d", State: service.Running, PID: 4123}
				var screen tea.Model = tui.NewScreen("gui/501", readsInTurn(readGiving(running, other), readGiving(other)), nil, nil, stops.record, nil, refreshEvery2s)
				first := screen.Init()
				screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
				screen, tick := screen.Update(first())
				screen = pressKeys(screen, keyX)

				screen, _ = readAgain(screen, tick)
				selectedAfterRefresh := selectedName(screen)
				questionAfterRefresh := bottomLines(screen, 2)[0]
				_, stop := screen.Update(keyY)
				runAll(stop)

				require.Equal(t, "com.example.d", selectedAfterRefresh)
				require.Equal(t, " Stop com.example.c? (y/N)", questionAfterRefresh)
				require.Equal(t, []service.Service{running}, stops.get())
			})
		})

		t.Run("each key other than y and ctrl+c answers no: it hides the confirmation, calls nothing, and does nothing else", func(t *testing.T) {
			for _, key := range []tea.KeyPressMsg{keyN, escape, keyQ, up, down, keyJ, keyK, keyL, keyH, keyX, keyS, keyR, question, tab, tea.KeyPressMsg{Code: 'Y', Text: "Y"}} {
				starts, stops := &serviceCalls{}, &serviceCalls{}
				screen := openActing(t, readGiving(service.Service{Name: "com.example.a", State: service.Running, PID: 4121}, loaded, running), starts.record, stops.record, nil)
				screen = pressKeys(screen, down)
				before := screen.View().Content

				screen = pressKeys(screen, keyX)
				screen, cmd := screen.Update(key)
				runAll(cmd)

				require.Nil(t, cmd, key.String())
				require.Equal(t, before, screen.View().Content, key.String())
				require.Empty(t, starts.get(), key.String())
				require.Empty(t, stops.get(), key.String())
			}
		})

		t.Run("with the focus on the detail pane, h and l answer no, and the focus stays on the detail pane", func(t *testing.T) {
			for _, key := range []tea.KeyPressMsg{keyH, keyL} {
				stops := &serviceCalls{}
				screen := openActing(t, readGiving(running), nil, stops.record, nil)
				screen = pressKeys(screen, tab)
				before := screen.View().Content

				screen = pressKeys(screen, keyX)
				screen, cmd := screen.Update(key)
				runAll(cmd)

				require.Nil(t, cmd, key.String())
				require.Equal(t, before, screen.View().Content, key.String())
				require.Empty(t, stops.get(), key.String())
			}
		})

		t.Run("ctrl+c ends the screen while the confirmation shows", func(t *testing.T) {
			screen := openActing(t, readGiving(running), nil, nil, nil)

			screen = pressKeys(screen, keyX)
			question := bottomLines(screen, 2)[0]
			_, cmd := screen.Update(ctrlC)

			require.Equal(t, " Stop com.example.c? (y/N)", question)
			require.Equal(t, tea.QuitMsg{}, cmd())
		})

		t.Run("x on a not loaded service shows no confirmation, and y after it calls nothing", func(t *testing.T) {
			stops := &serviceCalls{}
			screen := openActing(t, readGiving(notLoaded), nil, stops.record, nil)
			before := screen.View().Content

			screen, onNotLoaded := screen.Update(keyX)
			afterX := screen.View().Content
			_, afterY := screen.Update(keyY)
			runAll(onNotLoaded, afterY)

			require.Equal(t, before, afterX)
			require.Empty(t, stops.get())
		})

		t.Run("x shows no confirmation while the help shows, with no rows, while the list shows a read error, and while the start or the stop of the service has not ended", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return []service.Service{running}, errors.New("launchctl list: exit status 3")
			}
			inHelp := pressKeys(openActing(t, readGiving(running), nil, nil, nil), question, keyX)
			noRows := pressKeys(openActing(t, readGiving(), nil, nil, nil), keyX)
			readError := pressKeys(readNow(openActing(t, readsInTurn(readGiving(running), failing), nil, nil, nil)), keyX)
			startNotEnded := pressKeys(openActing(t, readGiving(loaded), func(service.Service) error { return nil }, nil, nil), keyS, keyX)
			stopNotEnded := pressKeys(openActing(t, readGiving(running), nil, func(service.Service) error { return nil }, nil), keyX, keyY, keyX)

			for name, screen := range map[string]tea.Model{"help": inHelp, "no rows": noRows, "read error": readError, "start not ended": startNotEnded, "stop not ended": stopNotEnded} {
				require.NotContains(t, bottomLines(screen, 2)[0], "Stop com.example", name)
			}
		})

		t.Run("after a stop ends, the row shows the state that the next refresh reads, the end of the stop starts no read, and later reads of the service not loaded start nothing", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reads := newReadTimes()
				read := reads.recording(changingAt(time.Second, readGiving(running), readGiving(service.Service{Name: "com.example.c", State: service.NotLoaded, PlistPath: running.PlistPath})))
				starts, stops := &serviceCalls{}, &serviceCalls{}
				stop := func(svc service.Service) error {
					_ = stops.record(svc)
					time.Sleep(time.Second)
					return nil
				}

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, starts.record, stop, nil, refreshEvery2s), 6500*time.Millisecond, keyX, keyY)

				require.Equal(t, []time.Duration{0, 2 * time.Second, 4 * time.Second, 6 * time.Second}, reads.get())
				require.Contains(t, ansi.Strip(screen.View().Content), "▌○ com.example.c")
				require.Equal(t, []service.Service{running}, stops.get())
				require.Empty(t, starts.get())
			})
		})

		t.Run("an error of the stop shows on the message line", func(t *testing.T) {
			screen := openActing(t, readGiving(running), nil, failingWith("Boot-out failed: 3: No such process"), nil)

			screen = pressKeys(screen, keyX)
			screen, stop := screen.Update(keyY)
			screen, _ = screen.Update(stop())

			require.Equal(t, []string{listBottom, " Boot-out failed: 3: No such process", helpLine}, bottomLines(screen, 3))
		})

		t.Run("an error that ends while the confirmation shows does not hide the confirmation, and shows after the answer", func(t *testing.T) {
			screen := openActing(t, readGiving(loaded, running), failingWith("Bootstrap failed: 5: Input/output error"), nil, nil)

			screen, start := screen.Update(keyS)
			screen = pressKeys(screen, down, keyX)
			screen, _ = screen.Update(start())
			whileAsked := bottomLines(screen, 2)[0]
			screen = pressKeys(screen, keyN)

			require.Equal(t, " Stop com.example.c? (y/N)", whileAsked)
			require.Equal(t, " Bootstrap failed: 5: Input/output error", bottomLines(screen, 2)[0])
		})

		t.Run("the confirmation shows each control character of the name as �", func(t *testing.T) {
			screen := openActing(t, readGiving(service.Service{Name: "com.example.\x1b[2Jworker", State: service.Running, PID: 4122}), nil, nil, nil)
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 100, Height: 12})

			screen = pressKeys(screen, keyX)

			require.Equal(t, " Stop com.example.�[2Jworker? (y/N)", bottomLines(screen, 2)[0])
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("the confirmation is cut at the width of the terminal, and ends with …", func(t *testing.T) {
			screen := openActing(t, readGiving(service.Service{Name: "com.example.a-very-long-name", State: service.Running, PID: 4122}), nil, nil, nil)
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 20, Height: 12})

			screen = pressKeys(screen, keyX)

			require.Equal(t, " Stop com.example.a…", bottomLines(screen, 2)[0])
		})
	})

	t.Run("restart a service", func(t *testing.T) {
		notLoaded := service.Service{Name: "com.example.a", State: service.NotLoaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.a.plist"}
		loaded := service.Service{Name: "com.example.b", State: service.Loaded, PlistPath: "/Users/me/Library/LaunchAgents/com.example.b.plist"}
		running := service.Service{Name: "com.example.c", State: service.Running, PID: 4122, PlistPath: "/Users/me/Library/LaunchAgents/com.example.c.plist"}
		listBottom := "╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯"
		helpLine := " s start · x stop · r restart · ? help"

		t.Run("r on a loaded service and on a running service shows the confirmation with the full name of the service on the message line, and calls nothing yet", func(t *testing.T) {
			stops, restarts := &serviceCalls{}, &serviceCalls{}
			screen := openActing(t, readGiving(loaded, running), nil, stops.record, restarts.record)

			screen, onLoaded := screen.Update(keyR)
			loadedLines := bottomLines(screen, 3)
			screen = pressKeys(screen, keyN, down)
			screen, onRunning := screen.Update(keyR)
			runAll(onLoaded, onRunning)

			require.Equal(t, []string{listBottom, " Restart com.example.b? (y/N)", helpLine}, loadedLines)
			require.Equal(t, []string{listBottom, " Restart com.example.c? (y/N)", helpLine}, bottomLines(screen, 3))
			require.Empty(t, stops.get())
			require.Empty(t, restarts.get())
		})

		t.Run("y hides the confirmation and calls the restart one time with the service that the confirmation names, and not the stop", func(t *testing.T) {
			stops, restarts := &serviceCalls{}, &serviceCalls{}
			screen := openActing(t, readGiving(loaded, running), nil, stops.record, restarts.record)

			screen = pressKeys(screen, down, keyR)
			screen, restart := screen.Update(keyY)
			runAll(restart)

			require.Equal(t, []service.Service{running}, restarts.get())
			require.Empty(t, stops.get())
			require.Equal(t, []string{listBottom, helpLine}, bottomLines(screen, 2))
		})

		t.Run("y after x calls the stop and not the restart", func(t *testing.T) {
			stops, restarts := &serviceCalls{}, &serviceCalls{}
			screen := openActing(t, readGiving(running), nil, stops.record, restarts.record)

			screen = pressKeys(screen, keyX)
			_, stop := screen.Update(keyY)
			runAll(stop)

			require.Equal(t, []service.Service{running}, stops.get())
			require.Empty(t, restarts.get())
		})

		t.Run("y restarts the service that the confirmation names, also when a refresh removed it from the list and changed the selected row while the confirmation showed", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				restarts := &serviceCalls{}
				other := service.Service{Name: "com.example.d", State: service.Running, PID: 4123}
				var screen tea.Model = tui.NewScreen("gui/501", readsInTurn(readGiving(running, other), readGiving(other)), nil, nil, nil, restarts.record, refreshEvery2s)
				first := screen.Init()
				screen, _ = screen.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
				screen, tick := screen.Update(first())
				screen = pressKeys(screen, keyR)

				screen, _ = readAgain(screen, tick)
				selectedAfterRefresh := selectedName(screen)
				questionAfterRefresh := bottomLines(screen, 2)[0]
				_, restart := screen.Update(keyY)
				runAll(restart)

				require.Equal(t, "com.example.d", selectedAfterRefresh)
				require.Equal(t, " Restart com.example.c? (y/N)", questionAfterRefresh)
				require.Equal(t, []service.Service{running}, restarts.get())
			})
		})

		t.Run("each key other than y and ctrl+c answers no: it hides the confirmation, calls nothing, and does nothing else", func(t *testing.T) {
			for _, key := range []tea.KeyPressMsg{keyN, escape, keyQ, up, down, keyJ, keyK, keyL, keyH, keyR, keyX, keyS, question, tab, tea.KeyPressMsg{Code: 'Y', Text: "Y"}} {
				starts, stops, restarts := &serviceCalls{}, &serviceCalls{}, &serviceCalls{}
				screen := openActing(t, readGiving(service.Service{Name: "com.example.a", State: service.Running, PID: 4121}, loaded, running), starts.record, stops.record, restarts.record)
				screen = pressKeys(screen, down)
				before := screen.View().Content

				screen = pressKeys(screen, keyR)
				screen, cmd := screen.Update(key)
				runAll(cmd)

				require.Nil(t, cmd, key.String())
				require.Equal(t, before, screen.View().Content, key.String())
				require.Empty(t, starts.get(), key.String())
				require.Empty(t, stops.get(), key.String())
				require.Empty(t, restarts.get(), key.String())
			}
		})

		t.Run("ctrl+c ends the screen while the confirmation shows", func(t *testing.T) {
			screen := openActing(t, readGiving(running), nil, nil, nil)

			screen = pressKeys(screen, keyR)
			question := bottomLines(screen, 2)[0]
			_, cmd := screen.Update(ctrlC)

			require.Equal(t, " Restart com.example.c? (y/N)", question)
			require.Equal(t, tea.QuitMsg{}, cmd())
		})

		t.Run("r on a not loaded service shows no confirmation, and y after it calls nothing", func(t *testing.T) {
			restarts := &serviceCalls{}
			screen := openActing(t, readGiving(notLoaded), nil, nil, restarts.record)
			before := screen.View().Content

			screen, onNotLoaded := screen.Update(keyR)
			afterR := screen.View().Content
			_, afterY := screen.Update(keyY)
			runAll(onNotLoaded, afterY)

			require.Equal(t, before, afterR)
			require.Empty(t, restarts.get())
		})

		t.Run("r shows no confirmation while the help shows, with no rows, while the list shows a read error, and while the start, the stop or the restart of the service has not ended", func(t *testing.T) {
			failing := func() ([]service.Service, error) {
				return []service.Service{running}, errors.New("launchctl list: exit status 3")
			}
			ends := func(service.Service) error { return nil }
			inHelp := pressKeys(openActing(t, readGiving(running), nil, nil, nil), question, keyR)
			noRows := pressKeys(openActing(t, readGiving(), nil, nil, nil), keyR)
			readError := pressKeys(readNow(openActing(t, readsInTurn(readGiving(running), failing), nil, nil, nil)), keyR)
			startNotEnded := pressKeys(openActing(t, readGiving(loaded), ends, nil, nil), keyS, keyR)
			stopNotEnded := pressKeys(openActing(t, readGiving(running), nil, ends, nil), keyX, keyY, keyR)
			restartNotEnded := pressKeys(openActing(t, readGiving(running), nil, nil, ends), keyR, keyY, keyR)

			for name, screen := range map[string]tea.Model{"help": inHelp, "no rows": noRows, "read error": readError, "start not ended": startNotEnded, "stop not ended": stopNotEnded, "restart not ended": restartNotEnded} {
				require.NotContains(t, bottomLines(screen, 2)[0], "Restart com.example", name)
			}
		})

		t.Run("while the restart of a service has not ended, s calls nothing and x shows no confirmation", func(t *testing.T) {
			starts := &serviceCalls{}
			ends := func(service.Service) error { return nil }
			screen := openActing(t, readGiving(loaded), starts.record, nil, ends)

			screen = pressKeys(screen, keyR, keyY)
			screen, start := screen.Update(keyS)
			screen = pressKeys(screen, keyX)
			runAll(start)

			require.Empty(t, starts.get())
			require.NotContains(t, bottomLines(screen, 2)[0], "Stop com.example")
		})

		t.Run("an error of the restart shows on the message line", func(t *testing.T) {
			screen := openActing(t, readGiving(running), nil, nil, failingWith("Could not kickstart service \"com.example.c\": 1: Operation not permitted"))

			screen = pressKeys(screen, keyR)
			screen, restart := screen.Update(keyY)
			screen, _ = screen.Update(restart())

			require.Equal(t, []string{listBottom, " Could not kickstart service \"com.example.c\": 1: Operation not permitted", helpLine}, bottomLines(screen, 3))
		})

		t.Run("the confirmation shows each control character of the name as �", func(t *testing.T) {
			screen := openActing(t, readGiving(service.Service{Name: "com.example.\x1b[2Jworker", State: service.Running, PID: 4122}), nil, nil, nil)
			screen, _ = screen.Update(tea.WindowSizeMsg{Width: 100, Height: 12})

			screen = pressKeys(screen, keyR)

			require.Equal(t, " Restart com.example.�[2Jworker? (y/N)", bottomLines(screen, 2)[0])
			require.NotContains(t, screen.View().Content, "\x1b[2J")
		})

		t.Run("after the restart of a running service ends, the detail pane shows the PID and the uptime that the next refresh reads, and the end of the restart starts no read", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reads := newReadTimes()
				read := reads.recording(changingAt(time.Second,
					readGiving(service.Service{Name: "com.example.c", State: service.Running, PID: 4122, Uptime: new(3 * time.Hour)}),
					readGiving(service.Service{Name: "com.example.c", State: service.Running, PID: 4123, Uptime: new(time.Second)})))
				restarts := &serviceCalls{}
				restart := func(svc service.Service) error {
					_ = restarts.record(svc)
					time.Sleep(time.Second)
					return nil
				}

				screen := runFor(t, tui.NewScreen("gui/501", read, nil, nil, nil, restart, refreshEvery2s), 2500*time.Millisecond, keyR, keyY)

				require.Equal(t, []service.Service{{Name: "com.example.c", State: service.Running, PID: 4122, Uptime: new(3 * time.Hour)}}, restarts.get())
				require.Equal(t, []time.Duration{0, 2 * time.Second}, reads.get())
				content := ansi.Strip(screen.View().Content)
				require.Contains(t, content, "● running     PID 4123")
				require.Contains(t, content, "up 1s")
			})
		})
	})

	t.Run("keys", func(t *testing.T) {
		t.Run("q ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s), "q"))
		})

		t.Run("ctrl+c ends the screen", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s), "\x03"))
		})

		t.Run("? shows each key of the screen with its action, in place of the panes", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 16)

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

		t.Run("j and k do not change the selected service while the help shows", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.a", "com.example.b", "com.example.c"), 80, 12)

			screen = pressKeys(screen, keyJ, question, keyJ, keyK, keyK, escape)

			require.Equal(t, "com.example.b", selectedName(screen))
		})

		t.Run("q ends the screen while the help shows", func(t *testing.T) {
			require.NoError(t, runWithInput(t, tui.NewScreen("gui/501", servicesNamed("com.example.worker"), nil, nil, nil, nil, refreshEvery2s), "?q"))
		})

		t.Run("the help line at the bottom of the screen shows s start, x stop, r restart and ? help, under the panes and under the help", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)
			panes := strings.Split(ansi.Strip(screen.View().Content), "\n")
			help := strings.Split(ansi.Strip(pressKeys(screen, question).View().Content), "\n")

			require.Equal(t, []string{"╰" + strings.Repeat("─", 30) + "╯╰" + strings.Repeat("─", 46) + "╯", " s start · x stop · r restart · ? help"}, panes[10:])
			require.Equal(t, []string{"╰" + strings.Repeat("─", 78) + "╯", " s start · x stop · r restart · ? help"}, help[10:])
		})

		t.Run("the help line is cut at the width of the terminal", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 12, 12)

			lines := strings.Split(ansi.Strip(screen.View().Content), "\n")
			require.Equal(t, " s start · x", lines[len(lines)-1])
		})

		t.Run("another key does not end the screen", func(t *testing.T) {
			screen := open(t, "gui/501", servicesNamed("com.example.worker"), 80, 12)

			require.Nil(t, press(screen, tea.KeyPressMsg{Code: 'z', Text: "z"}))
		})
	})
}
