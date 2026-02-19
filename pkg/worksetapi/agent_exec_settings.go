package worksetapi

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	agentShellAuto            = "auto"
	agentShellNone            = "none"
	agentShellModeLogin       = "login"
	agentShellModeInteractive = "interactive"
	agentShellModePlain       = "plain"
	agentShellModeLoginAndI   = "login-interactive"
	agentPTYAuto              = "auto"
	agentPTYAlways            = "always"
	agentPTYNever             = "never"
)

type agentExecSettings struct {
	ShellSetting string
	ShellMode    string
	PTYMode      string
}

func resolveAgentExecSettings() agentExecSettings {
	return agentExecSettings{
		ShellSetting: agentShellAuto,
		ShellMode:    agentShellModeLogin,
		PTYMode:      agentPTYAuto,
	}
}

func shouldWrapAgentCommand(settings agentExecSettings) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return settings.ShellSetting != agentShellNone
}

func resolveAgentShellPath(settings agentExecSettings) (string, error) {
	if settings.ShellSetting == agentShellNone {
		return "", errors.New("agent shell disabled")
	}
	shell := settings.ShellSetting
	if shell == agentShellAuto {
		shell = strings.TrimSpace(os.Getenv("SHELL"))
		if shell == "" {
			shell = "/bin/sh"
		}
	}
	if shell == "" {
		return "", errors.New("agent shell required")
	}
	if hasPathSeparator(shell) {
		shell = normalizeCLIPath(shell)
		if !isExecutableCandidate(shell) {
			return "", fmt.Errorf("agent shell is not executable: %s", shell)
		}
		return filepath.Clean(shell), nil
	}
	resolved, err := exec.LookPath(shell)
	if err != nil {
		return "", fmt.Errorf("agent shell not found: %s", shell)
	}
	return resolved, nil
}

func resolveAgentCommandPath(command []string) []string {
	if len(command) == 0 {
		return command
	}
	if hasPathSeparator(command[0]) {
		return command
	}
	if resolved, err := exec.LookPath(command[0]); err == nil {
		command[0] = resolved
		return command
	}
	if resolved := resolveCLIPath(command[0]); resolved != "" {
		command[0] = resolved
	}
	return command
}
