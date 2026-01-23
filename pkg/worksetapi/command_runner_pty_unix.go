//go:build !windows

package worksetapi

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

func runCommandWithPTY(ctx context.Context, root string, command []string, env []string, stdin string) (CommandResult, error) {
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
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return CommandResult{}, err
	}
	var stdout bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = stdout.ReadFrom(ptmx)
		close(done)
	}()
	if strings.TrimSpace(stdin) != "" {
		if !strings.HasSuffix(stdin, "\n") {
			stdin += "\n"
		}
		_, _ = ptmx.Write([]byte(stdin))
	}
	err = cmd.Wait()
	_ = ptmx.Close()
	<-done
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
		Stderr:   "",
		ExitCode: exitCode,
	}, err
}
