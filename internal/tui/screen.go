package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
)

const (
	headerHeight    = 3
	helpLine        = " ? help"
	accent          = ansi.Blue
	detailLeftWidth = 14
)

type refresh struct{}

type servicesRead struct {
	rows []row
	err  error
}

type runEnded struct {
	name   string
	result status.Result
}

type statusTick struct {
	group config.Group
}

type row struct {
	service    service.Service
	group      config.Group
	groupIndex int
}

func (r row) compare(other row) int {
	return cmp.Or(cmp.Compare(r.groupIndex, other.groupIndex), strings.Compare(r.service.Name, other.service.Name))
}

func (r row) shortName() string {
	return r.group.ShortName(r.service.Name)
}

type Screen struct {
	domain      string
	read        func() ([]service.Service, error)
	runStatus   func(status.Command, service.Service) status.Result
	settings    config.Settings
	groupOrder  []config.Group
	lastRead    servicesRead
	hasRead     bool
	statuses    map[string]status.Result
	runNotEnded map[string]bool
	selected    string
	listTop     int
	showHelp    bool
	width       int
	height      int
}

func NewScreen(domain string, read func() ([]service.Service, error), runStatus func(status.Command, service.Service) status.Result, settings config.Settings) Screen {
	return Screen{
		domain:      domain,
		read:        read,
		runStatus:   runStatus,
		settings:    settings,
		groupOrder:  slices.Concat(settings.Groups, []config.Group{config.Other}),
		statuses:    map[string]status.Result{},
		runNotEnded: map[string]bool{},
	}
}

func (s Screen) Init() tea.Cmd {
	return s.readServices
}

func (s Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return s, tea.Quit
		case "?":
			s.showHelp = !s.showHelp
		case "esc":
			s.showHelp = false
		case "up":
			if !s.showHelp {
				s.selected = s.selectedAfterMove(-1)
			}
		case "down":
			if !s.showHelp {
				s.selected = s.selectedAfterMove(1)
			}
		}
	case servicesRead:
		firstRead := !s.hasRead
		s.lastRead, s.hasRead = msg, true
		s.selected = s.selectedAfterRead()
		cmd = tea.Tick(s.settings.Refresh, func(time.Time) tea.Msg { return refresh{} })
		if firstRead {
			cmd = tea.Batch(cmd, s.startStatusRuns())
		}
	case statusTick:
		cmd = tea.Batch(s.runStatuses(msg.group), s.nextStatusTick(msg.group))
	case runEnded:
		delete(s.runNotEnded, msg.name)
		s.statuses[msg.name] = msg.result
	case refresh:
		return s, s.readServices
	}
	s.listTop = s.listTopInView()
	return s, cmd
}

func (s Screen) View() tea.View {
	view := tea.NewView(s.content())
	view.AltScreen = true
	return view
}

func (s Screen) readServices() tea.Msg {
	services, err := s.read()
	rows := make([]row, 0, len(services))
	for _, svc := range services {
		rows = append(rows, s.rowOf(svc))
	}
	slices.SortFunc(rows, row.compare)
	return servicesRead{rows: rows, err: err}
}

func (s Screen) startStatusRuns() tea.Cmd {
	var cmds []tea.Cmd
	for _, group := range s.settings.Groups {
		if group.Status != nil {
			cmds = append(cmds, s.runStatuses(group), s.nextStatusTick(group))
		}
	}
	return tea.Batch(cmds...)
}

func (s Screen) nextStatusTick(group config.Group) tea.Cmd {
	return tea.Tick(group.Status.Every, func(time.Time) tea.Msg { return statusTick{group: group} })
}

func (s Screen) runStatuses(group config.Group) tea.Cmd {
	var runs []tea.Cmd
	for _, r := range s.rows() {
		if r.group == group && !s.runNotEnded[r.service.Name] {
			s.runNotEnded[r.service.Name] = true
			runs = append(runs, s.runStatusOf(r))
		}
	}
	return tea.Batch(runs...)
}

func (s Screen) runStatusOf(r row) tea.Cmd {
	run, command, svc := s.runStatus, *r.group.Status, r.service
	return func() tea.Msg {
		return runEnded{name: svc.Name, result: run(command, svc)}
	}
}

