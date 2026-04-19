package runner

import (
	"context"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/zoenetic/orc"
)

type taskEventMsg orc.TaskEvent
type outputMsg struct {
	task string
	line string
}

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
	cancel       context.CancelFunc
}

func newModel(stages [][]*orc.Task, verbose bool, cancel context.CancelFunc) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Lavender().Hex))

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
		cancel:       cancel,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case taskEventMsg:
		t, ok := m.tasks[msg.Task.Name()]
		if !ok {
			return m, nil
		}
		t.status = msg.Status
		t.duration = msg.Duration
		t.err = msg.Err
		if msg.Status == orc.StatusRunning {
			t.started = time.Now()
		}
		if msg.Status.IsTerminal() {
			t.output = nil
		}
		return m, nil

	case outputMsg:
		t, ok := m.tasks[msg.task]
		if !ok {
			return m, nil
		}
		t.output = append(t.output, msg.line)
		if !m.verbose && len(t.output) > 1 {
			t.output = t.output[len(t.output)-1:]
		}
		return m, nil

	case doneMsg:
		m.done = true
		return m, func() tea.Msg { return tea.Quit() }

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, func() tea.Msg { return tea.Quit() }
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	var b strings.Builder
	b.WriteString("\n")

	showStages := len(m.stages) > 1

	for i, stage := range m.stages {
		if showStages {
			b.WriteString(m.formatStage(i + 1))
			b.WriteString("\n")
		}

		sorted := make([]*orc.Task, len(stage))
		copy(sorted, stage)
		sort.Slice(sorted, func(a, b int) bool {
			return sorted[a].Name() < sorted[b].Name()
		})

		for _, t := range sorted {
			tm := m.tasks[t.Name()]
			b.WriteString(m.formatTask(tm))
			b.WriteString("\n")
			for _, line := range tm.output {
				b.WriteString(m.formatOutput(line))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	return tea.NewView(b.String())
}
