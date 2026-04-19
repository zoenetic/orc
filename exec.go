package orc

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

type headlessExecutor struct{}

func (e headlessExecutor) Execute(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	result := rb.Run(ctx, opts)
	if !result.Completed {
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: run: %v\n", err)
		}
		return false
	}
	return true
}

func (e headlessExecutor) Preview(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	result := rb.Preview(ctx, opts)
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: preview: %v\n", err)
		}
		return false
	}

	fmt.Printf("\n  %s  %d tasks · %d stages\n\n", rb.name, len(result.Tasks), len(result.Stages))
	for i, stage := range result.Stages {
		for _, t := range stage {
			fmt.Printf("  [%d] %s\n", i+1, t.name)
		}
	}
	fmt.Println()
	return true
}

func (e headlessExecutor) Validate(ctx context.Context, rb *Runbook, opts RunOptions) bool {
	result := rb.Validate(ctx)
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: validate: %v\n", err)
		}
		return false
	}
	fmt.Println("ok")
	return true
}

func (e headlessExecutor) Rollback(ctx context.Context, rb *Runbook, opts RunOptions, runID string) bool {
	result := rb.Rollback(ctx, opts, runID)
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: rollback: %v\n", err)
		}
		return false
	}
	return true
}

type RunStatus string

const (
	RunSucceeded     RunStatus = "succeeded"
	RunFailed        RunStatus = "failed"
	RunConfirmFailed RunStatus = "confirm_failed"
	RunCancelled     RunStatus = "cancelled"
)

type RunOptions struct {
	Plan       string
	Verbose    bool
	Env        []*EnvVar
	TaskOutput func(taskName string) io.Writer
	OnEvent    func(event TaskEvent)
}

type Executor interface {
	Execute(ctx context.Context, rb *Runbook, opts RunOptions) bool
	Preview(ctx context.Context, rb *Runbook, opts RunOptions) bool
	Validate(ctx context.Context, rb *Runbook, opts RunOptions) bool
	Rollback(ctx context.Context, rb *Runbook, opts RunOptions, runID string) bool
}

func (rb *Runbook) executeTask(
	ctx context.Context,
	t *Task,
	opts RunOptions,
	states map[*Task]*taskState,
	children map[*Task][]*Task,
	readyCh chan<- *Task,
	mu *sync.Mutex,
	remaining *int,
	closeDone func(),
) {
	state := states[t]
	emit := func(status TaskStatus, err error) {
		if opts.OnEvent != nil {
			d := state.finished.Sub(state.started)
			opts.OnEvent(TaskEvent{
				Task:     t,
				Status:   status,
				Err:      err,
				Duration: d,
			})
		}
	}

	if ctx.Err() != nil {
		mu.Lock()
		if !state.status.IsTerminal() {
			state.transitionTo(StatusCancelled)
			*remaining--
			if *remaining == 0 {
				closeDone()
			}
		}
		mu.Unlock()
		return
	}

	mu.Lock()
	state.transitionTo(StatusRunning)
	mu.Unlock()
	emit(StatusRunning, nil)

	var out io.Writer
	if opts.TaskOutput != nil {
		out = opts.TaskOutput(t.name)
	}

	allSatisfied := true
	for _, clause := range t.doClauses {
		run, err := clause.shouldRun(ctx)
		if err != nil {
			rb.failTask(state, err, states, children, mu, remaining, closeDone)
			emit(StatusFailed, err)
			return
		}
		if !run {
			continue
		}
		allSatisfied = false
		for _, cmd := range clause.cmds {
			if err := cmd.execute(ctx, out); err != nil {
				rb.failTask(state, fmt.Errorf("execute command %q: %w", cmd.String(), err), states, children, mu, remaining, closeDone)
				emit(StatusFailed, err)
				return
			}
		}
	}

	mu.Lock()
	if allSatisfied {
		state.transitionTo(StatusSatisfied)
	} else {
		state.transitionTo(StatusSucceeded)
	}
	rb.completeTask(t, states, children, readyCh, remaining, closeDone)
	mu.Unlock()
	emit(StatusSucceeded, nil)
}

func (rb *Runbook) completeTask(
	t *Task,
	states map[*Task]*taskState,
	children map[*Task][]*Task,
	readyCh chan<- *Task,
	remaining *int,
	closeDone func(),
) {
	*remaining--
	if *remaining == 0 {
		closeDone()
		return
	}

	for _, child := range children[t] {
		cs := states[child]
		cs.pendingCount--
		if cs.pendingCount == 0 {
			cs.transitionTo(StatusReady)
			readyCh <- child
		}
	}
}

func (rb *Runbook) failTask(
	state *taskState,
	err error,
	states map[*Task]*taskState,
	children map[*Task][]*Task,
	mu *sync.Mutex,
	remaining *int,
	closeDone func(),
) {
	mu.Lock()
	defer mu.Unlock()

	state.transitionTo(StatusFailed)
	state.err = err
	*remaining--

	for _, desc := range rb.descendantsOf(state.task, children) {
		ds := states[desc]
		if !ds.status.IsTerminal() {
			ds.transitionTo(StatusSkipped)
			*remaining--
		}
	}

	if *remaining == 0 {
		closeDone()
	}
}

func (rb *Runbook) collectResult(ctx context.Context, states map[*Task]*taskState) RunResult {
	result := RunResult{
		Completed: true,
		Tasks:     make(map[string]TaskResult, len(states)),
		Outputs:   make(map[string]any),
		Errors:    []error{},
	}

	for _, s := range states {
		tr := TaskResult{
			Name:     s.task.name,
			Status:   s.status,
			Err:      s.err,
			Started:  s.started,
			Finished: s.finished,
		}
		if !s.started.IsZero() && !s.finished.IsZero() {
			tr.Duration = s.finished.Sub(s.started)
		}

		if !s.status.IsTerminal() {
			tr.Status = StatusCancelled
		}

		if s.status != StatusSatisfied && s.status != StatusSucceeded {
			result.Completed = false
		}

		result.Tasks[tr.Name] = tr
	}

	if ctx.Err() != nil {
		result.Completed = false
		result.Errors = append(result.Errors, ctx.Err())
	}

	for _, tr := range result.Tasks {
		if tr.Err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("task %s: %w", tr.Name, tr.Err))
		}
	}

	return result
}
