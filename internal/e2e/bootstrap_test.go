package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func buildWorksetBinary(tmp string) (string, error) {
	worksetPath := filepath.Join(tmp, "workset")
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "build", "-o", worksetPath, "./cmd/workset")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOMODCACHE="+filepath.Join(tmp, "gomodcache"),
		"GOCACHE="+filepath.Join(tmp, "gocache"),
		"GOSUMDB=off",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build ./cmd/workset: %w (%s)", err, string(output))
	}

	return worksetPath, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
