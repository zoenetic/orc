package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Dispatch(action, planName, runID string, flags ...string) error {
	if planName == "" {
		planName = "default"
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); os.IsNotExist(err) {
		return fmt.Errorf("no go.mod found; run 'orc init' to create a new project")
	}

	if devPath := os.Getenv("ORC_DEV"); devPath != "" {
		if err := ensureWorkFile(cwd, devPath); err != nil {
			return err
		}
	}

	goArgs := []string{"run", ".", action}
	goArgs = append(goArgs, flags...)
	goArgs = append(goArgs, planName)
	if runID != "" {
		goArgs = append(goArgs, runID)
	}

	cmd := exec.Command("go", goArgs...)
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = &exitStatusFilter{w: os.Stderr}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func ensureWorkFile(dir, devPath string) error {
	workFile := filepath.Join(dir, "go.work")
	if _, err := os.Stat(workFile); err == nil {
		return nil
	}
	abs, err := filepath.Abs(devPath)
	if err != nil {
		return fmt.Errorf("resolve ORC_DEV path: %w", err)
	}
	c := exec.Command("go", "work", "init", ".", abs)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("go work init: %s: %w", out, err)
	}
	return nil
}

type exitStatusFilter struct {
	w   io.Writer
	buf []byte
}

func (f *exitStatusFilter) Write(p []byte) (int, error) {
	f.buf = append(f.buf, p...)
	for {
		idx := bytes.IndexByte(f.buf, '\n')
		if idx < 0 {
			break
		}
		line := f.buf[:idx]
		f.buf = f.buf[idx+1:]
		if !strings.HasPrefix(string(line), "exit status ") {
			f.w.Write(append(line, '\n'))
		}
	}
	return len(p), nil
}
