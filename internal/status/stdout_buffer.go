package status

import (
	"bytes"
	"errors"
)

const maxStdout = 1 << 20

type stdoutBuffer struct {
	data     []byte
	tooLarge bool
}

func (b *stdoutBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > maxStdout {
		b.tooLarge = true
		return 0, errors.New("the program printed more on stdout than svcview keeps")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *stdoutBuffer) firstLines() []string {
	var lines []string
	for line := range bytes.Lines(b.data) {
		if len(lines) == maxFailureLines {
			break
		}
		line = bytes.TrimSuffix(line, []byte("\n"))
		lines = append(lines, string(line[:min(len(line), maxLineBytes)]))
	}
	return lines
}
