package tui

import (
	"cmp"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hpcsc/svcview/internal/config"
	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
	"github.com/hpcsc/svcview/internal/terminal"
)

const (
	headerHeight    = 3
	accent          = ansi.Blue
	detailLeftWidth = 14
	helpKeyWidth    = 10
	barCells        = 10
)

type refresh struct {
	readsStarted int
}

type servicesRead struct {
	rows []row
	err  error
	all  bool
}

type runEnded struct {
	name   string
	result status.Result
}

type statusTick struct {
	group config.Group
}

type launchdActionEnded struct {
	name string
	err  error
}

type programEnded struct {
	commandLine []string
	err         error
}

type confirmation struct {
	question string
	yes      func(Screen) tea.Cmd
}

func newConfirmation(ask string, svc service.Service, yes func(Screen) tea.Cmd) *confirmation {
	return &confirmation{question: ask + " " + terminal.ReplaceControlCharacters(svc.Name) + "? (y/N)", yes: yes}
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

func (r row) nameContains(filter string) bool {
	return strings.Contains(strings.ToLower(r.service.Name), strings.ToLower(filter))
}

type pane int

const (
	listPane pane = iota
	detailPane
)

func (p pane) other() pane {
	if p == listPane {
		return detailPane
	}
	return listPane
}

type Screen struct {
	domain                string
	readWithPlist         func() ([]service.Service, error)
	readAll               func() ([]service.Service, error)
	all                   bool
	runStatus             func(status.Command, service.Service) status.Result
	start                 func(service.Service) error
	stop                  func(service.Service) error
	restart               func(service.Service) error
	settings              config.Settings
	groupOrder            []config.Group
	lastRead              servicesRead
	hasRead               bool
	readNotEnded          bool
	readsStarted          int
	statuses              map[string]status.Result
	runNotEnded           map[string]bool
	launchdActionNotEnded map[string]bool
	message               string
	confirmation          *confirmation
	filter                string
	filterLineOpen        bool
	selected              string
	listTop               int
	detailTop             int
	focus                 pane
	showHelp              bool
	width                 int
	height                int
}

func NewScreen(domain string, readWithPlist, readAll func() ([]service.Service, error), all bool, runStatus func(status.Command, service.Service) status.Result, start, stop, restart func(service.Service) error, settings config.Settings) Screen {
	return Screen{
		domain:                domain,
		readWithPlist:         readWithPlist,
		readAll:               readAll,
		all:                   all,
		runStatus:             runStatus,
		start:                 start,
		stop:                  stop,
		restart:               restart,
		settings:              settings,
		groupOrder:            slices.Concat(settings.Groups, []config.Group{config.Other}),
		readNotEnded:          true,
		statuses:              map[string]status.Result{},
		runNotEnded:           map[string]bool{},
		launchdActionNotEnded: map[string]bool{},
	}
}

func (s Screen) Init() tea.Cmd {
	return s.readServices
}

func (s Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	selected := s.selected
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch {
		case s.confirmation != nil:
			s, cmd = s.answer(msg)
		case s.filterLineOpen:
			s, cmd = s.pressInFilterLine(msg)
		default:
			s, cmd = s.press(msg)
		}
	case servicesRead:
		if msg.all == s.all {
			s, cmd = s.showRead(msg)
		} else {
			s, cmd = s.startRead()
		}
	case statusTick:
		cmd = tea.Batch(s.runStatuses(msg.group), s.nextStatusTick(msg.group))
	case runEnded:
		delete(s.runNotEnded, msg.name)
		s.statuses[msg.name] = msg.result
	case launchdActionEnded:
		delete(s.launchdActionNotEnded, msg.name)
		if msg.err != nil {
			s.message = terminal.ReplaceControlCharacters(strings.ReplaceAll(msg.err.Error(), "\n", " "))
		}
	case programEnded:
		if msg.err != nil {
			s.message = terminal.ReplaceControlCharacters(s.programFailureText(msg))
		}
	case refresh:
		if msg.readsStarted == s.readsStarted {
			s, cmd = s.startRead()
		}
	}
	s.selected = s.selectedInList()
	if s.selected != selected {
		s.detailTop = 0
	}
	s.detailTop = s.detailTopInView()
	s.listTop = s.listTopInView()
	return s, cmd
}

