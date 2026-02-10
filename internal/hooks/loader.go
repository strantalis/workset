package hooks

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const RepoHooksPath = ".workset/hooks.yaml"

func RepoHooksFile(repoPath string) string {
	return filepath.Join(repoPath, RepoHooksPath)
}

func ParseRepoHooks(data []byte) (File, error) {
	var cfg File
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return File{}, err
	}
	return cfg, nil
}

func LoadRepoHooks(repoPath string) (File, bool, error) {
	path := RepoHooksFile(repoPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, false, nil
		}
		return File{}, false, err
	}
	cfg, err := ParseRepoHooks(data)
	if err != nil {
		return File{}, true, err
	}
	return cfg, true, nil
}
