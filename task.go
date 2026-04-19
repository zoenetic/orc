package orc

import (
	"fmt"
	"time"
)

type Task struct {
	name         string
	doClauses    []*DoClause
	undoClauses  []*UndoClause
	dependencies []*Task
	env          []*EnvVar
}

func (t *Task) Name() string {
	return t.name
}

type taskOption interface {
	apply(*Task)
}

type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusBlocked
	StatusReady
	StatusRunning
	StatusSatisfied
	StatusSucceeded
	StatusConfirmFailed
	StatusSkipped
	StatusCancelled
	StatusFailed
)

type taskState struct {
	task         *Task
	status       TaskStatus
	pendingCount int
	started      time.Time
	finished     time.Time
	duration     time.Duration
	err          error
}

type TaskEvent struct {
	Task     *Task
	Status   TaskStatus
	Err      error
	Duration time.Duration
}

type TaskResult struct {
	Name     string
	Status   TaskStatus
	Err      error
	Duration time.Duration
	Started  time.Time
	Finished time.Time
}

func (r *Runbook) Task(name string, opts ...taskOption) *Task {
	if _, dup := r.tasks[name]; dup {
		panic(fmt.Sprintf("runbook: duplicate task name %q", name))
	}
	t := &Task{name: name}
	for _, opt := range opts {
		opt.apply(t)
	}
	if len(t.doClauses) == 0 {
		panic(fmt.Sprintf("runbook: task %q has no Do clauses", name))
	}
	r.tasks[name] = t
	return t
}

func (t *Task) Env(name, value string) *Task {
	t.env = append(t.env, &EnvVar{name: name, value: value})
	return t
}

func (s *taskState) transitionTo(next TaskStatus) {
	if !isLegalTransition(s.status, next) {
		panic(fmt.Sprintf("illegal status transition from %s to %s", s.status, next))
	}
	s.status = next
	switch next {
	case StatusRunning:
		s.started = time.Now()
	case StatusSatisfied, StatusSucceeded, StatusFailed, StatusSkipped, StatusCancelled:
		s.finished = time.Now()
	}
}

func isLegalTransition(from, to TaskStatus) bool {
	switch from {
	case StatusPending:
		return to == StatusBlocked || to == StatusReady || to == StatusCancelled
	case StatusBlocked:
		return to == StatusReady || to == StatusSkipped || to == StatusCancelled
	case StatusReady:
		return to == StatusRunning || to == StatusCancelled
	case StatusRunning:
		return to == StatusSucceeded || to == StatusFailed || to == StatusCancelled
	case StatusSatisfied, StatusSucceeded, StatusFailed, StatusSkipped, StatusCancelled:
		return false
	default:
		return false
	}
}

func (s TaskStatus) String() string {
	return [...]string{
		"pending", "blocked", "ready", "running", "skipped",
		"cancelled", "failed", "satisfied", "succeeded",
	}[s]
}

func (s TaskStatus) IsTerminal() bool {
	return s == StatusSatisfied || s == StatusSucceeded || s == StatusFailed || s == StatusSkipped || s == StatusCancelled
}

func (s TaskStatus) IsComplete() bool {
	return s == StatusSatisfied || s == StatusSucceeded
}