func (s Screen) press(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	s.message = ""
	switch key.String() {
	case "q", "ctrl+c":
		return s, tea.Quit
	case "?":
		s.showHelp = !s.showHelp
	case "esc":
		if s.showHelp {
			s.showHelp = false
		} else {
			s.filter = ""
		}
	case "tab":
		if !s.showHelp {
			s.focus = s.focus.other()
		}
	case "l":
		if !s.showHelp {
			s.focus = detailPane
		}
	case "h":
		if !s.showHelp {
			s.focus = listPane
		}
	case "/":
		if !s.showHelp {
			s.filterLineOpen, s.focus = true, listPane
		}
	case "a":
		if !s.showHelp {
			s.all = !s.all
			if !s.readNotEnded {
				return s.startRead()
			}
		}
	case "s":
		if svc, ok := s.serviceForLaunchdAction(); ok && svc.State != service.Running {
			return s, s.beginLaunchdAction(svc, s.start)
		}
	case "x":
		if svc, ok := s.serviceForLaunchdAction(); ok && svc.State != service.NotLoaded {
			s.confirmation = newConfirmation("Stop", svc, func(screen Screen) tea.Cmd { return screen.beginLaunchdAction(svc, screen.stop) })
		}
	case "r":
		svc, ok := s.serviceForLaunchdAction()
		switch {
		case !ok:
		case svc.State == service.NotLoaded:
			s.message = terminal.ReplaceControlCharacters(svc.Name) + " is not loaded, so svcview cannot restart it"
		default:
			s.confirmation = newConfirmation("Restart", svc, func(screen Screen) tea.Cmd { return screen.beginLaunchdAction(svc, screen.restart) })
		}
	case "o":
		svc, ok := s.serviceForKeys()
		switch {
		case !ok:
		case svc.LogPath() == "":
			s.message = terminal.ReplaceControlCharacters(svc.Name) + " has no log path in its plist"
		default:
			return s, s.giveTerminalTo([]string{"less", "+F", "--", svc.LogPath()})
		}
	case "up", "k", "down", "j":
		step := 1
		if key.String() == "up" || key.String() == "k" {
			step = -1
		}
		switch {
		case s.showHelp:
		case s.focus == detailPane:
			s.detailTop += step
		default:
			s.selected = s.selectedAfterMove(step)
		}
	default:
		return s.runAction(key.String())
	}
	return s, nil
}

func (s Screen) runAction(key string) (Screen, tea.Cmd) {
	svc, ok := s.serviceForKeys()
	if !ok {
		return s, nil
	}
	actions := s.selectedActions()
	at := slices.IndexFunc(actions, func(action config.Action) bool { return action.Key == key })
	if at < 0 {
		return s, nil
	}
	commandLine, failure := actions[at].Command.Apply(svc)
	switch {
	case failure != nil:
		s.message = terminal.ReplaceControlCharacters(failure.Text)
	case actions[at].Confirm:
		ask := "Run " + terminal.ReplaceControlCharacters(actions[at].Name) + " for"
		s.confirmation = newConfirmation(ask, svc, func(screen Screen) tea.Cmd { return screen.giveTerminalTo(commandLine) })
	default:
		return s, s.giveTerminalTo(commandLine)
	}
	return s, nil
}

func (s Screen) answer(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	answered := *s.confirmation
	s.confirmation = nil
	switch key.String() {
	case "ctrl+c":
		return s, tea.Quit
	case "y":
		return s, answered.yes(s)
	}
	return s, nil
}

