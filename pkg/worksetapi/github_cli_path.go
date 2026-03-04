package worksetapi

import (
	"os"
	"path/filepath"
	"strings"
)

func ensureGitHubCLIPath() string {
	if ghPath := normalizeCLIPath(os.Getenv("GH_PATH")); ghPath != "" {
		if isGitHubCLIBinaryPath(ghPath) && isExecutableFile(ghPath) {
			return ghPath
		}
		_ = os.Unsetenv("GH_PATH")
	}
	if path := resolveCLIPath("gh"); path != "" {
		_ = os.Setenv("GH_PATH", path)
		return path
	}
	return ""
}

func isGitHubCLIBinaryPath(path string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	return name == "gh" || name == "gh.exe"
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
