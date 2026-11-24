package terminal

import (
	"strings"
	"unicode"
)

func ReplaceControlCharacters(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, text)
}
