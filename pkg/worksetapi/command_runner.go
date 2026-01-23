package worksetapi

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
)

// CommandResult captures command output for integrations that need stdout/stderr.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes a command with optional stdin and captures its output.
type CommandRunner func(ctx context.Context, root string, command []string, env []string, stdin string) (CommandResult, error)

func runCommandCapture(ctx context.Context, root string, command []string, env []string, stdin string) (CommandResult, error) {
	if len(command) == 0 {
		return CommandResult{}, errors.New("command required")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = root
	if len(env) > 0 {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}
	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}
