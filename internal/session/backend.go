package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Backend string

const (
	BackendAuto   Backend = "auto"
	BackendTmux   Backend = "tmux"
	BackendScreen Backend = "screen"
	BackendExec   Backend = "exec"
)

type CommandSpec struct {
	Name          string
	Args          []string
	Dir           string
	Env           []string
	Stdin         *os.File
	Stdout        *os.File
	Stderr        *os.File
	CaptureOutput bool
}

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Runner interface {
	LookPath(name string) error
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}

type ExecRunner struct{}

func (ExecRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (ExecRunner) Run(ctx context.Context, spec CommandSpec) (CommandResult, error) {
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
	return CommandResult{
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

func ParseBackend(value string) (Backend, error) {
	normalized := Backend(strings.ToLower(strings.TrimSpace(value)))
	if normalized == "" {
		return BackendAuto, nil
	}
	switch normalized {
	case BackendAuto, BackendTmux, BackendScreen, BackendExec:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported session backend %q (use auto, tmux, screen, exec)", value)
	}
}

func ResolveBackend(preferred Backend, runner Runner) (Backend, error) {
	if preferred == "" {
		preferred = BackendAuto
	}
	if preferred != BackendAuto {
		if preferred == BackendExec {
			return preferred, nil
		}
		if err := runner.LookPath(string(preferred)); err != nil {
			return "", fmt.Errorf("%s not available", preferred)
		}
		return preferred, nil
	}
	if runner.LookPath(string(BackendTmux)) == nil {
		return BackendTmux, nil
	}
	if runner.LookPath(string(BackendScreen)) == nil {
		return BackendScreen, nil
	}
	return BackendExec, nil
}

func Start(ctx context.Context, runner Runner, backend Backend, root, name string, command []string, env []string, interactive bool) error {
	switch backend {
	case BackendTmux:
		args := []string{"new-session", "-d", "-s", name, "-c", root}
		if len(command) > 0 {
			args = append(args, command...)
		}
		_, err := runner.Run(ctx, CommandSpec{Name: "tmux", Args: args, Env: env})
		return err
	case BackendScreen:
		args := []string{"-dmS", name}
		if len(command) > 0 {
			args = append(args, command...)
		}
		_, err := runner.Run(ctx, CommandSpec{Name: "screen", Args: args, Dir: root, Env: env})
		return err
	case BackendExec:
		if interactive {
			return RunExecWithPTY(ctx, root, command, env)
		}
		execName, execArgs := ResolveExecCommand(command)
		_, err := runner.Run(ctx, CommandSpec{
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

func Attach(ctx context.Context, runner Runner, backend Backend, name string) error {
	switch backend {
	case BackendTmux:
		args := []string{"attach", "-t", name}
		if strings.TrimSpace(os.Getenv("TMUX")) != "" {
			args = []string{"switch-client", "-t", name}
		}
		_, err := runner.Run(ctx, CommandSpec{
			Name:   "tmux",
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		return err
	case BackendScreen:
		args := []string{"-r", name}
		if strings.TrimSpace(os.Getenv("STY")) != "" {
			args = []string{"-x", name}
		}
		_, err := runner.Run(ctx, CommandSpec{
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

func Stop(ctx context.Context, runner Runner, backend Backend, name string) error {
	switch backend {
	case BackendTmux:
		_, err := runner.Run(ctx, CommandSpec{Name: "tmux", Args: []string{"kill-session", "-t", name}})
		return err
	case BackendScreen:
		_, err := runner.Run(ctx, CommandSpec{Name: "screen", Args: []string{"-S", name, "-X", "quit"}})
		return err
	default:
		return fmt.Errorf("stop not supported for backend %q", backend)
	}
}

func Exists(ctx context.Context, runner Runner, backend Backend, name string) (bool, error) {
	switch backend {
	case BackendTmux:
		result, err := runner.Run(ctx, CommandSpec{Name: "tmux", Args: []string{"has-session", "-t", name}})
		if err == nil {
			return true, nil
		}
		if result.ExitCode == 1 {
			return false, nil
		}
		return false, err
	case BackendScreen:
		result, err := runner.Run(ctx, CommandSpec{Name: "screen", Args: []string{"-ls"}, CaptureOutput: true})
		if err != nil && result.ExitCode <= 0 {
			return false, err
		}
		return ScreenHasSession(result.Stdout, name), nil
	case BackendExec:
		return false, nil
	default:
		return false, fmt.Errorf("unsupported session backend %q", backend)
	}
}

func Running(ctx context.Context, runner Runner, backend Backend, name string) (bool, error) {
	if backend == "" || backend == BackendExec {
		return false, nil
	}
	normalized, _, err := NormalizeNameForBackend(backend, name)
	if err != nil {
		return false, err
	}
	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return Exists(statusCtx, runner, backend, normalized)
}

func NormalizeNameForBackend(backend Backend, name string) (string, bool, error) {
	if backend != BackendTmux {
		return name, false, nil
	}
	normalized := sanitizeTmuxSessionName(name)
	if normalized == "" {
		return "", false, fmt.Errorf("tmux session name derived from %q is empty; use --name to set one", name)
	}
	return normalized, normalized != name, nil
}

func ScreenHasSession(output, name string) bool {
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

func sanitizeTmuxSessionName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if isTmuxNameRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func isTmuxNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '-' || r == '_' || r == '.':
		return true
	default:
		return false
	}
}
