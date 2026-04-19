# orc

A Go library for defining and executing task runbooks — with dependency ordering, conditional execution, undo support, and a bubbletea TUI.

## Install

Install the CLI:

```
go install github.com/zoenetic/orc/cmd/orc@latest
```

## Getting started

Use `orc init` to scaffold a new project:

```
orc init my-project
cd my-project
```

Or in an existing directory:

```
orc init .
```

This creates `main.go`, `go.mod`, and a `.gitignore` with the right entries. Flags:

```
orc init [directory] [--git] [--yes]

  --git|-g  initialize as a git repository
  --yes|-y  skip confirmation prompt
```

Or add orc to an existing go project and do it yourself:

```
go get github.com/zoenetic/orc
```

## Defining plans

Each exported method on your `Plans` struct that returns `*orc.Runbook` is a named plan. The method name is used as the plan name (case-insensitive). A method named `Default` is used when no plan is specified.

```go
package main

import "github.com/zoenetic/orc"

type Plans struct{}

func main() { orc.Main[Plans]() }

func (p *Plans) Default() *orc.Runbook {
    rb := orc.New("my runbook", orc.Options{Concurrency: 4})

    a := rb.Task("step a", orc.Do(orc.Sh("echo a")))

    rb.Task("step b",
        orc.Do(orc.Sh("echo b")),
        orc.Undo(orc.Sh("echo undo b")),
        orc.DependsOn(a),
    )

    return rb
}
```

## CLI commands

`orc.Main` selects the bubbletea TUI automatically when stdout is a terminal, and falls back to plain output otherwise (CI, pipes).

```
orc run      [plan] [-v]          execute a plan
orc preview  [plan]               show what would run and last-run status per task
orc validate [plan]               check for dependency cycles and errors
orc rollback [plan] [run-id]      run undo commands in reverse stage order
orc history  [-n N]               show recent run history (default: last 20)
```

All commands except `history` accept an optional plan name (defaults to `default`). `rollback` also accepts a run ID to target a specific past run.

## Task options

| Option | Description |
|---|---|
| `Do(cmds...)` | Commands to execute |
| `Undo(cmds...)` | Commands to run on rollback, in reverse stage order |
| `DependsOn(tasks...)` | Declare dependencies (determines stage ordering) |

## Commands

`orc.Sh(cmd)` runs a shell string via `sh -c`. Env vars can be passed as additional arguments:

```go
orc.Sh("echo $GREETING", orc.Env("GREETING", "hello"))
```

`orc.Cmd(cmd, args...)` runs a command with explicit arguments without a shell:

```go
orc.Cmd("kubectl", "apply", "-f", "deploy.yaml")
```

## Clauses

`If`, `Unless`, and `Confirm` are methods on `Do` and `Undo` clauses. They control whether the clause runs and assert post-conditions.

**`If(cmd)`** — run the clause only if the command exits 0:

```go
orc.Do(orc.Sh("make build")).If(orc.Sh("git diff --quiet HEAD"))
```

**`Unless(cmd)`** — skip the clause if the command exits 0:

```go
orc.Do(orc.Sh("docker build .")).Unless(orc.Sh("docker image inspect myapp:latest"))
```

**`Confirm(cmd)`** — assert a condition after the clause executes; fails the task if it exits non-zero:

```go
orc.Do(orc.Sh("kubectl apply -f deploy.yaml")).
    Confirm(orc.Sh("kubectl rollout status deployment/myapp"))
```

Multiple calls can be chained, and all three can be combined on a single clause. `If` and `Unless` conditions are evaluated before execution; `Confirm` is evaluated after. The same methods are available on `Undo` clauses.

## Environment variables

Env vars cascade from runbook → task → command, with each level able to override the parent.

```go
rb := orc.New("deploy", orc.Options{})
rb.Env(orc.Env("ENV", "staging"))   // applies to all tasks

task := rb.Task("deploy",
    orc.Do(orc.Sh("./deploy.sh", orc.Env("ENV", "prod"))),  // overrides at command level
)
task.Env("REGION", "eu-west-1")     // overrides at task level
```

## Package dependencies

`rb.Use` declares a required CLI tool. orc checks it exists in PATH before running.

```go
rb.Use("kubectl")
rb.Use("helm")
```

## License

MIT
