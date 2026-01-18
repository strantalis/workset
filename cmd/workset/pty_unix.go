//go:build !windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func runExecWithPTY(ctx context.Context, dir string, command []string, env []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("interactive exec requires a TTY")
	}
	execName, execArgs := resolveExecCommand(command)
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	cmd.Dir = dir
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), state) }()

	_ = pty.InheritSize(os.Stdin, ptmx)
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	defer signal.Stop(resize)
	go func() {
		for range resize {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()

	go func() {
		_, _ = io.Copy(ptmx, os.Stdin)
	}()
	_, _ = io.Copy(os.Stdout, ptmx)
	return cmd.Wait()
}
