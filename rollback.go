package orc

import (
	"context"
	"fmt"
	"io"
)

type RollbackResult struct {
	Completed bool
	Tasks     map[string]TaskResult
	Errors    []error
}

type rollbackOutcome int

const (
	rollbackNone    rollbackOutcome = iota // no undo clauses defined
	rollbackSkipped                        // undo defined but shouldRun returned false
	rollbackRan                            // undo commands executed
)

func (rb *Runbook) Rollback(ctx context.Context, opts RunOptions, runID string) RollbackResult {
	var record *RunRecord
	if runID == "" {
		var err error
		record, err = LoadRecord()
		if err != nil {
			return RollbackResult{Completed: false, Errors: []error{err}}
		}
	} else {
		var err error
		record, err = LoadRecordByID(runID)
		if err != nil {
			return RollbackResult{Completed: false, Errors: []error{err}}
		}
	}

	if record == nil {
		return RollbackResult{Completed: false, Errors: []error{fmt.Errorf("no run record found")}}
	}

	if record.Runbook != rb.name {
		return RollbackResult{
			Completed: false,
			Errors:    []error{fmt.Errorf("run %s was for a different runbook", record.Runbook)},
		}
	}
	if record.Plan != opts.Plan {
		return RollbackResult{
			Completed: false,
			Errors:    []error{fmt.Errorf("run %s was for a different plan", record.Plan)},
		}
	}

	taskStages, _, err := rb.Stages()
	if err != nil {
		return RollbackResult{Completed: false, Errors: []error{err}}
	}

	if err := rb.prepare(); err != nil {
		return RollbackResult{Completed: false, Errors: []error{err}}
	}

	for i, j := 0, len(taskStages)-1; i < j; i, j = i+1, j-1 {
		taskStages[i], taskStages[j] = taskStages[j], taskStages[i]
	}

	result := RollbackResult{
		Completed: true,
		Tasks:     make(map[string]TaskResult, len(rb.tasks)),
		Errors:    []error{},
	}

	for _, stage := range taskStages {
		for _, t := range stage {
			outcome, err := rb.rollbackTask(ctx, t, opts)
			tr := TaskResult{Name: t.name}
			switch outcome {
			case rollbackRan:
				tr.Status = StatusSucceeded
			case rollbackSkipped:
				tr.Status = StatusSkipped
			case rollbackNone:
				tr.Status = StatusSatisfied
			}
			if err != nil {
				tr.Status = StatusFailed
				tr.Err = err
				result.Completed = false
				result.Errors = append(result.Errors, fmt.Errorf("rollback %s: %w", t.name, err))
			}
			result.Tasks[t.name] = tr
		}
	}

	return result
}

func (rb *Runbook) rollbackTask(
	ctx context.Context,
	t *Task,
	opts RunOptions,
) (rollbackOutcome, error) {
	if len(t.undoClauses) == 0 {
		return rollbackNone, nil
	}

	var out io.Writer
	if opts.TaskOutput != nil {
		out = opts.TaskOutput(t.name)
	}

	anyRan := false
	for _, clause := range t.undoClauses {
		run, err := clause.shouldRun(ctx)
		if err != nil {
			return rollbackNone, err
		}
		if !run {
			continue
		}
		for _, cmd := range clause.cmds {
			if err := cmd.execute(ctx, out); err != nil {
				return rollbackNone, err
			}
			anyRan = true
		}
		for _, cmd := range clause.confirmCmds {
			ok, err := cmd.checkSatisfied(ctx)
			if err != nil {
				return rollbackNone, err
			}
			if !ok {
				return rollbackNone, fmt.Errorf("undo confirmation failed: %s", cmd.String())
			}
		}
	}

	if anyRan {
		return rollbackRan, nil
	}
	return rollbackSkipped, nil
}
