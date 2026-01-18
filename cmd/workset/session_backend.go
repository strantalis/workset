package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type sessionBackend string

const (
	sessionBackendAuto   sessionBackend = "auto"
	sessionBackendTmux   sessionBackend = "tmux"
	sessionBackendScreen sessionBackend = "screen"
	sessionBackendExec   sessionBackend = "exec"
)

type commandSpec struct {
	Name          string
	Args          []string
	Dir           string
	Env           []string
	Stdin         *os.File
	Stdout        *os.File
	Stderr        *os.File
	CaptureOutput bool
}

type commandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type sessionRunner interface {
	LookPath(name string) error
	Run(ctx context.Context, spec commandSpec) (commandResult, error)
}

type execRunner struct{}

func (execRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (execRunner) Run(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Env != nil {
		cmd.Env = spec.Env
	}

	var stdout, stderr bytes.Buffer
	if spec.CaptureOutput {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	} else {
		cmd.Stdin = spec.Stdin
		cmd.Stdout = spec.Stdout
		cmd.Stderr = spec.Stderr
	}

	err := cmd.Run()
	code, err := exitCodeFromError(err)
	return commandResult{
		ExitCode: code,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, err
}

func exitCodeFromError(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func parseSessionBackend(value string) (sessionBackend, error) {
	normalized := sessionBackend(strings.ToLower(strings.TrimSpace(value)))
	if normalized == "" {
		return sessionBackendAuto, nil
	}
	switch normalized {
	case sessionBackendAuto, sessionBackendTmux, sessionBackendScreen, sessionBackendExec:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported session backend %q (use auto, tmux, screen, exec)", value)
	}
}

func resolveSessionBackend(preferred sessionBackend, runner sessionRunner) (sessionBackend, error) {
	if preferred == "" {
		preferred = sessionBackendAuto
	}
	if preferred != sessionBackendAuto {
		if preferred == sessionBackendExec {
			return preferred, nil
		}
		if err := runner.LookPath(string(preferred)); err != nil {
			return "", fmt.Errorf("%s not available", preferred)
		}
		return preferred, nil
	}
	if runner.LookPath(string(sessionBackendTmux)) == nil {
		return sessionBackendTmux, nil
	}
	if runner.LookPath(string(sessionBackendScreen)) == nil {
		return sessionBackendScreen, nil
	}
	return sessionBackendExec, nil
}

func startSession(ctx context.Context, runner sessionRunner, backend sessionBackend, root, name string, command []string, env []string, interactive bool) error {
	switch backend {
	case sessionBackendTmux:
		args := []string{"new-session", "-d", "-s", name, "-c", root}
		if len(command) > 0 {
			args = append(args, command...)
		}
		_, err := runner.Run(ctx, commandSpec{Name: "tmux", Args: args})
		return err
	case sessionBackendScreen:
		args := []string{"-dmS", name}
		if len(command) > 0 {
			args = append(args, command...)
		}
		_, err := runner.Run(ctx, commandSpec{Name: "screen", Args: args, Dir: root})
		return err
	case sessionBackendExec:
		if interactive {
			return runExecWithPTY(ctx, root, command, env)
		}
		execName, execArgs := resolveExecCommand(command)
		_, err := runner.Run(ctx, commandSpec{
			Name:   execName,
			Args:   execArgs,
			Dir:    root,
			Env:    env,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		return err
	default:
		return fmt.Errorf("unsupported session backend %q", backend)
	}
}

func attachSession(ctx context.Context, runner sessionRunner, backend sessionBackend, name string) error {
	switch backend {
	case sessionBackendTmux:
		args := []string{"attach", "-t", name}
		if strings.TrimSpace(os.Getenv("TMUX")) != "" {
			args = []string{"switch-client", "-t", name}
		}
		_, err := runner.Run(ctx, commandSpec{
			Name:   "tmux",
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		return err
	case sessionBackendScreen:
		args := []string{"-r", name}
		if strings.TrimSpace(os.Getenv("STY")) != "" {
			args = []string{"-x", name}
		}
		_, err := runner.Run(ctx, commandSpec{
			Name:   "screen",
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		return err
	default:
		return fmt.Errorf("attach not supported for backend %q", backend)
	}
}

func stopSession(ctx context.Context, runner sessionRunner, backend sessionBackend, name string) error {
	switch backend {
	case sessionBackendTmux:
		_, err := runner.Run(ctx, commandSpec{Name: "tmux", Args: []string{"kill-session", "-t", name}})
		return err
	case sessionBackendScreen:
		_, err := runner.Run(ctx, commandSpec{Name: "screen", Args: []string{"-S", name, "-X", "quit"}})
		return err
	default:
		return fmt.Errorf("stop not supported for backend %q", backend)
	}
}

func sessionExists(ctx context.Context, runner sessionRunner, backend sessionBackend, name string) (bool, error) {
	switch backend {
	case sessionBackendTmux:
		result, err := runner.Run(ctx, commandSpec{Name: "tmux", Args: []string{"has-session", "-t", name}})
		if err == nil {
			return true, nil
		}
		if result.ExitCode == 1 {
			return false, nil
		}
		return false, err
	case sessionBackendScreen:
		result, err := runner.Run(ctx, commandSpec{Name: "screen", Args: []string{"-ls"}, CaptureOutput: true})
		if err != nil && result.ExitCode <= 0 {
			return false, err
		}
		return screenHasSession(result.Stdout, name), nil
	case sessionBackendExec:
		return false, nil
	default:
		return false, fmt.Errorf("unsupported session backend %q", backend)
	}
}

func screenHasSession(output, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			parts := strings.Split(line, "\t")
			line = strings.TrimSpace(parts[0])
		}
		if strings.HasSuffix(line, "."+name) || line == name {
			return true
		}
	}
	return false
}