func (s Screen) rowOf(svc service.Service) row {
	group := s.settings.GroupOf(svc.Name)
	return row{service: svc, group: group, groupIndex: slices.Index(s.groupOrder, group)}
}

func (s Screen) rows() []row {
	if s.lastRead.err != nil {
		return nil
	}
	return s.lastRead.rows
}

func (s Screen) selectedAfterRead() string {
	rows := s.rows()
	if len(rows) == 0 {
		return s.selected
	}
	if s.selected == "" {
		return rows[0].service.Name
	}
	at, _ := slices.BinarySearchFunc(rows, s.rowOf(service.Service{Name: s.selected}), row.compare)
	return rows[min(at, len(rows)-1)].service.Name
}

func (s Screen) selectedAfterMove(step int) string {
	at := s.selectedAt()
	if at < 0 {
		return s.selected
	}
	rows := s.rows()
	return rows[min(max(at+step, 0), len(rows)-1)].service.Name
}

func (s Screen) listTopInView() int {
	at := s.selectedAt()
	if at < 0 {
		return 0
	}
	line := s.lineOf(at)
	shown := max(s.paneHeight()-2, 1)
	top := min(max(s.listTop, line-shown+1), line)
	if s.startsGroup(at) {
		top = min(top, max(line-1, line-shown+1))
	}
	return max(min(top, s.lineOf(len(s.rows())-1)+1-shown), 0)
}

func (s Screen) selectedAt() int {
	return slices.IndexFunc(s.rows(), func(r row) bool { return r.service.Name == s.selected })
}

func (s Screen) startsGroup(at int) bool {
	rows := s.rows()
	return at == 0 || rows[at].groupIndex != rows[at-1].groupIndex
}

func (s Screen) linesAbove(at int) []string {
	if !s.startsGroup(at) {
		return nil
	}
	groupLine := s.printable(s.rows()[at].group.Name)
	if at == 0 {
		return []string{groupLine}
	}
	return []string{"", groupLine}
}

func (s Screen) lineOf(at int) int {
	line := at
	for i := range at + 1 {
		line += len(s.linesAbove(i))
	}
	return line
}

func (s Screen) content() string {
	if s.width == 0 {
		return ""
	}
	header := box{title: "svcview", right: s.domain, width: s.width, height: headerHeight, text: s.headerText()}
	lines := header.render()
	if s.showHelp {
		help := box{title: "Help", width: s.width, height: s.paneHeight(), text: s.helpText()}
		lines = append(lines, help.render()...)
	} else {
		lines = append(lines, s.panes()...)
	}
	return strings.Join(append(lines, ansi.Truncate(helpLine, s.width, "")), "\n")
}

func (s Screen) paneHeight() int {
	return s.height - headerHeight - 1
}

func (s Screen) panes() []string {
	list := box{title: "Services", width: s.width * 40 / 100, height: s.paneHeight()}
	list.text = s.listText(list.textWidth())[s.listTop:]
	detail := box{width: s.width - list.width, height: list.height}
	if at := s.selectedAt(); at >= 0 {
		svc := s.rows()[at].service
		detail.title, detail.text = s.printable(svc.Name), s.detailText(svc)
	}
	var lines []string
	detailLines := detail.render()
	for i, line := range list.render() {
		lines = append(lines, line+detailLines[i])
	}
	return lines
}

func (s Screen) helpText() []string {
	return []string{
		"↑ ↓       select a service",
		"?         show or hide this help",
		"esc       hide this help",
		"q ctrl+c  quit",
	}
}

