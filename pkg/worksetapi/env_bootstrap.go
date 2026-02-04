package worksetapi

import (
	"bytes"
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

// EnsureLoginEnv loads the login-shell environment once per process.
func EnsureLoginEnv(ctx context.Context) (EnvSnapshotResultJSON, error) {
	return loadLoginEnv(ctx, false)
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
	if ctx == nil {
		return EnvSnapshotResultJSON{}, errors.New("context required")
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
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return EnvSnapshotResultJSON{}, err
	}
	snapshot := parseEnvSnapshot(string(output))
	if runtime.GOOS == "darwin" {
		snapshot = applyDefaultPathSnapshot(snapshot)
	}
	changed := applyEnvSnapshot(snapshot)
	loginEnvLoaded = true
	return EnvSnapshotResultJSON{
		Updated:     len(changed) > 0,
		AppliedKeys: changed,
	}, nil
}

func applyDefaultPathSnapshot(snapshot map[string]string) map[string]string {
	current := snapshot["PATH"]
	base := current
	if isMinimalPath(current) {
		if helperPath := pathFromHelper(); helperPath != "" {
			base = helperPath
		}
	}
	next := appendPathEntries(base, defaultPathCandidates())
	if next == "" || next == current {
		return snapshot
	}
	if snapshot == nil {
		snapshot = map[string]string{}
	}
	snapshot["PATH"] = next
	return snapshot
}

func defaultPathCandidates() []string {
	candidates := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/opt/local/bin",
		"/run/current-system/sw/bin",
		"/nix/var/nix/profiles/default/bin",
		"/snap/bin",
		"/var/lib/snapd/snap/bin",
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".nix-profile", "bin"))
		candidates = append(candidates, filepath.Join(home, ".local", "bin"))
		candidates = append(candidates, filepath.Join(home, ".asdf", "shims"))
		xdgState := os.Getenv("XDG_STATE_HOME")
		if xdgState == "" {
			xdgState = filepath.Join(home, ".local", "state")
		}
		candidates = append(candidates, filepath.Join(xdgState, "nix", "profiles", "profile", "bin"))
	}
	return candidates
}

func isMinimalPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return true
	}
	parts := strings.Split(path, string(os.PathListSeparator))
	minimal := map[string]bool{
		"/usr/bin":  true,
		"/bin":      true,
		"/usr/sbin": true,
		"/sbin":     true,
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !minimal[part] {
			return false
		}
	}
	return true
}

func pathFromHelper() string {
	cmd := exec.Command("/usr/libexec/path_helper", "-s")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "PATH=") {
			continue
		}
		value := strings.TrimPrefix(line, "PATH=")
		value = strings.SplitN(value, ";", 2)[0]
		value = strings.TrimSpace(strings.Trim(value, "\"'"))
		if value != "" {
			return value
		}
	}
	return ""
}

func appendPathEntries(path string, candidates []string) string {
	path = strings.TrimSpace(path)
	entries := map[string]bool{}
	order := []string{}
	if path != "" {
		for _, part := range strings.Split(path, string(os.PathListSeparator)) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			part = filepath.Clean(part)
			if !entries[part] {
				entries[part] = true
				order = append(order, part)
			}
		}
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if entries[candidate] {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			entries[candidate] = true
			order = append(order, candidate)
		}
	}
	return strings.Join(order, string(os.PathListSeparator))
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
