//go:build unit

package terminal_test

import (
	"testing"

	"github.com/hpcsc/svcview/internal/terminal"
	"github.com/stretchr/testify/require"
)

func TestReplaceControlCharacters(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		t.Run("shows each control character as �, so a text cannot send a sequence to the terminal or start a line", func(t *testing.T) {
			require.Equal(t, "a�[2Jb�c�d�e�]52;c;eA==�", terminal.ReplaceControlCharacters("a\x1b[2Jb\nc\td\x7fe\u009b]52;c;eA==\a"))
		})

		t.Run("keeps each other character, also a character that is not ASCII and a %", func(t *testing.T) {
			require.Equal(t, "Wörker · 12m %s ▰▱ ✓", terminal.ReplaceControlCharacters("Wörker · 12m %s ▰▱ ✓"))
		})
	})
}
