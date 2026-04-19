package orc

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"

	"github.com/mattn/go-isatty"
)

var runbookPtrType = reflect.TypeFor[*Runbook]()

func Main[T any](exec ...Executor) {
	args := os.Args[1:]

	action := "run"
	planName := "default"
	verbose := false

	var positional []string
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		default:
			if !strings.HasPrefix(a, "-") {
				positional = append(positional, a)
			}
		}
	}
	if len(positional) >= 1 {
		action = positional[0]
	}
	if len(positional) >= 2 {
		planName = positional[1]
	}

	rb, err := callPlan[T](planName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orc: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := rb.ensurePackages(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "orc: %v\n", err)
		os.Exit(1)
	}

	var e Executor
	if len(exec) > 0 && exec[0] != nil {
		e = exec[0]
	} else if isatty.IsTerminal(os.Stdout.Fd()) {
		e = Display{}
	} else {
		e = headlessExecutor{}
	}

	opts := RunOptions{
		Plan:    planName,
		Verbose: verbose,
	}

	switch action {
	case "run":
		e.Execute(ctx, rb, opts)
	case "preview":
		e.Preview(ctx, rb, opts)
	case "validate":
		e.Validate(ctx, rb, opts)
	case "rollback":
		var runID string
		if len(positional) >= 3 {
			runID = positional[2]
		}
		e.Rollback(ctx, rb, opts, runID)
	default:
		fmt.Fprintf(os.Stderr, "orc: unknown command %q\n", action)
		os.Exit(1)
	}
}

func callPlan[T any](name string) (*Runbook, error) {
	pt := reflect.PointerTo(reflect.TypeFor[T]())

	for m := range pt.Methods() {
		if !strings.EqualFold(m.Name, name) {
			continue
		}
		mt := m.Type
		if mt.NumIn() != 1 || mt.NumOut() != 1 || mt.Out(0) != runbookPtrType {
			continue
		}
		v := reflect.New(pt.Elem())
		res := v.MethodByName(m.Name).Call(nil)
		rb, _ := res[0].Interface().(*Runbook)
		if rb == nil {
			return nil, fmt.Errorf("plan %q returned nil", name)
		}
		return rb, nil
	}

	return nil, fmt.Errorf("no plan %q found on %s", name, pt.Elem().Name())
}
