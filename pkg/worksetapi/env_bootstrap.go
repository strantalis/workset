package worksetapi

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	loginEnvMu     sync.Mutex
	loginEnvLoaded bool
)

func ensureLoginEnv() {
	_, _ = loadLoginEnv(context.Background(), false)
}

func reloadLoginEnv(ctx context.Context) (EnvSnapshotResultJSON, error) {
	return loadLoginEnv(ctx, true)
}

func parseEnvSnapshot(output string) map[string]string {
	env := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}
		env[key] = value
	}
	return env
}

func applyEnvSnapshot(snapshot map[string]string) []string {
	if len(snapshot) == 0 {
		return nil
	}
	current := envMap(os.Environ())
	changed := make([]string, 0, len(snapshot))
	for key, value := range snapshot {
		if value == "" {
			continue
		}
		if shouldOverrideEnvKey(key) || current[key] == "" {
			if current[key] == value {
				continue
			}
			_ = os.Setenv(key, value)
			changed = append(changed, key)
		}
	}
	return changed
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func shouldOverrideEnvKey(key string) bool {
	switch key {
	case "PATH", "SSH_AUTH_SOCK", "SSH_AGENT_PID", "SSH_ASKPASS", "GIT_SSH_COMMAND", "LANG", "LC_ALL", "LC_CTYPE":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}

func loadLoginEnv(ctx context.Context, force bool) (EnvSnapshotResultJSON, error) {
	if runtime.GOOS == "windows" {
		return EnvSnapshotResultJSON{}, nil
	}
	if envSnapshotDisabled() {
		if force {
			return EnvSnapshotResultJSON{}, ValidationError{Message: "environment snapshot disabled (WORKSET_ENV_SNAPSHOT=0)"}
		}
		return EnvSnapshotResultJSON{}, nil
	}
	loginEnvMu.Lock()
	defer loginEnvMu.Unlock()
	if loginEnvLoaded && !force {
		return EnvSnapshotResultJSON{}, nil
	}
	shell, err := resolveLoginShellPath()
	if err != nil {
		return EnvSnapshotResultJSON{}, err
	}
	shellBase := strings.ToLower(filepath.Base(shell))
	command := "env"
	args := shellArgsForMode(shellBase, command, agentShellModeLogin)
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return EnvSnapshotResultJSON{}, err
	}
	snapshot := parseEnvSnapshot(string(output))
	changed := applyEnvSnapshot(snapshot)
	loginEnvLoaded = true
	return EnvSnapshotResultJSON{
		Updated:     len(changed) > 0,
		AppliedKeys: changed,
	}, nil
}

func envSnapshotDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("WORKSET_ENV_SNAPSHOT"))) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func resolveLoginShellPath() (string, error) {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = lookupUserShell()
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	shell = normalizeCLIPath(shell)
	if hasPathSeparator(shell) {
		if isExecutableCandidate(shell) {
			return shell, nil
		}
		return "", errors.New("login shell is not executable")
	}
	if resolved, err := exec.LookPath(shell); err == nil {
		return resolved, nil
	}
	if resolved := resolveCLIPath(shell); resolved != "" {
		return resolved, nil
	}
	return "", errors.New("login shell not found")
}

func lookupUserShell() string {
	current, err := user.Current()
	if err != nil || current.Username == "" {
		return ""
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] == current.Username {
			return strings.TrimSpace(parts[6])
		}
	}
	return ""
}
