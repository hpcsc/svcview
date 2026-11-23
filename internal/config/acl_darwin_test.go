//go:build unit

package config

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func answerWith(offset int32, security []byte) []byte {
	answer := binary.NativeEndian.AppendUint32(nil, uint32(12+len(security)))
	answer = binary.NativeEndian.AppendUint32(answer, uint32(offset))
	answer = binary.NativeEndian.AppendUint32(answer, uint32(len(security)))
	return append(answer, security...)
}

func extendedSecurity(magic, count uint32, entries int) []byte {
	security := binary.NativeEndian.AppendUint32(nil, magic)
	security = append(security, make([]byte, 32)...)
	security = binary.NativeEndian.AppendUint32(security, count)
	security = binary.NativeEndian.AppendUint32(security, 0)
	return append(security, make([]byte, entries*24)...)
}

func TestEntriesInAnswer(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		t.Run("an answer with no extended security has 0 entries", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, nil))

			require.NoError(t, err)
			require.Equal(t, 0, entries)
		})

		t.Run("an answer whose ACL has 3 entries has 3 entries", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, extendedSecurity(0x012cc16d, 3, 3)))

			require.NoError(t, err)
			require.Equal(t, 3, entries)
		})

		t.Run("an answer whose ACL has no entries has 0 entries", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, extendedSecurity(0x012cc16d, 0, 0)))

			require.NoError(t, err)
			require.Equal(t, 0, entries)
		})

		t.Run("an answer whose extended security says that the file has no ACL has 0 entries", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, extendedSecurity(0x012cc16d, 0xffffffff, 0)))

			require.NoError(t, err)
			require.Equal(t, 0, entries)
		})

		t.Run("an extended security that does not start with the magic gives an error, not a count", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, extendedSecurity(0x012cc16e, 1, 1)))

			require.ErrorContains(t, err, "magic")
			require.Zero(t, entries)
		})

		t.Run("an extended security one byte shorter than its header gives an error, not a count", func(t *testing.T) {
			entries, err := entriesInAnswer(answerWith(8, extendedSecurity(0x012cc16d, 0, 0)[:43]))

			require.ErrorContains(t, err, "magic")
			require.Zero(t, entries)
		})

		t.Run("an extended security one byte shorter than its entry count says gives an error, not a count", func(t *testing.T) {
			security := extendedSecurity(0x012cc16d, 3, 3)

			entries, err := entriesInAnswer(answerWith(8, security[:len(security)-1]))

			require.ErrorContains(t, err, "3 entries")
			require.Zero(t, entries)
		})

		t.Run("an extended security that ends after the answer gives an error, not a count", func(t *testing.T) {
			answer := answerWith(8, extendedSecurity(0x012cc16d, 1, 1))

			entries, err := entriesInAnswer(answer[:len(answer)-1])

			require.ErrorContains(t, err, "outside the answer")
			require.Zero(t, entries)
		})

		t.Run("an extended security that starts before the end of the reference to it gives an error, not a count", func(t *testing.T) {
			for _, offset := range []int32{7, -100} {
				entries, err := entriesInAnswer(answerWith(offset, extendedSecurity(0x012cc16d, 1, 1)))

				require.ErrorContains(t, err, "outside the answer", offset)
				require.Zero(t, entries, offset)
			}
		})
	})
}
