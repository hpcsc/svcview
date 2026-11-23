package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/service"
)

const (
	headerHeight = 3
	refreshEvery = 2 * time.Second
	accent       = ansi.Blue
	detailColumn = 14
)

type refresh struct{}

type servicesRead struct {
	services []service.Service
	err      error
}

type Screen struct {
	domain   string
	read     func() ([]service.Service, error)
	lastRead servicesRead
	hasRead  bool
	selected string
	width    int
	height   int
}

func NewScreen(domain string, read func() ([]service.Service, error)) Screen {
	return Screen{domain: domain, read: read}
}

func (s Screen) Init() tea.Cmd {
	return s.readServices
}

func (s Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return s, tea.Quit
		case "up":
			s.selected = s.selectedAfterMove(-1)
		case "down":
			s.selected = s.selectedAfterMove(1)
		}
	case servicesRead:
		slices.SortFunc(msg.services, func(a, b service.Service) int {
			return strings.Compare(a.Name, b.Name)
		})
		s.lastRead, s.hasRead = msg, true
		s.selected = s.selectedAfterRead()
		return s, tea.Tick(refreshEvery, func(time.Time) tea.Msg { return refresh{} })
	case refresh:
		return s, s.readServices
	}
	return s, nil
}

func (s Screen) View() tea.View {
	view := tea.NewView(s.content())
	view.AltScreen = true
	return view
}

func (s Screen) readServices() tea.Msg {
	services, err := s.read()
	return servicesRead{services: services, err: err}
}

func (s Screen) rows() []service.Service {
	if s.lastRead.err != nil {
		return nil
	}
	return s.lastRead.services
}

func (s Screen) selectedAfterRead() string {
	rows := s.rows()
	if len(rows) == 0 {
		return s.selected
	}
	at, _ := slices.BinarySearchFunc(rows, s.selected, func(svc service.Service, name string) int {
		return strings.Compare(svc.Name, name)
	})
	return rows[min(at, len(rows)-1)].Name
}

func (s Screen) selectedAfterMove(step int) string {
	at := s.selectedAt()
	if at < 0 {
		return s.selected
	}
	rows := s.rows()
	return rows[min(max(at+step, 0), len(rows)-1)].Name
}

func (s Screen) selectedAt() int {
	return slices.IndexFunc(s.rows(), func(svc service.Service) bool { return svc.Name == s.selected })
}

func (s Screen) content() string {
	if s.width == 0 {
		return ""
	}
	header := box{title: "svcview", right: s.domain, width: s.width, height: headerHeight, text: s.headerText()}
	list := box{title: "Services", width: s.width * 40 / 100, height: s.height - headerHeight}
	list.text = s.listText(list.textWidth())
	detail := box{width: s.width - list.width, height: list.height}
	if at := s.selectedAt(); at >= 0 {
		svc := s.rows()[at]
		detail.title, detail.text = s.printable(svc.Name), s.detailText(svc)
	}
	lines := header.render()
	detailLines := detail.render()
	for i, line := range list.render() {
		lines = append(lines, line+detailLines[i])
	}
	return strings.Join(lines, "\n")
}

func (s Screen) headerText() []string {
	if !s.hasRead || s.lastRead.err != nil {
		return nil
	}
	counts := map[service.State]int{}
	for _, svc := range s.lastRead.services {
		counts[svc.State]++
	}
	var text []string
	for _, state := range []service.State{service.Running, service.Loaded, service.NotLoaded} {
		text = append(text, fmt.Sprintf("%s %d %s", s.stateGlyph(state), counts[state], state))
	}
	return []string{" " + strings.Join(text, "   ")}
}

func (s Screen) listText(width int) []string {
	if s.lastRead.err != nil {
		var text []string
		for line := range strings.SplitSeq(s.lastRead.err.Error(), "\n") {
			text = append(text, strings.Split(ansi.Wrap(s.printable(line), width, ""), "\n")...)
		}
		return text
	}
	if len(s.lastRead.services) == 0 {
		return nil
	}
	text := []string{"Other"}
	for _, svc := range s.lastRead.services {
		if svc.Name == s.selected {
			text = append(text, s.selectedRow(svc, width))
			continue
		}
		text = append(text, " "+s.rowGlyph(svc)+" "+s.printable(svc.Name))
	}
	return text
}

func (s Screen) selectedRow(svc service.Service, width int) string {
	nameWidth := max(width-2, 0)
	name := ansi.Truncate(" "+s.printable(svc.Name), nameWidth, "…")
	name += strings.Repeat(" ", nameWidth-ansi.StringWidth(name))
	return ansi.NewStyle().ForegroundColor(accent).Styled("▌") +
		s.rowGlyph(svc) +
		ansi.NewStyle().BackgroundColor(ansi.BrightBlack).Styled(name)
}

func (s Screen) detailText(svc service.Service) []string {
	pid, uptime := "-", "-"
	if svc.PID != 0 {
		pid = strconv.Itoa(svc.PID)
	}
	if svc.Uptime != nil {
		uptime = s.uptimeText(*svc.Uptime)
	}
	return []string{
		s.detailColumns(s.stateGlyph(svc.State)+" "+svc.State.String(), "PID "+pid),
		s.detailColumns("up "+uptime, "last exit "+s.lastExitText(svc.LastExit)),
	}
}

func (s Screen) detailColumns(left, right string) string {
	return left + strings.Repeat(" ", max(detailColumn-ansi.StringWidth(left), 1)) + right
}

func (s Screen) uptimeText(uptime time.Duration) string {
	switch {
	case uptime < time.Minute:
		return fmt.Sprintf("%ds", int(uptime.Seconds()))
	case uptime < time.Hour:
		return fmt.Sprintf("%dm%ds", int(uptime.Minutes()), int(uptime.Seconds())%60)
	case uptime < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(uptime.Hours()), int(uptime.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(uptime.Hours())/24, int(uptime.Hours())%24)
	}
}

func (s Screen) lastExitText(exit service.LastExit) string {
	text := exit.Signal
	if text == "" {
		text = strconv.Itoa(exit.Code)
	}
	if exit.Failed() {
		return ansi.NewStyle().ForegroundColor(ansi.Red).Styled(text)
	}
	return text
}

func (s Screen) stateGlyph(state service.State) string {
	shape, color := s.stateLook(state)
	return ansi.NewStyle().ForegroundColor(color).Styled(shape)
}

func (s Screen) rowGlyph(svc service.Service) string {
	shape, color := s.stateLook(svc.State)
	if svc.LastExit.Failed() {
		color = ansi.Red
	}
	return ansi.NewStyle().ForegroundColor(color).Styled(shape)
}

func (s Screen) stateLook(state service.State) (shape string, color ansi.BasicColor) {
	switch state {
	case service.Running:
		return "●", ansi.Green
	case service.Loaded:
		return "◐", ansi.Yellow
	default:
		return "○", ansi.BrightBlack
	}
}

func (s Screen) printable(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, text)
}
