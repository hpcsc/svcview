package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type box struct {
	title  string
	right  string
	width  int
	height int
	text   []string
}

func (b box) textWidth() int {
	return max(b.width-4, 0)
}

func (b box) render() []string {
	lines := []string{b.top()}
	for i := range b.height - 2 {
		text := ""
		if i < len(b.text) {
			text = ansi.Truncate(b.text[i], b.textWidth(), "…")
		}
		lines = append(lines, "│ "+text+strings.Repeat(" ", b.textWidth()-ansi.StringWidth(text))+" │")
	}
	return append(lines, "╰"+strings.Repeat("─", max(b.width-2, 0))+"╯")
}

func (b box) top() string {
	right := ""
	if b.right != "" {
		right = " " + b.right + " "
	}
	left := ""
	if b.title != "" {
		left = " " + ansi.Truncate(b.title, max(b.width-4-ansi.StringWidth(right), 0), "…") + " "
	}
	rule := max(b.width-2-ansi.StringWidth(left)-ansi.StringWidth(right), 0)
	return "╭" + left + strings.Repeat("─", rule) + right + "╮"
}
