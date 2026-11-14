package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/madalinpopa/cmt/internal/command"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

func Run(ctx context.Context, root *command.Command, args []string) int {
	err := root.Execute(ctx, args)
	if err == nil {
		return exitSuccess
	}

	if _, writeErr := fmt.Fprintf(root.Err(), "error: %v\n", err); writeErr != nil {
		return exitFailure
	}

	if _, ok := errors.AsType[*command.UsageError](err); ok {
		return exitUsage
	}
	return exitFailure
}
