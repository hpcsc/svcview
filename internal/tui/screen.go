package tui

import (
	"slices"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/service"
)

const (
	headerHeight = 2
	refreshEvery = 2 * time.Second
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
		}
	case servicesRead:
		slices.SortFunc(msg.services, func(a, b service.Service) int {
			return strings.Compare(a.Name, b.Name)
		})
		s.lastRead = msg
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

func (s Screen) content() string {
	if s.width == 0 {
		return ""
	}
	header := box{title: "svcview", right: s.domain, width: s.width, height: headerHeight}
	list := box{title: "Services", width: s.width * 40 / 100, height: s.height - headerHeight}
	list.text = s.listText(list.textWidth())
	return header.render() + "\n" + list.render()
}

func (s Screen) listText(width int) []string {
	if s.lastRead.err != nil {
		return strings.Split(ansi.Wrap(s.printable(s.lastRead.err.Error()), width, ""), "\n")
	}
	if len(s.lastRead.services) == 0 {
		return nil
	}
	text := []string{"Other"}
	for _, svc := range s.lastRead.services {
		text = append(text, " "+s.printable(svc.Name))
	}
	return text
}

func (s Screen) printable(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, text)
}
