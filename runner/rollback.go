package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/zoenetic/orc"
)

func (Display) Rollback(
	ctx context.Context,
	rb *orc.Runbook,
	opts orc.RunOptions,
	runID string,
) bool {
	result := rb.Rollback(ctx, opts, runID)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: rollback: %v\n", e)
		}
		return false
	}
	fmt.Println(dispDone.Render("✓") + " rollback completed")
	return true
}
