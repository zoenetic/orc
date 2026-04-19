package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/zoenetic/orc"
)

func (Display) Execute(
	ctx context.Context,
	rb *orc.Runbook,
	opts orc.RunOptions,
) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskStages, _, err := rb.Stages()
	if err != nil {
		fmt.Fprintf(os.Stderr, "orc: run: %v\n", err)
		return false
	}

	m := newModel(taskStages, opts.Verbose, cancel)
	p := tea.NewProgram(m)

	opts.OnEvent = func(e orc.TaskEvent) {
		p.Send(taskEventMsg(e))
	}

	opts.TaskOutput = func(name string) io.Writer {
		return &tuiWriter{program: p, task: name}
	}

	var result orc.RunResult
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result = rb.Run(ctx, opts)
		p.Send(doneMsg{})
	}()

	if _, err := p.Run(); err != nil {
		cancel()
		wg.Wait()
		fmt.Fprintf(os.Stderr, "orc: %v\n", err)
		return false
	}

	wg.Wait()
	return result.Completed
}
