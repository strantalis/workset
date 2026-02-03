package worksetapi

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

const (
	agentLaunchAuto   = "auto"
	agentLaunchStrict = "strict"

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
	ShellSetting   string
	ShellMode      string
	PTYMode        string
	AllowPathGuess bool
}

func parseAgentLaunchMode(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", agentLaunchAuto:
		return agentLaunchAuto, true
	case agentLaunchStrict, "no-shell", "noshell", "strict-path":
		return agentLaunchStrict, true
	default:
		return "", false
	}
}

func normalizeAgentLaunchMode(value string) string {
	if normalized, ok := parseAgentLaunchMode(value); ok {
		return normalized
	}
	return agentLaunchAuto
}

func resolveAgentExecSettings(defaults config.Defaults) agentExecSettings {
	launch := normalizeAgentLaunchMode(defaults.AgentLaunch)
	switch launch {
	case agentLaunchStrict:
		return agentExecSettings{
			ShellSetting:   agentShellNone,
			ShellMode:      agentShellModePlain,
			PTYMode:        agentPTYNever,
			AllowPathGuess: false,
		}
	default:
		return agentExecSettings{
			ShellSetting:   agentShellAuto,
			ShellMode:      agentShellModeLogin,
			PTYMode:        agentPTYAuto,
			AllowPathGuess: true,
		}
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

func resolveAgentCommandPath(command []string, allowGuess bool) []string {
	if len(command) == 0 {
		return command
	}
	if hasPathSeparator(command[0]) {
		return command
	}
	if !allowGuess {
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
