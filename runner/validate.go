package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/zoenetic/orc"
)

func (Display) Validate(
	ctx context.Context,
	rb *orc.Runbook,
	opts orc.RunOptions,
) bool {
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
