package orc

import "context"

type ValidateResult struct {
	Errors []error
}

func (rb *Runbook) Validate(ctx context.Context) *ValidateResult {
	_, _, err := rb.Stages()
	if err != nil {
		return &ValidateResult{
			Errors: []error{err},
		}
	}
	return &ValidateResult{}
}
