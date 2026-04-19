package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/zoenetic/orc"
)

var (
	previewTitle  = lipgloss.NewStyle().Bold(true)
	previewMeta   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Lavender().Hex))
	previewStage  = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex))
	previewReady  = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Green().Hex))
	previewDone   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Blue().Hex))
	previewSkip   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex))
	previewErr    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Red().Hex))
	previewDimmed = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex))
	previewLast   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex)).Italic(true)
)

func (Display) Preview(
	ctx context.Context,
	rb *orc.Runbook,
	opts orc.RunOptions,
) bool {
	result := rb.Preview(ctx, opts)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "orc: preview: %v\n", e)
		}
		return false
	}
	printPlan(rb, result)
	return true
}

func printPlan(rb *orc.Runbook, result *orc.PreviewResult) {
	last, _ := orc.LoadState()

	taskWord := "tasks"
	if len(result.Tasks) == 1 {
		taskWord = "task"
	}
	stageWord := "stages"
	if len(result.Stages) == 1 {
		stageWord = "stage"
	}

	fmt.Println()
	header := fmt.Sprintf("  %s  %s",
		previewTitle.Render(rb.Name()),
		previewMeta.Render(fmt.Sprintf("%d %s · %d %s",
			len(result.Tasks), taskWord,
			len(result.Stages), stageWord,
		)),
	)
	if last != nil {
		age := fmtAge(time.Since(last.Finished))
		header += "  " + previewMeta.Render(fmt.Sprintf("last run %s · %s", age, last.Status))
	}
	fmt.Println(header)
	fmt.Println()

	for i, stage := range result.Stages {
		label := fmt.Sprintf("Stage %d ", i+1)
		line := label + strings.Repeat("─", max(0, 44-len(label)))
		fmt.Printf("  %s\n", previewStage.Render(line))

		for _, t := range stage {
			planStatus := result.Tasks[t.Name()].Status
			icon, nameStyle, cmdStyle := previewTaskStyle(planStatus)

			lastNote := ""
			if last != nil {
				if ts, ok := last.Tasks[t.Name()]; ok {
					lastNote = previewLast.Render(fmtTaskLastRun(&ts))
				}
			}

			fmt.Printf("  %s  %s  %s\n", icon, nameStyle.Render(t.Name()), lastNote)
			for _, clause := range t.DoClauses() {
				hasConditions := len(clause.IfCmds()) > 0 || len(clause.UnlessCmds()) > 0

				for _, cmd := range clause.IfCmds() {
					fmt.Printf("       %s  %s\n", previewMeta.Render("if:     "), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.UnlessCmds() {
					fmt.Printf("       %s  %s\n", previewMeta.Render("unless: "), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.DoCmds() {
					label := "        "
					if hasConditions {
						label = "do:     "
					}
					fmt.Printf("       %s  %s\n", previewMeta.Render(label), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.ConfirmCmds() {
					fmt.Printf("       %s  %s\n", previewMeta.Render("confirm:"), cmdStyle.Render("$ "+cmd.String()))
				}
			}
		}
		fmt.Println()
	}
}

func previewTaskStyle(status orc.TaskStatus) (icon string, nameStyle, cmdStyle lipgloss.Style) {
	switch status {
	case orc.StatusSatisfied:
		return previewDone.Render("✓"), previewDimmed, previewDimmed
	case orc.StatusSkipped:
		return previewSkip.Render("—"), previewDimmed, previewDimmed
	case orc.StatusFailed:
		return previewErr.Render("✕"), previewErr, previewErr
	default:
		return previewReady.Render("○"), lipgloss.NewStyle(), previewMeta
	}
}
