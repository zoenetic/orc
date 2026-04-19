package cmd

import "github.com/spf13/cobra"

var rollbackCmd = &cobra.Command{
	Use:   "rollback [plan]",
	Short: "Execute a plan's undo commands.",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plan := ""
		runID := ""
		if len(args) >= 1 {
			plan = args[0]
		}
		if len(args) >= 2 {
			runID = args[1]
		}
		return Dispatch("rollback", plan, runID)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
