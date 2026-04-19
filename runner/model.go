package runner

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/zoenetic/orc"
)

type taskEventMsg orc.TaskEvent
type tickMsg time.Time
type doneMsg struct{}

type taskModel struct {
	name     string
	status   orc.TaskStatus
	duration time.Duration
	err      error
	output   []string
	started  time.Time
}

type model struct {
	stages       [][]*orc.Task
	tasks        map[string]*taskModel
	spinner      spinner.Model
	verbose      bool
	done         bool
	width        int
	maxNameWidth int
}

func newModel(stages [][]*orc.Task, verbose bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	maxNameWidth := 0
	tasks := make(map[string]*taskModel)
	for _, stage := range stages {
		for _, t := range stage {
			tasks[t.Name()] = &taskModel{name: t.Name()}
			if len(t.Name()) > maxNameWidth {
				maxNameWidth = len(t.Name())
			}
		}
	}

	return model{
		stages:       stages,
		tasks:        tasks,
		spinner:      s,
		verbose:      verbose,
		width:        80,
		maxNameWidth: maxNameWidth,
	}
}
