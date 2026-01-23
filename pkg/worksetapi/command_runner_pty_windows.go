//go:build windows

package worksetapi

import (
	"context"
	"errors"
)

func runCommandWithPTY(_ context.Context, _ string, _ []string, _ []string, _ string) (CommandResult, error) {
	return CommandResult{}, errors.New("pty runner is not supported on windows")
}
