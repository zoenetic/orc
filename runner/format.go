package runner

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	catppuccino "github.com/catppuccin/go"
	"github.com/zoenetic/orc"
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
	case orc.StatusRunning:
		icon = m.spinner.View()
		statusStyle = dispRunning.Width(statusWidth)
	case orc.StatusSucceeded:
		icon = dispDone.Render("✓")
		statusStyle = dispDone.Width(statusWidth)
	case orc.StatusSatisfied:
		icon = dispDone.Render("✓")
		statusStyle = dispDone.Width(statusWidth)
	case orc.StatusConfirmFailed:
		icon = dispFail.Render("✗")
		statusStyle = dispFail.Width(statusWidth)
	case orc.StatusFailed:
		icon = dispFail.Render("✗")
		statusStyle = dispFail.Width(statusWidth)
	case orc.StatusSkipped:
		icon = dispSkip.Render("—")
		statusStyle = dispSkip.Width(statusWidth)
	default:
		icon = dispMeta.Render(".")
		statusStyle = dispMeta.Width(statusWidth)
	}

	name := lipgloss.NewStyle().Width(m.maxNameWidth).Render(t.name)
	statusText := statusStyle.Render(t.status.String())

	var trailing string
	switch t.status {
	case orc.StatusRunning:
		if !t.started.IsZero() {
			trailing = dispRunning.Render(fmtDuration(time.Since(t.started)))
		}
	case orc.StatusSucceeded, orc.StatusSatisfied:
		trailing = dispMeta.Render(fmtDuration(t.duration))
	case orc.StatusFailed, orc.StatusConfirmFailed:
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

func fmtTaskLastRun(last *orc.TaskRecord) string {
	if last == nil {
		return ""
	}
	if last.Status == orc.StatusSucceeded.String() || last.Status == orc.StatusSatisfied.String() {
		return fmt.Sprintf("last run %s", fmtAge(time.Since(last.Finished)))
	}
	return fmt.Sprintf("last run %s with status %s", fmtAge(time.Since(last.Finished)), last.Status)
}
