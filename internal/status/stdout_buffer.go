package status

import "errors"

const maxStdout = 1 << 20

type stdoutBuffer struct {
	data []byte
}

func (b *stdoutBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > maxStdout {
		return 0, errors.New("the program printed more on stdout than svcview keeps")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