func (s Screen) headerText() []string {
	if !s.hasRead || s.lastRead.err != nil {
		return nil
	}
	counts := map[service.State]int{}
	for _, r := range s.lastRead.rows {
		counts[r.service.State]++
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
	var text []string
	nameWidths := s.nameWidths()
	for at, r := range s.rows() {
		text = append(text, s.linesAbove(at)...)
		name, statusText, color := s.rowText(r, nameWidths[r.groupIndex])
		if r.service.Name == s.selected {
			text = append(text, s.selectedRow(r, name, statusText, color, width))
			continue
		}
		text = append(text, " "+s.rowGlyph(r.service)+name+s.textStyle(color, nil).Styled(statusText))
	}
	return text
}

func (s Screen) nameWidths() map[int]int {
	widths := map[int]int{}
	for _, r := range s.rows() {
		widths[r.groupIndex] = max(widths[r.groupIndex], ansi.StringWidth(s.printable(r.shortName())))
	}
	return widths
}

func (s Screen) rowText(r row, nameWidth int) (name, statusText string, color ansi.Color) {
	shortName := s.printable(r.shortName())
	statusText, color = s.statusText(r.service.Name)
	if statusText == "" {
		return " " + shortName, "", nil
	}
	return " " + shortName + strings.Repeat(" ", nameWidth-ansi.StringWidth(shortName)+2), statusText, color
}

func (s Screen) statusText(name string) (string, ansi.Color) {
	result := s.statuses[name]
	switch {
	case result.Failure != nil:
		return s.printable(s.failureRowText(result.Failure)), ansi.Red
	case result.Output != nil:
		return s.printable(result.Output.Summary), s.levelColor(result.Output.Level)
	}
	return "", nil
}

func (s Screen) failureRowText(failure *status.Failure) string {
	switch failure.Kind {
	case status.FailureKindError:
		return "status error"
	case status.FailureKindTimeout:
		return "status timeout"
	case status.FailureKindInvalid:
		return "status invalid"
	}
	return failure.Text
}

func (s Screen) levelColor(level status.Level) ansi.Color {
	switch level {
	case status.LevelWarn:
		return ansi.Yellow
	case status.LevelError:
		return ansi.Red
	}
	return nil
}

func (s Screen) textStyle(foreground, background ansi.Color) ansi.Style {
	style := ansi.NewStyle()
	if background != nil {
		style = style.BackgroundColor(background)
	}
	if foreground != nil {
		style = style.ForegroundColor(foreground)
	}
	return style
}

func (s Screen) selectedRow(r row, name, statusText string, color ansi.Color, width int) string {
	padding := strings.Repeat(" ", max(width-2-ansi.StringWidth(name+statusText), 0))
	line := s.textStyle(nil, ansi.BrightBlack).Styled(name + statusText + padding)
	if color != nil {
		line = s.textStyle(nil, ansi.BrightBlack).Styled(name) + s.textStyle(color, ansi.BrightBlack).Styled(statusText) + s.textStyle(nil, ansi.BrightBlack).Styled(padding)
	}
	return ansi.NewStyle().ForegroundColor(accent).Styled("▌") + s.rowGlyph(r.service) + line
}

func (s Screen) detailText(svc service.Service) []string {
	pid, uptime := "-", "-"
	if svc.PID != 0 {
		pid = strconv.Itoa(svc.PID)
	}
	if svc.Uptime != nil {
		uptime = s.durationText(*svc.Uptime)
	}
	text := []string{
		s.detailLine(s.stateGlyph(svc.State)+" "+svc.State.String(), "PID "+pid),
		s.detailLine("up "+uptime, "last exit "+s.lastExitText(svc.LastExit)),
	}
	if result, ok := s.statuses[svc.Name]; ok {
		text = append(text, "Status "+s.durationText(s.statusAge(result))+" ago")
		if result.Failure != nil {
			text = append(text, s.printable(result.Failure.Text))
			for _, line := range result.Failure.Lines {
				text = append(text, s.printable(line))
			}
		}
		if result.Output != nil {
			text = append(text, s.fieldLines(result.Output.Fields)...)
		}
	}
	return text
}

func (s Screen) fieldLines(fields []status.Field) []string {
	nameWidth := 0
	for _, field := range fields {
		nameWidth = max(nameWidth, ansi.StringWidth(s.printable(field.Name)))
	}
	var lines []string
	for _, field := range fields {
		name := s.printable(field.Name)
		lines = append(lines, name+strings.Repeat(" ", nameWidth-ansi.StringWidth(name)+2)+s.printable(field.Value))
	}
	return lines
}

func (s Screen) statusAge(result status.Result) time.Duration {
	return max(time.Since(result.Time()), 0)
}

func (s Screen) detailLine(left, right string) string {
	return left + strings.Repeat(" ", detailLeftWidth-ansi.StringWidth(left)) + right
}

func (s Screen) durationText(duration time.Duration) string {
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(duration.Hours()), int(duration.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(duration.Hours())/24, int(duration.Hours())%24)
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
