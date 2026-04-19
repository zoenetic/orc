# orc

A Go library for defining and executing task runbooks — with dependency ordering, conditional execution, undo support, and a bubbletea TUI.

## Install

Install the CLI:

```
go install github.com/zoenetic/orc/cmd/orc@latest
```

Add the library to a project:

```
go get github.com/zoenetic/orc
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
| `SkipIf(cmd)` | Skip task if command exits 0 |
| `RunIf(cmd)` | Skip task if command exits non-zero |
| `CheckFn(fn)` | Skip task based on a Go function |
| `Confirm(cmd)` | Assert a condition after execution |

## Commands

`orc.Sh(args...)` runs a shell command. `orc.ShF(format, args...)` is a `Sprintf`-style shorthand.

## License

MIT
