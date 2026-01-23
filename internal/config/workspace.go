package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadWorkspace(path string) (WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceConfig{}, err
	}
	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return WorkspaceConfig{}, err
	}
	return cfg, nil
}

func SaveWorkspace(path string, cfg WorkspaceConfig) error {
	cfg = stripLegacyWorkspaceRemotes(cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func stripLegacyWorkspaceRemotes(cfg WorkspaceConfig) WorkspaceConfig {
	for i := range cfg.Repos {
		cfg.Repos[i].LegacyRemotes = nil
	}
	return cfg
}

func WorkspaceExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}
