package cmd

import "github.com/spf13/cobra"

var validateCmd = &cobra.Command{
	Use:   "validate [plan]",
	Short: "Validate a plan without executing it.",
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
		return Dispatch("validate", plan, "", flags...)
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
