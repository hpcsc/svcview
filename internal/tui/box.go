package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type box struct {
	title       string
	right       string
	width       int
	height      int
	text        []string
	borderStyle ansi.Style
}

func (b box) textWidth() int {
	return max(b.width-4, 0)
}

func (b box) render() []string {
	side := b.borderStyle.Styled("│")
	lines := []string{b.top()}
	for i := range b.height - 2 {
		text := ""
		if i < len(b.text) {
			text = ansi.Truncate(b.text[i], b.textWidth(), "…")
		}
		lines = append(lines, side+" "+text+strings.Repeat(" ", b.textWidth()-ansi.StringWidth(text))+" "+side)
	}
	return append(lines, b.borderStyle.Styled("╰"+strings.Repeat("─", max(b.width-2, 0))+"╯"))
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
	return b.borderStyle.Styled("╭") + left + b.borderStyle.Styled(strings.Repeat("─", rule)) + right + b.borderStyle.Styled("╮")
}
