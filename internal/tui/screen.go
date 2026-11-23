package tui

import (
	"fmt"
	"slices"
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
	rows := s.rows()
	at := slices.IndexFunc(rows, func(svc service.Service) bool { return svc.Name == s.selected })
	if at < 0 {
		return s.selected
	}
	return rows[min(max(at+step, 0), len(rows)-1)].Name
}

func (s Screen) content() string {
	if s.width == 0 {
		return ""
	}
	header := box{title: "svcview", right: s.domain, width: s.width, height: headerHeight, text: s.headerText()}
	list := box{title: "Services", width: s.width * 40 / 100, height: s.height - headerHeight}
	list.text = s.listText(list.textWidth())
	return header.render() + "\n" + list.render()
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
