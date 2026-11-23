package status

import (
	"bytes"
	"slices"
)

const (
	maxFailureLines = 20
	maxLineBytes    = 4096
)

type lastLines struct {
	finished   []string
	unfinished []byte
}

func (l *lastLines) Write(p []byte) (int, error) {
	written := len(p)
	for {
		line, rest, found := bytes.Cut(p, []byte("\n"))
		l.unfinished = append(l.unfinished, line[:min(len(line), maxLineBytes-len(l.unfinished))]...)
		if !found {
			return written, nil
		}
		l.finished = append(l.finished, string(l.unfinished))
		l.finished = l.finished[max(len(l.finished)-maxFailureLines, 0):]
		l.unfinished = l.unfinished[:0]
		p = rest
	}
}

func (l *lastLines) lines() []string {
	lines := l.finished
	if len(l.unfinished) > 0 {
		lines = append(slices.Clip(lines), string(l.unfinished))
	}
	return lines[max(len(lines)-maxFailureLines, 0):]
}
