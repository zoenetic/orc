package orc

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	catppuccino "github.com/catppuccin/go"
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

var palette = catppuccino.Mocha

var (
	dispRunning = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Blue().Hex))
	dispDone    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Green().Hex))
	dispFail    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Red().Hex))
	dispSkip    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Overlay0().Hex))
	dispMeta    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Overlay0().Hex))
	dispOutput  = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Overlay1().Hex))
)

const statusWidth = 16

func (m model) formatStage(n int) string {
	label := fmt.Sprintf("Stage %d", n)
	rule := strings.Repeat("─", m.width-lipgloss.Width(label))
	return fmt.Sprintf("%s %s", label, rule)
}

func (m model) formatOutput(line string) string {
	return "       " + dispMeta.Render(">") + " " + dispOutput.Render(line)
}

func (m model) formatTask(t *taskModel) string {
	var icon string
	var statusStyle lipgloss.Style

	switch t.status {
	case StatusRunning:
		icon = m.spinner.View()
		statusStyle = dispRunning.Width(statusWidth)
	case StatusSucceeded:
		icon = dispDone.Render("✓")
		statusStyle = dispDone.Width(statusWidth)
	case StatusSatisfied:
		icon = dispDone.Render("✓")
		statusStyle = dispDone.Width(statusWidth)
	case StatusConfirmFailed:
		icon = dispFail.Render("✗")
		statusStyle = dispFail.Width(statusWidth)
	case StatusFailed:
		icon = dispFail.Render("✗")
		statusStyle = dispFail.Width(statusWidth)
	case StatusSkipped:
		icon = dispSkip.Render("—")
		statusStyle = dispSkip.Width(statusWidth)
	default:
		icon = dispMeta.Render(".")
		statusStyle = dispMeta.Width(statusWidth)
	}

	name := lipgloss.NewStyle().Width(m.maxNameWidth).Render(t.name)
	statusText := statusStyle.Render(string(t.status))

	var trailing string
	switch t.status {
	case StatusRunning:
		if !t.started.IsZero() {
			trailing = dispRunning.Render(fmtDuration(time.Since(t.started)))
		}
	case StatusSucceeded, StatusSatisfied:
		trailing = dispMeta.Render(fmtDuration(t.duration))
	case StatusFailed, StatusConfirmFailed:
		if t.err != nil {
			trailing = dispFail.Render(t.err.Error())
		}
	}

	return fmt.Sprintf("  %s  %s  %s  %s", icon, name, statusText, trailing)
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func fmtAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func fmtTaskLastRun(last *TaskRecord) string {
	if last == nil {
		return ""
	}
	if last.Status == string(StatusSucceeded) || last.Status == string(StatusSatisfied) {
		return fmt.Sprintf("last run %s", fmtAge(time.Since(last.Finished)))
	}
	return fmt.Sprintf("last run %s with status %s", fmtAge(time.Since(last.Finished)), last.Status)
}

func printPlan(rb *Runbook, result *PreviewResult) {
	last, _ := LoadState()

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
				hasConditions := len(clause.If) > 0 || len(clause.Unless) > 0

				for _, cmd := range clause.If {
					fmt.Printf("       %s  %s\n", previewMeta.Render("if:     "), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.Unless {
					fmt.Printf("       %s  %s\n", previewMeta.Render("unless: "), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.Cmds {
					label := "        "
					if hasConditions {
						label = "do:     "
					}
					fmt.Printf("       %s  %s\n", previewMeta.Render(label), cmdStyle.Render("$ "+cmd.String()))
				}
				for _, cmd := range clause.Confirm {
					fmt.Printf("       %s  %s\n", previewMeta.Render("confirm:"), cmdStyle.Render("$ "+cmd.String()))
				}
			}
		}
		fmt.Println()
	}
}

func previewTaskStyle(status TaskStatus) (icon string, nameStyle, cmdStyle lipgloss.Style) {
	switch status {
	case StatusSatisfied:
		return previewDone.Render("✓"), previewDimmed, previewDimmed
	case StatusSkipped:
		return previewSkip.Render("—"), previewDimmed, previewDimmed
	case StatusFailed:
		return previewErr.Render("✕"), previewErr, previewErr
	default:
		return previewReady.Render("○"), lipgloss.NewStyle(), previewMeta
	}
}
