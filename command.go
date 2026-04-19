package orc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

type commandType int

const (
	cmdSh commandType = iota
	cmdShArgs
)

type Command struct {
	raw  string
	cmd  string
	args []string
	env  []*EnvVar
	typ  commandType
}

func (c *Command) checkSatisfied(ctx context.Context) (bool, error) {
	err := c.execute(ctx, nil)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

func (c *Command) execute(ctx context.Context, out io.Writer) error {
	var cmd *exec.Cmd
	switch c.typ {
	case cmdSh:
		cmd = exec.CommandContext(ctx, "sh", "-c", c.raw)
	case cmdShArgs:
		cmd = exec.CommandContext(ctx, c.cmd, c.args...)
	default:
		panic("invalid command type")
	}
	if len(c.env) > 0 {
		env := os.Environ()
		for _, v := range c.env {
			env = append(env, v.String())
		}
		cmd.Env = env
	}
	cmd.Stdout, cmd.Stderr = out, out
	return cmd.Run()
}

func (c *Command) String() string {
	return strings.Join(append([]string{c.cmd}, c.args...), " ")
}

func Sh(cmd string, env ...*EnvVar) *Command {
	return &Command{raw: cmd, env: env, typ: cmdSh}
}

func ShArgs(cmd string, args ...string) *Command {
	return &Command{cmd: cmd, args: args, typ: cmdShArgs}
}

type DoClause struct {
	cmds        []*Command
	ifCmds      []*Command
	unlessCmds  []*Command
	confirmCmds []*Command
}

func Do(commands ...*Command) *DoClause {
	return &DoClause{
		cmds: commands,
	}
}

func (d *DoClause) DoCmds() []*Command {
	return d.cmds
}

func (d *DoClause) IfCmds() []*Command {
	return d.ifCmds
}

func (d *DoClause) UnlessCmds() []*Command {
	return d.unlessCmds
}

func (d *DoClause) ConfirmCmds() []*Command {
	return d.confirmCmds
}

func (d *DoClause) apply(t *Task) {
	t.doClauses = append(t.doClauses, d)
}

func (d *DoClause) shouldRun(ctx context.Context) (bool, error) {
	for _, cmd := range d.ifCmds {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	for _, cmd := range d.unlessCmds {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}

	return true, nil
}

func (d *DoClause) If(conditions ...*Command) *DoClause {
	d.ifCmds = append(d.ifCmds, conditions...)
	return d
}

func (d *DoClause) Unless(conditions ...*Command) *DoClause {
	d.unlessCmds = append(d.unlessCmds, conditions...)
	return d
}

func (d *DoClause) Confirm(conditions ...*Command) *DoClause {
	d.confirmCmds = append(d.confirmCmds, conditions...)
	return d
}

type UndoClause struct {
	cmds        []*Command
	ifCmds      []*Command
	unlessCmds  []*Command
	confirmCmds []*Command
}

func Undo(commands ...*Command) *UndoClause {
	return &UndoClause{
		cmds: commands,
	}
}

func (u *UndoClause) DoCmds() []*Command {
	return u.cmds
}

func (u *UndoClause) IfCmds() []*Command {
	return u.ifCmds
}

func (u *UndoClause) UnlessCmds() []*Command {
	return u.unlessCmds
}

func (u *UndoClause) ConfirmCmds() []*Command {
	return u.confirmCmds
}

func (u *UndoClause) apply(t *Task) {
	t.undoClauses = append(t.undoClauses, u)
}

func (u *UndoClause) shouldRun(ctx context.Context) (bool, error) {
	for _, cmd := range u.ifCmds {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	for _, cmd := range u.unlessCmds {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}

	return true, nil
}

func (u *UndoClause) If(conditions ...*Command) *UndoClause {
	u.ifCmds = append(u.ifCmds, conditions...)
	return u
}

func (u *UndoClause) Unless(conditions ...*Command) *UndoClause {
	u.unlessCmds = append(u.unlessCmds, conditions...)
	return u
}

func (u *UndoClause) Confirm(conditions ...*Command) *UndoClause {
	u.ifCmds = append(u.ifCmds, conditions...)
	return u
}

type dependsOnOption struct {
	tasks []*Task
}

func DependsOn(tasks ...*Task) *dependsOnOption {
	return &dependsOnOption{tasks: tasks}
}

func (d *dependsOnOption) apply(t *Task) {
	t.dependencies = append(t.dependencies, d.tasks...)
}
