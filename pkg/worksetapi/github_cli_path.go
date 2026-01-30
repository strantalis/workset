package worksetapi

import (
	"os"
	"path/filepath"
)

func ensureGitHubCLIPath() string {
	if ghPath := os.Getenv("GH_PATH"); ghPath != "" {
		return ghPath
	}
	if path := resolveCLIPath("gh"); path != "" {
		_ = os.Setenv("GH_PATH", path)
		return path
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
