package orc

import "context"

type ValidateResult struct {
	Errors []error
}

func (rb *Runbook) Validate(ctx context.Context) *ValidateResult {
	_, _, err := rb.stages()
	if err != nil {
		return &ValidateResult{
			Errors: []error{err},
		}
	}
	return &ValidateResult{}
}
