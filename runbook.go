package orc

import (
	"context"
	"fmt"
	"os/exec"
)

type Runbook struct {
	name        string
	concurrency int
	env         []*EnvVar
	packages    []*Pkg
	tasks       map[string]*Task
}

type Options struct {
	Concurrency int
	PkgSource
}

func New(name string, opts Options) *Runbook {
	return &Runbook{
		name:        name,
		concurrency: opts.Concurrency,
		tasks:       make(map[string]*Task),
	}
}

func (rb *Runbook) Use(name, version string, sources ...PkgSource) *Runbook {
	rb.packages = append(rb.packages, &Pkg{
		name:    name,
		version: version,
		sources: sources,
	})
	return rb
}

func (rb *Runbook) ensurePackages(ctx context.Context) error {
	for _, p := range rb.packages {
		if _, err := exec.LookPath(p.name); err == nil {
			continue
		}
		if len(p.sources) == 0 {
			return fmt.Errorf("package %q not found in PATH", p.name)
		}
		if err := installPkg(ctx, p); err != nil {
			return fmt.Errorf("install package %q: %w", p.name, err)
		}
	}
	return nil
}

func installPkg(ctx context.Context, p *Pkg) error {
	var errs []error
	for _, src := range p.sources {
		if !src.Available() {
			continue
		}
		if err := src.Install(ctx, p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", src.Name(), err))
			continue
		}
		if _, err := exec.LookPath(p.name); err == nil {
			return nil
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to install package %q:\n%v", p.name, errs)
	}
	return fmt.Errorf("package %q not found in PATH and no available sources", p.name)
}

func (rb *Runbook) previewTask(
	ctx context.Context,
	t *Task,
	results map[string]PreviewTaskResult,
) PreviewTaskResult {
	for _, dep := range t.dependencies {
		if results[dep.name].Status == StatusFailed {
			return PreviewTaskResult{Status: StatusSkipped}
		}
	}

	allSatisfied := true
	for _, clause := range t.doClauses {
		run, err := clause.shouldRun(ctx)
		if err != nil {
			return PreviewTaskResult{Status: StatusFailed, Err: err}
		}
		if run {
			allSatisfied = false
			break
		}
	}

	if allSatisfied {
		return PreviewTaskResult{Status: StatusSatisfied}
	}

	return PreviewTaskResult{Status: StatusReady}
}
