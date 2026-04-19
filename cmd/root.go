package cmd

import "github.com/spf13/cobra"

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "orc",
	Short: "Shell command orchestration tool.",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
