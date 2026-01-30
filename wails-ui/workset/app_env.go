package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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

func ensureDefaultPath() {
	if runtime.GOOS != "darwin" {
		return
	}
	current := os.Getenv("PATH")
	base := current
	if isMinimalPath(current) {
		if helperPath := pathFromHelper(); helperPath != "" {
			base = helperPath
		}
	}
	next := appendPathEntries(base, defaultPathCandidates())
	if next != "" && next != current {
		_ = os.Setenv("PATH", next)
	}
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
		value = strings.TrimSpace(strings.Trim(value, `"'`))
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
