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
	do   *Do
	undo *Undo
}

func (c *Command) checkSatisfied(ctx context.Context) (bool, error) {
	err := c.execute(ctx, nil)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return false, exitErr
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
	if c.typ == cmdSh {
		return c.raw
	}
	return strings.Join(append([]string{c.cmd}, c.args...), " ")
}

func Sh(cmd string, env ...*EnvVar) []*Command {
	return []*Command{{raw: cmd, env: env, typ: cmdSh}}
}

func Cmd(cmd string, args ...string) *Command {
	return &Command{cmd: cmd, args: args, typ: cmdShArgs}
}

type Do struct {
	Cmds    []*Command
	If      []*Command
	Unless  []*Command
	Confirm []*Command
}

func (d Do) isTaskOption() {}

func (d *Do) shouldRun(ctx context.Context) (bool, error) {
	for _, cmd := range d.If {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	for _, cmd := range d.Unless {
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

type Undo struct {
	Cmds    []*Command
	If      []*Command
	Unless  []*Command
	Confirm []*Command
}

func (u Undo) isTaskOption() {}

func (u *Undo) shouldRun(ctx context.Context) (bool, error) {
	for _, cmd := range u.If {
		ok, err := cmd.checkSatisfied(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	for _, cmd := range u.Unless {
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

type DependsOn []*Task

func (d DependsOn) isTaskOption() {}
