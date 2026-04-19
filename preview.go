package orc

import "context"

type PreviewResult struct {
	Stages [][]*Task
	Tasks  map[string]PreviewTaskResult
	Errors []error
}

type PreviewTaskResult struct {
	Status TaskStatus
	Err    error
}

func (rb *Runbook) Preview(ctx context.Context, opts RunOptions) *PreviewResult {
	stages, _, err := rb.Stages()
	if err != nil {
		return &PreviewResult{Errors: []error{err}}
	}
	tasks := make(map[string]PreviewTaskResult, len(rb.tasks))
	for _, stage := range stages {
		for _, t := range stage {
			tasks[t.name] = rb.previewTask(ctx, t, tasks)
		}
	}

	result := PreviewResult{
		Stages: stages,
		Tasks:  tasks,
		Errors: []error{},
	}
	for _, tr := range tasks {
		if tr.Err != nil {
			result.Errors = append(result.Errors, tr.Err)
		}
	}
	return &result
}
