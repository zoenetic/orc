package orc

import (
	"fmt"
	"time"
)

type Task struct {
	name  string
	dos   []Do
	undos []Undo
	deps  []*Task
	env   []*EnvVar
}

func (t *Task) DoClauses() []Do {
	return t.dos
}

func (t *Task) UndoClauses() []Undo {
	return t.undos
}

func (t *Task) Name() string {
	return t.name
}

type TaskStatus string

const (
	StatusPending       TaskStatus = "pending"
	StatusBlocked       TaskStatus = "blocked"
	StatusReady         TaskStatus = "ready"
	StatusRunning       TaskStatus = "running"
	StatusSatisfied     TaskStatus = "satisfied"
	StatusSucceeded     TaskStatus = "succeeded"
	StatusConfirmFailed TaskStatus = "confirm_failed"
	StatusSkipped       TaskStatus = "skipped"
	StatusCancelled     TaskStatus = "cancelled"
	StatusFailed        TaskStatus = "failed"
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

type TaskOption interface {
	isTaskOption()
}

func (r *Runbook) Task(name string, opts ...TaskOption) *Task {
	if _, dup := r.tasks[name]; dup {
		panic(fmt.Sprintf("runbook: duplicate task name %q", name))
	}
	t := &Task{name: name}

	for _, opt := range opts {
		switch v := opt.(type) {
		case Do:
			t.dos = append(t.dos, v)
		case Undo:
			t.undos = append(t.undos, v)
		case DependsOn:
			for _, d := range v {
				t.deps = append(t.deps, d)
			}
		default:
			panic(fmt.Sprintf("runbook: unknown task option type %T", opt))
		}
	}

	if len(t.dos) == 0 {
		panic(fmt.Sprintf("runbook: task %q has no Do clauses", name))
	}
	r.tasks[name] = t
	return t
}

// func (r *Runbook) Task(name string, opts ...taskOption) *Task {
// 	if _, dup := r.tasks[name]; dup {
// 		panic(fmt.Sprintf("runbook: duplicate task name %q", name))
// 	}
// 	t := &Task{name: name}
// 	for _, opt := range opts {
// 		opt.apply(t)
// 	}
// 	if len(t.doClauses) == 0 {
// 		panic(fmt.Sprintf("runbook: task %q has no Do clauses", name))
// 	}
// 	r.tasks[name] = t
// 	return t
// }

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
		return to == StatusSatisfied || to == StatusSucceeded || to == StatusFailed || to == StatusCancelled
	case StatusSatisfied, StatusSucceeded, StatusFailed, StatusSkipped, StatusCancelled:
		return false
	default:
		return false
	}
}

func (s TaskStatus) IsTerminal() bool {
	return s == StatusSatisfied || s == StatusSucceeded || s == StatusFailed || s == StatusSkipped || s == StatusCancelled
}

func (s TaskStatus) IsComplete() bool {
	return s == StatusSatisfied || s == StatusSucceeded
}
