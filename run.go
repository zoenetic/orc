package orc

import (
	"context"
	"sync"
	"time"
)

type RunResult struct {
	Completed bool
	Tasks     map[string]TaskResult
	Outputs   map[string]any
	Errors    []error
}

func (rb *Runbook) Run(ctx context.Context, opts RunOptions) RunResult {
	startedAt := time.Now()

	stages, children, err := rb.Stages()
	if err != nil {
		return RunResult{
			Completed: false,
			Errors:    []error{err},
		}
	}

	if err := rb.prepare(); err != nil {
		return RunResult{
			Completed: false,
			Errors:    []error{err},
		}
	}

	states := make(map[*Task]*taskState, len(rb.tasks))
	for _, t := range rb.tasks {
		states[t] = &taskState{
			task:         t,
			pendingCount: len(t.dependencies),
		}
	}

	readyCh := make(chan *Task, len(rb.tasks))

	for _, t := range stages[0] {
		states[t].transitionTo(StatusReady)
		readyCh <- t
	}
	for _, t := range rb.tasks {
		if states[t].status != StatusReady {
			states[t].transitionTo(StatusBlocked)
		}
	}

	remaining := len(rb.tasks)

	var mu sync.Mutex
	var wg sync.WaitGroup

	concurrency := rb.concurrency
	if concurrency <= 0 {
		concurrency = len(rb.tasks)
	}
	sem := make(chan struct{}, concurrency)

	done := make(chan struct{})
	var doneOnce sync.Once
	closeDone := func() { doneOnce.Do(func() { close(done) }) }

	go func() {
		for {
			select {
			case t := <-readyCh:
				sem <- struct{}{}
				wg.Add(1)
				go func(t *Task) {
					defer wg.Done()
					defer func() { <-sem }()
					rb.executeTask(ctx, t, opts, states, children, readyCh, &mu, &remaining, closeDone)
				}(t)
			case <-done:
				return
			case <-ctx.Done():
				mu.Lock()
				for _, s := range states {
					if !s.status.IsTerminal() {
						s.transitionTo(StatusCancelled)
						remaining--
					}
				}
				if remaining <= 0 {
					closeDone()
				}
				mu.Unlock()
				return
			}
		}
	}()

	<-done
	wg.Wait()

	result := rb.collectResult(ctx, states)
	record := rb.buildRunRecord(opts.Plan, startedAt, time.Now(), states)
	if err := persistRecord(record); err != nil {
		result.Errors = append(result.Errors, err)
	}

	return result
}