func (s Screen) pressInFilterLine(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	s.message = ""
	switch key.String() {
	case "ctrl+c":
		return s, tea.Quit
	case "esc":
		s.filter, s.filterLineOpen = "", false
	case "enter":
		s.filterLineOpen = false
	case "backspace":
		_, size := utf8.DecodeLastRuneInString(s.filter)
		s.filter = s.filter[:len(s.filter)-size]
	default:
		s.filter += key.Text
	}
	return s, nil
}

func (s Screen) View() tea.View {
	view := tea.NewView(s.content())
	view.AltScreen = true
	return view
}

func (s Screen) startRead() (Screen, tea.Cmd) {
	s.readNotEnded = true
	s.readsStarted++
	return s, s.readServices
}

func (s Screen) showRead(read servicesRead) (Screen, tea.Cmd) {
	firstRead := !s.hasRead
	s.lastRead, s.hasRead, s.readNotEnded = read, true, false
	readsStarted := s.readsStarted
	cmd := tea.Tick(s.settings.Refresh, func(time.Time) tea.Msg { return refresh{readsStarted: readsStarted} })
	if firstRead {
		cmd = tea.Batch(cmd, s.startStatusRuns())
	}
	return s, cmd
}

func (s Screen) readServices() tea.Msg {
	read := s.readWithPlist
	if s.all {
		read = s.readAll
	}
	services, err := read()
	rows := make([]row, 0, len(services))
	for _, svc := range services {
		rows = append(rows, s.rowOf(svc))
	}
	slices.SortFunc(rows, row.compare)
	return servicesRead{rows: rows, err: err, all: s.all}
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
	for _, r := range s.rowsOfRead() {
		if r.group.Name == group.Name && !s.runNotEnded[r.service.Name] {
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

func (s Screen) beginLaunchdAction(svc service.Service, launchdAction func(service.Service) error) tea.Cmd {
	if s.launchdActionNotEnded[svc.Name] {
		return nil
	}
	s.launchdActionNotEnded[svc.Name] = true
	return func() tea.Msg {
		return launchdActionEnded{name: svc.Name, err: launchdAction(svc)}
	}
}

func (s Screen) giveTerminalTo(commandLine []string) tea.Cmd {
	return tea.ExecProcess(exec.Command(commandLine[0], commandLine[1:]...), func(err error) tea.Msg {
		return programEnded{commandLine: commandLine, err: err}
	})
}

func (s Screen) programFailureText(ended programEnded) string {
	text := strings.Join(ended.commandLine, " ")
	exited, ok := errors.AsType[*exec.ExitError](ended.err)
	if !ok {
		return fmt.Sprintf("run %s: %s", text, ended.err)
	}
	if wait, _ := exited.Sys().(syscall.WaitStatus); wait.Signaled() {
		return fmt.Sprintf("%s ended by %s", text, service.SignalName(int(wait.Signal())))
	}
	return fmt.Sprintf("%s exited with code %d", text, exited.ExitCode())
}

func (s Screen) rowOf(svc service.Service) row {
	group := s.settings.GroupOf(svc.Name)
	return row{service: svc, group: group, groupIndex: slices.IndexFunc(s.groupOrder, func(other config.Group) bool { return other.Name == group.Name })}
}

func (s Screen) rowsOfRead() []row {
	if s.lastRead.err != nil {
		return nil
	}
	return s.lastRead.rows
}

func (s Screen) rowsInList() []row {
	var rows []row
	for _, r := range s.rowsOfRead() {
		if r.nameContains(s.filter) {
			rows = append(rows, r)
		}
	}
	return rows
}

func (s Screen) selectedInList() string {
	rows := s.rowsInList()
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
	rows := s.rowsInList()
	return rows[min(max(at+step, 0), len(rows)-1)].service.Name
}

func (s Screen) listTopInView() int {
	at := s.selectedAt()
	if at < 0 {
		return 0
	}
	rows := s.rowsInList()
	line := s.lineOf(rows, at)
	shown := max(s.paneHeight()-2, 1)
	top := min(max(s.listTop, line-shown+1), line)
	if s.startsGroup(rows, at) {
		top = min(top, max(line-1, line-shown+1))
	}
	return max(min(top, s.lineOf(rows, len(rows)-1)+1-shown), 0)
}

func (s Screen) detailTopInView() int {
	shown := max(s.paneHeight()-2, 0)
	return max(min(s.detailTop, len(s.selectedDetailText())-shown), 0)
}

func (s Screen) selectedService() (service.Service, bool) {
	at := s.selectedAt()
	if at < 0 {
		return service.Service{}, false
	}
	return s.rowsInList()[at].service, true
}

func (s Screen) selectedActions() []config.Action {
	at := s.selectedAt()
	if at < 0 {
		return nil
	}
	return s.rowsInList()[at].group.Actions
}

func (s Screen) serviceForKeys() (service.Service, bool) {
	svc, ok := s.selectedService()
	return svc, ok && !s.showHelp
}

func (s Screen) serviceForLaunchdAction() (service.Service, bool) {
	svc, ok := s.serviceForKeys()
	return svc, ok && !s.launchdActionNotEnded[svc.Name]
}

func (s Screen) selectedAt() int {
	return slices.IndexFunc(s.rowsInList(), func(r row) bool { return r.service.Name == s.selected })
}

func (s Screen) startsGroup(rows []row, at int) bool {
	return at == 0 || rows[at].groupIndex != rows[at-1].groupIndex
}

func (s Screen) linesAbove(rows []row, at int) []string {
	if !s.startsGroup(rows, at) {
		return nil
	}
	groupLine := terminal.ReplaceControlCharacters(rows[at].group.Name)
	if at == 0 {
		return []string{groupLine}
	}
	return []string{"", groupLine}
}

func (s Screen) lineOf(rows []row, at int) int {
	line := at
	for i := range at + 1 {
		line += len(s.linesAbove(rows, i))
	}
	return line
}

func (s Screen) content() string {
	if s.width == 0 {
		return ""
	}
	header := box{title: "svcview", right: s.headerRight(), width: s.width, height: headerHeight, text: s.headerText()}
	lines := header.render()
	if s.showHelp {
		help := box{title: "Help", width: s.width, height: s.paneHeight(), text: s.helpText()}
		lines = append(lines, help.render()...)
	} else {
		lines = append(lines, s.panes()...)
	}
	lines = append(lines, s.filterLines()...)
	lines = append(lines, s.messageLines()...)
	return strings.Join(append(lines, ansi.Truncate(s.helpLine(), s.width, "")), "\n")
}

func (s Screen) paneHeight() int {
	return s.height - headerHeight - len(s.filterLines()) - len(s.messageLines()) - 1
}

func (s Screen) filterLines() []string {
	if !s.filterLineOpen && s.filter == "" {
		return nil
	}
	cursor := ""
	if s.filterLineOpen {
		cursor = ansi.NewStyle().Reverse(true).Styled(" ")
	}
	return []string{ansi.Truncate(" /"+terminal.ReplaceControlCharacters(s.filter)+cursor, s.width, "…")}
}

func (s Screen) messageLines() []string {
	text := s.message
	if s.confirmation != nil {
		text = s.confirmation.question
	}
	if text == "" {
		return nil
	}
	return []string{ansi.Truncate(" "+text, s.width, "…")}
}

func (s Screen) panes() []string {
	list := box{title: "Services", width: s.listWidth(), height: s.paneHeight(), borderStyle: s.borderStyle(listPane)}
	list.text = s.listText(list.textWidth())[s.listTop:]
	detail := box{width: s.detailWidth(), height: list.height, borderStyle: s.borderStyle(detailPane)}
	if at := s.selectedAt(); at >= 0 {
		detail.title, detail.text = terminal.ReplaceControlCharacters(s.rowsInList()[at].service.Name), s.selectedDetailText()[s.detailTop:]
	}
	var lines []string
	detailLines := detail.render()
	for i, line := range list.render() {
		lines = append(lines, line+detailLines[i])
	}
	return lines
}

func (s Screen) listWidth() int {
	return s.width * 40 / 100
}

func (s Screen) detailWidth() int {
	return s.width - s.listWidth()
}

func (s Screen) borderStyle(p pane) ansi.Style {
	if p == s.focus {
		return ansi.NewStyle().ForegroundColor(accent)
	}
	return ansi.NewStyle().Faint()
}

func (s Screen) helpLine() string {
	if s.filterLineOpen {
		return " enter keep the filter · esc clear the filter"
	}
	keys := []string{"s start", "x stop", "r restart", "o log"}
	for _, action := range s.selectedActions() {
		keys = append(keys, terminal.ReplaceControlCharacters(action.Key+" "+action.Name))
	}
	return " " + strings.Join(append(keys, "/ filter", "a all", "? help"), " · ")
}

func (s Screen) helpText() []string {
	text := []string{
		"↑ ↓ k j   select a service, or scroll the detail pane when it has the focus",
		"tab       move the focus to the other pane",
		"l         move the focus to the detail pane",
		"h         move the focus to the list",
		"/         filter the list by name",
		"enter     keep the filter",
		"backspace delete the last character of the filter",
		"a         show all services, or only the services with a plist",
		"s         start the selected service",
		"x         stop the selected service, after a confirmation",
		"r         restart the selected service, after a confirmation",
		"o         show the log of the selected service",
	}
	for _, action := range s.selectedActions() {
		key := terminal.ReplaceControlCharacters(action.Key)
		text = append(text, key+strings.Repeat(" ", helpKeyWidth-ansi.StringWidth(key))+terminal.ReplaceControlCharacters(action.Name))
	}
	return append(text,
		"?         show or hide this help",
		"esc       hide this help, or clear the filter",
		"q ctrl+c  quit",
	)
}

func (s Screen) headerRight() string {
	if s.lastRead.all {
		return s.domain + " · all"
	}
	return s.domain
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
			text = append(text, strings.Split(ansi.Wrap(terminal.ReplaceControlCharacters(line), width, ""), "\n")...)
		}
		return text
	}
	if s.hasRead && len(s.rowsInList()) == 0 && s.filter != "" {
		return []string{"no service matches"}
	}
	var text []string
	nameWidths := s.nameWidths()
	rows := s.rowsInList()
	for at, r := range rows {
		text = append(text, s.linesAbove(rows, at)...)
		firstCell, background := " ", ansi.Color(nil)
		if r.service.Name == s.selected {
			firstCell, background = ansi.NewStyle().ForegroundColor(accent).Styled("▌"), ansi.BrightBlack
		}
		text = append(text, firstCell+s.rowGlyph(r.service)+s.rowText(r, nameWidths[r.groupIndex], background, width-2))
	}
	return text
}

func (s Screen) nameWidths() map[int]int {
	widths := map[int]int{}
	for _, r := range s.rowsInList() {
		widths[r.groupIndex] = max(widths[r.groupIndex], ansi.StringWidth(terminal.ReplaceControlCharacters(r.shortName())))
	}
	return widths
}

func (s Screen) rowText(r row, nameWidth int, background ansi.Color, width int) string {
	shortName := terminal.ReplaceControlCharacters(r.shortName())
	name := " " + shortName
	statusText, color, mark := s.statusText(r)
	if statusText != "" || mark != "" {
		name += strings.Repeat(" ", nameWidth-ansi.StringWidth(shortName)+2)
	}
	statusStyle := s.textStyle(color, background)
	if statusText != "" && mark != "" {
		statusStyle, mark = statusStyle.Faint(), "  "+mark
		if room := width - ansi.StringWidth(name+mark); room > 0 {
			statusText = ansi.Truncate(statusText, room, "…")
		}
	}
	plain := s.textStyle(nil, background)
	padding := strings.Repeat(" ", max(width-ansi.StringWidth(name+statusText+mark), 0))
	if statusText == "" || slices.Equal(statusStyle, plain) {
		return plain.Styled(name + statusText + mark + padding)
	}
	return plain.Styled(name) + statusStyle.Styled(statusText) + plain.Styled(mark+padding)
}

func (s Screen) statusText(r row) (text string, color ansi.Color, mark string) {
	result := s.statuses[r.service.Name]
	switch {
	case result.Failure != nil:
		return terminal.ReplaceControlCharacters(s.failureRowText(result.Failure)), ansi.Red, ""
	case result.Output != nil:
		return terminal.ReplaceControlCharacters(result.Output.Summary), s.levelColor(result.Output.Level), s.staleMark(r, result)
	}
	return "", nil, ""
}

func (s Screen) staleMark(r row, result status.Result) string {
	if !result.Stale(r.group.Status.StaleAfter, time.Now()) {
		return ""
	}
	return "stale " + s.largestUnitText(s.statusAge(result))
}

func (s Screen) failureRowText(failure *status.Failure) string {
	if failure.Kind == status.FailureKindTemplateValue {
		return failure.Text
	}
	return failure.Kind.String()
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

func (s Screen) selectedDetailText() []string {
	at := s.selectedAt()
	if at < 0 {
		return nil
	}
	return s.detailText(s.rowsInList()[at].service, box{width: s.detailWidth()}.textWidth())
}

func (s Screen) detailText(svc service.Service, width int) []string {
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
			text = append(text, terminal.ReplaceControlCharacters(result.Failure.Text))
			for _, line := range result.Failure.Lines {
				text = append(text, terminal.ReplaceControlCharacters(line))
			}
		}
		if result.Output != nil {
			text = append(text, s.fieldLines(result.Output.Fields)...)
			text = append(text, s.eventLines(result.Output.Events)...)
		}
	}
	return append(text, s.plistLines(svc, width-detailLeftWidth)...)
}

