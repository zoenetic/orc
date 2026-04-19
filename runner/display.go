package runner

import (
	"bytes"
	"strings"

	tea "charm.land/bubbletea/v2"
)

var Default = Display{}

type Display struct{}

type tuiWriter struct {
	program *tea.Program
	task    string
	buf     bytes.Buffer
}

func (w *tuiWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		w.program.Send(outputMsg{task: w.task, line: strings.TrimRight(line, "\n")})
	}
	return len(p), nil
}
