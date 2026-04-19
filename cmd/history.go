package cmd

import (
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/zoenetic/orc"
)

var (
	histOK   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Green().Hex))
	histFail = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Red().Hex))
	histMeta = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex))
	histPlan = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Lavender().Hex))
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent run history.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("last")
		runs, err := orc.LoadHistory(n)
		if err != nil {
			return err
		}
		if len(runs) == 0 {
			fmt.Println(histMeta.Render("  no run history found"))
			return nil
		}
		printHistory(runs)
		return nil
	},
}

func init() {
	historyCmd.Flags().IntP("last", "n", 20, "Number of recent runs to show")
	rootCmd.AddCommand(historyCmd)
}

func printHistory(runs []orc.RunRecord) {
	fmt.Fprintln(os.Stdout)
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]

		var icon string
		var statusStyle lipgloss.Style

		switch r.Status {
		case orc.RunSucceeded:
			icon = histOK.Render("✓")
			statusStyle = histOK

		case orc.RunFailed:
			icon = histFail.Render("✗")
			statusStyle = histFail

		case orc.RunConfirmFailed:
			icon = histFail.Render("!")
			statusStyle = histFail

		case orc.RunCancelled:
			icon = histFail.Render("x")
			statusStyle = histFail

		default:
			icon = histMeta.Render("?")
			statusStyle = histMeta
		}

		ts := histMeta.Render(r.Started.Format("2006-01-02 15:04"))
		plan := histPlan.Render(r.Plan)
		status := statusStyle.Render(string(r.Status))
		dur := histMeta.Render(fmtDuration(r.Duration))
		commit := ""
		if r.Commit != "" {
			commit = histMeta.Render(r.Commit)
		}
		id := histMeta.Render(r.ID)

		fmt.Printf("  %s  %s  %s  %s  %s  %s  %s\n", icon, ts, plan, status, dur, commit, id)
	}
	fmt.Fprintln(os.Stdout)
}
