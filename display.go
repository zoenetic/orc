package orc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// Display is the bubbletea-based TUI executor. Pass it to Main to enable the
// rich interactive run/preview/validate/rollback experience.
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

func (Display) Execute(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskStages, _, err := rb.Stages()
	if err != nil {
		fmt.Fprintf(os.Stderr, "orc: run: %v\n", err)
		return false
	}

	m := newModel(taskStages, opts.Verbose, cancel)
	p := tea.NewProgram(m)

	opts.OnEvent = func(e TaskEvent) {
		p.Send(taskEventMsg(e))
	}
	opts.TaskOutput = func(name string) io.Writer {
		return &tuiWriter{program: p, task: name}
	}

	var result RunResult
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

func (Display) Preview(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	result := rb.Preview(ctx, opts)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: preview: %v\n", e)
		}
		return false
	}
	printPlan(rb, result)
	return true
}

func (Display) Validate(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	result := rb.Validate(ctx)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: validate: %v\n", e)
		}
		return false
	}
	fmt.Println(dispDone.Render("✓") + " valid")
	return true
}

func (Display) Rollback(ctx context.Context, rb *Runbook, opts RunOptions, runID string) bool {
	result := rb.Rollback(ctx, opts, runID)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: rollback: %v\n", e)
		}
		return false
	}
	anyRan := false
	for _, tr := range result.Tasks {
		if tr.Status == StatusSucceeded {
			anyRan = true
			break
		}
	}
	if anyRan {
		fmt.Println(dispDone.Render("✓") + " rollback completed")
	} else {
		fmt.Println(dispMeta.Render("—") + " no undo actions defined")
	}
	return true
}
