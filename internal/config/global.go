package config

import (
	"errors"
	"os"
	"path/filepath"

	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
)

func GlobalConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "workset", "config.yaml"), nil
}

func LoadGlobal(path string) (GlobalConfig, error) {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return GlobalConfig{}, err
		}
	}

	defaults := DefaultConfig()

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(map[string]interface{}{
		"defaults.base_branch":               defaults.Defaults.BaseBranch,
		"defaults.workspace":                 defaults.Defaults.Workspace,
		"defaults.workspace_root":            defaults.Defaults.WorkspaceRoot,
		"defaults.repo_store_root":           defaults.Defaults.RepoStoreRoot,
		"defaults.session_backend":           defaults.Defaults.SessionBackend,
		"defaults.session_name_format":       defaults.Defaults.SessionNameFormat,
		"defaults.session_theme":             defaults.Defaults.SessionTheme,
		"defaults.session_tmux_status_style": defaults.Defaults.SessionTmuxStyle,
		"defaults.session_tmux_status_left":  defaults.Defaults.SessionTmuxLeft,
		"defaults.session_tmux_status_right": defaults.Defaults.SessionTmuxRight,
		"defaults.session_screen_hardstatus": defaults.Defaults.SessionScreenHard,
		"defaults.parallelism":               defaults.Defaults.Parallelism,
	}, "."), nil); err != nil {
		return GlobalConfig{}, err
	}

	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, err
	}

	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, err
	}
	cfg.EnsureMaps()
	if cfg.Defaults.BaseBranch == "" {
		cfg.Defaults = defaults.Defaults
	}
	if cfg.Defaults.Workspace == "" {
		cfg.Defaults.Workspace = defaults.Defaults.Workspace
	}
	if cfg.Defaults.WorkspaceRoot == "" {
		cfg.Defaults.WorkspaceRoot = defaults.Defaults.WorkspaceRoot
	}
	if cfg.Defaults.RepoStoreRoot == "" {
		cfg.Defaults.RepoStoreRoot = defaults.Defaults.RepoStoreRoot
	}
	if cfg.Defaults.SessionBackend == "" {
		cfg.Defaults.SessionBackend = defaults.Defaults.SessionBackend
	}
	if cfg.Defaults.SessionNameFormat == "" {
		cfg.Defaults.SessionNameFormat = defaults.Defaults.SessionNameFormat
	}
	if cfg.Defaults.SessionTheme == "" {
		cfg.Defaults.SessionTheme = defaults.Defaults.SessionTheme
	}
	if cfg.Defaults.SessionTmuxStyle == "" {
		cfg.Defaults.SessionTmuxStyle = defaults.Defaults.SessionTmuxStyle
	}
	if cfg.Defaults.SessionTmuxLeft == "" {
		cfg.Defaults.SessionTmuxLeft = defaults.Defaults.SessionTmuxLeft
	}
	if cfg.Defaults.SessionTmuxRight == "" {
		cfg.Defaults.SessionTmuxRight = defaults.Defaults.SessionTmuxRight
	}
	if cfg.Defaults.SessionScreenHard == "" {
		cfg.Defaults.SessionScreenHard = defaults.Defaults.SessionScreenHard
	}
	if cfg.Defaults.Parallelism == 0 {
		cfg.Defaults.Parallelism = defaults.Defaults.Parallelism
	}
	return cfg, nil
}

func SaveGlobal(path string, cfg GlobalConfig) error {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return err
		}
	}
	cfg.EnsureMaps()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
