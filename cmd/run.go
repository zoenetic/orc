package cmd

import "github.com/spf13/cobra"

var runCmd = &cobra.Command{
	Use:   "run [plan]",
	Short: "Execute a plan.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plan := ""
		if len(args) == 1 {
			plan = args[0]
		}
		verbose, _ := cmd.Flags().GetBool("verbose")
		var flags []string
		if verbose {
			flags = append(flags, "--verbose")
		}
		return Dispatch("run", plan, "", flags...)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	
}