func (s Screen) fieldLines(fields []status.Field) []string {
	nameWidth := 0
	for _, field := range fields {
		nameWidth = max(nameWidth, ansi.StringWidth(terminal.ReplaceControlCharacters(field.Name)))
	}
	valueColumn := nameWidth + 2
	var lines []string
	for _, field := range fields {
		name := terminal.ReplaceControlCharacters(field.Name)
		lines = append(lines, name+strings.Repeat(" ", valueColumn-ansi.StringWidth(name))+terminal.ReplaceControlCharacters(field.Value))
		if field.Progress != nil {
			lines = append(lines, strings.Repeat(" ", valueColumn)+s.bar(*field.Progress))
		}
	}
	return lines
}

func (s Screen) eventLines(events []status.Event) []string {
	if len(events) == 0 {
		return nil
	}
	lines := []string{"", "Events"}
	for _, event := range events {
		lines = append(lines, event.At.Local().Format("15:04")+" "+s.textStyle(s.levelColor(event.Level), nil).Styled("●")+" "+terminal.ReplaceControlCharacters(event.Text))
	}
	return lines
}

func (s Screen) plistLines(svc service.Service, room int) []string {
	if svc.PlistPath == "" {
		return nil
	}
	lines := append([]string{""}, s.partLines("Plist", svc.PlistPath, room)...)
	for _, property := range svc.Properties {
		if len(property.Parts) == 0 {
			lines = append(lines, property.Name)
		}
		name := property.Name
		for _, part := range property.Parts {
			lines = append(lines, s.partLines(name, part, room)...)
			name = ""
		}
	}
	return lines
}

func (s Screen) partLines(name, part string, room int) []string {
	var lines []string
	for line := range strings.SplitSeq(ansi.Hardwrap(terminal.ReplaceControlCharacters(part), room, true), "\n") {
		lines = append(lines, s.detailLine(name, line))
		name = ""
	}
	return lines
}

func (s Screen) bar(progress float64) string {
	done := int(progress * barCells)
	return ansi.NewStyle().ForegroundColor(accent).Styled(strings.Repeat("▰", done)) + ansi.NewStyle().Faint().Styled(strings.Repeat("▱", barCells-done))
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

func (s Screen) largestUnitText(duration time.Duration) string {
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours())/24)
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
