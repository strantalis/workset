package hooks

import (
	"context"
	"io"
	"os/exec"

	"github.com/strantalis/workset/internal/session"
)

type RunRequest struct {
	Command []string
	Cwd     string
	Env     []string
	Stdout  io.Writer
	Stderr  io.Writer
}

type Runner interface {
	Run(ctx context.Context, req RunRequest) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, req RunRequest) error {
	execName, execArgs := session.ResolveExecCommand(req.Command)
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	cmd.Dir = req.Cwd
	cmd.Stdout = req.Stdout
	cmd.Stderr = req.Stderr
	cmd.Env = req.Env
	return cmd.Run()
}
