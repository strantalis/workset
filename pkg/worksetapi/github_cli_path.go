package worksetapi

import (
	"os"
	"os/exec"
	"path/filepath"
)

var githubCLICandidates = []string{
	"/opt/homebrew/bin/gh",
	"/usr/local/bin/gh",
	"/opt/local/bin/gh",
	"/run/current-system/sw/bin/gh",
	"/nix/var/nix/profiles/default/bin/gh",
	"/snap/bin/gh",
	"/var/lib/snapd/snap/bin/gh",
}

func ensureGitHubCLIPath() string {
	if ghPath := os.Getenv("GH_PATH"); ghPath != "" {
		return ghPath
	}
	if path, err := exec.LookPath("gh"); err == nil {
		_ = os.Setenv("GH_PATH", path)
		return path
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidate := filepath.Join(home, ".nix-profile", "bin", "gh")
		if isExecutableFile(candidate) {
			_ = os.Setenv("GH_PATH", candidate)
			return candidate
		}
		xdgState := os.Getenv("XDG_STATE_HOME")
		if xdgState == "" {
			xdgState = filepath.Join(home, ".local", "state")
		}
		candidate = filepath.Join(xdgState, "nix", "profiles", "profile", "bin", "gh")
		if isExecutableFile(candidate) {
			_ = os.Setenv("GH_PATH", candidate)
			return candidate
		}
		candidate = filepath.Join(home, ".asdf", "shims", "gh")
		if isExecutableFile(candidate) {
			_ = os.Setenv("GH_PATH", candidate)
			return candidate
		}
	}
	for _, candidate := range githubCLICandidates {
		if isExecutableFile(candidate) {
			_ = os.Setenv("GH_PATH", candidate)
			return candidate
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Mode()&0o111 == 0 {
		return false
	}
	return filepath.Clean(path) == path
}
